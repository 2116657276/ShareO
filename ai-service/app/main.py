import asyncio
import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI, Header, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field
from qdrant_client import AsyncQdrantClient
from redis.asyncio import Redis

from app.config import settings
from app.core.embedding import ImageEmbedder
from app.core.vectorstore import ImageVectorStore

logger = logging.getLogger(__name__)
embedder = ImageEmbedder()
vector_store = ImageVectorStore(settings.qdrant_url)


class ImageSearchRequest(BaseModel):
    query: str = Field(min_length=1, max_length=200)
    limit: int = Field(default=10, ge=1, le=20)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Startup and shutdown lifecycle for ai-service."""
    logger.setLevel(settings.log_level)
    logger.info("ai-service starting (log_level=%s)", settings.log_level)
    yield
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


@app.post("/v1/search/images")
async def search_images(
    payload: ImageSearchRequest, x_internal_token: str | None = Header(default=None)
):
    require_internal_token(x_internal_token)
    try:
        vector = await asyncio.to_thread(embedder.encode_text, payload.query.strip())
        results = await vector_store.search(vector, min(payload.limit, settings.search_max_limit))
        return {"results": results}
    except HTTPException:
        raise
    except Exception as exc:
        logger.exception("image search failed: %s", exc)
        raise HTTPException(status_code=503, detail="image search unavailable") from exc
