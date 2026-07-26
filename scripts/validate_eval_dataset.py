#!/usr/bin/env python3
from __future__ import annotations
"""Validate evaluation datasets for Phase 7A.

Checks:
  - JSONL format and required fields
  - Exact category distribution
  - relevant_post_ids / expected_source_post_ids point to valid posts
  - No duplicate IDs, no empty labels on non-exempt queries
  - Optional: verify against live database (post status and deletion)

Usage:
  python3 scripts/validate_eval_dataset.py docs/eval/image_search_v1.jsonl
  python3 scripts/validate_eval_dataset.py docs/eval/rag_qa_v1.jsonl
  python3 scripts/validate_eval_dataset.py --db-check docs/eval/image_search_v1.jsonl
"""

import argparse
import json
import os
import sys
from collections import Counter
from pathlib import Path
from typing import Any

# ── Dataset specifications ──

IMAGE_SEARCH_SPEC = {
    "object": 8,
    "scene": 10,
    "color": 8,
    "style": 6,
    "composition": 4,
    "motion": 2,
    "no-match": 2,
}

RAG_QA_SPEC = {
    "factual": 10,
    "advice": 8,
    "comparison": 5,
    "multi-source": 4,
    "no-answer": 3,
}

SPECS = {
    "image_search_v1": IMAGE_SEARCH_SPEC,
    "rag_qa_v1": RAG_QA_SPEC,
}

# ── Validation helpers ──

FAIL = 0


def fail(msg: str) -> None:
    global FAIL
    print(f"  FAIL: {msg}")
    FAIL += 1


def detect_dataset_type(path: Path) -> str | None:
    name = path.stem  # e.g., "image_search_v1"
    for key in SPECS:
        if name.startswith(key):
            return key
    return None


def validate_jsonl(path: Path) -> list[dict]:
    """Parse JSONL, reporting line-level errors."""
    entries: list[dict] = []
    if not path.exists():
        fail(f"file not found: {path}")
        return entries

    raw = path.read_text()
    if raw.endswith("\n"):
        raw = raw.rstrip("\n")

    lines = raw.splitlines()
    for i, line in enumerate(lines, 1):
        line = line.strip()
        if not line:
            continue
        try:
            entries.append(json.loads(line))
        except json.JSONDecodeError as e:
            fail(f"line {i}: invalid JSON — {e}")

    if not lines or (len(lines) == 1 and not lines[0].strip()):
        fail("file is empty")

    return entries


def validate_image_search(entries: list[dict]) -> None:
    spec = IMAGE_SEARCH_SPEC
    expected_total = sum(spec.values())

    if len(entries) != expected_total:
        fail(f"expected {expected_total} queries, got {len(entries)}")

    categories: Counter[str] = Counter()
    empty_labels = 0
    no_match_empty = 0
    seen_ids: set[int] = set()

    for i, entry in enumerate(entries, 1):
        # Required fields
        for field in ("query", "category"):
            if field not in entry:
                fail(f"line {i}: missing required field '{field}'")

        cat = entry.get("category", "")
        categories[cat] += 1

        # No-match entries
        if entry.get("expect_no_results", False):
            if "relevant_post_ids" in entry and entry["relevant_post_ids"]:
                fail(f"line {i}: no-match entry should not have relevant_post_ids")
            if cat != "no-match":
                fail(f"line {i}: expect_no_results but category is '{cat}' not 'no-match'")
            continue

        # Regular entries — must have labels
        ids = entry.get("relevant_post_ids", [])
        if not ids:
            if entry.get("label_status") == "pending":
                empty_labels += 1
            else:
                fail(f"line {i}: empty relevant_post_ids (not marked pending)")
            continue

        # Check IDs are valid
        for pid in ids:
            if not isinstance(pid, int) or pid <= 0:
                fail(f"line {i}: invalid post_id {pid!r}")

        # Check for duplicate IDs within an entry
        if len(ids) != len(set(ids)):
            fail(f"line {i}: duplicate post IDs in relevant_post_ids")

        # Check for cross-entry duplicates (alert only)
        for pid in ids:
            if pid in seen_ids:
                pass  # same post can be relevant to multiple queries — not an error
        seen_ids.update(ids)

    # Category distribution
    for cat, expected in spec.items():
        actual = categories.get(cat, 0)
        if actual != expected:
            fail(f"category '{cat}': expected {expected}, got {actual}")

    for cat in sorted(categories):
        if cat not in spec:
            fail(f"unknown category '{cat}'")

    if empty_labels > 0:
        print(f"  INFO: {empty_labels} queries still pending labeling (label_status=pending)")

    if empty_labels == 0:
        print("  OK: all queries have complete labels")


