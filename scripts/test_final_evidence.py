#!/usr/bin/env python3
"""Unit tests for repeated-evidence aggregation and redacted handoff tooling."""

from __future__ import annotations

import json
import unittest
from pathlib import Path
from tempfile import TemporaryDirectory
from types import SimpleNamespace
from unittest.mock import patch

from scripts.collect_final_evidence import (
    aggregate_round_reports,
    merge_human_handoffs,
    numeric_summary,
    update_retained_runs,
)
from scripts.collect_worktree_manifest import build_manifest


class EvidenceToolTests(unittest.TestCase):
    def test_numeric_summary_keeps_distribution(self) -> None:
        self.assertEqual(
            numeric_summary([1.0, 2.0, 3.0]),
            {
                "status": "available",
                "count": 3,
                "mean": 2.0,
                "median": 2.0,
                "min": 1.0,
                "max": 3.0,
                "stdev": 0.8165,
            },
        )
        self.assertEqual(numeric_summary([])["status"], "unavailable")

    def test_aggregate_reports_does_not_choose_best_round(self) -> None:
        result = aggregate_round_reports(
            [
                {"reports": {"rag_image": {"metrics": {"rag.report.latency_p95_ms": 100}}}},
                {"reports": {"rag_image": {"metrics": {"rag.report.latency_p95_ms": 200}}}},
            ]
        )
        summary = result["groups"]["rag_image"]["rag.report.latency_p95_ms"]
        self.assertEqual(summary["mean"], 150.0)
        self.assertEqual(summary["min"], 100.0)
        self.assertEqual(summary["max"], 200.0)
        self.assertEqual(result["round_count"], 2)

    def test_merge_handoffs_unions_rounds(self) -> None:
        row = {
            "id": "agent-001",
            "category": "injection",
            "question": "问题",
            "reference_answer": "参考",
            "answer": "答案",
            "expected_source_post_ids": [1],
            "citation_post_ids": [1],
            "score": 2,
            "passes": [{"reason": "分歧"}],
        }
        payload = {
            "agent": {
                "rows": [row],
                "human_fallback": [{"id": "agent-001", "reasons": ["score_below_3"]}],
            },
            "rag": {},
        }
        result = merge_human_handoffs([payload, payload], "run-1")
        self.assertEqual(result["count"], 1)
        self.assertEqual(result["items"][0]["rounds"], [1, 2])
        self.assertEqual(result["items"][0]["score_min"], 2)

    def test_failed_run_is_retained_without_replacing_authoritative_pointer(self) -> None:
        with TemporaryDirectory() as temporary:
            evidence_dir = Path(temporary)
            (evidence_dir / "retained-runs.json").write_text(
                '{"authoritative_run_id":"old-run","runs":[]}', encoding="utf-8"
            )
            with patch("scripts.collect_final_evidence.EVIDENCE_DIR", evidence_dir):
                update_retained_runs(
                    "new-run",
                    {"git": {}, "safe_env": {}, "dataset_hashes": {}},
                    {
                        "quality_gate_passed": False,
                        "source_fingerprint": {"stable": False},
                        "reports": {},
                        "quality_judgement": {},
                    },
                    {},
                    {
                        "started_at": "",
                        "finished_at": "",
                        "status_counts": {},
                        "quality_gate_passed": False,
                    },
                )
            retained = json.loads(
                (evidence_dir / "retained-runs.json").read_text(encoding="utf-8")
            )
        self.assertEqual(retained["authoritative_run_id"], "old-run")
        self.assertFalse(retained["runs"][0]["authoritative"])

    def test_passed_run_advances_authoritative_pointer(self) -> None:
        with TemporaryDirectory() as temporary:
            evidence_dir = Path(temporary)
            (evidence_dir / "retained-runs.json").write_text(
                '{"authoritative_run_id":"old-run","runs":[]}', encoding="utf-8"
            )
            with patch("scripts.collect_final_evidence.EVIDENCE_DIR", evidence_dir):
                update_retained_runs(
                    "new-run",
                    {"git": {}, "safe_env": {}, "dataset_hashes": {}},
                    {
                        "quality_gate_passed": True,
                        "source_fingerprint": {"stable": True},
                        "reports": {},
                        "quality_judgement": {},
                    },
                    {},
                    {
                        "started_at": "",
                        "finished_at": "",
                        "status_counts": {},
                        "quality_gate_passed": True,
                    },
                )
            retained = json.loads(
                (evidence_dir / "retained-runs.json").read_text(encoding="utf-8")
            )
        self.assertEqual(retained["authoritative_run_id"], "new-run")
        self.assertTrue(retained["runs"][0]["authoritative"])

    def test_source_only_manifest_excludes_playwright_cli_artifacts(self) -> None:
        status = "?? .playwright-cli/traces/run.trace\n M internal/service/foo.go\n"
        with (
            patch("scripts.collect_worktree_manifest.command", return_value=status),
            patch(
                "scripts.collect_worktree_manifest.subprocess.run",
                return_value=SimpleNamespace(stdout=b""),
            ),
        ):
            manifest = build_manifest(source_only=True)
        self.assertIn(".playwright-cli/", manifest["scope_policy"]["excluded_prefixes"])
        self.assertEqual(
            [entry["path"] for entry in manifest["entries"]],
            ["internal/service/foo.go"],
        )


if __name__ == "__main__":
    unittest.main()
