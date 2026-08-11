"""PostgreSQL/pgvector operations for image embeddings."""

from collections.abc import Sequence
from datetime import datetime
from typing import Any

from pgvector import Vector

from app.core.pgvector import PostgresVectorDatabase


COLLECTION_NAME = "images"
VECTOR_SIZE = 512


class ImageVectorStore:
    def __init__(self, database: PostgresVectorDatabase, collection_name: str = COLLECTION_NAME):
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
            WHERE a.attrelid = 'ai.image_embeddings'::regclass
              AND a.attname = 'embedding'
              AND NOT a.attisdropped
            """
        )
        if row is None or str(row.get("type_name", "")) != f"vector({VECTOR_SIZE})":
            raise RuntimeError(
                "incompatible images vector column: "
                f"expected vector({VECTOR_SIZE}), got {row.get('type_name') if row else 'missing'}"
            )

    async def ensure_payload_indexes(self) -> None:
        """Compatibility name retained; indexes are created by migrations."""
        await self.ensure_collection()

    async def upsert(
        self,
        points: Sequence[dict[str, Any]],
    ) -> None:
        if not points:
            return
        await self.ensure_collection()
        async with self.database.transaction() as connection:
            async with connection.cursor() as cursor:
                for point in points:
                    payload = point["payload"]
                    await cursor.execute(
                        """
                        INSERT INTO ai.image_embeddings
                            (image_id, post_id, object_key, created_at, model_revision, embedding)
                        VALUES (%s, %s, %s, %s, %s, %s)
                        ON CONFLICT (image_id) DO UPDATE SET
                            post_id = EXCLUDED.post_id,
                            object_key = EXCLUDED.object_key,
                            created_at = EXCLUDED.created_at,
                            model_revision = EXCLUDED.model_revision,
                            embedding = EXCLUDED.embedding
                        """,
                        (
                            int(point["image_id"]),
                            int(payload["post_id"]),
                            str(payload["object_key"]),
                            payload["created_at"],
                            str(payload.get("model_revision", "")),
                            Vector(list(point["vector"])),
                        ),
                    )

    async def delete_post(self, post_id: int) -> None:
        await self.ensure_collection()
        await self.database.execute(
            "DELETE FROM ai.image_embeddings WHERE post_id = %s", (int(post_id),)
        )

    async def search(self, vector: Sequence[float], limit: int) -> list[dict[str, Any]]:
        await self.ensure_collection()
        query_vector = Vector(list(vector))
        rows = await self.database.fetch_all(
            """
            SELECT image_id, post_id, object_key, created_at,
                   1 - (embedding <=> %s) AS score
            FROM ai.image_embeddings
            ORDER BY embedding <=> %s
            LIMIT %s
            """,
            (query_vector, query_vector, int(limit)),
        )
        return [
            {
                "image_id": int(row["image_id"]),
                "post_id": int(row["post_id"]),
                "object_key": str(row["object_key"]),
                "created_at": row["created_at"].isoformat()
                if isinstance(row["created_at"], datetime)
                else str(row["created_at"]),
                "score": float(row["score"]),
            }
            for row in rows
        ]

    async def close(self) -> None:
        """Store instances share the application-owned database pool."""
        return None

    async def metadata(self) -> dict[str, Any]:
        await self.ensure_collection()
        count = await self.database.fetch_one(
            "SELECT count(*)::bigint AS count FROM ai.image_embeddings"
        )
        index = await self.database.fetch_one(
            """
            SELECT EXISTS (
                SELECT 1 FROM pg_indexes
                WHERE schemaname = 'ai' AND indexname = 'idx_ai_image_embeddings_post'
            ) AS indexed
            """
        )
        return {
            "collection": self.collection_name,
            "dimension": VECTOR_SIZE,
            "distance": "Cosine",
            "points_count": int((count or {}).get("count", 0)),
            "post_id_indexed": bool((index or {}).get("indexed", False)),
        }

    async def inventory(self) -> list[dict[str, Any]]:
        await self.ensure_collection()
        rows = await self.database.fetch_all(
            "SELECT image_id, post_id, model_revision FROM ai.image_embeddings ORDER BY image_id"
        )
        return [
            {
                "image_id": int(row["image_id"]),
                "post_id": int(row["post_id"]),
                "model_revision": str(row["model_revision"]),
            }
            for row in rows
        ]


def image_payload(
    post_id: int,
    image_id: int,
    object_key: str,
    created_at: datetime | str,
    model_revision: str = "",
) -> dict[str, Any]:
    timestamp = created_at.isoformat() if isinstance(created_at, datetime) else str(created_at)
    return {
        "post_id": int(post_id),
        "image_id": int(image_id),
        "object_key": object_key,
        "created_at": timestamp,
        "model_revision": model_revision,
    }
