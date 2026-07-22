import numpy as np

from app.rag.embedding import TextEmbedder


class FakeModel:
    def passage_embed(self, documents):
        return iter([np.ones(512, dtype=np.float32) for _ in documents])

    def query_embed(self, questions):
        return iter([np.ones(512, dtype=np.float32) for _ in questions])


class FakeFactory:
    calls = []

    def __new__(cls, **kwargs):
        cls.calls.append(kwargs)
        return FakeModel()


def test_fastembed_wrapper_is_lazy_normalized_and_configured():
    FakeFactory.calls.clear()
    embedder = TextEmbedder(
        model_name="BAAI/bge-small-zh-v1.5",
        cache_dir="/tmp/text-models",
        model_factory=FakeFactory,
    )
    assert not embedder.loaded
    query = embedder.embed_query("怎么拍夜景？")
    documents = embedder.embed_documents(["使用三脚架。"])
    assert embedder.loaded
    assert FakeFactory.calls == [
        {"model_name": "BAAI/bge-small-zh-v1.5", "cache_dir": "/tmp/text-models"}
    ]
    assert len(query) == 512
    assert abs(np.linalg.norm(query) - 1.0) < 1e-5
    assert abs(np.linalg.norm(documents[0]) - 1.0) < 1e-5
