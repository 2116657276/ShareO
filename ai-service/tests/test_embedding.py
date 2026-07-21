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
