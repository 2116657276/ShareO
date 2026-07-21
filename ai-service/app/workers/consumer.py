"""Redis Stream consumer with XREADGROUP, XAUTOCLAIM, and XACK support."""

import asyncio
import logging
import time
from collections.abc import Awaitable, Callable
from typing import Any

from redis.asyncio import Redis

logger = logging.getLogger(__name__)

Handler = Callable[[str, dict[str, Any]], Awaitable[None]]


class StreamConsumer:
    """Generic Redis Stream consumer with consumer group support."""

    def __init__(
        self,
        redis: Redis,
        stream: str,
        group: str,
        consumer: str,
        max_retries: int = 3,
        reclaim_interval_ms: int = 10_000,
        min_idle_time_ms: int = 30_000,
    ):
        self.redis = redis
        self.stream = stream
        self.group = group
        self.consumer = consumer
        self.max_retries = max_retries
        self.reclaim_interval_ms = reclaim_interval_ms
        self.min_idle_time_ms = min_idle_time_ms
        self._running = False

    async def ensure_group(self):
        """Create the consumer group if it does not exist."""
        try:
            await self.redis.xgroup_create(self.stream, self.group, id="0-0", mkstream=True)
        except Exception as e:
            if "BUSYGROUP" not in str(e):
                raise

    async def _delivery_count(self, msg_id: str) -> int:
        pending = await self.redis.xpending_range(
            self.stream, self.group, min=msg_id, max=msg_id, count=1
        )
        if not pending:
            return 1
        value = pending[0].get("times_delivered", pending[0].get(b"times_delivered", 1))
        return int(value)

    async def _process_message(self, handler: Handler, msg_id: str, fields: dict[str, Any]):
        try:
            await handler(msg_id, fields)
        except Exception:
            attempts = await self._delivery_count(msg_id)
            if attempts >= self.max_retries + 1:
                logger.exception(
                    "dropping message after retries stream=%s id=%s attempts=%d fields=%r",
                    self.stream,
                    msg_id,
                    attempts,
                    fields,
                )
                await self.redis.xack(self.stream, self.group, msg_id)
            else:
                logger.exception(
                    "message remains pending stream=%s id=%s attempts=%d",
                    self.stream,
                    msg_id,
                    attempts,
                )
            return
        await self.redis.xack(self.stream, self.group, msg_id)

    async def reclaim_pending(self, handler: Handler):
        """Claim idle pending messages and process the claimed payloads immediately."""
        claimed = await self.redis.xautoclaim(
            self.stream,
            self.group,
            self.consumer,
            min_idle_time=self.min_idle_time_ms,
            start_id="0-0",
            count=10,
        )
        messages = claimed[1]
        if messages:
            logger.info("auto-claimed %d pending messages from %s", len(messages), self.stream)
        for msg_id, fields in messages:
            await self._process_message(handler, msg_id, fields)

    async def run(self, handler: Handler):
        """Main loop: XREADGROUP → handle → XACK."""
        self._running = True
        while self._running:
            try:
                await self.ensure_group()
                await self.reclaim_pending(handler)
                break
            except Exception as exc:
                logger.error("consumer startup error in %s: %s", self.stream, exc)
                await asyncio.sleep(1)
        last_reclaim = time.monotonic()
        while self._running:
            try:
                results = await self.redis.xreadgroup(
                    self.group,
                    self.consumer,
                    {self.stream: ">"},
                    count=10,
                    block=5000,
                )
                if not results:
                    results = []
                for _, messages in results:
                    for msg_id, fields in messages:
                        await self._process_message(handler, msg_id, fields)
                now = time.monotonic()
                if (now - last_reclaim) * 1000 >= self.reclaim_interval_ms:
                    await self.reclaim_pending(handler)
                    last_reclaim = now
            except Exception as exc:
                logger.error("consumer loop error in %s: %s", self.stream, exc)
                await asyncio.sleep(1)

    def stop(self):
        self._running = False
