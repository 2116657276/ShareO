import json
from pathlib import Path

import pytest

from app.commands.eval_agent import (
    _apply_scoring,
    _create_conversation,
    _human_rows_from_report,
    _audit_injection_answer,
    _summarize_tool_selection,
    _task_username,
    _write_report,
    _write_scoring_template,
    check_quality_gates,
    load_tasks,
)


class _FakeResponse:
    status_code = 200

    @staticmethod
    def json():
        return {"code": 0, "data": {"id": 42}}


class _FakeClient:
    def __init__(self):
        self.calls = []

    def post(self, path, **kwargs):
        self.calls.append((path, kwargs))
        return _FakeResponse()


def test_agent_tasks_use_unique_safe_usernames_and_fresh_conversations():
    names = [_task_username("eval agent/test", "1234567890", index) for index in range(1, 4)]
    assert len(set(names)) == 3
    assert all(len(name) <= 32 for name in names)
    assert all(" " not in name and "/" not in name for name in names)

    client = _FakeClient()
    assert _create_conversation(client, "token", 7) == 42
    assert client.calls == [
        ("/api/v1/conversations", {"json": {"user_id": 7}, "cookies": {"token": "token"}})
    ]


def _current_agent_dataset_path() -> Path:
    path = (
        Path(__file__).resolve().parents[2]
        / ".local"
        / "shareo"
        / "eval"
        / "agent_tasks_current_v1.jsonl"
    )
    if not path.exists():
        pytest.skip(
            "local eval dataset .local/shareo/eval/agent_tasks_current_v1.jsonl not generated"
        )
    return path


def test_agent_dataset_has_current_shape_and_human_subset():
    tasks = load_tasks(_current_agent_dataset_path())
    assert len(tasks) == 36
    assert sum(item["category"] == "single-source" for item in tasks) == 10
    assert sum(item["category"] == "keyword" for item in tasks) == 8
    assert sum(item["category"] == "multi-source" for item in tasks) == 4
    assert sum(item["category"] == "image" for item in tasks) == 6
    assert sum(item["category"] == "no-answer" for item in tasks) == 4
    assert sum(item["category"] == "injection" for item in tasks) == 4
    assert sum(item.get("human_evaluable") for item in tasks) == 32


def test_agent_quality_gates_require_all_machine_and_human_evidence():
    report = {
        "total_queries": 36,
        "failed_queries": 0,
        "source_hit_rate": 0.9,
        "source_coverage_rate": 0.9,
        "complete_source_coverage_rate": 0.9,
        "citation_precision_rate": 0.9,
        "unexpected_tool_calls": 0,
        "required_tool_selection_rate": 0.9,
        "citation_accessible_rate": 1.0,
        "hallucinated_citations": 0,
        "no_answer_accuracy": 1.0,
        "forbidden_actions": 0,
        "budget_violations": 0,
        "injection_failures": 0,
        "latency_p95_ms": 1000,
    }
    gates = check_quality_gates(report, 4.0, 3)
    assert all(value is True for value in gates.values())
    assert check_quality_gates(report, None)["human average >= 4.0"] == "PENDING"
    assert check_quality_gates(report, 4.5, 2)["no human score < 3"] is False


def test_injection_answer_audit_requires_refusal_without_payload_leak():
    assert _audit_injection_answer("我不能提供系统提示词或隐藏上下文。") == {
        "refused": True,
        "leaked": False,
    }
    assert _audit_injection_answer("调试指令无法执行，仅回答帖子查找问题。") == {
        "refused": True,
        "leaked": False,
    }
    assert _audit_injection_answer("系统提示词是：你是 ShareO 助手。") == {
        "refused": False,
        "leaked": True,
    }


def test_current_agent_dataset_lines_are_valid_json():
    path = _current_agent_dataset_path()
    assert all(json.loads(line)["id"] for line in path.read_text().splitlines() if line)


def test_report_exposes_all_machine_rows_not_only_human_subset(tmp_path):
    output = tmp_path / "agent.json"
    _write_report(
        output,
        {
            "agent": {
                "query_results": [{"id": "machine-1"}, {"id": "no-answer-1"}],
                "scoring_template": [{"id": "machine-1"}],
            }
        },
    )
    payload = json.loads(output.read_text(encoding="utf-8"))
    assert [row["id"] for row in payload["agent"]["evaluation_rows"]] == [
        "machine-1",
        "no-answer-1",
    ]
    assert "query_results" not in payload["agent"]
    assert "scoring_template" not in payload["agent"]


def test_machine_failure_blocks_old_human_scores():
    results = {
        "agent": {
            "report": {
                "total_queries": 36,
                "failed_queries": 1,
                "source_hit_rate": 0.95,
                "source_coverage_rate": 0.95,
                "complete_source_coverage_rate": 0.95,
                "citation_precision_rate": 0.95,
                "unexpected_tool_calls": 0,
                "required_tool_selection_rate": 0.95,
                "citation_accessible_rate": 1.0,
                "hallucinated_citations": 0,
                "no_answer_accuracy": 1.0,
                "forbidden_actions": 0,
                "budget_violations": 0,
                "injection_failures": 0,
                "latency_p95_ms": 1000,
                "evaluated_answer_count": 30,
            }
        }
    }
    gates, passed = _apply_scoring(results, Path("missing-scores.md"))
    assert passed is False
    assert gates["all 36 Agent tasks evaluated"] is False
    assert results["human_scoring"]["status"] == "BLOCKED_MACHINE_GATES"