def validate_rag_qa(entries: list[dict]) -> None:
    spec = RAG_QA_SPEC
    expected_total = sum(spec.values())

    if len(entries) != expected_total:
        fail(f"expected {expected_total} questions, got {len(entries)}")

    categories: Counter[str] = Counter()
    empty_labels = 0

    for i, entry in enumerate(entries, 1):
        for field in ("question", "category"):
            if field not in entry:
                fail(f"line {i}: missing required field '{field}'")

        cat = entry.get("category", "")
        categories[cat] += 1

        # No-answer entries
        if entry.get("expect_no_answer", False):
            if "expected_source_post_ids" in entry and entry["expected_source_post_ids"]:
                fail(f"line {i}: no-answer entry should not have source_post_ids")
            if cat != "no-answer":
                fail(
                    f"line {i}: expect_no_answer but category is '{cat}' not 'no-answer'"
                )
            continue

        # Regular entries — must have source IDs and reference answer
        ids = entry.get("expected_source_post_ids", [])
        if not ids:
            if entry.get("label_status") == "pending":
                empty_labels += 1
            else:
                fail(f"line {i}: empty expected_source_post_ids (not marked pending)")
        else:
            for pid in ids:
                if not isinstance(pid, int) or pid <= 0:
                    fail(f"line {i}: invalid post_id {pid!r}")

        if not ids and not entry.get("reference_answer"):
            fail(f"line {i}: missing reference_answer")

    # Category distribution
    for cat, expected in spec.items():
        actual = categories.get(cat, 0)
        if actual != expected:
            fail(f"category '{cat}': expected {expected}, got {actual}")

    for cat in sorted(categories):
        if cat not in spec:
            fail(f"unknown category '{cat}'")

    if empty_labels > 0:
        print(f"  INFO: {empty_labels} questions still pending labeling (label_status=pending)")

    if empty_labels == 0:
        print("  OK: all questions have complete labels and reference answers")


def db_verify_image_search(entries: list[dict], dsn: str) -> None:
    """Verify that all relevant_post_ids reference approved, non-deleted posts."""
    import urllib.request

    all_ids: set[int] = set()
    for entry in entries:
        if entry.get("expect_no_results"):
            continue
        for pid in entry.get("relevant_post_ids", []):
            if isinstance(pid, int) and pid > 0:
                all_ids.add(pid)

    if not all_ids:
        print("  INFO: no post IDs to verify (all labels pending)")
        return

    # Query via Go API
    base_url = os.environ.get("SHAREO_BASE_URL", "http://127.0.0.1:8080")
    bad_ids: list[int] = []

    for pid in sorted(all_ids):
        try:
            req = urllib.request.Request(f"{base_url}/api/v1/posts/{pid}")
            req.add_header("Accept", "application/json")
            with urllib.request.urlopen(req, timeout=5) as resp:
                data = json.loads(resp.read())
                post = data.get("data", {})
                status = post.get("status", "")
                is_deleted = post.get("is_deleted", 0)
                if status != "approved" or is_deleted != 0:
                    bad_ids.append(pid)
                    fail(f"post {pid}: status={status}, is_deleted={is_deleted}")
        except Exception as e:
            fail(f"post {pid}: could not verify — {e}")

    if not bad_ids:
        print(f"  OK: all {len(all_ids)} referenced posts are approved and not deleted")


def db_verify_rag_qa(entries: list[dict], dsn: str) -> None:
    """Same as image search, for RAG QA dataset."""
    import urllib.request

    all_ids: set[int] = set()
    for entry in entries:
        if entry.get("expect_no_answer"):
            continue
        for pid in entry.get("expected_source_post_ids", []):
            if isinstance(pid, int) and pid > 0:
                all_ids.add(pid)

    if not all_ids:
        print("  INFO: no post IDs to verify (all labels pending)")
        return

    base_url = os.environ.get("SHAREO_BASE_URL", "http://127.0.0.1:8080")
    bad_ids: list[int] = []

    for pid in sorted(all_ids):
        try:
            req = urllib.request.Request(f"{base_url}/api/v1/posts/{pid}")
            req.add_header("Accept", "application/json")
            with urllib.request.urlopen(req, timeout=5) as resp:
                data = json.loads(resp.read())
                post = data.get("data", {})
                if post.get("status") != "approved" or post.get("is_deleted", 0) != 0:
                    bad_ids.append(pid)
                    fail(f"post {pid}: not approved or deleted")
        except Exception as e:
            fail(f"post {pid}: could not verify — {e}")

    if not bad_ids:
        print(f"  OK: all {len(all_ids)} referenced posts are valid")


# ── Main ──


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("dataset", type=Path, help="Path to the JSONL dataset")
    parser.add_argument(
        "--db-check",
        action="store_true",
        help="Also verify post IDs against live database",
    )
    parser.add_argument(
        "--dsn",
        default=os.environ.get("SHAREO_DB_PASSWORD", ""),
        help="Not used — kept for compatibility; DB checks use the Go API",
    )
    args = parser.parse_args()

    ds_type = detect_dataset_type(args.dataset)
    if ds_type is None:
        print(f"ERROR: unknown dataset type: {args.dataset.name}")
        print(f"  Supported: {', '.join(SPECS.keys())}")
        sys.exit(1)

    print(f"Validating {args.dataset.name} (type: {ds_type})")
    print(f"  Expected: {sum(SPECS[ds_type].values())} entries")
    print()

    entries = validate_jsonl(args.dataset)
    if FAIL > 0:
        print(f"\n{FAIL} format error(s) — aborting")
        sys.exit(1)

    if ds_type == "image_search_v1":
        validate_image_search(entries)
    elif ds_type == "rag_qa_v1":
        validate_rag_qa(entries)

    if args.db_check:
        print()
        print("Database verification:")
        if ds_type == "image_search_v1":
            db_verify_image_search(entries, args.dsn)
        elif ds_type == "rag_qa_v1":
            db_verify_rag_qa(entries, args.dsn)

    print()
    if FAIL == 0:
        print("PASS: all validation checks passed.")
        sys.exit(0)
    else:
        print(f"FAIL: {FAIL} validation error(s).")
        sys.exit(1)


if __name__ == "__main__":
    main()
