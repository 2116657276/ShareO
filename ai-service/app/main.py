import asyncio
import logging
import os
import time
from contextlib import asynccontextmanager
from typing import Literal

import httpx
from fastapi import FastAPI, Header, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field, field_validator
from qdrant_client import AsyncQdrantClient
from redis.asyncio import Redis

from app.config import settings
from app.agent.graph import AgentRunner
from app.agent.tools import AgentToolRuntime
from app.core.embedding import ImageEmbedder
from app.core.source_fingerprint import PROCESS_SOURCE_FINGERPRINT, PROCESS_STARTED_AT
from app.core.vectorstore import ImageVectorStore
from app.rag.embedding import TextEmbedder
from app.rag.pipeline import RAGPipeline
from app.rag.provider import LLMError, OpenAICompatibleProvider
from app.rag.vectorstore import TextVectorStore
from app.workers.runtime import STREAM_INDEX_POST, WorkerRuntime

logger = logging.getLogger(__name__)
embedder = ImageEmbedder()
vector_store = ImageVectorStore(settings.qdrant_url, settings.image_collection)
text_embedder = TextEmbedder()
text_vector_store = TextVectorStore(settings.qdrant_url, settings.text_collection)
llm_provider = OpenAICompatibleProvider(
    settings.llm_base_url,
    settings.llm_api_key,
    settings.llm_model,
    settings.llm_timeout_seconds,
)
rag_pipeline = RAGPipeline(text_embedder, text_vector_store, llm_provider)
agent_tool_runtime = AgentToolRuntime(
    httpx.AsyncClient(timeout=10.0),
    text_embedder,
    text_vector_store,
    embedder,
    vector_store,
)
agent_runner = AgentRunner(llm_provider, agent_tool_runtime)


class ImageSearchRequest(BaseModel):
    query: str = Field(min_length=1, max_length=200)
    limit: int = Field(default=10, ge=1, le=20)

    @field_validator("query")
    @classmethod
    def query_must_have_content(cls, value: str) -> str:
        value = value.strip()
        if not value:
            raise ValueError("query must not be blank")
        return value


class PostSearchRequest(BaseModel):
    query: str = Field(min_length=1, max_length=200)
    limit: int = Field(default=50, ge=1, le=200)

    @field_validator("query")
    @classmethod
    def query_must_have_content(cls, value: str) -> str:
        value = value.strip()
        if not value:
            raise ValueError("query must not be blank")
        return value


class HistoryMessage(BaseModel):
    role: Literal["user", "bot"]
    content: str = Field(min_length=1, max_length=2000)


