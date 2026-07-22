from unittest.mock import AsyncMock

import pytest

from app.rag.indexer import TextIndexer


class FakeEmbedder:
    model_name = "BAAI/bge-small-zh-v1.5"

    def embed_documents(self, documents):
        return [[1.0] * 512 for _ in documents]


@pytest.mark.asyncio
async def test_text_indexer_builds_deterministic_chunks(monkeypatch):
    monkeypatch.setattr("app.rag.indexer.settings.chunk_size", 10)
    monkeypatch.setattr("app.rag.indexer.settings.chunk_overlap", 2)
    store = AsyncMock()
    indexer = TextIndexer(FakeEmbedder(), store)
    await indexer.replace_post(
        {"post_id": 7, "content": "第一句。第二句。第三句。", "created_at": "2026-01-01"}
    )
    post_id, points = store.replace_post.await_args.args
    assert post_id == 7
    assert [item["chunk_id"] for item in points] == ["7:0", "7:1"]
    assert all(item["payload"]["model_name"] == FakeEmbedder.model_name for item in points)


@pytest.mark.asyncio
async def test_text_indexer_deletes_empty_content():
    store = AsyncMock()
    await TextIndexer(FakeEmbedder(), store).replace_post({"post_id": 7, "content": "  "})
    store.delete_post.assert_awaited_once_with(7)
