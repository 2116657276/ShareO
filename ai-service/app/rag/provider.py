"""Small OpenAI-compatible chat-completions provider."""

import time
from dataclasses import dataclass
from typing import Any

import httpx


class LLMError(RuntimeError):
    category = "provider"


class LLMConfigurationError(LLMError):
    category = "configuration"


class LLMTimeoutError(LLMError):
    category = "timeout"


class LLMResponseError(LLMError):
    category = "response"


@dataclass(frozen=True)
class LLMResult:
    content: str
    model: str
    prompt_tokens: int
    completion_tokens: int
    duration_ms: float


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

    async def complete(self, messages: list[dict[str, str]]) -> LLMResult:
        if not self.configured:
            raise LLMConfigurationError("LLM provider is not configured")
        started = time.perf_counter()
        try:
            response = await self.client.post(
                f"{self.base_url}/chat/completions",
                headers={"Authorization": f"Bearer {self.api_key}"},
                json={
                    "model": self.model,
                    "messages": messages,
                    "temperature": 0,
                    "response_format": {"type": "json_object"},
                },
                timeout=self.timeout_seconds,
            )
        except httpx.TimeoutException as exc:
            raise LLMTimeoutError("LLM request timed out") from exc
        except httpx.HTTPError as exc:
            raise LLMError("LLM request failed") from exc
        if response.status_code >= 400:
            raise LLMError(f"LLM returned HTTP {response.status_code}")
        try:
            payload: dict[str, Any] = response.json()
            content = str(payload["choices"][0]["message"]["content"])
            usage = payload.get("usage") or {}
        except (KeyError, IndexError, TypeError, ValueError) as exc:
            raise LLMResponseError("LLM response shape is invalid") from exc
        return LLMResult(
            content=content,
            model=str(payload.get("model") or self.model),
            prompt_tokens=int(usage.get("prompt_tokens") or 0),
            completion_tokens=int(usage.get("completion_tokens") or 0),
            duration_ms=(time.perf_counter() - started) * 1000,
        )

    async def close(self) -> None:
        if self._owns_client:
            await self.client.aclose()
