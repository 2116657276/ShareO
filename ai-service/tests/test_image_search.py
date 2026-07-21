from unittest.mock import AsyncMock

import pytest
from httpx import ASGITransport, AsyncClient

from app import main


@pytest.mark.asyncio
async def test_image_search_requires_internal_token(monkeypatch):
    monkeypatch.setattr(main.settings, "internal_token", "secret")
    transport = ASGITransport(app=main.app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post("/v1/search/images", json={"query": "山"})
    assert response.status_code == 401


@pytest.mark.asyncio
async def test_image_search_returns_qdrant_results(monkeypatch):
    monkeypatch.setattr(main.settings, "internal_token", "secret")
    monkeypatch.setattr(main.embedder, "encode_text", lambda query: [1.0] * 512)
    store = AsyncMock()
    store.search.return_value = [
        {"image_id": 3, "post_id": 9, "object_key": "posts/medium/a.jpg", "score": 0.91}
    ]
    monkeypatch.setattr(main, "vector_store", store)
    transport = ASGITransport(app=main.app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post(
            "/v1/search/images",
            headers={"X-Internal-Token": "secret"},
            json={"query": "山", "limit": 5},
        )
    assert response.status_code == 200
    assert response.json()["results"][0]["post_id"] == 9
    store.search.assert_awaited_once_with([1.0] * 512, 15)


@pytest.mark.asyncio
async def test_image_search_validates_limits(monkeypatch):
    monkeypatch.setattr(main.settings, "internal_token", "secret")
    transport = ASGITransport(app=main.app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post(
            "/v1/search/images",
            headers={"X-Internal-Token": "secret"},
            json={"query": "", "limit": 1},
        )
    assert response.status_code == 422
