import pytest
from httpx import ASGITransport, AsyncClient
from unittest.mock import AsyncMock

from app import main

app = main.app


@pytest.mark.asyncio
async def test_healthz():
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/healthz")
        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "ok"
        assert data["service"] == "shareo-ai"


@pytest.mark.asyncio
async def test_readyz_ready(monkeypatch):
    monkeypatch.setattr(main, "check_redis", AsyncMock())
    monkeypatch.setattr(main, "check_qdrant", AsyncMock())
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/readyz")
    assert resp.status_code == 200
    assert resp.json()["status"] == "ready"


@pytest.mark.asyncio
async def test_readyz_degraded(monkeypatch):
    monkeypatch.setattr(main, "check_redis", AsyncMock(side_effect=RuntimeError("offline")))
    monkeypatch.setattr(main, "check_qdrant", AsyncMock())
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/readyz")
    assert resp.status_code == 503
    assert resp.json()["dependencies"]["redis"] == "unavailable"


@pytest.mark.asyncio
async def test_search_readyz_reports_warming(monkeypatch):
    monkeypatch.setattr(main.settings, "internal_token", "secret")
    monkeypatch.setattr(main.embedder, "_model", None)
    monkeypatch.setattr(main.embedder, "_state", "loading")
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/readyz/search", headers={"X-Internal-Token": "secret"})
    assert resp.status_code == 503
    assert resp.json()["status"] == "warming"


@pytest.mark.asyncio
async def test_search_readyz_reports_model_and_collection(monkeypatch):
    monkeypatch.setattr(main.settings, "internal_token", "secret")
    monkeypatch.setattr(main.embedder, "_model", object())
    monkeypatch.setattr(main.embedder, "_state", "ready")
    store = AsyncMock()
    store.metadata.return_value = {"collection": "images", "dimension": 512}
    monkeypatch.setattr(main, "vector_store", store)
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/readyz/search", headers={"X-Internal-Token": "secret"})
    assert resp.status_code == 200
    assert resp.json()["vector_store"]["dimension"] == 512


@pytest.mark.asyncio
async def test_image_search_rejects_blank_query_before_model_load(monkeypatch):
    monkeypatch.setattr(main.settings, "internal_token", "secret")
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/v1/search/images",
            headers={"X-Internal-Token": "secret"},
            json={"query": "   ", "limit": 10},
        )
    assert resp.status_code == 422
