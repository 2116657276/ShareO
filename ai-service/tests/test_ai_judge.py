from __future__ import annotations

import json

import pytest

from app.commands import ai_judge


def _score(score: int = 4, confidence: float = 0.9) -> dict:
    return {
        "factual_accuracy": score,
        "completeness": score,
        "citation_support": score,
        "safety": score,
        "score": score,
        "confidence": confidence,
        "reason": "符合输入中的参考答案和机器审计。",
    }


def test_parse_judge_payload_validates_schema():
    payload = json.dumps({"scores": [{"id": "rag-001", **_score()}]}, ensure_ascii=False)
    parsed = ai_judge._parse_payload(payload, {"rag-001"})
    assert parsed["rag-001"]["score"] == 4


def test_parse_judge_payload_rejects_invalid_score():
    payload = json.dumps({"scores": [{"id": "rag-001", **_score(6)}]}, ensure_ascii=False)
    with pytest.raises(ValueError, match="between 1 and 5"):
        ai_judge._parse_payload(payload, {"rag-001"})


def test_parse_support_payload_validates_schema():
    payload = json.dumps({"support": [{"id": "rag-001", "supported": True, "confidence": 0.9}]})
    parsed = ai_judge._parse_support_payload(payload, {"rag-001"})
    assert parsed["rag-001"]["supported"] is True


def test_finalize_dataset_support_requires_two_consistent_passes():
    rows = [{"id": "rag-001", "expected_source_post_ids": [1]}]
    prepared = [{"id": "rag-001", "sources": [{"post_id": "1"}]}]
    result = ai_judge._finalize_dataset_support(
        rows,
        prepared,
        [
            {"rag-001": {"supported": True, "confidence": 0.9, "reason": "ok"}},
            {"rag-001": {"supported": False, "confidence": 0.9, "reason": "no"}},
        ],
        kind="rag",
        model="judge-model",
        timing_ms=[10.0, 12.0],
    )
    assert result["status"] == "needs_human_review"
    assert result["human_fallback_count"] == 1
    assert result["quality_gate_passed"] is False


def test_deterministic_audit_marks_missing_sources_and_agent_safety():
    audit = ai_judge.deterministic_audit(
        {
            "expected_source_post_ids": [1, 2],
            "citation_post_ids": [1],
            "actual_tools": ["read_posts", "delete_post"],
            "forbidden_tools": ["delete_post"],
            "injection_leaked": True,
            "machine_status": "completed",
        },
        "agent",
    )
    assert "missing_expected_sources" in audit["flags"]
    assert "forbidden_tool" in audit["hard_failures"]
    assert "injection_leak" in audit["hard_failures"]


def test_finalize_uses_conservative_lower_score_and_human_fallback():
    rows = [{"id": "agent-001", "expected_source_post_ids": [1], "citation_post_ids": [1]}]
    prepared = [ai_judge._judge_input(rows[0], 1, "agent")]
    audits = {prepared[0]["id"]: prepared[0]["machine_audit"]}
    result = ai_judge._finalize(
        prepared,
        [{"agent-001": _score(5)}, {"agent-001": _score(3)}],
        audits,
        kind="agent",
        model="judge-model",
        timing_ms=[10.0, 12.0],
    )
    assert result["rows"][0]["score"] == 3
    assert result["status"] == "needs_human_review"
    assert result["human_fallback_count"] == 1
    assert result["quality_gate_passed"] is False


def test_judge_rows_blocks_without_provider():
    result = ai_judge.judge_rows(
        [{"question": "问题", "expected_source_post_ids": [1], "citation_post_ids": [1]}],
        kind="rag",
        base_url="",
        api_key="",
        model="",
    )
    assert result["status"] == "blocked"
    assert result["quality_gate_passed"] is False


def test_judge_reports_attaches_judgements(tmp_path, monkeypatch):
    rag = tmp_path / "rag.json"
    agent = tmp_path / "agent.json"
    rag.write_text(
        json.dumps({"rag": {"evaluation_rows": [{"question": "问题"}]}}),
        encoding="utf-8",
    )
    agent.write_text(
        json.dumps({"agent": {"evaluation_rows": [{"id": "agent-001"}]}}),
        encoding="utf-8",
    )

    def fake_judge(rows, *, kind, **kwargs):
        return {
            "method": "llm_judge",
            "status": "complete",
            "kind": kind,
            "quality_gate_passed": True,
            "rows": [{"question": rows[0].get("question", "")}],
        }

    monkeypatch.setattr(ai_judge, "judge_rows", fake_judge)
    result = ai_judge.judge_reports(rag, agent)
    assert result["quality_gate_passed"] is True
    rag_payload = json.loads(rag.read_text(encoding="utf-8"))
    assert rag_payload["quality_judgement"]["kind"] == "rag"
    assert rag_payload["quality_judgement"]["rows"][0]["question"] == "问题"
    assert json.loads(agent.read_text(encoding="utf-8"))["quality_judgement"]["kind"] == "agent"
