import json

import httpx
import pytest

from app.commands.eval_image_search import (
    evaluate,
    ndcg_at,
    precision_at,
    relevance_map,
)


def test_image_metrics_support_graded_labels():
    item = {"relevance": {"1": 2, "2": 1}}
    relevance = relevance_map(item)
    assert relevance == {1: 2, 2: 1}
    assert precision_at([1, 9, 2], set(relevance), 3) == pytest.approx(2 / 3)
    assert ndcg_at([2, 1], relevance, 5) < 1


def test_image_metrics_keep_legacy_binary_labels():
    assert relevance_map({"relevant_post_ids": [3, 4]}) == {3: 2, 4: 2}


def test_image_evaluation_is_semantic_only_and_can_apply_threshold(monkeypatch):
    requests = []

    def handler(request: httpx.Request) -> httpx.Response:
        requests.append(request)
        return httpx.Response(
            200,
            json={
                "data": {
                    "results": [
                        {"post_id": 1, "score": 0.61},
                        {"post_id": 2, "score": 0.49},
                    ]
                }
            },
        )

    real_client = httpx.Client

    def client_factory(*args, **kwargs):
        kwargs["transport"] = httpx.MockTransport(handler)
        return real_client(*args, **kwargs)

    monkeypatch.setattr(httpx, "Client", client_factory)
    report = evaluate(
        "http://shareo.test",
        [
            {
                "id": "image-1",
                "query": "湖边日落",
                "relevance": {"1": 2},
                "expect_no_results": False,
            }
        ],
        score_threshold=0.5,
    )

    assert len(requests) == 1
    assert requests[0].url.path == "/api/v1/search/images"
    assert report["semantic"]["mrr"] == 1
    assert report["query_results"][0]["actual_post_ids"] == [1]
    json.dumps(report)
