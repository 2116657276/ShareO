from types import SimpleNamespace
from unittest.mock import AsyncMock

import pytest
from qdrant_client.models import Distance, VectorParams

from app.rag.vectorstore import TextVectorStore, point_id


def test_chunk_point_id_is_deterministic_uuid():
    assert point_id("7:0") == point_id("7:0")
    assert point_id("7:0") != point_id("7:1")


@pytest.mark.asyncio
async def test_text_collection_uses_512_cosine_and_two_indexes():
    store = TextVectorStore("http://qdrant")
    store.client = AsyncMock()
    store.client.get_collections.return_value = SimpleNamespace(collections=[])
    store.client.get_collection.return_value = SimpleNamespace(
        config=SimpleNamespace(
            params=SimpleNamespace(vectors=VectorParams(size=512, distance=Distance.COSINE))
        ),
        payload_schema={},
    )
    await store.ensure_collection()
    assert store.client.create_payload_index.await_count == 2


@pytest.mark.asyncio
async def test_incompatible_text_collection_is_rejected():
    store = TextVectorStore("http://qdrant")
    store.client = AsyncMock()
    store.client.get_collection.return_value = SimpleNamespace(
        config=SimpleNamespace(
            params=SimpleNamespace(vectors=VectorParams(size=384, distance=Distance.COSINE))
        ),
        payload_schema={},
    )
    with pytest.raises(RuntimeError, match="incompatible post_chunks collection"):
        await store.validate_schema()
