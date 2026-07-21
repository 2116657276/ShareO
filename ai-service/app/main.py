import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.responses import JSONResponse
from qdrant_client import AsyncQdrantClient
from redis.asyncio import Redis

from app.config import settings

logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Startup and shutdown lifecycle for ai-service."""
    logger.setLevel(settings.log_level)
    logger.info("ai-service starting (log_level=%s)", settings.log_level)
    yield
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
    client = AsyncQdrantClient(url=settings.qdrant_url)
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
