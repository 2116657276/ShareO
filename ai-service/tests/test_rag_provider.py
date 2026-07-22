import httpx
import pytest

from app.rag.provider import (
    LLMConfigurationError,
    LLMResponseError,
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
