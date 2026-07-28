import json
import asyncio

import pytest

from app.agent.graph import AgentRunner, parse_agent_output
from app.agent.tools import AgentToolError, AgentToolRuntime, TOOL_DEFINITIONS, ToolExecution
from app.rag.provider import (
    LLMRequestError,
    LLMResult,
    LLMServerError,
    LLMStructuredOutputError,
    LLMToolCall,
)


class FakeProvider:
    def __init__(self) -> None:
        self.calls = 0
        self.messages = []
        self.configured = True

    async def complete(self, messages, tools=None, tool_choice=None):
        self.calls += 1
        self.messages.append(messages)
        if any(item.get("role") == "tool" for item in messages):
            content = json.dumps(
                {"answer": "已根据社区资料整理。", "source_chunk_ids": ["12:0"]},
                ensure_ascii=False,
            )
            return LLMResult(content, "mock", 10, 5, 1)
        return LLMResult(
            "",
            "mock",
            10,
            5,
            1,
            tool_calls=(
                LLMToolCall("call-1", "semantic_search_posts", '{"query":"夜景","limit":2}'),
            ),
            finish_reason="tool_calls",
        )


class FakeTools:
    async def execute(self, name, arguments):
        assert name == "semantic_search_posts"
        assert json.loads(arguments)["limit"] == 2
        item = {"post_id": 12, "chunk_id": "12:0", "content": "夜景资料"}
        return ToolExecution(name, "success", {"items": [item]}, [CitationPayload(item)], 1)


class CitationPayload(dict):
    def __init__(self, item):
        super().__init__(
            post_id=item["post_id"], chunk_id=item["chunk_id"], excerpt=item["content"], score=0.9
        )


@pytest.mark.asyncio
async def test_agent_graph_executes_tool_and_returns_allowlisted_citation():
    provider = FakeProvider()
    result = await AgentRunner(provider, FakeTools()).answer("怎么拍夜景？")
    assert result.answer == "已根据社区资料整理。"
    assert result.citations[0].post_id == 12
    assert result.trace["steps"][0]["tool"] == "semantic_search_posts"
    tool_messages = [item for item in provider.messages[1] if item.get("role") == "tool"]
    assert tool_messages and set(tool_messages[0]) == {"role", "tool_call_id", "content"}


def test_agent_tool_definitions_do_not_request_beta_strict_mode():
    assert all("strict" not in definition["function"] for definition in TOOL_DEFINITIONS)
    assert all(
        definition["function"]["parameters"].get("additionalProperties") is False
        for definition in TOOL_DEFINITIONS
    )


def test_agent_output_filters_unobserved_sources():
    answer, source_ids = parse_agent_output(
        '{"answer":"答案","source_chunk_ids":["12:0","99:0","12:0"]}', {"12:0"}
    )
    assert answer == "答案"
    assert source_ids == ["12:0"]


def test_agent_tool_argument_guards():
    with pytest.raises(AgentToolError):
        AgentToolRuntime._query({"query": " ", "limit": 1})
    with pytest.raises(AgentToolError):
        AgentToolRuntime._query({"query": "问题", "limit": 11})
    with pytest.raises(AgentToolError):
        AgentToolRuntime._query({"query": "问题", "limit": 1, "extra": True})
    with pytest.raises(AgentToolError):
        AgentToolRuntime._post_ids({"post_ids": [1, 1]})