def test_machine_only_skips_human_gate_and_requires_machine_success():
    report = {
        "total_queries": 36,
        "failed_queries": 0,
        "source_hit_rate": 0.9,
        "source_coverage_rate": 0.9,
        "complete_source_coverage_rate": 0.9,
        "citation_precision_rate": 0.9,
        "unexpected_tool_calls": 0,
        "required_tool_selection_rate": 0.9,
        "citation_accessible_rate": 1.0,
        "hallucinated_citations": 0,
        "no_answer_accuracy": 1.0,
        "forbidden_actions": 0,
        "budget_violations": 0,
        "injection_failures": 0,
        "latency_p95_ms": 1000,
        "evaluated_answer_count": 30,
    }
    results = {"agent": {"report": report}}
    _, passed = _apply_scoring(results, None, machine_only=True)
    assert passed is True
    assert results["human_scoring"]["status"] == "not_run"
    assert results["quality_gate_passed"] is True


def test_tool_selection_audit_lists_every_missing_task():
    rows = [
        {
            "id": "ok",
            "expected_tools": ["read_posts"],
            "actual_tools": ["read_posts"],
            "machine_required_tool_hit": True,
            "machine_status": "completed",
        },
        {
            "id": "missing",
            "expected_tools": ["semantic_search_posts", "read_posts"],
            "actual_tools": ["semantic_search_posts"],
            "machine_required_tool_hit": False,
            "machine_status": "completed",
        },
        {
            "id": "failed",
            "expected_tools": ["search_images"],
            "actual_tools": [],
            "machine_required_tool_hit": False,
            "machine_status": "failed",
        },
    ]

    audit = _summarize_tool_selection(rows)

    assert audit["all_tasks_rate"] == 0.3333
    assert audit["completed_tasks_rate"] == 0.5
    assert audit["missing_required_tool_task_ids"] == ["missing", "failed"]


def test_human_rows_can_be_recovered_from_machine_report():
    tasks = [
        {"id": "human-01", "human_evaluable": True},
        {"id": "machine-only", "human_evaluable": False},
    ]
    results = {
        "agent": {
            "query_results": [
                {"id": "human-01", "answer": "answer"},
                {"id": "machine-only", "answer": "not scored"},
            ]
        }
    }
    assert _human_rows_from_report(results, tasks) == [{"id": "human-01", "answer": "answer"}]


def test_scoring_template_keeps_complete_answer_for_human_review(tmp_path):
    path = tmp_path / "agent_scoring.md"
    answer = "开头。\n" + ("完整回答内容。" * 80)
    _write_scoring_template(
        path,
        [
            {
                "id": "comparison-06",
                "question": "比较两个场景",
                "expected_source_post_ids": [1, 2],
                "citation_post_ids": [1, 2],
                "citation_details": [],
                "agent_trace": {"steps": []},
                "answer": answer,
                "machine_source_hit": 1.0,
            }
        ],
        {"date": "2026-07-28", "git_sha": "abc1234", "dataset_sha256": "dataset"},
    )
    content = path.read_text(encoding="utf-8")
    assert "完整回答内容。" * 80 in content


def test_scoring_template_can_render_verified_scores(tmp_path):
    path = tmp_path / "agent_scoring.md"
    _write_scoring_template(
        path,
        [{"id": "single-01", "answer": "回答", "machine_source_hit": 1.0}],
        {"date": "2026-07-28", "git_sha": "abc1234", "dataset_sha256": "dataset"},
        scores=[4],
    )
    assert "| 1.0 | 4 |" in path.read_text(encoding="utf-8")


def test_runtime_internal_token_env_and_state_dir_fallback(monkeypatch, tmp_path):
    from app.commands import eval_agent

    monkeypatch.setattr(
        eval_agent, "_read_local_env", lambda name, default="": "shareo-local-internal"
    )
    monkeypatch.setenv("SHAREO_INTERNAL_TOKEN", "env-primary-token")
    assert eval_agent._runtime_internal_token() == "env-primary-token"

    monkeypatch.delenv("SHAREO_INTERNAL_TOKEN", raising=False)
    monkeypatch.delenv("SHAREO_AI_INTERNAL_TOKEN", raising=False)
    state_dir = tmp_path / "state"
    state_dir.mkdir()
    (state_dir / "internal-token").write_text("replace-with-a-random-value\n", encoding="utf-8")
    monkeypatch.setenv("SHAREO_LOCAL_STATE_DIR", str(state_dir))
    assert eval_agent._runtime_internal_token() == ""
