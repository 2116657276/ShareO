from types import SimpleNamespace
from unittest.mock import AsyncMock

import pytest
from httpx import ASGITransport, AsyncClient

from app import main
from app.rag.pipeline import Citation, RAGAnswer

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
    monkeypatch.setattr(
        main.app.state,
        "worker_runtime",
        SimpleNamespace(status=lambda: {main.STREAM_INDEX_POST: "running"}),
        raising=False,
    )
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/readyz/image-search", headers={"X-Internal-Token": "secret"})
    assert resp.status_code == 503
    assert resp.json()["status"] == "warming"


@pytest.mark.asyncio
async def test_search_readyz_reports_model_and_collection(monkeypatch):
    monkeypatch.setattr(main.settings, "internal_token", "secret")
    monkeypatch.setattr(main.embedder, "_model", object())
    monkeypatch.setattr(main.embedder, "_state", "ready")
    monkeypatch.setattr(
        main.app.state,
        "worker_runtime",
        SimpleNamespace(status=lambda: {main.STREAM_INDEX_POST: "running"}),
        raising=False,
    )
    store = AsyncMock()
    store.metadata.return_value = {"collection": "images", "dimension": 512}
    monkeypatch.setattr(main, "vector_store", store)
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/readyz/image-search", headers={"X-Internal-Token": "secret"})
    assert resp.status_code == 200
    assert resp.json()["vector_store"]["dimension"] == 512
    assert resp.json()["consumer"] == "running"


@pytest.mark.asyncio
async def test_image_search_readyz_reports_stopped_consumer(monkeypatch):
    monkeypatch.setattr(main.settings, "internal_token", "secret")
    monkeypatch.setattr(main.embedder, "_model", object())
    monkeypatch.setattr(main.embedder, "_state", "ready")
    monkeypatch.setattr(
        main.app.state,
        "worker_runtime",
        SimpleNamespace(status=lambda: {main.STREAM_INDEX_POST: "stopped"}),
        raising=False,
    )
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/readyz/image-search", headers={"X-Internal-Token": "secret"})
    assert resp.status_code == 503
    assert resp.json()["consumer"] == "stopped"


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


@pytest.mark.asyncio
async def test_rag_readyz_requires_llm_configuration(monkeypatch):
    monkeypatch.setattr(main.settings, "internal_token", "secret")
    monkeypatch.setattr(main.text_embedder, "_model", object())
    monkeypatch.setattr(main.text_embedder, "_state", "ready")
    monkeypatch.setattr(main.llm_provider, "base_url", "")
    monkeypatch.setattr(main.llm_provider, "api_key", "")
    monkeypatch.setattr(
        main.app.state,
        "worker_runtime",
        SimpleNamespace(status=lambda: {main.STREAM_INDEX_POST: "running"}),
        raising=False,
    )
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/readyz/rag", headers={"X-Internal-Token": "secret"})
    assert resp.status_code == 503
    assert resp.json()["llm"] == "missing_configuration"


@pytest.mark.asyncio
async def test_rag_readyz_reports_model_collection_and_provider(monkeypatch):
    monkeypatch.setattr(main.settings, "internal_token", "secret")
    monkeypatch.setattr(main.text_embedder, "_model", object())
    monkeypatch.setattr(main.text_embedder, "_state", "ready")
    monkeypatch.setattr(main.llm_provider, "base_url", "http://llm/v1")
    monkeypatch.setattr(main.llm_provider, "api_key", "secret")
    monkeypatch.setattr(
        main.app.state,
        "worker_runtime",
        SimpleNamespace(status=lambda: {main.STREAM_INDEX_POST: "running"}),
        raising=False,
    )
    store = AsyncMock()
    store.metadata.return_value = {"collection": "post_chunks", "dimension": 512}
    monkeypatch.setattr(main, "text_vector_store", store)
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/readyz/rag", headers={"X-Internal-Token": "secret"})
    assert resp.status_code == 200
    assert resp.json()["vector_store"]["collection"] == "post_chunks"


@pytest.mark.asyncio
async def test_rag_answer_returns_only_pipeline_citations(monkeypatch):
    monkeypatch.setattr(main.settings, "internal_token", "secret")
    monkeypatch.setattr(main.llm_provider, "base_url", "http://llm/v1")
    monkeypatch.setattr(main.llm_provider, "api_key", "secret")
    pipeline = AsyncMock()
    pipeline.answer.return_value = RAGAnswer(
        "使用三脚架。",
        [Citation(post_id=7, chunk_id="7:0", excerpt="夜景使用三脚架。", score=0.9)],
    )
    monkeypatch.setattr(main, "rag_pipeline", pipeline)
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/v1/rag/answer",
            headers={"X-Internal-Token": "secret"},
            json={"question": "夜景怎么拍？", "history": [], "top_k": 8},
        )
    assert resp.status_code == 200
    assert resp.json()["citations"][0]["chunk_id"] == "7:0"


@pytest.mark.asyncio
async def test_rag_answer_rejects_missing_provider_before_retrieval(monkeypatch):
    monkeypatch.setattr(main.settings, "internal_token", "secret")
    monkeypatch.setattr(main.llm_provider, "base_url", "")
    monkeypatch.setattr(main.llm_provider, "api_key", "")
    pipeline = AsyncMock()
    monkeypatch.setattr(main, "rag_pipeline", pipeline)
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/v1/rag/answer",
            headers={"X-Internal-Token": "secret"},
            json={"question": "问题", "history": [], "top_k": 8},
        )
    assert resp.status_code == 503
    pipeline.answer.assert_not_awaited()
