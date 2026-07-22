import asyncio
from unittest.mock import AsyncMock

import pytest

from app.workers.runtime import STREAM_BOT_TASKS, STREAM_INDEX_POST, WorkerRuntime


class FakeConsumer:
    def __init__(self, _redis, stream, _group, _name, **options):
        self.stream = stream
        self.options = options
        self.started = asyncio.Event()
        self.stopped = False

    async def run(self, _handler):
        self.started.set()
        await asyncio.Event().wait()

    def stop(self):
        self.stopped = True


@pytest.mark.asyncio
async def test_runtime_starts_both_streams_and_closes_shared_resources():
    redis = AsyncMock()
    http = AsyncMock()
    store = AsyncMock()
    text_store = AsyncMock()
    runtime = WorkerRuntime(
        object(),
        store,
        text_embedder=object(),
        text_vector_store=text_store,
        redis=redis,
        http_client=http,
        consumer_factory=FakeConsumer,
    )

    await runtime.start()
    await asyncio.gather(*(consumer.started.wait() for consumer in runtime.consumers))
    assert runtime.status() == {
        STREAM_INDEX_POST: "running",
        STREAM_BOT_TASKS: "running",
    }
    assert {consumer.stream for consumer in runtime.consumers} == {
        STREAM_INDEX_POST,
        STREAM_BOT_TASKS,
    }
    assert len(runtime.tasks) == 2
    bot_consumer = next(item for item in runtime.consumers if item.stream == STREAM_BOT_TASKS)
    assert bot_consumer.options["max_retries"] == 3
    assert bot_consumer.options["dead_letter_handler"] is not None

    await runtime.stop()
    assert runtime.status() == {
        STREAM_INDEX_POST: "stopped",
        STREAM_BOT_TASKS: "stopped",
    }
    assert all(consumer.stopped for consumer in runtime.consumers)
    http.aclose.assert_awaited_once()
    redis.aclose.assert_awaited_once()
    store.close.assert_awaited_once()
    text_store.close.assert_awaited_once()
