from unittest.mock import AsyncMock

import pytest

from app.workers.consumer import StreamConsumer


def make_consumer(redis: AsyncMock, **kwargs) -> StreamConsumer:
    return StreamConsumer(redis, "events", "workers", "worker-1", **kwargs)


@pytest.mark.asyncio
async def test_successful_message_is_acked():
    redis = AsyncMock()
    handler = AsyncMock()
    consumer = make_consumer(redis)

    await consumer._process_message(handler, "1-0", {"value": "ok"})

    handler.assert_awaited_once()
    redis.xack.assert_awaited_once_with("events", "workers", "1-0")


@pytest.mark.asyncio
async def test_failed_message_remains_pending_before_retry_limit():
    redis = AsyncMock()
    redis.xpending_range.return_value = [{"times_delivered": 1}]
    handler = AsyncMock(side_effect=RuntimeError("boom"))
    consumer = make_consumer(redis, max_retries=3)

    await consumer._process_message(handler, "2-0", {"value": "bad"})

    redis.xack.assert_not_awaited()


@pytest.mark.asyncio
async def test_failed_message_is_acked_after_three_retries():
    redis = AsyncMock()
    redis.xpending_range.return_value = [{"times_delivered": 4}]
    handler = AsyncMock(side_effect=RuntimeError("boom"))
    consumer = make_consumer(redis, max_retries=3)

    await consumer._process_message(handler, "3-0", {"value": "bad"})

    redis.xack.assert_awaited_once_with("events", "workers", "3-0")


@pytest.mark.asyncio
async def test_message_gets_four_total_attempts():
    redis = AsyncMock()
    redis.xpending_range.side_effect = [
        [{"times_delivered": 1}],
        [{"times_delivered": 2}],
        [{"times_delivered": 3}],
        [{"times_delivered": 4}],
    ]
    handler = AsyncMock(side_effect=RuntimeError("boom"))
    consumer = make_consumer(redis, max_retries=3)

    for _ in range(4):
        await consumer._process_message(handler, "3-1", {"value": "bad"})

    assert handler.await_count == 4
    redis.xack.assert_awaited_once_with("events", "workers", "3-1")


@pytest.mark.asyncio
async def test_reclaimed_messages_are_processed_immediately():
    redis = AsyncMock()
    redis.xautoclaim.return_value = ("0-0", [("4-0", {"value": "claimed"})], [])
    handler = AsyncMock()
    consumer = make_consumer(redis, min_idle_time_ms=0)

    await consumer.reclaim_pending(handler)

    handler.assert_awaited_once_with("4-0", {"value": "claimed"})
    redis.xack.assert_awaited_once_with("events", "workers", "4-0")


@pytest.mark.asyncio
async def test_consumer_group_creation_is_idempotent():
    redis = AsyncMock()
    redis.xgroup_create.side_effect = RuntimeError("BUSYGROUP Consumer Group name already exists")
    consumer = make_consumer(redis)

    await consumer.ensure_group()

    redis.xgroup_create.assert_awaited_once_with("events", "workers", id="0-0", mkstream=True)


@pytest.mark.asyncio
async def test_restart_reclaims_pending_before_reading_new_messages():
    redis = AsyncMock()
    redis.xautoclaim.return_value = ("0-0", [("5-0", {"value": "from-old-worker"})], [])
    consumer = make_consumer(redis, min_idle_time_ms=0)

    async def handle_and_stop(*_args):
        consumer.stop()

    await consumer.run(handle_and_stop)

    redis.xack.assert_awaited_once_with("events", "workers", "5-0")
    redis.xreadgroup.assert_not_awaited()
