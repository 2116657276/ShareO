from app.core.embedding import ImageEmbedder


def test_image_embedding_is_normalized_512_dimensions():
    import torch
    from PIL import Image

    class Processor:
        def __call__(self, **_kwargs):
            return {"pixel_values": torch.ones((1, 3, 2, 2))}

    class Model:
        def get_image_features(self, **_kwargs):
            return torch.ones((1, 512))

    embedder = ImageEmbedder(device="cpu")
    embedder._processor = Processor()
    embedder._model = Model()
    vectors = embedder.encode_images([Image.new("RGB", (2, 2))])
    assert len(vectors) == 1
    assert len(vectors[0]) == 512
    assert abs(sum(value * value for value in vectors[0]) - 1.0) < 1e-5


def test_explicit_cpu_device_is_supported():
    embedder = ImageEmbedder(device="cpu")
    assert embedder.device == "cpu"


def test_embedder_is_lazy():
    embedder = ImageEmbedder(device="cpu")
    assert embedder._model is None
    assert embedder._processor is None


def test_warmup_pins_revision_and_cache():
    calls = []

    class ProcessorFactory:
        @staticmethod
        def from_pretrained(model, **kwargs):
            calls.append(("processor", model, kwargs))
            return object()

    class Model:
        def to(self, _device):
            return self

        def eval(self):
            return self

    class ModelFactory:
        @staticmethod
        def from_pretrained(model, **kwargs):
            calls.append(("model", model, kwargs))
            return Model()

    embedder = ImageEmbedder(
        device="cpu",
        revision="fixed-rev",
        cache_dir="/tmp/models",
        processor_factory=ProcessorFactory,
        model_factory=ModelFactory,
    )
    embedder.warmup()
    assert embedder.loaded
    assert embedder.metadata()["revision"] == "fixed-rev"
    assert all(call[2] == {"revision": "fixed-rev", "cache_dir": "/tmp/models"} for call in calls)
