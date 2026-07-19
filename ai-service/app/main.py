import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

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
    """Health check endpoint — reports service and dependency status."""
    status = {"status": "ok", "service": "shareo-ai"}

    # Check Redis connectivity
    try:
        import redis.asyncio as aioredis
        r = aioredis.from_url(settings.redis_url)
        await r.ping()
        await r.close()
        status["redis"] = "connected"
    except Exception as e:
        status["redis"] = f"unavailable: {e}"

    # Check Qdrant connectivity
    try:
        from qdrant_client import QdrantClient
        c = QdrantClient(url=settings.qdrant_url)
        c.get_collections()
        c.close()
        status["qdrant"] = "connected"
    except Exception as e:
        status["qdrant"] = f"unavailable: {e}"

    return JSONResponse(status)
