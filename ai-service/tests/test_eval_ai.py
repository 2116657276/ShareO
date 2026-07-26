from __future__ import annotations

import pytest

from app.commands import eval_ai


class DummyClient:
    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        return False


def test_human_scores_accept_json_and_markdown(tmp_path):
    json_path = tmp_path / "scores.json"
    json_path.write_text('{"scores": [4, 5]}', encoding="utf-8")
    assert eval_ai._load_human_scores(json_path, 2) == ([4, 5], 4.5)

    markdown_path = tmp_path / "scores.md"
    markdown_path.write_text(
        "| # | Question | Score |\n"
        "|---|---|---|\n"
        "| 1 | 含有 \\| 的问题 | 3 |\n"
        "| 2 | 第二个问题 | 5 |\n",
        encoding="utf-8",
    )
    assert eval_ai._load_human_scores(markdown_path, 2) == ([3, 5], 4.0)


def test_human_scores_reject_missing_or_out_of_range(tmp_path):
    path = tmp_path / "scores.json"
    path.write_text('{"scores": [4]}', encoding="utf-8")
    with pytest.raises(ValueError, match="expected 2 human scores"):
        eval_ai._load_human_scores(path, 2)

    path.write_text('{"scores": [4, 6]}', encoding="utf-8")
    with pytest.raises(ValueError, match="1 to 5"):
        eval_ai._load_human_scores(path, 2)


def test_scoring_template_contains_expected_and_actual_citations(tmp_path):
    path = tmp_path / "scoring.md"
    eval_ai._write_scoring_template(
        path,
        [
            {
                "question": "问题",
                "expected_answer": "参考答案",
                "expected_source_post_ids": [7, 8],
                "bot_answer": "回答",
                "citation_post_ids": [8],
                "citation_details": [{"post_id": 8, "chunk_id": "8:0"}],
                "source_hit_rate": 1.0,
                "relevance_score": "",
            }
        ],
        {"date": "2026-07-26", "git_sha": "abc1234"},
    )
    content = path.read_text(encoding="utf-8")
    assert "Expected Source IDs" in content
    assert "[7, 8]" in content
    assert "[8]" in content
    assert '"chunk_id": "8:0"' in content


def test_quality_gates_require_full_results_and_human_score():
    image_report = {
        "queries": 40,
        "semantic": {"recall_5": 0.8, "mrr": 0.6},
        "semantic_duplicate_queries": 0,
    }
    rag_metrics = {
        "total_queries": 30,
        "skipped": 0,
        "failed_queries": 0,
        "source_hit_rate": 0.9,
        "citation_accessible_rate": 1.0,
        "total_hallucinated": 0,
    }
    pending = eval_ai.check_quality_gates(image_report, rag_metrics)
    assert pending["human_relevance_avg >= 4.0"] == "PENDING"

    passed = eval_ai.check_quality_gates(image_report, rag_metrics, 4.0)
    assert all(value is True for value in passed.values())

    failed = eval_ai.check_quality_gates(image_report, {"error": "request failed"}, 5.0)
    assert failed["source_hit_rate >= 0.80"] is False
    assert failed["hallucinated citations 0"] is False


def test_evaluate_rag_counts_failed_and_no_answer_queries(monkeypatch):
    monkeypatch.setattr(eval_ai.httpx, "Client", lambda **kwargs: DummyClient())
    monkeypatch.setattr(eval_ai, "_get_user_token", lambda client, username, password: "token")
    monkeypatch.setattr(eval_ai, "_find_bot_user", lambda client, token: 1)
    monkeypatch.setattr(eval_ai, "_ensure_conversation", lambda client, token, bot_id: 2)

    def send_message(client, token, conv_id, content):
        if content == "失败":
            return None
        return 10 if content == "无答案" else 11

    def wait_for_reply(client, token, conv_id, source_message_id, timeout=120):
        if source_message_id == 10:
            return {
                "content": "不确定，未找到足够的相关内容。",
                "meta": {"citations": []},
            }
        return {
            "content": "答案",
            "meta": {"citations": [{"post_id": 7, "chunk_id": "7:0"}]},
        }

    monkeypatch.setattr(eval_ai, "_send_message", send_message)
    monkeypatch.setattr(eval_ai, "_wait_for_bot_reply", wait_for_reply)
    monkeypatch.setattr(eval_ai, "_verify_post_accessible", lambda client, post_id: True)
    monkeypatch.setattr(eval_ai, "_verify_post_approved", lambda client, post_id: True)

    result = eval_ai.evaluate_rag(
        "http://go.test",
        [
            {
                "question": "正常",
                "category": "factual",
                "expected_source_post_ids": [7],
                "reference_answer": "答案",
            },
            {
                "question": "无答案",
                "category": "no-answer",
                "expected_source_post_ids": [],
                "expect_no_answer": True,
            },
            {
                "question": "失败",
                "category": "factual",
                "expected_source_post_ids": [7],
                "reference_answer": "答案",
            },
        ],
    )

    report = result["report"]
    assert report["total_queries"] == 3
    assert report["failed_queries"] == 1
    assert report["no_answer_accuracy"] == 1.0
    assert report["source_hit_rate"] == 0.6667
    assert report["citation_accessible_rate"] == 0.6667
