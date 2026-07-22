"""Minimal OpenAI-compatible provider used only by the AI E2E stack."""

import asyncio
import json
import os
from typing import Any

from fastapi import FastAPI

app = FastAPI(title="ShareO mock LLM")


@app.get("/healthz")
async def healthz() -> dict[str, str]:
    return {"status": "ok"}


def _sources(messages: list[dict[str, Any]]) -> list[dict[str, Any]]:
    user_message = next(
        (item.get("content", "") for item in reversed(messages) if item.get("role") == "user"),
        "",
    )
    marker = "REFERENCE_DATA_JSON="
    end_marker = "\nCONVERSATION_HISTORY_JSON="
    if marker not in user_message or end_marker not in user_message:
        return []
    raw = user_message.split(marker, 1)[1].split(end_marker, 1)[0]
    try:
        value = json.loads(raw)
    except json.JSONDecodeError:
        return []
    return value if isinstance(value, list) else []


@app.post("/v1/chat/completions")
async def complete(payload: dict[str, Any]) -> dict[str, Any]:
    messages = payload.get("messages") or []
    question = "".join(
        str(item.get("content", "")) for item in messages if item.get("role") == "user"
    )
    if "测试超时" in question:
        await asyncio.sleep(float(os.getenv("MOCK_LLM_DELAY_SECONDS", "2")))
    sources = _sources(messages)
    citations = [str(sources[0].get("chunk_id"))] if sources else []
    answer = "模拟回答：请参考已审核帖子。" if sources else "不确定，未找到足够的相关内容。"
    content = json.dumps({"answer": answer, "source_chunk_ids": citations}, ensure_ascii=False)
    return {
        "id": "mock-completion",
        "object": "chat.completion",
        "model": str(payload.get("model") or "shareo-mock"),
        "choices": [
            {
                "index": 0,
                "message": {"role": "assistant", "content": content},
                "finish_reason": "stop",
            }
        ],
        "usage": {"prompt_tokens": 10, "completion_tokens": 10},
    }
