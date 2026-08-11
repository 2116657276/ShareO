import os
import uuid

import pytest
from redis.asyncio import Redis

from app.core.pgvector import PostgresVectorDatabase
from app.core.vectorstore import ImageVectorStore, image_payload
from app.rag.vectorstore import TextVectorStore
from app.workers.consumer import StreamConsumer


@pytest.mark.integration
async def test_redis_stream_round_trip():
    redis_url = os.getenv("SHAREO_TEST_REDIS_URL")
    if not redis_url:
        pytest.skip("set SHAREO_TEST_REDIS_URL to run Redis integration tests")

    redis = Redis.from_url(redis_url, decode_responses=True)
    stream = "shareo:test:integration"
    try:
        message_id = await redis.xadd(stream, {"kind": "probe"})
        messages = await redis.xrange(stream, min=message_id, max=message_id)
        assert messages == [(message_id, {"kind": "probe"})]
    finally:
        await redis.delete(stream)
        await redis.aclose()


@pytest.mark.integration
async def test_pending_message_is_reclaimed_then_acked_after_retry_budget():
    redis_url = os.getenv("SHAREO_TEST_REDIS_URL")
    if not redis_url:
        pytest.skip("set SHAREO_TEST_REDIS_URL to run Redis integration tests")

    redis = Redis.from_url(redis_url, decode_responses=True)
    suffix = uuid.uuid4().hex
    stream = f"shareo:test:retry:{suffix}"
    group = f"workers:{suffix}"
    message_id = await redis.xadd(stream, {"kind": "failure"})
    await redis.xgroup_create(stream, group, id="0-0")
    delivered = await redis.xreadgroup(group, "old-worker", {stream: ">"}, count=1)
    fields = delivered[0][1][0][1]
    attempts = 0

    async def fail_handler(_message_id, _fields):
        nonlocal attempts
        attempts += 1
        raise RuntimeError("expected integration failure")

    old = StreamConsumer(redis, stream, group, "old-worker", min_idle_time_ms=0)
    replacement = StreamConsumer(redis, stream, group, "replacement", min_idle_time_ms=0)
    try:
        await old._process_message(fail_handler, message_id, fields)
        for _ in range(3):
            await replacement.reclaim_pending(fail_handler)
        assert attempts == 4
        assert await redis.xpending_range(stream, group, "-", "+", 10) == []
    finally:
        await redis.delete(stream)
        await redis.aclose()


@pytest.mark.integration
async def test_text_pgvector_round_trip():
    database_url = os.getenv("SHAREO_TEST_AI_DATABASE_URL")
    if not database_url:
        pytest.skip("set SHAREO_TEST_AI_DATABASE_URL to run PostgreSQL/pgvector integration tests")
    database = PostgresVectorDatabase(database_url)
    store = TextVectorStore(database)
    vector = [1.0 / (512**0.5)] * 512
    opened = False
    try:
        await database.open()
        opened = True
        await store.ensure_collection()
        await store.replace_post(
            7,
            [
                {
                    "chunk_id": "7:0",
                    "vector": vector,
                    "payload": {
                        "post_id": 7,
                        "chunk_id": "7:0",
                        "chunk_text": "夜景使用三脚架。",
                        "model_name": "BAAI/bge-small-zh-v1.5",
                        "created_at": "2026-01-01",
                    },
                }
            ],
        )
        results = await store.search(vector, 5)
        assert results[0]["chunk_id"] == "7:0"
        await store.delete_post(7)
        assert await store.search(vector, 5) == []
    finally:
        if opened:
            await database.execute("DELETE FROM ai.post_chunk_embeddings WHERE post_id = %s", (7,))
        await database.close()


@pytest.mark.integration
async def test_image_pgvector_upsert_search_replace_and_delete():
    database_url = os.getenv("SHAREO_TEST_AI_DATABASE_URL")
    if not database_url:
        pytest.skip("set SHAREO_TEST_AI_DATABASE_URL to run PostgreSQL/pgvector integration tests")
    database = PostgresVectorDatabase(database_url)
    store = ImageVectorStore(database)
    vector = [1.0 / (512**0.5)] * 512
    opened = False
    try:
        await database.open()
        opened = True
        await store.ensure_collection()
        await store.upsert(
            [
                {
                    "image_id": 7001,
                    "vector": vector,
                    "payload": image_payload(
                        70,
                        7001,
                        "posts/original/2026/08/11/7001.jpg",
                        "2026-01-01",
                        "integration",
                    ),
                }
            ]
        )
        first = await store.search(vector, 5)
        assert first and first[0]["image_id"] == 7001
        await store.upsert(
            [
                {
                    "image_id": 7001,
                    "vector": vector,
                    "payload": image_payload(
                        71,
                        7001,
                        "posts/original/2026/08/11/7001-replaced.jpg",
                        "2026-01-02",
                        "integration-v2",
                    ),
                }
            ]
        )
        metadata = await store.metadata()
        assert metadata["dimension"] == 512 and metadata["points_count"] == 1
        assert (await store.inventory())[0]["post_id"] == 71
        await store.delete_post(71)
        assert await store.search(vector, 5) == []
    finally:
        if opened:
            await database.execute("DELETE FROM ai.image_embeddings WHERE image_id = %s", (7001,))
        await database.close()