@pytest.mark.asyncio
async def test_image_search_observation_exposes_citation_chunk_id():
    class Embedder:
        def encode_text(self, query):
            assert query == "夜景"
            return [0.1]

    class VectorStore:
        async def search(self, vector, limit):
            assert limit == 6
            return [{"image_id": 101, "post_id": 14, "score": 0.9}]

    class Response:
        def raise_for_status(self):
            return None

        def json(self):
            return {"data": {"items": [{"post_id": 14, "content": "夜景灯光"}]}}

    class HTTP:
        async def request(self, method, url, **kwargs):
            assert method == "POST"
            assert url.endswith("/internal/posts/agent/read")
            return Response()

    runtime = AgentToolRuntime(
        HTTP(),
        text_embedder=None,
        text_vector_store=None,
        image_embedder=Embedder(),
        image_vector_store=VectorStore(),
        go_base_url="http://go",
        internal_token="token",
    )
    result = await runtime.execute("search_images", {"query": "夜景", "limit": 2})
    assert result.output["items"][0]["chunk_id"] == "14:0"
    assert result.citations[0]["chunk_id"] == "14:0"


@pytest.mark.asyncio
async def test_agent_timeout_returns_controlled_no_answer(monkeypatch):
    class SlowProvider(FakeProvider):
        async def complete(self, messages, tools=None, tool_choice=None):
            await asyncio.sleep(0.05)
            return await super().complete(messages, tools, tool_choice)

    monkeypatch.setattr("app.agent.graph.settings.agent_timeout_seconds", 0.001)
    result = await AgentRunner(SlowProvider(), FakeTools()).answer("夜景")
    assert result.answer == "不确定，未找到足够的相关内容。"
    assert result.trace["status"] == "failed"
    assert result.trace["stop_reason"] == "timeout"


@pytest.mark.asyncio
async def test_agent_retries_one_structured_output_failure_within_trace_budget():
    class StructuredRetryProvider(FakeProvider):
        def __init__(self):
            super().__init__()
            self.failures = 1

        async def complete(self, messages, tools=None, tool_choice=None):
            if any(item.get("role") == "tool" for item in messages) and self.failures:
                self.failures -= 1
                raise LLMStructuredOutputError("invalid final JSON")
            return await super().complete(messages, tools, tool_choice)

    result = await AgentRunner(StructuredRetryProvider(), FakeTools()).answer("夜景")
    assert result.trace["status"] == "completed"
    assert result.trace["provider_retries"] == 1
    assert result.citations[0].post_id == 12


@pytest.mark.asyncio
async def test_agent_provider_failure_returns_auditable_controlled_trace():
    class FailingProvider:
        async def complete(self, messages, tools=None, tool_choice=None):
            raise LLMServerError("provider offline")

    result = await AgentRunner(FailingProvider(), FakeTools()).answer("夜景")
    assert result.answer == "不确定，未找到足够的相关内容。"
    assert result.citations == []
    assert result.trace["status"] == "failed"
    assert result.trace["failure_category"] == "server"


class MultiCallProvider:
    def __init__(self, calls: tuple[LLMToolCall, ...], fail_after_tools: bool = False):
        self.calls = calls
        self.fail_after_tools = fail_after_tools
        self.requests: list[list[dict]] = []

    async def complete(self, messages, tools=None, tool_choice=None):
        self.requests.append(messages)
        tool_messages = [item for item in messages if item.get("role") == "tool"]
        if not tool_messages:
            return LLMResult("", "mock", 1, 1, 1, tool_calls=self.calls)
        if self.fail_after_tools:
            raise LLMRequestError("invalid tool message chain", provider_attempts=1)
        return LLMResult(
            json.dumps(
                {"answer": "已完成多工具检索。", "source_chunk_ids": []},
                ensure_ascii=False,
            ),
            "mock",
            1,
            1,
            1,
        )


class RecordingTools:
    def __init__(self):
        self.calls: list[str] = []

    async def execute(self, name, arguments):
        self.calls.append(name)
        if name == "unknown_tool":
            raise AgentToolError("unknown tool")
        payload = json.loads(arguments)
        if payload.get("invalid"):
            raise AgentToolError("invalid arguments")
        if payload.get("explode"):
            raise RuntimeError("tool exploded")
        return ToolExecution(name, "success", {"items": []}, [], 0)


