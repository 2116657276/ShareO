"""Redis Stream consumer with XREADGROUP, XAUTOCLAIM, and XACK support."""

import asyncio
import logging
from typing import Any, Callable

from redis.asyncio import Redis

logger = logging.getLogger(__name__)

Handler = Callable[[str, dict[str, Any]], None]


class StreamConsumer:
    """Generic Redis Stream consumer with consumer group support."""

    def __init__(
        self,
        redis: Redis,
        stream: str,
        group: str,
        consumer: str,
        max_retries: int = 3,
    ):
        self.redis = redis
        self.stream = stream
        self.group = group
        self.consumer = consumer
        self.max_retries = max_retries
        self._running = False

    async def ensure_group(self):
        """Create consumer group if it doesn't exist. Claim pending messages."""
        try:
            await self.redis.xgroup_create(self.stream, self.group, mkstream=True)
        except Exception as e:
            if "BUSYGROUP" not in str(e):
                raise
        # Reclaim pending messages from previous runs
        pending = await self.redis.xautoclaim(
            self.stream, self.group, self.consumer, min_idle_time=30000
        )
        if pending[1]:
            logger.info("auto-claimed %d pending messages from %s", len(pending[1]), self.stream)

    async def run(self, handler: Handler):
        """Main loop: XREADGROUP → handle → XACK."""
        self._running = True
        await self.ensure_group()
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
                    continue
                for _, messages in results:
                    for msg_id, fields in messages:
                        try:
                            await handler(msg_id, fields)
                            await self.redis.xack(self.stream, self.group, msg_id)
                        except Exception as e:
                            logger.error(
                                "failed to process %s/%s: %s", self.stream, msg_id, e
                            )
                            await self.redis.xack(self.stream, self.group, msg_id)
            except Exception as e:
                logger.error("consumer loop error in %s: %s", self.stream, e)
                await asyncio.sleep(1)

    def stop(self):
        self._running = False
