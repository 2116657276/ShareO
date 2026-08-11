"""PostgreSQL/pgvector operations for approved post text chunks."""

import uuid
from collections.abc import Sequence
from typing import Any

from pgvector import Vector

from app.core.pgvector import PostgresVectorDatabase

COLLECTION_NAME = "post_chunks"
VECTOR_SIZE = 512
POINT_NAMESPACE = uuid.UUID("e59602e9-cc92-58c3-88e1-2078dd174a3d")


def point_id(chunk_id: str) -> str:
    return str(uuid.uuid5(POINT_NAMESPACE, chunk_id))


class TextVectorStore:
    def __init__(
        self, database: PostgresVectorDatabase, collection_name: str = COLLECTION_NAME
    ) -> None:
        self.database = database
        self.collection_name = collection_name
        self._schema_validated = False

    async def ensure_collection(self) -> None:
        if self._schema_validated:
            return
        await self.validate_schema()
        self._schema_validated = True

    async def validate_schema(self) -> None:
        row = await self.database.fetch_one(
            """
            SELECT format_type(a.atttypid, a.atttypmod) AS type_name
            FROM pg_attribute AS a
            WHERE a.attrelid = 'ai.post_chunk_embeddings'::regclass
              AND a.attname = 'embedding'
              AND NOT a.attisdropped
            """
        )
        if row is None or str(row.get("type_name", "")) != f"vector({VECTOR_SIZE})":
            raise RuntimeError(
                "incompatible post_chunks vector column: "
                f"expected vector({VECTOR_SIZE}), got {row.get('type_name') if row else 'missing'}"
            )

    async def ensure_payload_indexes(self) -> None:
        await self.ensure_collection()

    async def replace_post(self, post_id: int, points: Sequence[dict[str, Any]]) -> None:
        await self.ensure_collection()
        async with self.database.transaction() as connection:
            async with connection.cursor() as cursor:
                await cursor.execute(
                    "DELETE FROM ai.post_chunk_embeddings WHERE post_id = %s", (int(post_id),)
                )
                for item in points:
                    payload = item["payload"]
                    chunk_id = str(payload["chunk_id"])
                    await cursor.execute(
                        """
                        INSERT INTO ai.post_chunk_embeddings
                            (id, post_id, chunk_id, chunk_text, model_name, created_at, embedding)
                        VALUES (%s, %s, %s, %s, %s, %s, %s)
                        ON CONFLICT (chunk_id) DO UPDATE SET
                            post_id = EXCLUDED.post_id,
                            chunk_text = EXCLUDED.chunk_text,
                            model_name = EXCLUDED.model_name,
                            created_at = EXCLUDED.created_at,
                            embedding = EXCLUDED.embedding
                        """,
                        (
                            point_id(chunk_id),
                            int(post_id),
                            chunk_id,
                            str(payload["chunk_text"]),
                            str(payload.get("model_name", "")),
                            payload.get("created_at"),
                            Vector(list(item["vector"])),
                        ),
                    )

    async def delete_post(self, post_id: int) -> None:
        await self.ensure_collection()
        await self.database.execute(
            "DELETE FROM ai.post_chunk_embeddings WHERE post_id = %s", (int(post_id),)
        )

    async def search(self, vector: Sequence[float], limit: int) -> list[dict[str, Any]]:
        await self.ensure_collection()
        query_vector = Vector(list(vector))
        rows = await self.database.fetch_all(
            """
            SELECT post_id, chunk_id, chunk_text,
                   1 - (embedding <=> %s) AS score
            FROM ai.post_chunk_embeddings
            ORDER BY embedding <=> %s
            LIMIT %s
            """,
            (query_vector, query_vector, int(limit)),
        )
        return [
            {
                "post_id": int(row["post_id"]),
                "chunk_id": str(row["chunk_id"]),
                "chunk_text": str(row["chunk_text"]),
                "score": float(row["score"]),
            }
            for row in rows
        ]

    async def metadata(self) -> dict[str, Any]:
        await self.ensure_collection()
        count = await self.database.fetch_one(
            "SELECT count(*)::bigint AS count FROM ai.post_chunk_embeddings"
        )
        return {
            "collection": self.collection_name,
            "dimension": VECTOR_SIZE,
            "distance": "Cosine",
            "points_count": int((count or {}).get("count", 0)),
            "payload_indexes": ["chunk_id", "post_id"],
        }

    async def inventory(self) -> list[dict[str, Any]]:
        await self.ensure_collection()
        rows = await self.database.fetch_all(
            "SELECT post_id, chunk_id, model_name FROM ai.post_chunk_embeddings ORDER BY post_id, chunk_id"
        )
        return [
            {
                "post_id": int(row["post_id"]),
                "chunk_id": str(row["chunk_id"]),
                "model_name": str(row["model_name"]),
            }
            for row in rows
        ]

    async def close(self) -> None:
        return None
