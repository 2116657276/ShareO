"""Lazy Chinese-CLIP image/text embedding with CPU/MPS/CUDA selection."""

from collections.abc import Sequence
from threading import Lock
from typing import Any

from PIL import Image

from app.config import settings


class ImageEmbedder:
    def __init__(
        self,
        model_name: str | None = None,
        device: str | None = None,
        revision: str | None = None,
        cache_dir: str | None = None,
        processor_factory: Any = None,
        model_factory: Any = None,
    ):
        self.model_name = model_name or settings.embedding_model
        self.requested_device = device or settings.embedding_device
        self.revision = revision if revision is not None else settings.embedding_revision
        self.cache_dir = cache_dir if cache_dir is not None else settings.model_cache_dir
        self._processor_factory = processor_factory
        self._model_factory = model_factory
        self._device: Any = None
        self._processor: Any = None
        self._model: Any = None
        self._state = "idle"
        self._error = ""
        self._load_lock = Lock()

    @property
    def state(self) -> str:
        return self._state

    @property
    def loaded(self) -> bool:
        return self._model is not None and self._state == "ready"

    @property
    def error(self) -> str:
        return self._error

    @property
    def device(self) -> str:
        if self._device is None:
            self._device = self._resolve_device()
        return str(self._device)

    def _resolve_device(self) -> Any:
        import torch

        if self.requested_device != "auto":
            return torch.device(self.requested_device)
        if torch.cuda.is_available():
            return torch.device("cuda")
        if hasattr(torch.backends, "mps") and torch.backends.mps.is_available():
            return torch.device("mps")
        return torch.device("cpu")

    def _load(self) -> None:
        if self._model is not None:
            return
        with self._load_lock:
            if self._model is not None:
                return
            self._state = "loading"
            self._error = ""
            try:
                if self._processor_factory is None or self._model_factory is None:
                    import transformers

                    processor_factory = transformers.ChineseCLIPProcessor
                    model_factory = transformers.ChineseCLIPModel
                else:
                    processor_factory = self._processor_factory
                    model_factory = self._model_factory

                load_options: dict[str, str] = {}
                if self.revision:
                    load_options["revision"] = self.revision
                if self.cache_dir:
                    load_options["cache_dir"] = self.cache_dir
                processor = processor_factory.from_pretrained(self.model_name, **load_options)
                model = model_factory.from_pretrained(self.model_name, **load_options)
                model.to(self.device)
                model.eval()
                self._processor = processor
                self._model = model
                self._state = "ready"
            except Exception as exc:
                self._state = "failed"
                self._error = str(exc)
                raise

    def warmup(self) -> None:
        self._load()

    def metadata(self) -> dict[str, Any]:
        return {
            "model": self.model_name,
            "revision": self.revision,
            "device": self.device,
            "dimension": 512,
            "state": self.state,
            "error": self.error,
        }

    @staticmethod
    def _normalize(values: Any) -> Any:
        import torch

        return values / torch.linalg.vector_norm(values, dim=-1, keepdim=True).clamp_min(1e-12)

    def encode_images(self, images: Sequence[Image.Image]) -> list[list[float]]:
        if not images:
            return []
        import torch

        self._load()
        inputs = self._processor(images=list(images), return_tensors="pt", padding=True)
        inputs = {key: value.to(self.device) for key, value in inputs.items()}
        with torch.inference_mode():
            vectors = self._normalize(self._model.get_image_features(**inputs))
        return vectors.detach().cpu().tolist()

    def encode_text(self, query: str) -> list[float]:
        import torch

        self._load()
        inputs = self._processor(text=[query], return_tensors="pt", padding=True)
        inputs = {key: value.to(self.device) for key, value in inputs.items()}
        with torch.inference_mode():
            vector = self._normalize(self._model.get_text_features(**inputs))[0]
        return vector.detach().cpu().tolist()
