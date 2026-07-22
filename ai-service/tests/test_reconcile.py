from app.commands.reconcile_index import calculate_diff, calculate_text_diff


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


def test_text_reconcile_detects_chunk_and_model_drift():
    desired = {1: {"1:0", "1:1"}, 2: {"2:0"}}
    inventory = [
        {"post_id": 1, "chunk_id": "1:0", "model_name": "bge"},
        {"post_id": 2, "chunk_id": "2:0", "model_name": "old"},
        {"post_id": 3, "chunk_id": "3:0", "model_name": "bge"},
    ]
    repair, stale = calculate_text_diff(desired, inventory, "bge")
    assert repair == [1, 2]
    assert stale == [3]
