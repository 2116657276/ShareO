#!/usr/bin/env python3
"""Validate ShareO Markdown links, phase contracts, status, and evaluation data."""

from __future__ import annotations

import json
import re
import subprocess
import sys
from collections import Counter
from pathlib import Path
from urllib.parse import unquote

ROOT = Path(__file__).resolve().parents[1]
LINK_RE = re.compile(r"(?<!!)\[[^]]*]\(([^)]+)\)")
SKIP_PREFIXES = ("http://", "https://", "mailto:", "#")
SKIP_DIRECTORIES = {".git", ".venv", "node_modules", "__pycache__"}

PHASE_DIR = ROOT / "docs" / "phases"
EXPECTED_PHASE_FILES = {
    "phase-0-scope-foundation.md",
    "phase-1-community.md",
    "phase-2-private-chat.md",
    "phase-3-ai-runtime.md",
    "phase-4-image-search.md",
    "phase-5-rag-retrieval.md",
    "phase-6-chat-bot.md",
    "phase-7-demo-evaluation-release.md",
}
REQUIRED_PHASE_HEADINGS = (
    "目标",
    "当前基线",
    "范围与非目标",
    "依赖",
    "工作包",
    "接口与数据流",
    "数据一致性",
    "安全边界",
    "失败模式",
    "测试矩阵",
    "退出标准",
    "提交与环境证据",
    "遗留项",
)
IMAGE_DISTRIBUTION = {
    "object": 8,
    "scene": 8,
    "color": 6,
    "style": 6,
    "composition": 4,
    "portrait": 4,
    "motion": 2,
    "no-match": 2,
}
RAG_DISTRIBUTION = {
    "factual": 10,
    "advice": 8,
    "comparison": 5,
    "multi-source": 4,
    "no-answer": 3,
}
STALE_ACTIVE_PATTERNS = (
    "phase-0-foundation.md",
    "phase-1-im.md",
    "phase-2-image-search.md",
    "phase-3-rag.md",
    "phase-4-agent.md",
    "docs/reviews/",
    "image_search_demo_v1.jsonl",
    "rag_qa_demo_v1.jsonl",
    "010_chat_hardening.sql",
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


def validate_phase_contracts(failures: list[str]) -> None:
    actual = {path.name for path in PHASE_DIR.glob("phase-*.md")}
    if actual != EXPECTED_PHASE_FILES:
        failures.append(
            "phase files mismatch: "
            f"missing={sorted(EXPECTED_PHASE_FILES - actual)} "
            f"unexpected={sorted(actual - EXPECTED_PHASE_FILES)}"
        )
    sha_re = re.compile(r"`[0-9a-f]{7,40}`")
    for name in sorted(EXPECTED_PHASE_FILES & actual):
        path = PHASE_DIR / name
        text = path.read_text(encoding="utf-8")
        headings = {match.group(1).strip() for match in re.finditer(r"^## (.+)$", text, re.M)}
        missing = [heading for heading in REQUIRED_PHASE_HEADINGS if heading not in headings]
        if missing:
            failures.append(f"{path.relative_to(ROOT)} missing headings: {', '.join(missing)}")
        if "> 状态：已完成" in text:
            shas = sha_re.findall(text)
            if not shas:
                failures.append(f"{path.relative_to(ROOT)} completed phase has no commit SHA")
            for raw_sha in shas:
                sha = raw_sha.strip("`")
                result = subprocess.run(
                    ["git", "cat-file", "-e", f"{sha}^{{commit}}"],
                    cwd=ROOT,
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL,
                    check=False,
                )
                if result.returncode != 0:
                    failures.append(
                        f"{path.relative_to(ROOT)} references unknown commit SHA: {sha}"
                    )
        if "当前工作区" in text or "待提交" in text:
            failures.append(f"{path.relative_to(ROOT)} uses unverifiable completion wording")


def validate_status(failures: list[str]) -> None:
    expectations = {
        ROOT / "TASK.md": "当前阶段：Phase 7 进行中",
        ROOT / "README.md": "Phase 7 Demo、评测与发布收口进行中",
        ROOT / "docs" / "roadmap.md": "| Phase 7 Demo 与发布 |",
        PHASE_DIR / "README.md": "Phase 7 — Demo、评测与发布",
    }
    for path, expected in expectations.items():
        if expected not in path.read_text(encoding="utf-8"):
            failures.append(f"{path.relative_to(ROOT)} missing canonical status: {expected}")


def validate_active_docs(files: list[Path], failures: list[str]) -> None:
    for source in files:
        if "docs/adr" in source.as_posix():
            continue
        text = source.read_text(encoding="utf-8")
        for stale in STALE_ACTIVE_PATTERNS:
            if stale in text:
                failures.append(f"{source.relative_to(ROOT)} contains stale reference: {stale}")
        if re.search(r"\d+\s*个\s*Markdown", text) or re.search(
            r"\d+\s*项\s*(?:非集成)?测试", text
        ):
            failures.append(f"{source.relative_to(ROOT)} hard-codes transient file/test counts")
    if (ROOT / "docs" / "reviews").exists():
        failures.append("docs/reviews must not exist after history cleanup")


def load_jsonl(path: Path, failures: list[str]) -> list[dict]:
    rows: list[dict] = []
    try:
        for line_no, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
            if line.strip():
                rows.append(json.loads(line))
    except (OSError, json.JSONDecodeError) as exc:
        failures.append(f"{path.relative_to(ROOT)} invalid JSONL: {exc}")
    return rows


def validate_distribution(
    path: Path, rows: list[dict], expected: dict[str, int], failures: list[str]
) -> None:
    counts = Counter(str(row.get("category", "")) for row in rows)
    if counts != Counter(expected):
        failures.append(
            f"{path.relative_to(ROOT)} category distribution mismatch: "
            f"expected={expected} actual={dict(counts)}"
        )


def validate_evaluation_data(failures: list[str]) -> list[str]:
    pending: list[str] = []
    task = (ROOT / "TASK.md").read_text(encoding="utf-8")
    phase_7a_complete = "- [x] 完成 40 条搜图标注" in task

    image_path = ROOT / "docs" / "eval" / "image_search_v1.jsonl"
    image_rows = load_jsonl(image_path, failures)
    if len(image_rows) != 40:
        failures.append(f"{image_path.relative_to(ROOT)} must contain exactly 40 rows")
    validate_distribution(image_path, image_rows, IMAGE_DISTRIBUTION, failures)
    for index, row in enumerate(image_rows, 1):
        relevant = row.get("relevant_post_ids")
        no_match = bool(row.get("expect_no_results"))
        if no_match:
            if relevant != []:
                failures.append(f"{image_path.relative_to(ROOT)}:{index} no-match must use []")
            continue
        if relevant:
            if row.get("label_status") == "pending":
                failures.append(
                    f"{image_path.relative_to(ROOT)}:{index} has labels but is marked pending"
                )
        elif phase_7a_complete:
            failures.append(f"{image_path.relative_to(ROOT)}:{index} ordinary query is unlabeled")
        elif row.get("label_status") != "pending":
            failures.append(
                f"{image_path.relative_to(ROOT)}:{index} unlabeled query must be explicit pending"
            )
    if not phase_7a_complete:
        pending.append("image-search labels")

    rag_path = ROOT / "docs" / "eval" / "rag_qa_v1.jsonl"
    if not rag_path.exists():
        if phase_7a_complete:
            failures.append("docs/eval/rag_qa_v1.jsonl missing after Phase 7A completion")
        else:
            pending.append("30-row RAG dataset")
        return pending

    rag_rows = load_jsonl(rag_path, failures)
    if len(rag_rows) != 30:
        failures.append(f"{rag_path.relative_to(ROOT)} must contain exactly 30 rows")
    validate_distribution(rag_path, rag_rows, RAG_DISTRIBUTION, failures)
    for index, row in enumerate(rag_rows, 1):
        sources = row.get("expected_source_post_ids")
        no_answer = bool(row.get("expect_no_answer"))
        if no_answer:
            if sources != []:
                failures.append(f"{rag_path.relative_to(ROOT)}:{index} no-answer must use []")
            continue
        if not sources and phase_7a_complete:
            failures.append(f"{rag_path.relative_to(ROOT)}:{index} ordinary question is unlabeled")
        if phase_7a_complete and not str(row.get("reference_answer", "")).strip():
            failures.append(f"{rag_path.relative_to(ROOT)}:{index} reference_answer is empty")
    if not phase_7a_complete:
        pending.append("RAG labels")
    return pending


def main() -> int:
    failures: list[str] = []
    files = markdown_files()
    validate_links(files, failures)
    validate_phase_contracts(failures)
    validate_status(failures)
    validate_active_docs(files, failures)
    pending = validate_evaluation_data(failures)
    if failures:
        print("Documentation validation failed:", file=sys.stderr)
        for failure in failures:
            print(f"  {failure}", file=sys.stderr)
        return 1
    suffix = f"; Phase 7A pending: {', '.join(pending)}" if pending else ""
    print(f"Documentation validation OK ({len(files)} Markdown files){suffix}.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
