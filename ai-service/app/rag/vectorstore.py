"""Qdrant operations for approved post text chunks."""

import uuid
from collections.abc import Sequence
from typing import Any

from qdrant_client import AsyncQdrantClient
from qdrant_client.models import (
    Distance,
    FieldCondition,
    Filter,
    MatchValue,
    PayloadSchemaType,
    PointStruct,
    VectorParams,
)

COLLECTION_NAME = "post_chunks"
VECTOR_SIZE = 512
POINT_NAMESPACE = uuid.UUID("e59602e9-cc92-58c3-88e1-2078dd174a3d")


def point_id(chunk_id: str) -> str:
    return str(uuid.uuid5(POINT_NAMESPACE, chunk_id))


class TextVectorStore:
    def __init__(self, url: str, collection_name: str = COLLECTION_NAME) -> None:
        self.client = AsyncQdrantClient(url=url, check_compatibility=False)
        self.collection_name = collection_name

    async def ensure_collection(self) -> None:
        collections = await self.client.get_collections()
        exists = any(item.name == self.collection_name for item in collections.collections)
        if not exists:
            await self.client.create_collection(
                collection_name=self.collection_name,
                vectors_config=VectorParams(size=VECTOR_SIZE, distance=Distance.COSINE),
            )
        await self.validate_schema()
        await self.ensure_payload_indexes()

    async def validate_schema(self) -> None:
        info = await self.client.get_collection(self.collection_name)
        vectors = info.config.params.vectors
        if isinstance(vectors, dict):
            raise RuntimeError("post_chunks collection must use one unnamed vector")
        if vectors.size != VECTOR_SIZE or vectors.distance != Distance.COSINE:
            raise RuntimeError(
                "incompatible post_chunks collection: "
                f"expected size={VECTOR_SIZE} distance={Distance.COSINE.value}, "
                f"got size={vectors.size} distance={vectors.distance.value}"
            )

    async def ensure_payload_indexes(self) -> None:
        info = await self.client.get_collection(self.collection_name)
        schema = info.payload_schema or {}
        indexes = (
            ("post_id", PayloadSchemaType.INTEGER),
            ("chunk_id", PayloadSchemaType.KEYWORD),
        )
        for field, field_type in indexes:
            if field not in schema:
                await self.client.create_payload_index(
                    collection_name=self.collection_name,
                    field_name=field,
                    field_schema=field_type,
                    wait=True,
                )

    async def replace_post(self, post_id: int, points: Sequence[dict[str, Any]]) -> None:
        await self.delete_post(post_id)
        if not points:
            return
        structs = [
            PointStruct(
                id=point_id(str(item["chunk_id"])),
                vector=item["vector"],
                payload=item["payload"],
            )
            for item in points
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
        for item in response.points:
            payload = dict(item.payload or {})
            results.append(
                {
                    "post_id": int(payload["post_id"]),
                    "chunk_id": str(payload["chunk_id"]),
                    "chunk_text": str(payload["chunk_text"]),
                    "score": float(item.score),
                }
            )
        return results

    async def metadata(self) -> dict[str, Any]:
        await self.ensure_collection()
        info = await self.client.get_collection(self.collection_name)
        schema = info.payload_schema or {}
        return {
            "collection": self.collection_name,
            "dimension": VECTOR_SIZE,
            "distance": Distance.COSINE.value,
            "points_count": int(info.points_count or 0),
            "payload_indexes": sorted(schema),
        }

    async def inventory(self) -> list[dict[str, Any]]:
        await self.ensure_collection()
        items: list[dict[str, Any]] = []
        offset = None
        while True:
            points, offset = await self.client.scroll(
                collection_name=self.collection_name,
                limit=100,
                offset=offset,
                with_payload=True,
                with_vectors=False,
            )
            for point in points:
                payload = dict(point.payload or {})
                items.append(
                    {
                        "post_id": int(payload.get("post_id", 0)),
                        "chunk_id": str(payload.get("chunk_id", "")),
                        "model_name": str(payload.get("model_name", "")),
                    }
                )
            if offset is None:
                break
        return items

    async def close(self) -> None:
        await self.client.close()
