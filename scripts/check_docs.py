#!/usr/bin/env python3
"""Validate the final ShareO documentation and sanitized evidence bundle."""

from __future__ import annotations

import json
import os
import re
import sys
from pathlib import Path
from urllib.parse import unquote

ROOT = Path(__file__).resolve().parents[1]
LINK_RE = re.compile(r"(?<!!)\[[^]]*]\(([^)]+)\)")
SKIP_PREFIXES = ("http://", "https://", "mailto:", "#")
SKIP_DIRECTORIES = {
    ".git",
    ".venv",
    ".cache",
    ".local",
    "node_modules",
    "__pycache__",
    "qdrant",
    "resources",
}
RETIRED_REFERENCES = (
    "docs/plan.md",
    "docs/roadmap.md",
    "docs/phases/",
    "docs/research/",
    "docs/evidence/phase7c/",
    "docs/evidence/phase8-agent/",
    "docs/eval/image_search_v1.jsonl",
    "docs/eval/rag_qa_v1.jsonl",
    "docs/eval/agent_tasks_v1.jsonl",
    "docs/eval/results/",
    "make demo-seed",
    "make test-image-e2e",
    "make test-ai-e2e",
    "make test-agent-e2e",
    "make test-degradation",
)


def markdown_files() -> list[Path]:
    return sorted(
        path for path in ROOT.rglob("*.md") if not SKIP_DIRECTORIES.intersection(path.parts)
    )


def validate_links(files: list[Path], failures: list[str]) -> None:
    for source in files:
        text = source.read_text(encoding="utf-8")
        for raw in LINK_RE.findall(text):
            target = raw.strip().split(maxsplit=1)[0].strip("<>")
            if not target or target.startswith(SKIP_PREFIXES):
                continue
            path_text = unquote(target.split("#", 1)[0])
            if not path_text:
                continue
            destination = (source.parent / path_text).resolve()
            if not destination.exists():
                failures.append(f"broken link: {source.relative_to(ROOT)} -> {target}")


def validate_retired_references(files: list[Path], failures: list[str]) -> None:
    for source in files:
        text = source.read_text(encoding="utf-8")
        for retired in RETIRED_REFERENCES:
            if retired in text:
                failures.append(f"{source.relative_to(ROOT)} contains retired reference: {retired}")


def validate_current_evidence(failures: list[str]) -> None:
    if os.environ.get("SHAREO_SKIP_CURRENT_EVIDENCE") == "1":
        return
    evidence_dir = ROOT / "docs" / "evidence" / "final-freeze"
    summary_path = evidence_dir / "quality-summary.json"
    manifest_path = evidence_dir / "run-manifest.json"
    try:
        summary = json.loads(summary_path.read_text(encoding="utf-8"))
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        failures.append(f"current final-freeze evidence is unavailable or invalid: {exc}")
        return

    expected_counts = {
        "image_search_current": 38,
        "post_search_current": 32,
        "rag_current": 30,
        "agent_current": 36,
    }
    if summary.get("dataset_counts") != expected_counts:
        failures.append(
            "current final-freeze dataset counts mismatch: "
            f"expected={expected_counts} actual={summary.get('dataset_counts')}"
        )
    if summary.get("run_id") != manifest.get("run_id"):
        failures.append("current final-freeze quality summary and manifest run_id differ")
    retained_path = evidence_dir / "retained-runs.json"
    try:
        retained = json.loads(retained_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        failures.append(f"retained-runs.json is unavailable or invalid: {exc}")
        retained = {}
    authoritative_run_id = retained.get("authoritative_run_id")
    if authoritative_run_id != manifest.get("run_id"):
        failures.append(
            "retained-runs authoritative_run_id does not point to current run-manifest"
        )
    if authoritative_run_id and not any(
        item.get("run_id") == authoritative_run_id for item in retained.get("runs", [])
    ):
        failures.append("retained-runs authoritative_run_id has no retained run entry")
    repeated = summary.get("ai_repetitions", {})
    if repeated.get("requested_rounds", 0) < 3:
        failures.append("current final-freeze must record at least three AI evaluation rounds")
    if summary.get("source_fingerprint", {}).get("stable") is not True:
        failures.append("current final-freeze source fingerprint is not stable")
    for report_name in ("post_search_local", "image_search_local", "rag_image", "agent"):
        report = summary.get("reports", {}).get(report_name, {})
        if report.get("status") != "available":
            failures.append(f"current final-freeze report unavailable: {report_name}")


def main() -> int:
    failures: list[str] = []
    files = markdown_files()
    validate_links(files, failures)
    validate_retired_references(files, failures)
    validate_current_evidence(failures)
    if failures:
        print("Documentation validation failed:", file=sys.stderr)
        for failure in failures:
            print(f"  {failure}", file=sys.stderr)
        return 1
    if os.environ.get("SHAREO_SKIP_CURRENT_EVIDENCE") == "1":
        print(
            f"Documentation validation OK ({len(files)} Markdown files); "
            "current evidence check skipped by caller."
        )
    else:
        print(
            f"Documentation validation OK ({len(files)} Markdown files); "
            "current evidence is authoritative."
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
