"""Idempotent post image indexing worker."""

import asyncio
import logging
import time
from io import BytesIO
from typing import Any

import httpx
from PIL import Image

from app.config import settings
from app.core.embedding import ImageEmbedder
from app.core.vectorstore import ImageVectorStore, image_payload

logger = logging.getLogger(__name__)


class ImageIndexer:
    def __init__(
        self,
        http_client: httpx.AsyncClient,
        embedder: ImageEmbedder,
        vector_store: ImageVectorStore,
        go_base_url: str | None = None,
        internal_token: str | None = None,
    ):
        self.http = http_client
        self.embedder = embedder
        self.vector_store = vector_store
        self.go_base_url = (go_base_url or settings.go_base_url).rstrip("/")
        self.internal_token = (
            internal_token if internal_token is not None else settings.internal_token
        )

    async def handle(self, _msg_id: str, fields: dict[str, Any]) -> None:
        started = time.perf_counter()
        action = str(fields.get("action", "upsert"))
        post_id = int(fields["post_id"])
        if action == "delete":
            delete_started = time.perf_counter()
            await self.vector_store.delete_post(post_id)
            logger.info(
                "image qdrant delete complete post_id=%d duration_ms=%.1f total_ms=%.1f",
                post_id,
                (time.perf_counter() - delete_started) * 1000,
                (time.perf_counter() - started) * 1000,
            )
            return
        if action != "upsert":
            raise ValueError(f"unsupported index action: {action}")
        if not self.internal_token:
            raise RuntimeError("SHAREO_AI_INTERNAL_TOKEN is not configured")

        headers = {"X-Internal-Token": self.internal_token}
        fetch_started = time.perf_counter()
        response = await self.http.get(
            f"{self.go_base_url}/internal/posts/{post_id}/index-payload",
            headers=headers,
        )
        logger.info(
            "image index payload fetch complete post_id=%d status=%d duration_ms=%.1f",
            post_id,
            response.status_code,
            (time.perf_counter() - fetch_started) * 1000,
        )
        if response.status_code == 404:
            delete_started = time.perf_counter()
            await self.vector_store.delete_post(post_id)
            logger.info(
                "image qdrant delete complete post_id=%d reason=payload_not_visible duration_ms=%.1f",
                post_id,
                (time.perf_counter() - delete_started) * 1000,
            )
            return
        response.raise_for_status()
        body = response.json()
        payload = body.get("data", body)
        images = payload.get("images", [])
        if not images:
            delete_started = time.perf_counter()
            await self.vector_store.delete_post(post_id)
            logger.info(
                "image qdrant delete complete post_id=%d reason=no_images duration_ms=%.1f",
                post_id,
                (time.perf_counter() - delete_started) * 1000,
            )
            return

        download_started = time.perf_counter()
        loaded: list[Image.Image] = []
        for image in images:
            image_response = await self.http.get(
                f"{self.go_base_url}{image['image_url']}",
                timeout=30.0,
            )
            image_response.raise_for_status()
            with Image.open(BytesIO(image_response.content)) as source:
                loaded.append(source.convert("RGB"))
        logger.info(
            "image download complete post_id=%d count=%d duration_ms=%.1f",
            post_id,
            len(loaded),
            (time.perf_counter() - download_started) * 1000,
        )
        try:
            encode_started = time.perf_counter()
            vectors = []
            batch_size = max(1, settings.embedding_batch_size)
            for start in range(0, len(loaded), batch_size):
                vectors.extend(
                    await asyncio.to_thread(
                        self.embedder.encode_images, loaded[start : start + batch_size]
                    )
                )
            logger.info(
                "image embedding complete post_id=%d count=%d duration_ms=%.1f",
                post_id,
                len(vectors),
                (time.perf_counter() - encode_started) * 1000,
            )
        finally:
            for image in loaded:
                image.close()
        if len(vectors) != len(images):
            raise RuntimeError("embedding count does not match image count")
        points = [
            {
                "image_id": image["image_id"],
                "vector": vector,
                "payload": image_payload(
                    post_id,
                    image["image_id"],
                    image["object_key"],
                    image["created_at"],
                    settings.embedding_revision,
                ),
            }
            for image, vector in zip(images, vectors, strict=True)
        ]
        delete_started = time.perf_counter()
        await self.vector_store.delete_post(post_id)
        logger.info(
            "image qdrant delete complete post_id=%d reason=replace duration_ms=%.1f",
            post_id,
            (time.perf_counter() - delete_started) * 1000,
        )
        upsert_started = time.perf_counter()
        await self.vector_store.upsert(points)
        logger.info(
            "image qdrant upsert complete post_id=%d images=%d duration_ms=%.1f total_ms=%.1f",
            post_id,
            len(points),
            (time.perf_counter() - upsert_started) * 1000,
            (time.perf_counter() - started) * 1000,
        )
