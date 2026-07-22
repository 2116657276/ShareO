"""Idempotent text chunk indexing for approved posts."""

import asyncio
import logging
import time
from typing import Any

from app.config import settings
from app.rag.chunking import chunk_text
from app.rag.embedding import TextEmbedder
from app.rag.vectorstore import TextVectorStore

logger = logging.getLogger(__name__)


class TextIndexer:
    def __init__(self, embedder: TextEmbedder, vector_store: TextVectorStore) -> None:
        self.embedder = embedder
        self.vector_store = vector_store

    async def delete_post(self, post_id: int) -> None:
        started = time.perf_counter()
        await self.vector_store.delete_post(post_id)
        logger.info(
            "text qdrant delete complete post_id=%d duration_ms=%.1f",
            post_id,
            (time.perf_counter() - started) * 1000,
        )

    async def replace_post(self, payload: dict[str, Any]) -> None:
        post_id = int(payload["post_id"])
        chunks = chunk_text(
            str(payload.get("content", "")),
            target_size=settings.chunk_size,
            overlap=settings.chunk_overlap,
        )
        if not chunks:
            await self.delete_post(post_id)
            return

        encode_started = time.perf_counter()
        vectors = await asyncio.to_thread(self.embedder.embed_documents, chunks)
        logger.info(
            "text embedding complete post_id=%d chunks=%d duration_ms=%.1f",
            post_id,
            len(chunks),
            (time.perf_counter() - encode_started) * 1000,
        )
        if len(vectors) != len(chunks):
            raise RuntimeError("text embedding count does not match chunk count")
        points = []
        for chunk_no, (chunk, vector) in enumerate(zip(chunks, vectors, strict=True)):
            chunk_id = f"{post_id}:{chunk_no}"
            points.append(
                {
                    "chunk_id": chunk_id,
                    "vector": vector,
                    "payload": {
                        "post_id": post_id,
                        "chunk_id": chunk_id,
                        "chunk_text": chunk,
                        "model_name": self.embedder.model_name,
                        "created_at": str(payload.get("created_at", "")),
                    },
                }
            )
        upsert_started = time.perf_counter()
        await self.vector_store.replace_post(post_id, points)
        logger.info(
            "text qdrant replace complete post_id=%d chunks=%d duration_ms=%.1f",
            post_id,
            len(points),
            (time.perf_counter() - upsert_started) * 1000,
        )
