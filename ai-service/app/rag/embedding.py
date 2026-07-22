"""Lazy FastEmbed wrapper for Chinese passage and query vectors."""

import logging
import time
from threading import Lock
from typing import Any

import numpy as np

from app.config import settings

logger = logging.getLogger(__name__)


class TextEmbedder:
    dimension = 512

    def __init__(
        self,
        model_name: str | None = None,
        cache_dir: str | None = None,
        model_factory: Any = None,
    ) -> None:
        self.model_name = model_name or settings.text_embedding_model
        configured_cache = settings.text_model_cache_dir or settings.model_cache_dir
        self.cache_dir = cache_dir if cache_dir is not None else configured_cache
        self._model_factory = model_factory
        self._model: Any = None
        self._state = "idle"
        self._error = ""
        self._load_lock = Lock()

    @property
    def loaded(self) -> bool:
        return self._model is not None and self._state == "ready"

    @property
    def state(self) -> str:
        return self._state

    def _load(self) -> None:
        if self._model is not None:
            return
        with self._load_lock:
            if self._model is not None:
                return
            self._state = "loading"
            self._error = ""
            started = time.perf_counter()
            try:
                if self._model_factory is None:
                    from fastembed import TextEmbedding

                    factory = TextEmbedding
                else:
                    factory = self._model_factory
                options = {"model_name": self.model_name}
                if self.cache_dir:
                    options["cache_dir"] = self.cache_dir
                self._model = factory(**options)
                self._state = "ready"
                logger.info(
                    "text model load complete model=%s duration_ms=%.1f",
                    self.model_name,
                    (time.perf_counter() - started) * 1000,
                )
            except Exception as exc:
                self._state = "failed"
                self._error = str(exc)
                logger.exception(
                    "text model load failed model=%s duration_ms=%.1f",
                    self.model_name,
                    (time.perf_counter() - started) * 1000,
                )
                raise

    @staticmethod
    def _normalized(vector: Any) -> list[float]:
        values = np.asarray(vector, dtype=np.float32)
        norm = float(np.linalg.norm(values))
        if norm <= 1e-12:
            raise ValueError("embedding vector has zero norm")
        return (values / norm).tolist()

    def warmup(self) -> None:
        self._load()

    def embed_documents(self, documents: list[str]) -> list[list[float]]:
        if not documents:
            return []
        started = time.perf_counter()
        self._load()
        vectors = [self._normalized(item) for item in self._model.passage_embed(documents)]
        logger.info(
            "text passage encode complete count=%d duration_ms=%.1f",
            len(documents),
            (time.perf_counter() - started) * 1000,
        )
        return vectors

    def embed_query(self, question: str) -> list[float]:
        started = time.perf_counter()
        self._load()
        vector = next(iter(self._model.query_embed([question])))
        result = self._normalized(vector)
        logger.info(
            "text query encode complete length=%d duration_ms=%.1f",
            len(question),
            (time.perf_counter() - started) * 1000,
        )
        return result

    def metadata(self) -> dict[str, Any]:
        return {
            "model": self.model_name,
            "dimension": self.dimension,
            "state": self.state,
            "error": self._error,
        }
