"""Read-only tools exposed to the ShareO knowledge Agent."""

import asyncio
import json
import time
from dataclasses import dataclass, field
from typing import Any

import httpx

from app.config import settings
from app.core.embedding import ImageEmbedder
from app.core.vectorstore import ImageVectorStore
from app.rag.embedding import TextEmbedder
from app.rag.vectorstore import TextVectorStore

TOOL_NAMES = frozenset(
    {
        "semantic_search_posts",
        "keyword_search_posts",
        "read_posts",
        "search_images",
    }
)

TOOL_DEFINITIONS: list[dict[str, Any]] = [
    {
        "type": "function",
        "function": {
            "name": "semantic_search_posts",
            "description": "Search approved community post chunks by meaning when the user gives a paraphrase or conceptual description.",
            "parameters": {
                "type": "object",
                "properties": {
                    "query": {"type": "string", "minLength": 1, "maxLength": 200},
                    "limit": {"type": "integer", "minimum": 1, "maximum": 10},
                },
                "required": ["query", "limit"],
                "additionalProperties": False,
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "keyword_search_posts",
            "description": "Search approved community posts by exact words or named topics; use this first for comparison questions that need separate sources.",
            "parameters": {
                "type": "object",
                "properties": {
                    "query": {"type": "string", "minLength": 1, "maxLength": 200},
                    "limit": {"type": "integer", "minimum": 1, "maximum": 10},
                },
                "required": ["query", "limit"],
                "additionalProperties": False,
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "read_posts",
            "description": "Read approved community posts by IDs returned by a search; use it to verify details before citing a source.",
            "parameters": {
                "type": "object",
                "properties": {
                    "post_ids": {
                        "type": "array",
                        "items": {"type": "integer", "minimum": 1},
                        "minItems": 1,
                        "maxItems": 10,
                    }
                },
                "required": ["post_ids"],
                "additionalProperties": False,
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "search_images",
            "description": "Search approved community images by a Chinese visual description; only use when the user explicitly asks to find a photo, image, or visual match.",
            "parameters": {
                "type": "object",
                "properties": {
                    "query": {"type": "string", "minLength": 1, "maxLength": 200},
                    "limit": {"type": "integer", "minimum": 1, "maximum": 8},
                },
                "required": ["query", "limit"],
                "additionalProperties": False,
            },
        },
    },
]


class AgentToolError(RuntimeError):
    """A user-visible, controlled tool failure."""


@dataclass(frozen=True)
class ToolExecution:
    name: str
    status: str
    output: dict[str, Any]
    citations: list[dict[str, Any]]
    result_count: int
    timings: dict[str, float] = field(default_factory=dict)


class AgentToolRuntime:
    def __init__(
        self,
        http: httpx.AsyncClient,
        text_embedder: TextEmbedder,
        text_vector_store: TextVectorStore,
        image_embedder: ImageEmbedder,
        image_vector_store: ImageVectorStore,
        go_base_url: str | None = None,
        internal_token: str | None = None,
    ) -> None:
        self.http = http
        self.text_embedder = text_embedder
        self.text_vector_store = text_vector_store
        self.image_embedder = image_embedder
        self.image_vector_store = image_vector_store
        self.go_base_url = (go_base_url or settings.go_base_url).rstrip("/")
        self.internal_token = (
            internal_token if internal_token is not None else settings.internal_token
        )

    @property
    def headers(self) -> dict[str, str]:
        if not self.internal_token:
            raise AgentToolError("internal token is not configured")
        return {"X-Internal-Token": self.internal_token}

    @staticmethod
    def _query(args: dict[str, Any]) -> tuple[str, int]:
        if set(args) != {"query", "limit"}:
            raise AgentToolError("tool arguments contain unknown fields")
        query = args.get("query")
        limit = args.get("limit")
        if not isinstance(query, str) or not query.strip() or len(query.strip()) > 200:
            raise AgentToolError("query must contain 1-200 characters")
        if isinstance(limit, bool) or not isinstance(limit, int) or not 1 <= limit <= 10:
            raise AgentToolError("limit must be between 1 and 10")
        return query.strip(), limit

    @staticmethod
    def _post_ids(args: dict[str, Any]) -> list[int]:
        if set(args) != {"post_ids"}:
            raise AgentToolError("tool arguments contain unknown fields")
        values = args.get("post_ids")
        if not isinstance(values, list) or not 1 <= len(values) <= 10:
            raise AgentToolError("post_ids must contain 1-10 IDs")
        if any(
            isinstance(value, bool) or not isinstance(value, int) or value <= 0 for value in values
        ):
            raise AgentToolError("post_ids must contain positive integers")
        if len(set(values)) != len(values):
            raise AgentToolError("post_ids must not contain duplicates")
        return values

    async def _go(self, method: str, path: str, **kwargs: Any) -> list[dict[str, Any]]:
        try:
            response = await self.http.request(
                method,
                f"{self.go_base_url}{path}",
                headers=self.headers,
                timeout=min(settings.agent_timeout_seconds, 10.0),
                **kwargs,
            )
            response.raise_for_status()
            body = response.json()
            data = body.get("data", body) if isinstance(body, dict) else {}
            items = data.get("items", []) if isinstance(data, dict) else []
            if not isinstance(items, list):
                raise AgentToolError("internal tool response is invalid")
            return [item for item in items if isinstance(item, dict)]
        except AgentToolError:
            raise
        except (httpx.HTTPError, ValueError, TypeError) as exc:
            raise AgentToolError("Go internal read failed") from exc

    @staticmethod
    def _post_citation(item: dict[str, Any]) -> dict[str, Any]:
        post_id = int(item["post_id"])
        return {
            "post_id": post_id,
            "chunk_id": str(item.get("chunk_id") or f"{post_id}:0"),
            "excerpt": str(item.get("content") or item.get("chunk_text") or "")[:240],
            "score": float(item.get("score") or 0.0),
        }

    async def _semantic_search(self, args: dict[str, Any]) -> ToolExecution:
        query, limit = self._query(args)
        embedding_started = time.perf_counter()
        vector = await asyncio.to_thread(self.text_embedder.embed_query, query)
        embedding_ms = (time.perf_counter() - embedding_started) * 1000
        retrieval_started = time.perf_counter()
        candidates = await self.text_vector_store.search(
            vector,
            min(
                limit * settings.agent_search_candidate_multiplier,
                settings.agent_search_max_candidates,
            ),
        )
        retrieval_ms = (time.perf_counter() - retrieval_started) * 1000
        post_ids: list[int] = []
        for candidate in candidates:
            post_id = int(candidate["post_id"])
            if post_id not in post_ids:
                post_ids.append(post_id)
        visibility_started = time.perf_counter()
        visible = await self._go(
            "POST", "/internal/posts/agent/read", json={"post_ids": post_ids[:10]}
        )
        visibility_ms = (time.perf_counter() - visibility_started) * 1000
        content_by_post = {int(item["post_id"]): str(item.get("content") or "") for item in visible}
        items: list[dict[str, Any]] = []
        citations: list[dict[str, Any]] = []
        seen_posts: set[int] = set()
        for candidate in candidates:
            post_id = int(candidate["post_id"])
            if post_id in seen_posts or post_id not in content_by_post or len(items) >= limit:
                continue
            seen_posts.add(post_id)
            item = {
                "post_id": post_id,
                "chunk_id": str(candidate["chunk_id"]),
                "content": content_by_post[post_id][:1200],
                "score": float(candidate["score"]),
            }
            items.append(item)
            citations.append(self._post_citation(item))
        return ToolExecution(
            "semantic_search_posts",
            "success" if items else "no_result",
            {"items": items},
            citations,
            len(items),
            {
                "embedding_ms": round(embedding_ms, 1),
                "retrieval_ms": round(retrieval_ms, 1),
                "visibility_ms": round(visibility_ms, 1),
            },
        )

    async def _keyword_search(self, args: dict[str, Any]) -> ToolExecution:
        query, limit = self._query(args)
        keyword_started = time.perf_counter()
        payloads = await self._go(
            "GET", "/internal/posts/agent/search", params={"q": query, "limit": limit}
        )
        keyword_ms = (time.perf_counter() - keyword_started) * 1000
        items = [
            {
                "post_id": int(item["post_id"]),
                "chunk_id": f"{int(item['post_id'])}:0",
                "content": str(item.get("content") or "")[:1200],
            }
            for item in payloads[:limit]
        ]
        citations = [self._post_citation(item) for item in items]
        return ToolExecution(
            "keyword_search_posts",
            "success" if items else "no_result",
            {"items": items},
            citations,
            len(items),
            {"keyword_ms": round(keyword_ms, 1)},
        )

    async def _read_posts(self, args: dict[str, Any]) -> ToolExecution:
        post_ids = self._post_ids(args)
        visibility_started = time.perf_counter()
        payloads = await self._go("POST", "/internal/posts/agent/read", json={"post_ids": post_ids})
        visibility_ms = (time.perf_counter() - visibility_started) * 1000
        items = [
            {
                "post_id": int(item["post_id"]),
                "chunk_id": f"{int(item['post_id'])}:0",
                "content": str(item.get("content") or "")[:1200],
                "image_count": len(item.get("images") or []),
            }
            for item in payloads
        ]
        citations = [self._post_citation(item) for item in items]
        return ToolExecution(
            "read_posts",
            "success" if items else "no_result",
            {"items": items},
            citations,
            len(items),
            {"visibility_ms": round(visibility_ms, 1)},
        )

    async def _search_images(self, args: dict[str, Any]) -> ToolExecution:
        query, limit = self._query(args)
        limit = min(limit, 8)
        embedding_started = time.perf_counter()
        vector = await asyncio.to_thread(self.image_embedder.encode_text, query)
        embedding_ms = (time.perf_counter() - embedding_started) * 1000
        retrieval_started = time.perf_counter()
        candidates = await self.image_vector_store.search(
            vector,
            min(
                limit * settings.agent_search_candidate_multiplier,
                settings.agent_image_search_max_candidates,
            ),
        )
        retrieval_ms = (time.perf_counter() - retrieval_started) * 1000
        post_ids: list[int] = []
        for candidate in candidates:
            post_id = int(candidate["post_id"])
            if post_id not in post_ids:
                post_ids.append(post_id)
        visibility_started = time.perf_counter()
        visible = await self._go(
            "POST", "/internal/posts/agent/read", json={"post_ids": post_ids[:10]}
        )
        visibility_ms = (time.perf_counter() - visibility_started) * 1000
        content_by_post = {int(item["post_id"]): str(item.get("content") or "") for item in visible}
        items: list[dict[str, Any]] = []
        seen_posts: set[int] = set()
        for candidate in candidates:
            post_id = int(candidate["post_id"])
            if post_id in seen_posts or post_id not in content_by_post or len(items) >= limit:
                continue
            seen_posts.add(post_id)
            items.append(
                {
                    "post_id": post_id,
                    "chunk_id": f"{post_id}:0",
                    "image_id": int(candidate["image_id"]),
                    "score": float(candidate["score"]),
                    "content": content_by_post[post_id][:600],
                }
            )
        citations = [self._post_citation(item) for item in items]
        return ToolExecution(
            "search_images",
            "success" if items else "no_result",
            {"items": items},
            citations,
            len(items),
            {
                "embedding_ms": round(embedding_ms, 1),
                "retrieval_ms": round(retrieval_ms, 1),
                "visibility_ms": round(visibility_ms, 1),
            },
        )

    async def execute(self, name: str, arguments: str | dict[str, Any]) -> ToolExecution:
        if name not in TOOL_NAMES:
            raise AgentToolError("tool is not allowlisted")
        try:
            args = json.loads(arguments) if isinstance(arguments, str) else arguments
        except json.JSONDecodeError as exc:
            raise AgentToolError("tool arguments are not valid JSON") from exc
        if not isinstance(args, dict):
            raise AgentToolError("tool arguments must be an object")
        if name == "semantic_search_posts":
            return await self._semantic_search(args)
        if name == "keyword_search_posts":
            return await self._keyword_search(args)
        if name == "read_posts":
            return await self._read_posts(args)
        return await self._search_images(args)
