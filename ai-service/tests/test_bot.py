"""Bot worker validation, callback classification, and fallback behavior."""

import json
from collections.abc import Callable

import httpx
import pytest

from app.config import settings
from app.rag.pipeline import NO_ANSWER, Citation, RAGAnswer
from app.agent.graph import AgentAnswer
from app.workers.bot import (
    FALLBACK_MESSAGE,
    MAX_CALLBACK_CITATIONS,
    BotTaskHandler,
    is_permanent_http_error,
    positive_int,
    _safe_answer,
)


class FakeProvider:
    def __init__(self, configured: bool = True) -> None:
        self.configured = configured


class FakePipeline:
    def __init__(self, configured: bool = True) -> None:
        self.provider = FakeProvider(configured)
        self.calls: list[tuple[str, list[dict[str, str]] | None]] = []

    async def answer(self, question: str, history=None):
        self.calls.append((question, history))
        return RAGAnswer(
            "可以使用三脚架。",
            [Citation(post_id=12, chunk_id="12:0", excerpt="三脚架", score=0.9)],
        )


class EmptyAnswerPipeline(FakePipeline):
    async def answer(self, question: str, history=None):
        self.calls.append((question, history))
        return RAGAnswer("", [])


class ManyCitationPipeline(FakePipeline):
    async def answer(self, question: str, history=None):
        return RAGAnswer(
            "参考回答。",
            [
                Citation(post_id=index, chunk_id=f"{index}:0", excerpt="资料", score=0.9)
                for index in range(1, MAX_CALLBACK_CITATIONS + 2)
            ],
        )


def task_payload() -> dict:
    return {
        "message": {"id": 9, "conversation_id": 7, "content": "怎么拍夜景？"},
        "conversation": {"id": 7},
        "history": [
            {"id": 8, "content": "我只有手机", "sender": {"is_bot": 0}},
            {"id": 9, "content": "怎么拍夜景？", "sender": {"is_bot": 0}},
        ],
    }


def agent_task_payload() -> dict:
    payload = task_payload()
    payload["message"]["meta"] = json.dumps({"ai_mode": "agent"})
    return payload


class FakeAgentRunner:
    async def answer(self, question, history=None):
        assert question == "怎么拍夜景？"
        return AgentAnswer(
            "已根据多个社区来源整理。",
            [Citation(post_id=12, chunk_id="12:0", excerpt="夜景", score=0.9)],
            {
                "version": "agent-trace-v1",
                "status": "completed",
                "stop_reason": "model_answer",
                "total_duration_ms": 12,
                "steps": [
                    {
                        "index": 0,
                        "tool": "semantic_search_posts",
                        "status": "success",
                        "result_count": 1,
                        "duration_ms": 4,
                    }
                ],
            },
        )


def transport_for(
    callback: Callable[[httpx.Request], httpx.Response],
) -> httpx.AsyncClient:
    return httpx.AsyncClient(transport=httpx.MockTransport(callback))


def assert_callback(request: httpx.Request, content: str) -> None:
    assert request.url.path == "/internal/bot/reply"
    assert request.headers["X-Internal-Token"] == "test-token"
    payload = json.loads(request.content)
    assert payload["source_message_id"] == 9
    assert payload["conversation_id"] == 7
    assert payload["content"] == content


def test_positive_int_rejects_noncanonical_values():
    assert positive_int({"id": "12"}, "id") == 12
    for raw in ("", "0", "01", "+1", "1.0", True, 2**63):
        with pytest.raises(ValueError):
            positive_int({"id": raw}, "id")


@pytest.mark.parametrize("answer", ["", "   ", None, 42])
def test_safe_answer_uses_rag_no_answer_for_empty_provider_result(answer):
    assert _safe_answer(answer) == NO_ANSWER


def test_safe_answer_strips_nonempty_provider_result():
    assert _safe_answer("  可以使用三脚架。 ") == "可以使用三脚架。"


@pytest.mark.parametrize("status_code", [400, 401, 403, 404, 422, 499])
def test_all_client_errors_are_permanent_callbacks(status_code: int):
    assert is_permanent_http_error(status_code)


@pytest.mark.parametrize("status_code", [200, 399, 500, 503])
def test_server_errors_and_successes_are_not_permanent_callbacks(status_code: int):
    assert not is_permanent_http_error(status_code)


@pytest.mark.asyncio
async def test_successful_task_calls_rag_and_posts_whitelisted_citation():
    requests: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        requests.append(request)
        if request.method == "GET":
            return httpx.Response(200, json={"data": task_payload()})
        assert_callback(request, "可以使用三脚架。")
        payload = json.loads(request.content)
        assert payload["citations"] == [{"post_id": 12, "chunk_id": "12:0"}]
        return httpx.Response(200, json={"code": 0, "data": {"id": 10}})

    pipeline = FakePipeline()
    client = transport_for(handler)
    worker = BotTaskHandler(client, pipeline, "http://go.test", "test-token")
    await worker("1-0", {"message_id": "9", "conversation_id": "7"})
    await client.aclose()

    assert [request.method for request in requests] == ["GET", "POST"]
    assert pipeline.calls == [("怎么拍夜景？", [{"role": "user", "content": "我只有手机"}])]


