from app.commands.eval_image_search import recall_at, reciprocal_rank


def test_recall_and_reciprocal_rank():
    ids = [9, 3, 5, 7]
    relevant = {3, 7}
    assert recall_at(ids, relevant, 2) == 0.5
    assert recall_at(ids, relevant, 10) == 1.0
    assert reciprocal_rank(ids, relevant) == 0.5
