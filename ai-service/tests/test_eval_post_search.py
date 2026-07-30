import pytest

from app.commands.eval_post_search import ndcg_at, precision_at, reciprocal_rank, recall_at


def test_post_search_metrics_support_graded_relevance():
    ids = [2, 1, 9]
    relevance = {1: 2, 2: 1}
    assert recall_at(ids, set(relevance), 5) == 1
    assert reciprocal_rank(ids, set(relevance)) == 1
    assert precision_at(ids, set(relevance), 3) == pytest.approx(2 / 3)
    assert 0 < ndcg_at(ids, relevance, 5) < 1


def test_post_search_no_match_metrics_require_empty_results():
    assert recall_at([], set(), 5) == 1
    assert reciprocal_rank([], set()) == 1
    assert precision_at([], set(), 3) == 1
    assert ndcg_at([], {}, 5) == 1
    assert recall_at([1], set(), 5) == 0
