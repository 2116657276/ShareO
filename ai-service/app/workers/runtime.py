"""Background Redis Stream consumers owned by the FastAPI process."""

import asyncio
import logging
import os
import socket
from collections.abc import Callable

import httpx
from redis.asyncio import Redis

from app.config import settings
from app.core.embedding import ImageEmbedder
from app.core.vectorstore import ImageVectorStore
from app.rag.embedding import TextEmbedder
from app.rag.indexer import TextIndexer
from app.rag.pipeline import RAGPipeline
from app.rag.vectorstore import TextVectorStore
from app.agent.graph import AgentRunner
from app.workers.bot import BotTaskHandler
from app.workers.consumer import StreamConsumer
from app.workers.indexer import ImageIndexer

logger = logging.getLogger(__name__)

STREAM_INDEX_POST = "shareo:stream:index_post"
STREAM_BOT_TASKS = "shareo:stream:bot_tasks"
CONSUMER_GROUP = "ai-workers"


class WorkerRuntime:
    """Start and stop both consumers with the API process lifecycle."""

    def __init__(
        self,
        embedder: ImageEmbedder,
        vector_store: ImageVectorStore,
        *,
        text_embedder: TextEmbedder | None = None,
        text_vector_store: TextVectorStore | None = None,
        rag_pipeline: RAGPipeline | None = None,
        agent_runner: AgentRunner | None = None,
        redis: Redis | None = None,
        http_client: httpx.AsyncClient | None = None,
        consumer_factory: Callable[..., StreamConsumer] = StreamConsumer,
    ) -> None:
        self.redis = redis or Redis.from_url(
            settings.redis_url,
            decode_responses=True,
            socket_timeout=10.0,
        )
        self.http = http_client or httpx.AsyncClient(timeout=10.0)
        self.vector_store = vector_store
        self.text_vector_store = text_vector_store
        text_indexer = (
            TextIndexer(text_embedder, text_vector_store)
            if text_embedder is not None and text_vector_store is not None
            else None
        )
        self.indexer = ImageIndexer(self.http, embedder, vector_store, text_indexer)
        self.bot_handler = BotTaskHandler(self.http, rag_pipeline, agent_runner=agent_runner)
        consumer_name = f"{socket.gethostname()}-{os.getpid()}"
        self.consumers = [
            consumer_factory(self.redis, STREAM_INDEX_POST, CONSUMER_GROUP, consumer_name),
            consumer_factory(
                self.redis,
                STREAM_BOT_TASKS,
                CONSUMER_GROUP,
                consumer_name,
                max_retries=3,
                reclaim_interval_ms=settings.bot_reclaim_interval_ms,
                min_idle_time_ms=settings.bot_min_idle_time_ms,
                dead_letter_handler=self.bot_handler.fallback,
            ),
        ]
        self.handlers = {
            STREAM_INDEX_POST: self.indexer.handle,
            STREAM_BOT_TASKS: self.bot_handler,
        }
        self.tasks: list[asyncio.Task] = []

    async def start(self) -> None:
        if self.tasks:
            return
        self.tasks = [
            asyncio.create_task(
                consumer.run(self.handlers[consumer.stream]),
                name=f"stream-consumer:{consumer.stream}",
            )
            for consumer in self.consumers
        ]
        logger.info("AI consumers started streams=%s", ",".join(self.handlers))

    async def stop(self) -> None:
        for consumer in self.consumers:
            consumer.stop()
        for task in self.tasks:
            task.cancel()
        if self.tasks:
            await asyncio.gather(*self.tasks, return_exceptions=True)
        self.tasks.clear()
        await self.http.aclose()
        await self.redis.aclose()
        await self.vector_store.close()
        if self.text_vector_store is not None:
            await self.text_vector_store.close()
        logger.info("AI consumers stopped")

    def status(self) -> dict[str, str]:
        """Expose stream-level liveness for feature readiness probes."""
        return {
            consumer.stream: (
                "running" if index < len(self.tasks) and not self.tasks[index].done() else "stopped"
            )
            for index, consumer in enumerate(self.consumers)
        }
