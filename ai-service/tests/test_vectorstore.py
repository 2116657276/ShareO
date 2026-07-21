from types import SimpleNamespace
from unittest.mock import AsyncMock

import pytest

from app.core.vectorstore import ImageVectorStore, image_payload


@pytest.mark.asyncio
async def test_collection_initialization_uses_512_cosine():
    store = ImageVectorStore("http://qdrant")
    store.client = AsyncMock()
    store.client.get_collections.return_value = SimpleNamespace(collections=[])
    await store.ensure_collection()
    args = store.client.create_collection.call_args.kwargs
    assert args["collection_name"] == "images"
    assert args["vectors_config"].size == 512
    assert args["vectors_config"].distance.value == "Cosine"


def test_image_payload_contains_required_fields():
    payload = image_payload(7, 11, "posts/medium/a.jpg", "2026-01-01T00:00:00Z")
    assert payload == {
        "post_id": 7,
        "image_id": 11,
        "object_key": "posts/medium/a.jpg",
        "created_at": "2026-01-01T00:00:00Z",
    }
