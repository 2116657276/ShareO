import os
import uuid

import pytest
from redis.asyncio import Redis

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
