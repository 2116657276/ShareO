import asyncio
import logging
import time
from contextlib import asynccontextmanager

from fastapi import FastAPI, Header, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field, field_validator
from qdrant_client import AsyncQdrantClient
from redis.asyncio import Redis

from app.config import settings
from app.core.embedding import ImageEmbedder
from app.core.vectorstore import ImageVectorStore

logger = logging.getLogger(__name__)
embedder = ImageEmbedder()
vector_store = ImageVectorStore(settings.qdrant_url, settings.image_collection)


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


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Startup and shutdown lifecycle for ai-service."""
    logger.setLevel(settings.log_level)
    logger.info("ai-service starting (log_level=%s)", settings.log_level)
    app.state.embedding_semaphore = asyncio.Semaphore(max(1, settings.embedding_concurrency))
    try:
        await vector_store.ensure_collection()
    except Exception as exc:
        logger.warning("image collection startup check failed: %s", exc)
    if settings.embedding_warmup:
        app.state.warmup_task = asyncio.create_task(asyncio.to_thread(embedder.warmup))

        def warmup_done(task: asyncio.Task) -> None:
            try:
                task.result()
                logger.info(
                    "image model warmup complete model=%s revision=%s device=%s",
                    embedder.model_name,
                    embedder.revision,
                    embedder.device,
                )
            except asyncio.CancelledError:
                logger.info("image model warmup cancelled")
            except Exception as exc:
                logger.error("image model warmup failed: %s", exc)

        app.state.warmup_task.add_done_callback(warmup_done)
    yield
    warmup_task = getattr(app.state, "warmup_task", None)
    if warmup_task is not None and not warmup_task.done():
        warmup_task.cancel()
    await vector_store.close()
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


@app.get("/readyz/search")
async def search_readyz(x_internal_token: str | None = Header(default=None)):
    require_internal_token(x_internal_token)
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
