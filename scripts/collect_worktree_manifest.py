#!/usr/bin/env python3
"""Write a redacted, machine-readable worktree and evidence manifest."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import subprocess
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
SECRET_PATTERNS = {
    "api_key": re.compile(r"(?i)\bapi[_-]?key\b\s*[:=]"),
    "authorization": re.compile(r"(?i)\bauthorization\b\s*[:=]"),
    "password": re.compile(r"(?i)\bpassword\b\s*[:=]"),
    "secret": re.compile(r"(?i)\bsecret\b\s*[:=]"),
    "token": re.compile(r"(?i)\btoken\b\s*[:=]"),
}
TEXT_SUFFIXES = {
    ".c",
    ".css",
    ".go",
    ".html",
    ".js",
    ".json",
    ".md",
    ".py",
    ".sh",
    ".sql",
    ".toml",
    ".yaml",
    ".yml",
}
SOURCE_ONLY_EXCLUDED_PREFIXES = (
    "docs/evidence/final-freeze/",
    "output/playwright/",
    ".playwright-cli/",
)


def command(args: list[str]) -> str:
    result = subprocess.run(
        args,
        cwd=ROOT,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
    )
    return result.stdout


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def classify(path: str) -> str:
    if path.startswith("docs/evidence/final-freeze/runs/"):
        return "temporary_evidence"
    if path.startswith("docs/evidence/final-freeze/"):
        return "authoritative_evidence"
    if path.startswith(".local/") or path.startswith(".cache/"):
        return "local_runtime"
    if path.endswith(".env") or path.endswith(".pem") or path.endswith(".key"):
        return "sensitive_candidate"
    return "source_or_docs_requires_scope_confirmation"


def parse_status(excluded_prefixes: tuple[str, ...] = ()) -> list[dict[str, Any]]:
    entries: list[dict[str, Any]] = []
    for line in command(["git", "status", "--short"]).splitlines():
        if not line:
            continue
        status = line[:2]
        path = line[3:].strip()
        if " -> " in path:
            path = path.split(" -> ", 1)[-1]
        if any(path.startswith(prefix) for prefix in excluded_prefixes):
            continue
        entry: dict[str, Any] = {
            "path": path,
            "status": status,
            "classification": classify(path),
        }
        absolute = ROOT / path
        if absolute.is_file():
            entry["bytes"] = absolute.stat().st_size
            if absolute.suffix in TEXT_SUFFIXES and absolute.stat().st_size <= 2_000_000:
                try:
                    content = absolute.read_text(encoding="utf-8", errors="ignore")
                except OSError:
                    content = ""
                entry["possible_sensitive_fields"] = [
                    name for name, pattern in SECRET_PATTERNS.items() if pattern.search(content)
                ]
        entries.append(entry)
    return entries


def build_manifest(source_only: bool = False) -> dict[str, Any]:
    excluded_prefixes = SOURCE_ONLY_EXCLUDED_PREFIXES if source_only else ()
    diff_args = ["git", "diff", "--no-ext-diff", "--binary"]
    if source_only:
        diff_args.extend(
            [f":(exclude){prefix}**" for prefix in SOURCE_ONLY_EXCLUDED_PREFIXES]
        )
    diff = subprocess.run(
        diff_args,
        cwd=ROOT,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
    ).stdout
    entries = parse_status(excluded_prefixes)
    return {
        "generated_at": datetime.now(timezone.utc).isoformat(timespec="seconds"),
        "git": {
            "sha": command(["git", "rev-parse", "HEAD"]).strip(),
            "branch": command(["git", "branch", "--show-current"]).strip(),
            "dirty": bool(entries),
            "diff_sha256": sha256_bytes(diff),
        },
        "scope_policy": {
            "ai_action": "inventory_only",
            "unknown_files": "requires_human_scope_confirmation",
            "destructive_cleanup": "not_performed",
            "commit": "not_performed",
            "source_only": source_only,
            "excluded_prefixes": list(excluded_prefixes),
        },
        "summary": {
            "total_entries": len(entries),
            "temporary_evidence": sum(
                item["classification"] == "temporary_evidence" for item in entries
            ),
            "authoritative_evidence": sum(
                item["classification"] == "authoritative_evidence" for item in entries
            ),
            "scope_confirmation_required": sum(
                item["classification"] == "source_or_docs_requires_scope_confirmation"
                for item in entries
            ),
            "possible_sensitive_files": sum(
                bool(item.get("possible_sensitive_fields")) for item in entries
            ),
        },
        "entries": entries,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--output",
        type=Path,
        default=ROOT / ".local" / "shareo" / "final-freeze" / "worktree-manifest.json",
    )
    parser.add_argument(
        "--source-only",
        action="store_true",
        help="exclude generated final-freeze and browser artifacts from the scope fingerprint",
    )
    args = parser.parse_args()
    output = args.output if args.output.is_absolute() else ROOT / args.output
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(
        json.dumps(build_manifest(args.source_only), ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    print(f"Worktree manifest written to {output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
