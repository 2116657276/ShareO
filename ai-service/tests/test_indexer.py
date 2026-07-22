from io import BytesIO
from unittest.mock import AsyncMock

import httpx
import pytest
from PIL import Image

from app.workers.indexer import ImageIndexer


class FakeEmbedder:
    def encode_images(self, images):
        return [[float(index)] * 512 for index, _ in enumerate(images, 1)]


class FakeStore:
    def __init__(self):
        self.deleted = []
        self.upserted = []

    async def delete_post(self, post_id):
        self.deleted.append(post_id)

    async def upsert(self, points):
        self.upserted.extend(points)


def response(status, *, json=None, content=b"", url="http://go"):
    request = httpx.Request("GET", url)
    if json is not None:
        return httpx.Response(status, json=json, request=request)
    return httpx.Response(status, content=content, request=request)


@pytest.mark.asyncio
async def test_indexer_upsert_is_idempotent_payload():
    image_buffer = BytesIO()
    Image.new("RGB", (1, 1), "red").save(image_buffer, format="PNG")
    payload = {
        "data": {
            "post_id": 7,
            "images": [
                {
                    "image_id": 11,
                    "object_key": "posts/medium/a.jpg",
                    "image_url": "/api/v1/images/posts/medium/a.jpg",
                    "created_at": "2026-01-01T00:00:00Z",
                }
            ],
        }
    }

    class Client:
        async def get(self, url, **kwargs):
            if "index-payload" in url:
                return response(200, json=payload, url=url)
            return response(200, content=image_buffer.getvalue(), url=url)

    store = FakeStore()
    await ImageIndexer(Client(), FakeEmbedder(), store, internal_token="secret").handle(
        "1-0", {"action": "upsert", "post_id": "7"}
    )
    assert store.deleted == [7]
    assert store.upserted[0]["image_id"] == 11
    assert store.upserted[0]["payload"]["post_id"] == 7


@pytest.mark.asyncio
async def test_indexer_delete_is_idempotent():
    store = FakeStore()
    await ImageIndexer(None, FakeEmbedder(), store).handle(
        "1-0", {"action": "delete", "post_id": "7"}
    )
    assert store.deleted == [7]


@pytest.mark.asyncio
async def test_post_without_images_still_updates_text_index():
    payload = {
        "data": {
            "post_id": 7,
            "content": "只有正文也必须被索引。",
            "created_at": "2026-01-01T00:00:00Z",
            "images": [],
        }
    }

    class Client:
        async def get(self, url, **_kwargs):
            return response(200, json=payload, url=url)

    text_indexer = AsyncMock()
    store = FakeStore()
    await ImageIndexer(
        Client(), FakeEmbedder(), store, text_indexer, internal_token="secret"
    ).handle("1-0", {"action": "upsert", "post_id": "7"})
    text_indexer.replace_post.assert_awaited_once_with(payload["data"])
    assert store.deleted == [7]


@pytest.mark.asyncio
async def test_failed_image_module_does_not_starve_text_module():
    payload = {"data": {"post_id": 7, "content": "正文", "images": []}}

    class Client:
        async def get(self, url, **_kwargs):
            return response(200, json=payload, url=url)

    image_store = AsyncMock()
    image_store.delete_post.side_effect = RuntimeError("qdrant image failure")
    text_indexer = AsyncMock()
    with pytest.raises(RuntimeError, match="post index operation"):
        await ImageIndexer(
            Client(), FakeEmbedder(), image_store, text_indexer, internal_token="secret"
        ).handle("1-0", {"action": "upsert", "post_id": "7"})
    text_indexer.replace_post.assert_awaited_once_with(payload["data"])
