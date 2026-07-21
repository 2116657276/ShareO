"""Lazy Chinese-CLIP image/text embedding with CPU/MPS/CUDA selection."""

from collections.abc import Sequence
from typing import Any

from PIL import Image

from app.config import settings


class ImageEmbedder:
    def __init__(self, model_name: str | None = None, device: str | None = None):
        self.model_name = model_name or settings.embedding_model
        self.requested_device = device or settings.embedding_device
        self._device: Any = None
        self._processor: Any = None
        self._model: Any = None

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
        from transformers import ChineseCLIPModel, ChineseCLIPProcessor

        self._processor = ChineseCLIPProcessor.from_pretrained(self.model_name)
        self._model = ChineseCLIPModel.from_pretrained(self.model_name)
        self._model.to(self.device)
        self._model.eval()

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
