"""Worker process entry point — runs all stream consumers concurrently."""

import asyncio
import logging

from redis.asyncio import Redis

from app.config import settings
from app.workers.consumer import StreamConsumer

STREAM_INDEX_POST = "shareo:stream:index_post"
STREAM_BOT_TASKS = "shareo:stream:bot_tasks"
CONSUMER_GROUP = "ai-workers"


async def main():
    logging.basicConfig(level=settings.log_level)
    logger = logging.getLogger(__name__)
    redis = Redis.from_url(settings.redis_url, decode_responses=True)

    async def handle_index_post(msg_id: str, fields: dict):
        logger.info("index_post: id=%s fields=%s", msg_id, fields)
        # TODO: implement in Phase 2 — vectorize and upsert to Qdrant

    async def handle_bot_task(msg_id: str, fields: dict):
        logger.info("bot_task: id=%s fields=%s", msg_id, fields)
        # TODO: implement in Phase 3 — RAG pipeline and reply

    consumers = [
        StreamConsumer(redis, STREAM_INDEX_POST, CONSUMER_GROUP, "worker-1"),
        StreamConsumer(redis, STREAM_BOT_TASKS, CONSUMER_GROUP, "worker-1"),
    ]

    handlers = {
        STREAM_INDEX_POST: handle_index_post,
        STREAM_BOT_TASKS: handle_bot_task,
    }

    try:
        await asyncio.gather(*(c.run(handlers[c.stream]) for c in consumers))
    finally:
        await redis.aclose()


if __name__ == "__main__":
    asyncio.run(main())
