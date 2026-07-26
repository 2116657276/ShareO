"""Private-message Bot task execution and idempotent Go callbacks."""

import logging
from typing import Any

import httpx

from app.config import settings
from app.rag.pipeline import NO_ANSWER, RAGPipeline
from app.rag.provider import LLMConfigurationError, LLMResponseError

logger = logging.getLogger(__name__)
FALLBACK_MESSAGE = "AI 当前暂不可用，请稍后重试。"
MAX_INT64 = 2**63 - 1
MAX_CALLBACK_CITATIONS = 5


def is_permanent_http_error(status_code: int) -> bool:
    return 400 <= status_code < 500


def positive_int(fields: dict[str, Any], name: str) -> int:
    raw = fields.get(name)
    if isinstance(raw, bool):
        raise ValueError(f"{name} must be a positive integer")
    try:
        value = int(raw)
    except (TypeError, ValueError) as exc:
        raise ValueError(f"{name} must be a positive integer") from exc
    if value <= 0 or value > MAX_INT64 or str(value) != str(raw):
        raise ValueError(f"{name} must be a canonical positive integer")
    return value


class BotTaskHandler:
    def __init__(
        self,
        http_client: httpx.AsyncClient,
        pipeline: RAGPipeline | None,
        go_base_url: str | None = None,
        internal_token: str | None = None,
    ) -> None:
        self.http = http_client
        self.pipeline = pipeline
        self.go_base_url = (go_base_url or settings.go_base_url).rstrip("/")
        self.internal_token = (
            internal_token if internal_token is not None else settings.internal_token
        )

    @property
    def headers(self) -> dict[str, str]:
        if not self.internal_token:
            raise RuntimeError("SHAREO_AI_INTERNAL_TOKEN is not configured")
        return {"X-Internal-Token": self.internal_token}

    async def _fetch_task(self, message_id: int) -> dict[str, Any] | None:
        response = await self.http.get(
            f"{self.go_base_url}/internal/bot/tasks/{message_id}",
            headers=self.headers,
        )
        if is_permanent_http_error(response.status_code):
            logger.warning(
                "dropping invalid bot task message_id=%d status=%d",
                message_id,
                response.status_code,
            )
            return None
        response.raise_for_status()
        body = response.json()
        if not isinstance(body, dict):
            raise ValueError("bot task response must be a JSON object")
        task = body.get("data", body)
        if not isinstance(task, dict):
            raise ValueError("bot task data must be a JSON object")
        return task

    async def _reply(
        self,
        message_id: int,
        conversation_id: int,
        content: str,
        citations: list[dict[str, Any]],
    ) -> bool:
        response = await self.http.post(
            f"{self.go_base_url}/internal/bot/reply",
            headers=self.headers,
            json={
                "source_message_id": message_id,
                "conversation_id": conversation_id,
                "content": content,
                "citations": citations,
            },
        )
        if is_permanent_http_error(response.status_code):
            logger.warning(
                "dropping permanent bot callback failure message_id=%d status=%d",
                message_id,
                response.status_code,
            )
            return False
        response.raise_for_status()
        return True

    async def fallback(self, _stream_id: str, fields: dict[str, Any]) -> None:
        try:
            message_id = positive_int(fields, "message_id")
            conversation_id = positive_int(fields, "conversation_id")
        except ValueError as exc:
            logger.error("dropping malformed bot task at dead-letter: %s fields=%r", exc, fields)
            return
        await self._reply(message_id, conversation_id, FALLBACK_MESSAGE, [])

    async def __call__(self, _stream_id: str, fields: dict[str, Any]) -> None:
        message_id = positive_int(fields, "message_id")
        conversation_id = positive_int(fields, "conversation_id")
        task = await self._fetch_task(message_id)
        if task is None:
            return
        source = task.get("message") or {}
        conversation = task.get("conversation") or {}
        if (
            int(source.get("id", 0)) != message_id
            or int(source.get("conversation_id", 0)) != conversation_id
            or int(conversation.get("id", 0)) != conversation_id
        ):
            raise ValueError("bot task identity does not match stream fields")
        if self.pipeline is None or not self.pipeline.provider.configured:
            await self._reply(message_id, conversation_id, FALLBACK_MESSAGE, [])
            return

        history = []
        for item in task.get("history", []):
            item_id = int(item.get("id", 0))
            if item_id <= 0 or item_id >= message_id:
                continue
            sender = item.get("sender") or {}
            history.append(
                {
                    "role": "bot" if int(sender.get("is_bot", 0)) != 0 else "user",
                    "content": str(item.get("content", ""))[:2000],
                }
            )
        try:
            result = await self.pipeline.answer(str(source.get("content", "")), history=history)
        except (LLMConfigurationError, LLMResponseError):
            await self._reply(message_id, conversation_id, FALLBACK_MESSAGE, [])
            return
        await self._reply(
            message_id,
            conversation_id,
            _safe_answer(result.answer),
            [
                {"post_id": item.post_id, "chunk_id": item.chunk_id}
                for item in result.citations[:MAX_CALLBACK_CITATIONS]
            ],
        )


def _safe_answer(answer: Any) -> str:
    """Keep the Go callback payload valid when a provider returns an empty answer."""
    if not isinstance(answer, str) or not answer.strip():
        return NO_ANSWER
    return answer.strip()
