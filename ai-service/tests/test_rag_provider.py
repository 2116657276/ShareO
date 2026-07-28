import json

import httpx
import pytest

from app.rag.provider import (
    LLMConfigurationError,
    LLMError,
    LLMRequestError,
    LLMResponseError,
    LLMServerError,
    LLMTimeoutError,
    OpenAICompatibleProvider,
)


@pytest.mark.asyncio
async def test_provider_parses_openai_compatible_response():
    async def handler(request):
        assert request.headers["Authorization"] == "Bearer secret"
        return httpx.Response(
            200,
            json={
                "model": "demo",
                "choices": [{"message": {"content": '{"answer":"答案","source_chunk_ids":[]}'}}],
                "usage": {"prompt_tokens": 10, "completion_tokens": 4},
            },
        )

    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    provider = OpenAICompatibleProvider("http://llm/v1", "secret", "demo", 1, client)
    result = await provider.complete([{"role": "user", "content": "问题"}])
    assert result.model == "demo"
    assert result.prompt_tokens == 10


@pytest.mark.asyncio
async def test_provider_classifies_missing_config_timeout_and_bad_shape():
    provider = OpenAICompatibleProvider("", "", "demo", 1, httpx.AsyncClient())
    with pytest.raises(LLMConfigurationError):
        await provider.complete([])
    await provider.client.aclose()


@pytest.mark.asyncio
async def test_provider_parses_function_tool_calls():
    async def handler(request):
        payload = json.loads(request.content)
        assert payload["tools"][0]["function"]["name"] == "read_posts"
        assert payload["tool_choice"] == "auto"
        return httpx.Response(
            200,
            json={
                "model": "demo-agent",
                "choices": [
                    {
                        "finish_reason": "tool_calls",
                        "message": {
                            "content": None,
                            "tool_calls": [
                                {
                                    "id": "call-1",
                                    "type": "function",
                                    "function": {
                                        "name": "read_posts",
                                        "arguments": '{"post_ids":[1]}',
                                    },
                                }
                            ],
                        },
                    }
                ],
                "usage": {"prompt_tokens": 3, "completion_tokens": 2},
            },
        )

    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    provider = OpenAICompatibleProvider("http://llm/v1", "secret", "demo", 1, client)
    result = await provider.complete(
        [{"role": "user", "content": "读取帖子"}],
        tools=[{"type": "function", "function": {"name": "read_posts"}}],
    )
    assert result.content == ""
    assert result.tool_calls[0].name == "read_posts"
    assert result.tool_calls[0].arguments == '{"post_ids":[1]}'
    await client.aclose()

    async def timeout(_request):
        raise httpx.ReadTimeout("slow")

    timeout_client = httpx.AsyncClient(transport=httpx.MockTransport(timeout))
    provider = OpenAICompatibleProvider("http://llm/v1", "secret", "demo", 1, timeout_client)
    with pytest.raises(LLMTimeoutError):
        await provider.complete([])
    await timeout_client.aclose()

    bad_client = httpx.AsyncClient(
        transport=httpx.MockTransport(lambda _: httpx.Response(200, json={}))
    )
    provider = OpenAICompatibleProvider("http://llm/v1", "secret", "demo", 1, bad_client)
    with pytest.raises(LLMResponseError):
        await provider.complete([])
    await bad_client.aclose()


@pytest.mark.asyncio
async def test_provider_retries_server_error_once_and_records_attempts():
    calls = 0

    async def handler(_request):
        nonlocal calls
        calls += 1
        if calls == 1:
            return httpx.Response(503)
        return httpx.Response(
            200,
            json={
                "model": "demo",
                "choices": [{"message": {"content": "{}"}}],
            },
        )

    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    provider = OpenAICompatibleProvider("http://llm/v1", "secret", "demo", 1, client)
    result = await provider.complete([])
    assert calls == 2
    assert result.provider_attempts == 2
    assert result.provider_retries == 1
    await client.aclose()


@pytest.mark.asyncio
async def test_provider_does_not_retry_auth_or_other_client_errors():
    calls = 0

    async def handler(_request):
        nonlocal calls
        calls += 1
        return httpx.Response(401)

    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    provider = OpenAICompatibleProvider("http://llm/v1", "secret", "demo", 1, client)
    with pytest.raises(LLMError) as caught:
        await provider.complete([])
    assert calls == 1
    assert caught.value.category == "request"
    assert caught.value.provider_retries == 0
    await client.aclose()


@pytest.mark.asyncio
async def test_provider_does_not_retry_bad_request_and_keeps_error_redacted():
    calls = 0

    async def handler(_request):
        nonlocal calls
        calls += 1
        return httpx.Response(400, text='{"error":{"message":"sensitive request details"}}')

    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    provider = OpenAICompatibleProvider("http://llm/v1", "secret", "demo", 1, client)
    with pytest.raises(LLMRequestError) as caught:
        await provider.complete([])
    assert calls == 1
    assert caught.value.category == "request"
    assert "sensitive request details" not in str(caught.value)
    await client.aclose()


@pytest.mark.asyncio
async def test_provider_retries_invalid_response_shape_once():
    calls = 0

    async def handler(_request):
        nonlocal calls
        calls += 1
        return httpx.Response(200, json={})

    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    provider = OpenAICompatibleProvider("http://llm/v1", "secret", "demo", 1, client)
    with pytest.raises(LLMResponseError) as caught:
        await provider.complete([])
    assert calls == 2
    assert caught.value.provider_attempts == 2
    assert isinstance(caught.value, LLMResponseError)
    await client.aclose()


@pytest.mark.asyncio
async def test_provider_error_classification_is_retryable_for_server_failures():
    async def handler(_request):
        return httpx.Response(500)

    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    provider = OpenAICompatibleProvider("http://llm/v1", "secret", "demo", 1, client)
    with pytest.raises(LLMServerError) as caught:
        await provider.complete([])
    assert caught.value.category == "server"
    assert caught.value.provider_attempts == 2
    await client.aclose()
