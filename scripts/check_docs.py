#!/usr/bin/env python3
"""Validate repository-local links in Markdown files."""

from __future__ import annotations

import re
import sys
from pathlib import Path
from urllib.parse import unquote

ROOT = Path(__file__).resolve().parents[1]
LINK_RE = re.compile(r"(?<!!)\[[^]]*]\(([^)]+)\)")
SKIP_PREFIXES = ("http://", "https://", "mailto:", "#")


def markdown_files() -> list[Path]:
    return sorted(path for path in ROOT.rglob("*.md") if ".git" not in path.parts)


def main() -> int:
    failures: list[str] = []
    for source in markdown_files():
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
                failures.append(f"{source.relative_to(ROOT)} -> {target}")
    if failures:
        print("Broken local Markdown links:", file=sys.stderr)
        for failure in failures:
            print(f"  {failure}", file=sys.stderr)
        return 1
    print(f"Markdown links OK ({len(markdown_files())} files).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