@pytest.mark.asyncio
async def test_eval_isolation_header_suppresses_conversation_history(monkeypatch):
    monkeypatch.setattr(settings, "agent_eval_isolated_history", True)

    def handler(request: httpx.Request) -> httpx.Response:
        if request.method == "GET":
            assert request.headers["X-ShareO-Agent-Eval-Isolated"] == "1"
            payload = task_payload()
            payload["history"] = []
            return httpx.Response(200, json={"data": payload})
        assert_callback(request, "可以使用三脚架。")
        return httpx.Response(200, json={"code": 0})

    pipeline = FakePipeline()
    client = transport_for(handler)
    worker = BotTaskHandler(client, pipeline, "http://go.test", "test-token")
    await worker("1-0", {"message_id": "9", "conversation_id": "7"})
    await client.aclose()

    assert pipeline.calls == [("怎么拍夜景？", [])]


@pytest.mark.asyncio
async def test_missing_provider_writes_fixed_fallback_without_rag_call():
    posted: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        if request.method == "GET":
            return httpx.Response(200, json={"data": task_payload()})
        posted.append(request)
        assert_callback(request, FALLBACK_MESSAGE)
        return httpx.Response(200, json={"code": 0})

    pipeline = FakePipeline(configured=False)
    client = transport_for(handler)
    worker = BotTaskHandler(client, pipeline, "http://go.test", "test-token")
    await worker("1-0", {"message_id": "9", "conversation_id": "7"})
    await client.aclose()

    assert len(posted) == 1
    assert pipeline.calls == []


@pytest.mark.asyncio
async def test_empty_rag_answer_writes_nonempty_no_answer_callback():
    def handler(request: httpx.Request) -> httpx.Response:
        if request.method == "GET":
            return httpx.Response(200, json={"data": task_payload()})
        assert_callback(request, NO_ANSWER)
        return httpx.Response(200, json={"code": 0})

    pipeline = EmptyAnswerPipeline()
    client = transport_for(handler)
    worker = BotTaskHandler(client, pipeline, "http://go.test", "test-token")
    await worker("1-0", {"message_id": "9", "conversation_id": "7"})
    await client.aclose()


@pytest.mark.asyncio
async def test_many_rag_citations_are_capped_to_go_callback_limit():
    def handler(request: httpx.Request) -> httpx.Response:
        if request.method == "GET":
            return httpx.Response(200, json={"data": task_payload()})
        payload = json.loads(request.content)
        assert len(payload["citations"]) == MAX_CALLBACK_CITATIONS
        return httpx.Response(200, json={"code": 0})

    client = transport_for(handler)
    worker = BotTaskHandler(client, ManyCitationPipeline(), "http://go.test", "test-token")
    await worker("1-0", {"message_id": "9", "conversation_id": "7"})
    await client.aclose()


@pytest.mark.asyncio
async def test_agent_task_posts_trace_and_mode():
    def handler(request: httpx.Request) -> httpx.Response:
        if request.method == "GET":
            return httpx.Response(200, json={"data": agent_task_payload()})
        payload = json.loads(request.content)
        assert payload["ai_mode"] == "agent"
        assert payload["agent_trace"]["steps"][0]["tool"] == "semantic_search_posts"
        assert payload["citations"] == [{"post_id": 12, "chunk_id": "12:0"}]
        return httpx.Response(200, json={"code": 0})

    client = transport_for(handler)
    worker = BotTaskHandler(
        client, FakePipeline(), "http://go.test", "test-token", FakeAgentRunner()
    )
    await worker("1-0", {"message_id": "9", "conversation_id": "7"})
    await client.aclose()


@pytest.mark.asyncio
async def test_agent_missing_provider_keeps_failed_trace():
    def handler(request: httpx.Request) -> httpx.Response:
        if request.method == "GET":
            return httpx.Response(200, json={"data": agent_task_payload()})
        payload = json.loads(request.content)
        assert payload["ai_mode"] == "agent"
        assert payload["agent_trace"]["status"] == "failed"
        assert payload["agent_trace"]["stop_reason"] == "provider_unavailable"
        return httpx.Response(200, json={"code": 0})

    client = transport_for(handler)
    worker = BotTaskHandler(client, FakePipeline(configured=False), "http://go.test", "test-token")
    await worker("1-0", {"message_id": "9", "conversation_id": "7"})
    await client.aclose()


@pytest.mark.asyncio
async def test_permanent_callback_error_is_dropped_without_retry():
    def handler(request: httpx.Request) -> httpx.Response:
        if request.method == "GET":
            return httpx.Response(200, json={"data": task_payload()})
        return httpx.Response(404, json={"message": "source disappeared"})

    client = transport_for(handler)
    worker = BotTaskHandler(client, FakePipeline(), "http://go.test", "test-token")
    await worker("1-0", {"message_id": "9", "conversation_id": "7"})
    await client.aclose()


@pytest.mark.asyncio
async def test_transient_callback_error_is_retried_by_consumer():
    def handler(request: httpx.Request) -> httpx.Response:
        if request.method == "GET":
            return httpx.Response(200, json={"data": task_payload()})
        return httpx.Response(503, json={"message": "temporary"})

    client = transport_for(handler)
    worker = BotTaskHandler(client, FakePipeline(), "http://go.test", "test-token")
    with pytest.raises(httpx.HTTPStatusError):
        await worker("1-0", {"message_id": "9", "conversation_id": "7"})
    await client.aclose()
