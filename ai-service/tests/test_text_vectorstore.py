from unittest.mock import AsyncMock

import pytest

from app.rag.vectorstore import TextVectorStore, point_id


def test_chunk_point_id_is_deterministic_uuid():
    assert point_id("7:0") == point_id("7:0")
    assert point_id("7:0") != point_id("7:1")


@pytest.mark.asyncio
async def test_text_store_validates_pgvector_dimension():
    database = AsyncMock()
    database.fetch_one.return_value = {"type_name": "vector(512)"}
    store = TextVectorStore(database)

    await store.ensure_collection()
    database.fetch_one.assert_awaited()


@pytest.mark.asyncio
async def test_incompatible_text_vector_dimension_is_rejected():
    database = AsyncMock()
    database.fetch_one.return_value = {"type_name": "vector(384)"}
    store = TextVectorStore(database)

    with pytest.raises(RuntimeError, match="incompatible post_chunks vector column"):
        await store.validate_schema()
