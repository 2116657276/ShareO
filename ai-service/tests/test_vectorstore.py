from unittest.mock import AsyncMock

import pytest

from app.core.vectorstore import ImageVectorStore, image_payload


@pytest.mark.asyncio
async def test_image_store_validates_pgvector_dimension_and_reports_metadata():
    database = AsyncMock()
    database.fetch_one.side_effect = [
        {"type_name": "vector(512)"},
        {"count": 2},
        {"indexed": True},
    ]
    store = ImageVectorStore(database)

    await store.ensure_collection()
    metadata = await store.metadata()

    assert metadata == {
        "collection": "images",
        "dimension": 512,
        "distance": "Cosine",
        "points_count": 2,
        "post_id_indexed": True,
    }


@pytest.mark.asyncio
async def test_image_store_rejects_incompatible_pgvector_dimension():
    database = AsyncMock()
    database.fetch_one.return_value = {"type_name": "vector(256)"}
    store = ImageVectorStore(database)

    with pytest.raises(RuntimeError, match="incompatible images vector column"):
        await store.validate_schema()


@pytest.mark.asyncio
async def test_image_store_search_returns_cosine_similarity_rows():
    database = AsyncMock()
    database.fetch_one.return_value = {"type_name": "vector(512)"}
    database.fetch_all.return_value = [
        {
            "image_id": 11,
            "post_id": 7,
            "object_key": "posts/medium/a.jpg",
            "created_at": "2026-01-01T00:00:00+00:00",
            "score": 0.91,
        }
    ]
    store = ImageVectorStore(database)

    result = await store.search([1.0] * 512, 5)

    assert result[0]["image_id"] == 11
    assert result[0]["score"] == 0.91
    query = database.fetch_all.call_args.args[0]
    assert "embedding <=>" in query
    assert "ORDER BY embedding <=>" in query


def test_image_payload_contains_required_fields():
    payload = image_payload(7, 11, "posts/medium/a.jpg", "2026-01-01T00:00:00Z")
    assert payload == {
        "post_id": 7,
        "image_id": 11,
        "object_key": "posts/medium/a.jpg",
        "created_at": "2026-01-01T00:00:00Z",
        "model_revision": "",
    }
