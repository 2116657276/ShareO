"""Bot worker validation, callback classification, and fallback behavior."""

import json
from collections.abc import Callable

import httpx
import pytest

from app.rag.pipeline import Citation, RAGAnswer
from app.workers.bot import (
    FALLBACK_MESSAGE,
    BotTaskHandler,
    is_permanent_http_error,
    positive_int,
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


def task_payload() -> dict:
    return {
        "message": {"id": 9, "conversation_id": 7, "content": "怎么拍夜景？"},
        "conversation": {"id": 7},
        "history": [
            {"id": 8, "content": "我只有手机", "sender": {"is_bot": 0}},
            {"id": 9, "content": "怎么拍夜景？", "sender": {"is_bot": 0}},
        ],
    }


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
