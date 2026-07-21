from io import BytesIO

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
