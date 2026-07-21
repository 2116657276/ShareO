"""Qdrant collection operations for image embeddings."""

from collections.abc import Sequence
from datetime import datetime
from typing import Any

from qdrant_client import AsyncQdrantClient
from qdrant_client.models import (
    Distance,
    FieldCondition,
    Filter,
    MatchValue,
    PointStruct,
    VectorParams,
)


COLLECTION_NAME = "images"
VECTOR_SIZE = 512


class ImageVectorStore:
    def __init__(self, url: str, collection_name: str = COLLECTION_NAME):
        self.client = AsyncQdrantClient(url=url, check_compatibility=False)
        self.collection_name = collection_name

    async def ensure_collection(self) -> None:
        collections = await self.client.get_collections()
        if any(item.name == self.collection_name for item in collections.collections):
            return
        await self.client.create_collection(
            collection_name=self.collection_name,
            vectors_config=VectorParams(size=VECTOR_SIZE, distance=Distance.COSINE),
        )

    async def upsert(
        self,
        points: Sequence[dict[str, Any]],
    ) -> None:
        if not points:
            return
        await self.ensure_collection()
        structs = [
            PointStruct(id=int(point["image_id"]), vector=point["vector"], payload=point["payload"])
            for point in points
        ]
        await self.client.upsert(self.collection_name, points=structs, wait=True)

    async def delete_post(self, post_id: int) -> None:
        await self.ensure_collection()
        await self.client.delete(
            self.collection_name,
            points_selector=Filter(
                must=[FieldCondition(key="post_id", match=MatchValue(value=int(post_id)))]
            ),
            wait=True,
        )

    async def search(self, vector: Sequence[float], limit: int) -> list[dict[str, Any]]:
        await self.ensure_collection()
        response = await self.client.query_points(
            self.collection_name,
            query=list(vector),
            limit=limit,
            with_payload=True,
        )
        results = []
        for point in response.points:
            payload = dict(point.payload or {})
            results.append(
                {
                    "image_id": int(point.id),
                    "post_id": int(payload.get("post_id", 0)),
                    "object_key": payload.get("object_key", ""),
                    "created_at": payload.get("created_at", ""),
                    "score": float(point.score),
                }
            )
        return results

    async def close(self) -> None:
        await self.client.close()


def image_payload(
    post_id: int, image_id: int, object_key: str, created_at: datetime | str
) -> dict[str, Any]:
    timestamp = created_at.isoformat() if isinstance(created_at, datetime) else str(created_at)
    return {
        "post_id": int(post_id),
        "image_id": int(image_id),
        "object_key": object_key,
        "created_at": timestamp,
    }
