from app.commands.reconcile_index import calculate_diff


def test_reconcile_detects_missing_stale_and_old_revision():
    desired = {1: {11, 12}, 2: {21}, 3: {31}}
    inventory = [
        {"post_id": 1, "image_id": 11, "model_revision": "rev-a"},
        {"post_id": 2, "image_id": 21, "model_revision": "old"},
        {"post_id": 4, "image_id": 41, "model_revision": "rev-a"},
    ]
    repair, stale = calculate_diff(desired, inventory, "rev-a")
    assert repair == [1, 2, 3]
    assert stale == [4]
