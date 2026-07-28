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


def _tool_observations(messages: list[dict[str, Any]]) -> list[dict[str, Any]]:
    observations = []
    for message in messages:
        if message.get("role") != "tool":
            continue
        try:
            value = json.loads(str(message.get("content") or "{}"))
        except json.JSONDecodeError:
            continue
        if isinstance(value, dict):
            observations.append(value)
    return observations


def _tool_call(name: str, arguments: dict[str, Any], number: int) -> dict[str, Any]:
    return {
        "id": f"mock-tool-{number}",
        "type": "function",
        "function": {"name": name, "arguments": json.dumps(arguments, ensure_ascii=False)},
    }


def _current_question(messages: list[dict[str, Any]]) -> str:
    """Read the current RAG or Agent question, excluding conversation history."""
    for message in reversed(messages):
        if message.get("role") != "user":
            continue
        content = str(message.get("content") or "")
        try:
            payload = json.loads(content)
        except json.JSONDecodeError:
            payload = None
        if isinstance(payload, dict) and isinstance(payload.get("question"), str):
            return payload["question"]
        marker = "QUESTION="
        if marker in content:
            return content.split(marker, 1)[1].splitlines()[0].strip()
    return ""


@app.post("/v1/chat/completions")
async def complete(payload: dict[str, Any]) -> dict[str, Any]:
    messages = payload.get("messages") or []
    question = _current_question(messages)
    if "测试超时" in question:
        await asyncio.sleep(float(os.getenv("MOCK_LLM_DELAY_SECONDS", "2")))
    if payload.get("tools"):
        observations = _tool_observations(messages)
        tool_names = [
            str(call.get("function", {}).get("name"))
            for message in messages
            if message.get("role") == "assistant"
            for call in message.get("tool_calls", [])
            if isinstance(call, dict) and isinstance(call.get("function"), dict)
        ]
        if not observations:
            tool_name = (
                "search_images"
                if "图片" in question or "照片" in question
                else "semantic_search_posts"
            )
            arguments = {
                "query": question[:200],
                "limit": 3 if tool_name == "semantic_search_posts" else 4,
            }
            return {
                "id": "mock-agent-tool-call",
                "object": "chat.completion",
                "model": str(payload.get("model") or "shareo-mock"),
                "choices": [
                    {
                        "index": 0,
                        "message": {
                            "role": "assistant",
                            "content": None,
                            "tool_calls": [_tool_call(tool_name, arguments, 1)],
                        },
                        "finish_reason": "tool_calls",
                    }
                ],
                "usage": {"prompt_tokens": 20, "completion_tokens": 8},
            }
        if any(
            marker in question
            for marker in ("Python", "Go语言", "3A游戏", "显卡", "机器学习", "天气", "交通")
        ):
            content = json.dumps(
                {"answer": "不确定，未找到足够的相关内容。", "source_chunk_ids": []},
                ensure_ascii=False,
            )
            return {
                "id": "mock-agent-no-answer",
                "object": "chat.completion",
                "model": str(payload.get("model") or "shareo-mock"),
                "choices": [
                    {
                        "index": 0,
                        "message": {"role": "assistant", "content": content},
                        "finish_reason": "stop",
                    }
                ],
                "usage": {"prompt_tokens": 25, "completion_tokens": 8},
            }
        source_ids: list[str] = []
        post_ids: list[int] = []
        for observation in observations:
            for item in observation.get("items", []):
                if not isinstance(item, dict):
                    continue
                chunk_id = item.get("chunk_id")
                if isinstance(chunk_id, str) and chunk_id not in source_ids:
                    source_ids.append(chunk_id)
                post_id = item.get("post_id")
                if isinstance(post_id, int) and post_id not in post_ids:
                    post_ids.append(post_id)
        if "read_posts" in tool_names or "search_images" in tool_names:
            content = json.dumps(
                {
                    "answer": "模拟 Agent 回答：已读取并整理已审核社区资料。",
                    "source_chunk_ids": source_ids[:3],
                },
                ensure_ascii=False,
            )
            return {
                "id": "mock-agent-final",
                "object": "chat.completion",
                "model": str(payload.get("model") or "shareo-mock"),
                "choices": [
                    {
                        "index": 0,
                        "message": {"role": "assistant", "content": content},
                        "finish_reason": "stop",
                    }
                ],
                "usage": {"prompt_tokens": 30, "completion_tokens": 12},
            }
        if post_ids:
            return {
                "id": "mock-agent-read-call",
                "object": "chat.completion",
                "model": str(payload.get("model") or "shareo-mock"),
                "choices": [
                    {
                        "index": 0,
                        "message": {
                            "role": "assistant",
                            "content": None,
                            "tool_calls": [_tool_call("read_posts", {"post_ids": post_ids[:3]}, 2)],
                        },
                        "finish_reason": "tool_calls",
                    }
                ],
                "usage": {"prompt_tokens": 25, "completion_tokens": 8},
            }
        content = json.dumps(
            {"answer": "不确定，未找到足够的相关内容。", "source_chunk_ids": []}, ensure_ascii=False
        )
        return {
            "id": "mock-agent-no-answer",
            "object": "chat.completion",
            "model": str(payload.get("model") or "shareo-mock"),
            "choices": [
                {
                    "index": 0,
                    "message": {"role": "assistant", "content": content},
                    "finish_reason": "stop",
                }
            ],
            "usage": {"prompt_tokens": 25, "completion_tokens": 8},
        }
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