@pytest.mark.asyncio
async def test_agent_closes_all_three_tool_calls_with_parallel_limit_two(monkeypatch):
    monkeypatch.setattr("app.agent.graph.settings.agent_max_parallel_tools", 2)
    calls = tuple(
        LLMToolCall(f"call-{index}", "semantic_search_posts", '{"query":"夜景","limit":2}')
        for index in range(3)
    )
    provider = MultiCallProvider(calls)
    tools = RecordingTools()

    result = await AgentRunner(provider, tools).answer("比较三个来源")

    assert tools.calls == ["semantic_search_posts"] * 3
    replies = [item for item in provider.requests[1] if item.get("role") == "tool"]
    assert {item["tool_call_id"] for item in replies} == {call.id for call in calls}
    assert len(result.trace["steps"]) == 3
    assert result.trace["rejected_tool_calls"] == 0


@pytest.mark.asyncio
async def test_agent_closes_calls_over_remaining_budget_without_executing_them(monkeypatch):
    monkeypatch.setattr("app.agent.graph.settings.agent_max_tool_calls", 2)
    calls = tuple(
        LLMToolCall(f"call-{index}", "semantic_search_posts", '{"query":"夜景","limit":2}')
        for index in range(4)
    )
    provider = MultiCallProvider(calls)
    tools = RecordingTools()

    result = await AgentRunner(provider, tools).answer("比较四个来源")

    assert len(tools.calls) == 2
    replies = [item for item in provider.requests[1] if item.get("role") == "tool"]
    assert {item["tool_call_id"] for item in replies} == {call.id for call in calls}
    rejected = [item for item in replies if "tool budget exhausted" in item["content"]]
    assert len(rejected) == 2
    assert len(result.trace["steps"]) == 2
    assert result.trace["rejected_tool_calls"] == 2


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("name", "arguments"),
    [
        ("unknown_tool", "{}"),
        ("semantic_search_posts", '{"invalid":true}'),
        ("semantic_search_posts", '{"explode":true}'),
    ],
)
async def test_agent_tool_failures_still_close_the_corresponding_call(name, arguments):
    provider = MultiCallProvider((LLMToolCall("failed-call", name, arguments),))
    result = await AgentRunner(provider, RecordingTools()).answer("失败路径")

    replies = [item for item in provider.requests[1] if item.get("role") == "tool"]
    assert len(replies) == 1
    assert replies[0]["tool_call_id"] == "failed-call"
    assert "tool unavailable" in replies[0]["content"]
    assert result.trace["steps"][0]["status"] == "error"


@pytest.mark.asyncio
async def test_provider_failure_after_tools_preserves_steps_and_attempt_counts():
    provider = MultiCallProvider(
        (LLMToolCall("call-1", "semantic_search_posts", '{"query":"夜景","limit":2}'),),
        fail_after_tools=True,
    )

    result = await AgentRunner(provider, RecordingTools()).answer("夜景")

    assert result.trace["status"] == "failed"
    assert result.trace["failure_category"] == "request"
    assert len(result.trace["steps"]) == 1
    assert result.trace["provider_attempts"] == 2


@pytest.mark.asyncio
async def test_structured_output_retry_reuses_observations_without_reexecuting_tools():
    class RetryProvider(FakeProvider):
        def __init__(self):
            super().__init__()
            self.final_requests = 0

        async def complete(self, messages, tools=None, tool_choice=None):
            if any(item.get("role") == "tool" for item in messages):
                self.final_requests += 1
                if self.final_requests == 1:
                    return LLMResult("not-json", "mock", 1, 1, 1)
            return await super().complete(messages, tools, tool_choice)

    provider = RetryProvider()
    tools = RecordingTools()
    result = await AgentRunner(provider, tools).answer("夜景")

    assert result.trace["status"] == "completed"
    assert len(tools.calls) == 1
    assert provider.final_requests == 2
    assert result.trace["provider_retries"] == 1