class RAGAnswerRequest(BaseModel):
    question: str = Field(min_length=1, max_length=500)
    history: list[HistoryMessage] = Field(default_factory=list, max_length=20)
    top_k: int = Field(default=8, ge=1, le=20)

    @field_validator("question")
    @classmethod
    def question_must_have_content(cls, value: str) -> str:
        value = value.strip()
        if not value:
            raise ValueError("question must not be blank")
        return value


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Startup and shutdown lifecycle for ai-service."""
    logging.basicConfig(
        level=settings.log_level,
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
    )
    logging.getLogger().setLevel(settings.log_level)
    logger.setLevel(settings.log_level)
    logger.info("ai-service starting (log_level=%s)", settings.log_level)
    app.state.embedding_semaphore = asyncio.Semaphore(max(1, settings.embedding_concurrency))
    try:
        await vector_store.ensure_collection()
    except Exception as exc:
        logger.warning("image collection startup check failed: %s", exc)
    try:
        await text_vector_store.ensure_collection()
    except Exception as exc:
        logger.warning("text collection startup check failed: %s", exc)
    runtime = WorkerRuntime(
        embedder,
        vector_store,
        text_embedder=text_embedder,
        text_vector_store=text_vector_store,
        rag_pipeline=rag_pipeline,
        agent_runner=agent_runner,
    )
    app.state.worker_runtime = runtime
    await runtime.start()
    app.state.warmup_tasks = []
    if settings.embedding_warmup:
        warmups = (("image", embedder), ("text", text_embedder))
        for kind, model in warmups:
            task = asyncio.create_task(asyncio.to_thread(model.warmup), name=f"warmup:{kind}")

            def warmup_done(done: asyncio.Task, label: str = kind) -> None:
                try:
                    done.result()
                    logger.info("%s model warmup complete", label)
                except asyncio.CancelledError:
                    logger.info("%s model warmup cancelled", label)
                except Exception as exc:
                    logger.error("%s model warmup failed: %s", label, exc)

            task.add_done_callback(warmup_done)
            app.state.warmup_tasks.append(task)

    yield
    warmup_tasks = [task for task in app.state.warmup_tasks if not task.done()]
    for task in warmup_tasks:
        task.cancel()
    if warmup_tasks:
        await asyncio.gather(*warmup_tasks, return_exceptions=True)
    await llm_provider.close()
    await agent_tool_runtime.http.aclose()
    await runtime.stop()
    logger.info("ai-service shutting down")


app = FastAPI(
    title="ShareO AI Service",
    version="0.1.0",
    lifespan=lifespan,
)


@app.get("/healthz")
async def healthz():
    """Liveness probe: the API process is running."""
    return {"status": "ok", "service": "shareo-ai"}


async def check_redis() -> None:
    client = Redis.from_url(settings.redis_url)
    try:
        await client.ping()
    finally:
        await client.aclose()


async def check_qdrant() -> None:
    client = AsyncQdrantClient(url=settings.qdrant_url, check_compatibility=False)
    try:
        await client.get_collections()
    finally:
        await client.close()


@app.get("/readyz")
async def readyz():
    """Readiness probe: both backing services must be reachable."""
    dependencies: dict[str, str] = {}
    for name, check in (("redis", check_redis), ("qdrant", check_qdrant)):
        try:
            await check()
            dependencies[name] = "connected"
        except Exception as exc:
            logger.warning("%s readiness check failed: %s", name, exc)
            dependencies[name] = "unavailable"

    ready = all(value == "connected" for value in dependencies.values())
    payload = {
        "status": "ready" if ready else "degraded",
        "service": "shareo-ai",
        "dependencies": dependencies,
    }
    return JSONResponse(payload, status_code=200 if ready else 503)


def require_internal_token(token: str | None) -> None:
    if not settings.internal_token:
        raise HTTPException(status_code=503, detail="internal token is not configured")
    if token != settings.internal_token:
        raise HTTPException(status_code=401, detail="unauthorized")


@app.get("/readyz/image-search")
async def image_search_readyz(x_internal_token: str | None = Header(default=None)):
    require_internal_token(x_internal_token)
    runtime = getattr(app.state, "worker_runtime", None)
    consumers = runtime.status() if runtime is not None else {}
    if consumers.get(STREAM_INDEX_POST) != "running":
        return JSONResponse(
            {
                "status": "degraded",
                "model": embedder.metadata(),
                "consumer": consumers.get(STREAM_INDEX_POST, "stopped"),
            },
            status_code=503,
        )
    if not embedder.loaded:
        return JSONResponse(
            {
                "status": "warming" if embedder.state != "failed" else "failed",
                "model": embedder.metadata(),
            },
            status_code=503,
        )
    try:
        await vector_store.ensure_collection()
        return {
            "status": "ready",
            "model": embedder.metadata(),
            "consumer": consumers[STREAM_INDEX_POST],
            "vector_store": await vector_store.metadata(),
        }
    except Exception as exc:
        logger.warning("image search readiness failed: %s", exc)
        return JSONResponse(
            {"status": "degraded", "model": embedder.metadata(), "vector_store": "unavailable"},
            status_code=503,
        )


@app.get("/v1/meta/image-search")
async def image_search_meta(x_internal_token: str | None = Header(default=None)):
    require_internal_token(x_internal_token)
    vector_meta: dict | str
    try:
        vector_meta = await vector_store.metadata()
    except Exception:
        vector_meta = "unavailable"
    return {"model": embedder.metadata(), "vector_store": vector_meta}


@app.get("/readyz/rag")
async def rag_readyz(x_internal_token: str | None = Header(default=None)):
    require_internal_token(x_internal_token)
    runtime = getattr(app.state, "worker_runtime", None)
    consumers = runtime.status() if runtime is not None else {}
    status = {
        "model": text_embedder.metadata(),
        "consumer": consumers.get(STREAM_INDEX_POST, "stopped"),
        "llm": "configured" if llm_provider.configured else "missing_configuration",
        "llm_model": settings.llm_model,
        "text_embedding_model": settings.text_embedding_model,
        "text_embedding_revision": os.environ.get(
            "SHAREO_AI_TEXT_EMBEDDING_REVISION", "unresolved"
        ),
    }
    if not text_embedder.loaded or status["consumer"] != "running" or not llm_provider.configured:
        return JSONResponse({"status": "degraded", **status}, status_code=503)
    try:
        return {
            "status": "ready",
            **status,
            "vector_store": await text_vector_store.metadata(),
            "prompt_version": settings.rag_prompt_version,
        }
    except Exception as exc:
        logger.warning("rag readiness failed: %s", exc)
        return JSONResponse(
            {"status": "degraded", **status, "vector_store": "unavailable"},
            status_code=503,
        )


@app.get("/readyz/agent")
async def agent_readyz(x_internal_token: str | None = Header(default=None)):
    require_internal_token(x_internal_token)
    runtime = getattr(app.state, "worker_runtime", None)
    consumers = runtime.status() if runtime is not None else {}
    status = {
        "enabled": settings.agent_enabled,
        "llm": "configured" if llm_provider.configured else "missing_configuration",
        "llm_model": settings.llm_model,
        "model": text_embedder.metadata(),
        "image_model": embedder.metadata(),
        "text_embedding_model": settings.text_embedding_model,
        "text_embedding_revision": os.environ.get(
            "SHAREO_AI_TEXT_EMBEDDING_REVISION", "unresolved"
        ),
        "rag_consumer": consumers.get(STREAM_INDEX_POST, "stopped"),
        "prompt_version": settings.agent_prompt_version,
        "trace_version": settings.agent_trace_version,
        "source_fingerprint": PROCESS_SOURCE_FINGERPRINT,
        "process_started_at": PROCESS_STARTED_AT,
    }
    if not settings.agent_enabled or not llm_provider.configured:
        return JSONResponse({"status": "degraded", **status}, status_code=503)
    try:
        vector_meta = await text_vector_store.metadata()
        return {"status": "ready", **status, "vector_store": vector_meta}
    except Exception as exc:
        logger.warning("agent readiness failed: %s", exc)
        return JSONResponse(
            {"status": "degraded", **status, "vector_store": "unavailable"},
            status_code=503,
        )


@app.post("/v1/rag/answer")
async def answer_rag(
    payload: RAGAnswerRequest, x_internal_token: str | None = Header(default=None)
):
    require_internal_token(x_internal_token)
    if not llm_provider.configured:
        raise HTTPException(status_code=503, detail="rag unavailable: configuration")
    try:
        result = await rag_pipeline.answer(
            payload.question,
            payload.top_k,
            [item.model_dump() for item in payload.history],
        )
        return {
            "answer": result.answer,
            "citations": [
                {
                    "post_id": item.post_id,
                    "chunk_id": item.chunk_id,
                    "excerpt": item.excerpt,
                    "score": item.score,
                }
                for item in result.citations
            ],
        }
    except LLMError as exc:
        logger.warning("rag provider failed category=%s error=%s", exc.category, exc)
        raise HTTPException(status_code=503, detail=f"rag unavailable: {exc.category}") from exc
    except Exception as exc:
        logger.exception("rag answer failed: %s", exc)
        raise HTTPException(status_code=503, detail="rag unavailable") from exc


@app.post("/v1/search/images")
async def search_images(
    payload: ImageSearchRequest, x_internal_token: str | None = Header(default=None)
):
    require_internal_token(x_internal_token)
    if embedder.state == "loading":
        raise HTTPException(status_code=503, detail="image model is warming up")
    if embedder.state == "failed":
        raise HTTPException(status_code=503, detail="image model failed to load")
    started = time.perf_counter()
    try:
        semaphore = getattr(app.state, "embedding_semaphore", None)
        if semaphore is None:
            semaphore = asyncio.Semaphore(max(1, settings.embedding_concurrency))
        async with semaphore:
            vector = await asyncio.to_thread(embedder.encode_text, payload.query)
        requested = min(payload.limit, settings.search_max_limit)
        candidates = min(
            requested * max(1, settings.search_candidate_multiplier),
            settings.search_max_candidates,
        )
        results = await vector_store.search(vector, candidates)
        logger.info(
            "image search complete query_length=%d requested=%d candidates=%d results=%d duration_ms=%.1f",
            len(payload.query),
            requested,
            candidates,
            len(results),
            (time.perf_counter() - started) * 1000,
        )
        return {"results": results}
    except HTTPException:
        raise
    except Exception as exc:
        logger.exception("image search failed: %s", exc)
        raise HTTPException(status_code=503, detail="image search unavailable") from exc


@app.post("/v1/search/posts")
async def search_posts(
    payload: PostSearchRequest, x_internal_token: str | None = Header(default=None)
):
    require_internal_token(x_internal_token)
    if text_embedder.state == "loading":
        raise HTTPException(status_code=503, detail="text model is warming up")
    if text_embedder.state == "failed":
        raise HTTPException(status_code=503, detail="text model failed to load")
    started = time.perf_counter()
    try:
        semaphore = getattr(app.state, "embedding_semaphore", None)
        if semaphore is None:
            semaphore = asyncio.Semaphore(max(1, settings.embedding_concurrency))
        async with semaphore:
            vector = await asyncio.to_thread(text_embedder.embed_query, payload.query)
        results = await text_vector_store.search(vector, payload.limit)
        logger.info(
            "post semantic search complete query_length=%d requested=%d results=%d duration_ms=%.1f",
            len(payload.query),
            payload.limit,
            len(results),
            (time.perf_counter() - started) * 1000,
        )
        return {"results": results}
    except HTTPException:
        raise
    except Exception as exc:
        logger.exception("post semantic search failed: %s", exc)
        raise HTTPException(status_code=503, detail="post search unavailable") from exc
