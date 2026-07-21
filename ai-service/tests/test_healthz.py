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
