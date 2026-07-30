from unittest.mock import AsyncMock

import pytest
from httpx import ASGITransport, AsyncClient

from app import main


@pytest.mark.asyncio
async def test_post_search_requires_internal_token(monkeypatch):
    monkeypatch.setattr(main.settings, "internal_token", "secret")
    transport = ASGITransport(app=main.app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post("/v1/search/posts", json={"query": "西湖"})
    assert response.status_code == 401


@pytest.mark.asyncio
async def test_post_search_returns_text_candidates(monkeypatch):
    monkeypatch.setattr(main.settings, "internal_token", "secret")
    monkeypatch.setattr(main.text_embedder, "embed_query", lambda query: [1.0] * 512)
    store = AsyncMock()
    store.search.return_value = [
        {
            "post_id": 9,
            "chunk_id": "9:0",
            "chunk_text": "西湖夜色真美",
            "score": 0.91,
        }
    ]
    monkeypatch.setattr(main, "text_vector_store", store)
    transport = ASGITransport(app=main.app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post(
            "/v1/search/posts",
            headers={"X-Internal-Token": "secret"},
            json={"query": "夜晚的湖边", "limit": 20},
        )
    assert response.status_code == 200
    assert response.json()["results"][0]["post_id"] == 9
    store.search.assert_awaited_once_with([1.0] * 512, 20)


@pytest.mark.asyncio
async def test_post_search_validates_limit(monkeypatch):
    monkeypatch.setattr(main.settings, "internal_token", "secret")
    transport = ASGITransport(app=main.app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post(
            "/v1/search/posts",
            headers={"X-Internal-Token": "secret"},
            json={"query": "西湖", "limit": 201},
        )
    assert response.status_code == 422
