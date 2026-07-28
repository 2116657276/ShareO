"""Small OpenAI-compatible chat-completions provider."""

import asyncio
import time
from dataclasses import dataclass, replace
from typing import Any

import httpx


class LLMError(RuntimeError):
    category = "provider"
    retryable = False

    def __init__(self, message: str, *, provider_attempts: int = 1) -> None:
        super().__init__(message)
        self.provider_attempts = provider_attempts
        self.provider_retries = max(0, provider_attempts - 1)


class LLMConfigurationError(LLMError):
    category = "configuration"


class LLMTimeoutError(LLMError):
    category = "timeout"
    retryable = True


class LLMTransportError(LLMError):
    category = "transport"
    retryable = True


class LLMRateLimitError(LLMError):
    category = "rate_limit"
    retryable = True


class LLMServerError(LLMError):
    category = "server"
    retryable = True


class LLMRequestError(LLMError):
    """The provider rejected the request; retrying would send the same invalid shape."""

    category = "request"


class LLMResponseError(LLMError):
    category = "response"
    retryable = True


class LLMStructuredOutputError(LLMResponseError):
    category = "structured_output"


@dataclass(frozen=True)
class LLMToolCall:
    id: str
    name: str
    arguments: str


@dataclass(frozen=True)
class LLMResult:
    content: str
    model: str
    prompt_tokens: int
    completion_tokens: int
    duration_ms: float
    tool_calls: tuple[LLMToolCall, ...] = ()
    finish_reason: str = "stop"
    provider_attempts: int = 1
    provider_retries: int = 0


class OpenAICompatibleProvider:
    def __init__(
        self,
        base_url: str,
        api_key: str,
        model: str,
        timeout_seconds: float,
        client: httpx.AsyncClient | None = None,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.model = model
        self.timeout_seconds = timeout_seconds
        self.client = client or httpx.AsyncClient()
        self._owns_client = client is None

    @property
    def configured(self) -> bool:
        return bool(self.base_url and self.api_key and self.model)

    async def complete(
        self,
        messages: list[dict[str, Any]],
        tools: list[dict[str, Any]] | None = None,
        tool_choice: str | dict[str, Any] | None = None,
    ) -> LLMResult:
        if not self.configured:
            raise LLMConfigurationError("LLM provider is not configured")
        request_payload: dict[str, Any] = {
            "model": self.model,
            "messages": messages,
            "temperature": 0,
        }
        if tools:
            request_payload["tools"] = tools
            request_payload["tool_choice"] = tool_choice or "auto"
        else:
            request_payload["response_format"] = {"type": "json_object"}
        attempts = 0
        while True:
            attempts += 1
            try:
                result = await self._complete_once(request_payload)
                return replace(
                    result,
                    provider_attempts=attempts,
                    provider_retries=attempts - 1,
                )
            except LLMError as exc:
                if not exc.retryable or attempts >= 2:
                    exc.provider_attempts = attempts
                    exc.provider_retries = attempts - 1
                    raise
                await asyncio.sleep(0.2)

    async def _complete_once(self, request_payload: dict[str, Any]) -> LLMResult:
        started = time.perf_counter()
        try:
            response = await self.client.post(
                f"{self.base_url}/chat/completions",
                headers={"Authorization": f"Bearer {self.api_key}"},
                json=request_payload,
                timeout=self.timeout_seconds,
            )
        except httpx.TimeoutException as exc:
            raise LLMTimeoutError("LLM request timed out") from exc
        except httpx.HTTPError as exc:
            raise LLMTransportError("LLM request failed") from exc
        if response.status_code == 429:
            raise LLMRateLimitError("LLM rate limit response")
        if response.status_code >= 500:
            raise LLMServerError("LLM server error response")
        if response.status_code >= 400:
            raise LLMRequestError(f"LLM request rejected with HTTP {response.status_code}")
        try:
            payload: dict[str, Any] = response.json()
            choice = payload["choices"][0]
            message = choice["message"]
            raw_content = message.get("content")
            content = str(raw_content) if raw_content is not None else ""
            raw_tool_calls = message.get("tool_calls") or []
            usage = payload.get("usage") or {}
            tool_calls = tuple(
                LLMToolCall(
                    id=str(item["id"]),
                    name=str(item["function"]["name"]),
                    arguments=str(item["function"].get("arguments") or "{}"),
                )
                for item in raw_tool_calls
                if isinstance(item, dict)
                and isinstance(item.get("function"), dict)
                and item.get("id")
                and item["function"].get("name")
            )
        except (KeyError, IndexError, TypeError, ValueError) as exc:
            raise LLMResponseError("LLM response shape is invalid") from exc
        return LLMResult(
            content=content,
            model=str(payload.get("model") or self.model),
            prompt_tokens=int(usage.get("prompt_tokens") or 0),
            completion_tokens=int(usage.get("completion_tokens") or 0),
            duration_ms=(time.perf_counter() - started) * 1000,
            tool_calls=tool_calls,
            finish_reason=str(payload["choices"][0].get("finish_reason") or "stop"),
        )

    async def close(self) -> None:
        if self._owns_client:
            await self.client.aclose()
