"""Idempotent post image indexing worker."""

from io import BytesIO
from typing import Any

import httpx
from PIL import Image

from app.config import settings
from app.core.embedding import ImageEmbedder
from app.core.vectorstore import ImageVectorStore, image_payload


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
        action = str(fields.get("action", "upsert"))
        post_id = int(fields["post_id"])
        if action == "delete":
            await self.vector_store.delete_post(post_id)
            return
        if action != "upsert":
            raise ValueError(f"unsupported index action: {action}")
        if not self.internal_token:
            raise RuntimeError("SHAREO_AI_INTERNAL_TOKEN is not configured")

        headers = {"X-Internal-Token": self.internal_token}
        response = await self.http.get(
            f"{self.go_base_url}/internal/posts/{post_id}/index-payload",
            headers=headers,
        )
        if response.status_code == 404:
            await self.vector_store.delete_post(post_id)
            return
        response.raise_for_status()
        body = response.json()
        payload = body.get("data", body)
        images = payload.get("images", [])
        if not images:
            await self.vector_store.delete_post(post_id)
            return

        loaded: list[Image.Image] = []
        for image in images:
            image_response = await self.http.get(
                f"{self.go_base_url}{image['image_url']}",
                timeout=30.0,
            )
            image_response.raise_for_status()
            with Image.open(BytesIO(image_response.content)) as source:
                loaded.append(source.convert("RGB"))
        try:
            vectors = []
            batch_size = max(1, settings.embedding_batch_size)
            for start in range(0, len(loaded), batch_size):
                vectors.extend(self.embedder.encode_images(loaded[start : start + batch_size]))
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
                ),
            }
            for image, vector in zip(images, vectors, strict=True)
        ]
        await self.vector_store.delete_post(post_id)
        await self.vector_store.upsert(points)
