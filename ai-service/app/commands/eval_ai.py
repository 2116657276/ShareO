"""Unified AI evaluation command for Phase 7B.

Evaluates both semantic image search and RAG bot quality against
frozen datasets, producing metrics, latency reports, and a human
scoring template.

Usage:
  python -m app.commands.eval_ai \\
      --base-url http://127.0.0.1:8080 \\
      --image-dataset ../../docs/eval/image_search_v1.jsonl \\
      --rag-dataset ../../docs/eval/rag_qa_v1.jsonl
"""

import argparse
import hashlib
import json
import os
import platform
import statistics
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

import httpx

from app.config import settings

# ── Reuse image search metrics from the existing eval command ──
from app.commands.eval_image_search import (
    evaluate as evaluate_image_search,
    load_queries as load_image_queries,
)


def load_rag_queries(path: Path) -> list[dict]:
    entries = [json.loads(line) for line in path.read_text().splitlines() if line.strip()]
    unlabeled = [
        item["question"]
        for item in entries
        if not item.get("expected_source_post_ids")
        and not item.get("expect_no_answer", False)
        and item.get("label_status") != "pending"
    ]
    if unlabeled:
        raise RuntimeError(
            f"RAG dataset has {len(unlabeled)} unlabeled questions; label expected_source_post_ids first"
        )
    return entries


def evaluate_rag(
    base_url: str,
    queries: list[dict],
    test_username: str = "",
    test_password: str = "",
) -> dict:
    """Evaluate RAG bot quality by sending questions through the chat API.

    Returns metrics for source hit rate, citation accessibility, hallucinated
    citations, and latency, plus a human scoring template.
    """
    metrics: dict[str, list[float]] = {
        "source_hit": [],
        "citation_accessible": [],
        "hallucinated_count": [],
        "source_count": [],
        "total_latency_ms": [],
    }
    scoring_rows: list[dict] = []
    skipped = 0
    failed_queries = 0
    no_answer_correct = 0
    no_answer_total = 0

    with httpx.Client(base_url=base_url, timeout=120.0) as client:
        # 1. Register test user (or reuse existing)
        suffix = datetime.now(timezone.utc).strftime("%s%f")[-10:]
        username = test_username or f"eval_rag_{suffix}"
        password = test_password or "eval_rag_test_2024"

        token = _get_user_token(client, username, password)
        if not token:
            return {"error": "failed to authenticate test user"}

        # 2. Find bot user
        bot_id = _find_bot_user(client, token)
        if not bot_id:
            return {"error": "shareo_bot user not found"}

        # 3. Ensure conversation with bot exists
        conv_id = _ensure_conversation(client, token, bot_id)
        if not conv_id:
            return {"error": "failed to create conversation with bot"}

        # 4. Evaluate each question
        for item in queries:
            question = item["question"]
            expected_ids = set(item.get("expected_source_post_ids", []))
            expect_no_answer = item.get("expect_no_answer", False)
            category = item.get("category", "")

            if item.get("label_status") == "pending":
                skipped += 1
                continue

            # Send message to bot
            started = time.perf_counter()
            source_id = _send_message(client, token, conv_id, question)
            if not source_id:
                failed_queries += 1
                elapsed = (time.perf_counter() - started) * 1000
                metrics["source_hit"].append(0.0)
                metrics["citation_accessible"].append(0.0)
                metrics["hallucinated_count"].append(0.0)
                metrics["source_count"].append(0.0)
                metrics["total_latency_ms"].append(elapsed)
                scoring_rows.append(
                    {
                        "question": question,
                        "category": category,
                        "expected_answer": item.get("reference_answer", ""),
                        "expected_source_post_ids": sorted(expected_ids),
                        "bot_answer": "SEND_FAILED",
                        "citations": [],
                        "citation_post_ids": [],
                        "citation_details": [],
                        "relevance_score": "",
                    }
                )
                continue

            # Wait for bot reply
            bot_message = _wait_for_bot_reply(client, token, conv_id, source_id, timeout=120)
            elapsed = (time.perf_counter() - started) * 1000
            metrics["total_latency_ms"].append(elapsed)

            if not bot_message:
                failed_queries += 1
                metrics["source_hit"].append(0.0)
                metrics["citation_accessible"].append(0.0)
                metrics["hallucinated_count"].append(0.0)
                metrics["source_count"].append(0.0)
                scoring_rows.append(
                    {
                        "question": question,
                        "category": category,
                        "expected_answer": item.get("reference_answer", ""),
                        "expected_source_post_ids": sorted(expected_ids),
                        "bot_answer": "NO_REPLY",
                        "citations": [],
                        "citation_post_ids": [],
                        "citation_details": [],
                        "relevance_score": "",
                    }
                )
                continue

            # Parse citations
            meta = _parse_meta(bot_message.get("meta"))
            citations = meta.get("citations", [])
            citation_post_ids = [c.get("post_id") for c in citations if c.get("post_id")]
            source_count = len(citation_post_ids)
            metrics["source_count"].append(source_count)

            bot_content = bot_message.get("content", "")

            # Source hit rate
            if expect_no_answer:
                no_answer_total += 1
                if not citation_post_ids:
                    no_answer_correct += 1
                hits = 1.0 if not citation_post_ids else 0.0
            elif expected_ids:
                hit_ids = set(citation_post_ids) & expected_ids
                hits = 1.0 if hit_ids else 0.0
            else:
                hits = 0.0
            metrics["source_hit"].append(hits)

            # Citation accessibility (verified via Go API)
            inaccessible = 0
            for pid in citation_post_ids:
                if not _verify_post_accessible(client, pid):
                    inaccessible += 1
            accessible_ratio = (
                1.0 - inaccessible / len(citation_post_ids) if citation_post_ids else 1.0
            )
            metrics["citation_accessible"].append(accessible_ratio)

            # Hallucinated citations (citations to non-existent or non-approved posts)
            hallucinated = 0
            for pid in citation_post_ids:
                if not _verify_post_approved(client, pid):
                    hallucinated += 1
            metrics["hallucinated_count"].append(hallucinated)

            scoring_rows.append(
                {
                    "question": question,
                    "category": category,
                    "expected_answer": item.get("reference_answer", ""),
                    "expected_source_post_ids": sorted(expected_ids),
                    "bot_answer": bot_content[:500],
                    "citations": citation_post_ids,
                    "citation_post_ids": citation_post_ids,
                    "citation_details": citations,
                    "source_hit_rate": round(hits, 3),
                    "accessible": accessible_ratio >= 1.0,
                    "relevance_score": "",
                }
            )

    # Compute summary
    total = len(queries) - skipped
    if total == 0:
        return {"error": "no labeled RAG queries to evaluate"}

    report = {
        "total_queries": total,
        "skipped": skipped,
        "failed_queries": failed_queries,
        "source_hit_rate": round(statistics.fmean(metrics["source_hit"]), 4)
        if metrics["source_hit"]
        else 0,
        "citation_accessible_rate": round(statistics.fmean(metrics["citation_accessible"]), 4)
        if metrics["citation_accessible"]
        else 0,
        "total_hallucinated": int(sum(metrics["hallucinated_count"])),
        "avg_sources_per_answer": round(statistics.fmean(metrics["source_count"]), 2)
        if metrics["source_count"]
        else 0,
        "latency_p50_ms": _percentile(metrics["total_latency_ms"], 0.50),
        "latency_p95_ms": _percentile(metrics["total_latency_ms"], 0.95),
    }

    if no_answer_total > 0:
        report["no_answer_accuracy"] = no_answer_correct / no_answer_total

    return {
        "report": report,
        "scoring_template": scoring_rows,
    }


# ── Helpers ──


def _get_user_token(client: httpx.Client, username: str, password: str) -> str:
    """Register or login to get a JWT token."""
    # Try register
    resp = client.post(
        "/api/v1/auth/register",
        json={"username": username, "password": password},
    )
    if resp.status_code == 200:
        data = resp.json()
        if data.get("code") == 0:
            return data["data"]["token"]

    # Try login
    resp = client.post(
        "/api/v1/auth/login",
        json={"username": username, "password": password},
    )
    if resp.status_code == 200:
        data = resp.json()
        if data.get("code") == 0:
            return data["data"]["token"]

    return ""


def _find_bot_user(client: httpx.Client, token: str) -> int | None:
    resp = client.get(
        "/api/v1/users/search",
        params={"q": "shareo_bot", "limit": 20},
        cookies={"token": token},
    )
    if resp.status_code != 200:
        return None
    data = resp.json().get("data", [])
    for user in data:
        if user.get("is_bot") and user.get("username") == "shareo_bot":
            return user["id"]
    return None


def _ensure_conversation(client: httpx.Client, token: str, bot_id: int) -> int | None:
    """Get or create conversation with bot."""
    # List existing conversations
    resp = client.get("/api/v1/conversations", cookies={"token": token})
    if resp.status_code == 200:
        data = resp.json().get("data", [])
        for conv in data:
            other = conv.get("other_user", {})
            if other.get("id") == bot_id:
                return conv["id"]

    # Create new conversation
    resp = client.post(
        "/api/v1/conversations",
        json={"user_id": bot_id},
        cookies={"token": token},
    )
    if resp.status_code == 200:
        data = resp.json()
        if data.get("code") == 0:
            return data["data"]["id"]

    return None


def _send_message(client: httpx.Client, token: str, conv_id: int, content: str) -> int | None:
    resp = client.post(
        f"/api/v1/conversations/{conv_id}/messages",
        json={"content": content},
        cookies={"token": token},
    )
    if resp.status_code == 200:
        data = resp.json()
        if data.get("code") == 0:
            return data["data"]["id"]
    return None


def _wait_for_bot_reply(
    client: httpx.Client,
    token: str,
    conv_id: int,
    source_message_id: int,
    timeout: int = 120,
) -> dict | None:
    """Poll conversation messages until bot replies to source_message_id."""
    deadline = time.perf_counter() + timeout
    while time.perf_counter() < deadline:
        resp = client.get(
            f"/api/v1/conversations/{conv_id}/messages",
            params={"limit": 100},
            cookies={"token": token},
        )
        if resp.status_code != 200:
            time.sleep(1)
            continue

        data = resp.json().get("data", {})
        messages = data.get("messages", [])
        for msg in messages:
            sender = msg.get("sender", {})
            if not sender.get("is_bot"):
                continue
            meta = _parse_meta(msg.get("meta"))
            if meta.get("source_message_id") == source_message_id:
                return msg

        time.sleep(1)

    return None


def _parse_meta(meta_raw: Any) -> dict:
    if meta_raw is None:
        return {}
    if isinstance(meta_raw, dict):
        return meta_raw
    if isinstance(meta_raw, str):
        try:
            return json.loads(meta_raw)
        except (json.JSONDecodeError, TypeError):
            return {}
    return {}


def _verify_post_accessible(client: httpx.Client, post_id: int) -> bool:
    """Check that a post exists and has an accessible URL."""
    try:
        resp = client.get(f"/api/v1/posts/{post_id}")
        return resp.status_code == 200
    except Exception:
        return False


def _verify_post_approved(client: httpx.Client, post_id: int) -> bool:
    """Check that a post is approved and not deleted."""
    try:
        resp = client.get(f"/api/v1/posts/{post_id}")
        if resp.status_code != 200:
            return False
        data = resp.json().get("data", {})
        return data.get("status") == "approved" and data.get("is_deleted") == 0
    except Exception:
        return False


def _percentile(values: list[float], fraction: float) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    index = min(len(ordered) - 1, max(0, round((len(ordered) - 1) * fraction)))
    return round(ordered[index], 1)


def _export_image_metrics(report: dict) -> dict:
    """Extract image search quality metrics in a flat dict for the unified report."""
    semantic = report.get("semantic", {})
    return {
        "recall_5": semantic.get("recall_5", 0),
        "recall_10": semantic.get("recall_10", 0),
        "mrr": semantic.get("mrr", 0),
        "duplicate_queries": report.get("semantic_duplicate_queries", 0),
        "latency_p50_ms": semantic.get("latency_p50_ms", 0),
        "latency_p95_ms": semantic.get("latency_p95_ms", 0),
    }


def _export_rag_metrics(rag_result: dict) -> dict:
    """Extract RAG quality metrics."""
    if "error" in rag_result:
        return {"error": rag_result["error"]}
    report = rag_result.get("report", {})
    return {
        "source_hit_rate": report.get("source_hit_rate", 0),
        "citation_accessible_rate": report.get("citation_accessible_rate", 0),
        "total_hallucinated": report.get("total_hallucinated", 0),
        "avg_sources_per_answer": report.get("avg_sources_per_answer", 0),
        "latency_p50_ms": report.get("latency_p50_ms", 0),
        "latency_p95_ms": report.get("latency_p95_ms", 0),
        "total_queries": report.get("total_queries", 0),
        "skipped": report.get("skipped", 0),
        "failed_queries": report.get("failed_queries", 0),
        "no_answer_accuracy": report.get("no_answer_accuracy"),
    }


# ── Quality gate checks ──


def check_quality_gates(
    image_report: dict,
    rag_metrics: dict,
    human_avg_score: float | None = None,
) -> dict[str, bool | str]:
    gates: dict[str, bool | str] = {}
    sem = image_report.get("semantic", {})
    image_available = bool(image_report.get("queries", 0)) and not image_report.get("error")
    rag_available = bool(rag_metrics.get("total_queries", 0)) and not rag_metrics.get("error")

    # Image search gates
    gates["recall_5 >= 0.70"] = image_available and sem.get("recall_5", 0) >= 0.70
    gates["mrr >= 0.55"] = image_available and sem.get("mrr", 0) >= 0.55
    gates["no duplicate posts"] = (
        image_available and image_report.get("semantic_duplicate_queries", 0) == 0
    )

    # RAG gates
    gates["all labeled RAG queries evaluated"] = (
        rag_available
        and rag_metrics.get("skipped", 0) == 0
        and rag_metrics.get("failed_queries", 0) == 0
    )
    gates["source_hit_rate >= 0.80"] = (
        rag_available and rag_metrics.get("source_hit_rate", 0) >= 0.80
    )
    gates["citation_accessible 100%"] = (
        rag_available and rag_metrics.get("citation_accessible_rate", 0) >= 1.0
    )
    gates["hallucinated citations 0"] = (
        rag_available and rag_metrics.get("total_hallucinated", 0) == 0
    )

    # Human scoring
    gates["human_relevance_avg >= 4.0"] = (
        human_avg_score >= 4.0 if human_avg_score is not None else "PENDING"
    )

    return gates


def _machine_quality_gates(image_report: dict, rag_metrics: dict) -> dict[str, bool]:
    return {
        name: value
        for name, value in check_quality_gates(image_report, rag_metrics).items()
        if name != "human_relevance_avg >= 4.0"
    }


def _dataset_sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def _safe_error(exc: Exception) -> str:
    return f"{type(exc).__name__}: request failed"


def _read_local_env(name: str, default: str = "") -> str:
    """Read one simple .env value without sourcing or printing the file."""
    path = Path(__file__).resolve().parents[3] / ".env"
    try:
        for line in path.read_text(encoding="utf-8").splitlines():
            if not line.startswith(f"{name}="):
                continue
            value = line.split("=", 1)[1].strip()
            if len(value) >= 2 and value[0] == value[-1] and value[0] in "'\"":
                value = value[1:-1]
            return value
    except OSError:
        pass
    return default


def _runtime_metadata(ai_base_url: str) -> dict[str, Any]:
    fallback = (
        "shareo-local-internal"
        if ai_base_url.startswith(("http://127.0.0.1:", "http://localhost:"))
        else "shareo-dev-internal"
    )
    token = _read_local_env("SHAREO_INTERNAL_TOKEN", fallback)
    metadata: dict[str, Any] = {"status": "unavailable", "ai_base_url": ai_base_url}
    try:
        with httpx.Client(base_url=ai_base_url, timeout=10.0, trust_env=False) as client:
            for name, path in (
                ("image_search", "/readyz/image-search"),
                ("rag", "/readyz/rag"),
            ):
                response = client.get(path, headers={"X-Internal-Token": token})
                if response.status_code != 200:
                    metadata[name] = {"status": "unavailable", "http_status": response.status_code}
                    continue
                payload = response.json()
                metadata[name] = payload if isinstance(payload, dict) else {"status": "invalid"}
        metadata["status"] = (
            "ready"
            if all(
                metadata.get(name, {}).get("status") == "ready" for name in ("image_search", "rag")
            )
            else "degraded"
        )
    except (httpx.HTTPError, ValueError, TypeError) as exc:
        metadata["reason"] = type(exc).__name__
    return metadata


def _split_markdown_row(line: str) -> list[str]:
    cells: list[str] = []
    current: list[str] = []
    escaped = False
    content = line.strip()
    if content.startswith("|"):
        content = content[1:]
    if content.endswith("|"):
        content = content[:-1]
    for char in content:
        if char == "|" and not escaped:
            cells.append("".join(current).strip())
            current = []
            continue
        if char == "\\" and not escaped:
            escaped = True
            continue
        current.append(char)
        escaped = False
    cells.append("".join(current).strip())
    return cells


def _load_human_scores(path: Path, expected_count: int) -> tuple[list[int], float]:
    raw = path.read_text(encoding="utf-8")
    scores: list[Any]
    try:
        payload = json.loads(raw)
        if isinstance(payload, dict):
            scores = payload.get("scores", [])
        elif isinstance(payload, list):
            scores = payload
        else:
            scores = []
    except json.JSONDecodeError:
        scores = []
        for line in raw.splitlines():
            if not line.lstrip().startswith("|"):
                continue
            cells = _split_markdown_row(line)
            if not cells or not cells[0].isdigit() or len(cells) < 2:
                continue
            scores.append(cells[-1])

    if len(scores) != expected_count:
        raise ValueError(f"expected {expected_count} human scores, got {len(scores)}")

    normalized: list[int] = []
    for index, score in enumerate(scores, 1):
        if isinstance(score, bool):
            raise ValueError(f"score {index} must be an integer from 1 to 5")
        try:
            value = int(score)
        except (TypeError, ValueError) as exc:
            raise ValueError(f"score {index} must be an integer from 1 to 5") from exc
        if value < 1 or value > 5 or str(score).strip() != str(value):
            raise ValueError(f"score {index} must be an integer from 1 to 5")
        normalized.append(value)
    return normalized, round(statistics.fmean(normalized), 4)


def _write_report(path: Path, results: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    output_data = {key: value for key, value in results.items()}
    if "rag" in output_data and "scoring_template" in output_data["rag"]:
        del output_data["rag"]["scoring_template"]
    path.write_text(json.dumps(output_data, ensure_ascii=False, indent=2), encoding="utf-8")


def _print_quality_gates(gates: dict[str, bool | str]) -> bool:
    print("── Quality Gates ──")
    all_passed = True
    for gate_name, passed in gates.items():
        status = "PASS" if passed is True else str(passed)
        print(f"  [{status}] {gate_name}")
        if passed is not True:
            all_passed = False
    print()
    return all_passed


# ── Main command ──


def _build_report_environment(args: argparse.Namespace) -> dict[str, Any]:
    repo_root = Path(__file__).resolve().parents[3]
    environment: dict[str, Any] = {
        "date": datetime.now(timezone.utc).strftime("%Y-%m-%d"),
        "base_url": args.base_url,
        "image_dataset": str(args.image_dataset),
        "rag_dataset": str(args.rag_dataset),
        "image_dataset_sha256": _dataset_sha256(args.image_dataset)
        if args.image_dataset.exists()
        else "missing",
        "rag_dataset_sha256": _dataset_sha256(args.rag_dataset)
        if args.rag_dataset.exists()
        else "missing",
        "python_version": sys.version.split()[0],
        "platform": platform.platform(),
        "embedding_model": settings.embedding_model,
        "embedding_revision": settings.embedding_revision,
        "embedding_device": settings.embedding_device,
        "text_embedding_model": settings.text_embedding_model,
        "rag_top_k": settings.rag_top_k,
        "rag_max_sources": settings.rag_max_sources,
        "rag_prompt_version": settings.rag_prompt_version,
        "llm_model": settings.llm_model,
        "network_environment": os.environ.get("SHAREO_EVAL_NETWORK", "unspecified"),
        "ai_base_url": args.ai_base_url,
    }
    runtime = _runtime_metadata(args.ai_base_url)
    environment["runtime_metadata"] = runtime
    image_status = runtime.get("image_search", {})
    image_model = image_status.get("model", {}) if isinstance(image_status, dict) else {}
    rag_status = runtime.get("rag", {})
    rag_model = rag_status.get("model", {}) if isinstance(rag_status, dict) else {}
    if isinstance(image_model, dict):
        environment["embedding_model"] = image_model.get("model", environment["embedding_model"])
        environment["embedding_revision"] = image_model.get(
            "revision", environment["embedding_revision"]
        )
        environment["embedding_device"] = image_model.get("device", environment["embedding_device"])
    if isinstance(rag_model, dict):
        environment["text_embedding_model"] = rag_model.get(
            "model", environment["text_embedding_model"]
        )
    if isinstance(rag_status, dict):
        environment["llm_model"] = rag_status.get("llm_model", environment["llm_model"])
        environment["rag_prompt_version"] = rag_status.get(
            "prompt_version", environment["rag_prompt_version"]
        )
    try:
        environment["git_sha"] = subprocess.check_output(
            ["git", "rev-parse", "HEAD"], cwd=repo_root, text=True
        ).strip()
        environment["git_dirty"] = bool(
            subprocess.check_output(
                ["git", "status", "--porcelain"], cwd=repo_root, text=True
            ).strip()
        )
    except Exception:
        environment["git_sha"] = "unknown"
        environment["git_dirty"] = True
    return environment


def _apply_human_scoring(
    results: dict[str, Any], scoring_input: Path | None, machine_only: bool = False
) -> tuple[dict[str, bool | str], bool]:
    image_report = results.get("image_search", {}).get("report", {})
    rag_metrics = results.get("rag", {}).get("metrics", {})
    machine_gates = _machine_quality_gates(image_report, rag_metrics)
    machine_passed = all(value is True for value in machine_gates.values())
    results["machine_quality_gates"] = machine_gates
    results["machine_quality_gate_passed"] = machine_passed
    human_avg_score: float | None = None
    if machine_only:
        results["human_scoring"] = {"status": "SKIPPED"}
    elif not machine_passed:
        results["human_scoring"] = {"status": "BLOCKED_MACHINE_GATES"}
    elif scoring_input is None:
        results["human_scoring"] = {"status": "PENDING"}
    else:
        expected_count = int(rag_metrics.get("total_queries", 0))
        try:
            scores, human_avg_score = _load_human_scores(scoring_input, expected_count)
            results["human_scoring"] = {
                "status": "complete",
                "input": str(scoring_input),
                "count": len(scores),
                "average": human_avg_score,
            }
        except (OSError, ValueError) as exc:
            results["human_scoring"] = {
                "status": "FAIL",
                "input": str(scoring_input),
                "error": str(exc),
            }

    gates = check_quality_gates(image_report, rag_metrics, human_avg_score)
    results["quality_gates"] = gates
    results["quality_gate_passed"] = (
        machine_passed if machine_only else all(value is True for value in gates.values())
    )
    return gates, results["quality_gate_passed"]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--base-url", default=os.environ.get("SHAREO_BASE_URL", "http://127.0.0.1:8080")
    )
    parser.add_argument(
        "--ai-base-url", default=os.environ.get("SHAREO_AI_BASE_URL", "http://127.0.0.1:8000")
    )
    parser.add_argument(
        "--image-dataset",
        default=os.environ.get(
            "SHAREO_IMAGE_DATASET",
            str(
                Path(__file__).resolve().parent.parent.parent.parent
                / "docs"
                / "eval"
                / "image_search_v1.jsonl"
            ),
        ),
        type=Path,
    )
    parser.add_argument(
        "--rag-dataset",
        default=os.environ.get(
            "SHAREO_RAG_DATASET",
            str(
                Path(__file__).resolve().parent.parent.parent.parent
                / "docs"
                / "eval"
                / "rag_qa_v1.jsonl"
            ),
        ),
        type=Path,
    )
    parser.add_argument(
        "--image-only",
        action="store_true",
        help="Only run image search evaluation",
    )
    parser.add_argument(
        "--rag-only",
        action="store_true",
        help="Only run RAG evaluation",
    )
    parser.add_argument(
        "--test-username",
        default="eval_rag_user",
        help="Test user username for RAG evaluation",
    )
    parser.add_argument(
        "--test-password",
        default="eval_rag_test_2024",
        help="Test user password for RAG evaluation",
    )
    parser.add_argument(
        "--output",
        type=Path,
        help="Write full report to JSON file",
    )
    parser.add_argument(
        "--scoring-output",
        type=Path,
        default=None,
        help="Write human scoring template to file",
    )
    parser.add_argument(
        "--scoring-input",
        type=Path,
        default=None,
        help="Read completed Markdown or JSON human scores",
    )
    parser.add_argument(
        "--report-input",
        type=Path,
        default=None,
        help="Reuse a previous JSON report and only re-evaluate quality gates",
    )
    parser.add_argument(
        "--machine-only",
        action="store_true",
        help="evaluate machine gates without requiring human scores",
    )
    args = parser.parse_args()

    if args.report_input and not args.output:
        parser.error("--output is required with --report-input")

    if args.report_input:
        try:
            results = json.loads(args.report_input.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as exc:
            print(f"Failed to load report: {type(exc).__name__}", file=sys.stderr)
            return 1
        try:
            gates, all_gates_passed = _apply_human_scoring(
                results, args.scoring_input, machine_only=args.machine_only
            )
        except Exception as exc:
            print(f"Failed to finalize report: {_safe_error(exc)}", file=sys.stderr)
            return 1
        _print_quality_gates(results["machine_quality_gates"] if args.machine_only else gates)
        _write_report(args.output, results)
        print(f"  Final report written to {args.output}")
        return 0 if all_gates_passed else 1

    run_image = not args.rag_only
    run_rag = not args.image_only

    report_env = _build_report_environment(args)

    print("=" * 60)
    print("  ShareO AI Evaluation — Phase 7B")
    print("=" * 60)
    print(f"  Base URL:  {args.base_url}")
    print(f"  Git SHA:   {report_env['git_sha']}")
    print(f"  Date:      {report_env['date']}")
    print()

    results: dict[str, Any] = {"environment": report_env}
    all_gates_passed = False

    # ── Image search evaluation ──
    if run_image:
        print("── Image Search Evaluation ──")
        try:
            image_queries = load_image_queries(args.image_dataset)
            print(f"  Loaded {len(image_queries)} queries")
            image_report = evaluate_image_search(args.base_url, image_queries)
            image_metrics = _export_image_metrics(image_report)
            results["image_search"] = {"report": image_report, "metrics": image_metrics}
            print(f"  Recall@5:      {image_metrics['recall_5']:.4f}  (target ≥ 0.70)")
            print(f"  Recall@10:     {image_metrics['recall_10']:.4f}")
            print(f"  MRR:           {image_metrics['mrr']:.4f}  (target ≥ 0.55)")
            print(f"  Duplicates:    {image_metrics['duplicate_queries']}  (target 0)")
            print(f"  Latency P50:   {image_metrics['latency_p50_ms']:.0f} ms")
            print(f"  Latency P95:   {image_metrics['latency_p95_ms']:.0f} ms")
        except Exception as exc:
            results["image_search"] = {"error": _safe_error(exc), "report": {}}
            print(f"  FAIL: image search evaluation ({type(exc).__name__})")
        print()

    # ── RAG evaluation ──
    if run_rag:
        print("── RAG Bot Evaluation ──")
        try:
            rag_queries = load_rag_queries(args.rag_dataset)
            labeled = [q for q in rag_queries if q.get("label_status") != "pending"]
            print(
                f"  Loaded {len(rag_queries)} questions ({len(labeled)} labeled, {len(rag_queries) - len(labeled)} pending)"
            )
            rag_result = evaluate_rag(
                args.base_url,
                rag_queries,
                test_username=args.test_username,
                test_password=args.test_password,
            )
            rag_metrics = _export_rag_metrics(rag_result)
            results["rag"] = {
                "report": rag_result.get("report", {}),
                "metrics": rag_metrics,
                "scoring_template": rag_result.get("scoring_template", []),
            }
            print(
                f"  Source Hit Rate:    {rag_metrics.get('source_hit_rate', 0):.4f}  (target ≥ 0.80)"
            )
            print(
                f"  Citation Access:    {rag_metrics.get('citation_accessible_rate', 0):.1%}  (target 100%)"
            )
            print(f"  Hallucinated Cites: {rag_metrics.get('total_hallucinated', 0)}  (target 0)")
            print(f"  Failed Queries:     {rag_metrics.get('failed_queries', 0)}  (target 0)")
            print(f"  Avg Sources/Answer: {rag_metrics.get('avg_sources_per_answer', 0):.1f}")
            print(f"  Latency P50:        {rag_metrics.get('latency_p50_ms', 0):.0f} ms")
            print(f"  Latency P95:        {rag_metrics.get('latency_p95_ms', 0):.0f} ms")
        except Exception as exc:
            results["rag"] = {"error": _safe_error(exc), "report": {}, "metrics": {}}
            print(f"  FAIL: RAG evaluation ({type(exc).__name__})")
        print()

    # ── Quality gates ──
    if run_image and run_rag:
        gates, all_gates_passed = _apply_human_scoring(
            results, args.scoring_input, machine_only=args.machine_only
        )
        _print_quality_gates(results["machine_quality_gates"] if args.machine_only else gates)
    else:
        results["quality_gates"] = {"full evaluation required": False}
        results["quality_gate_passed"] = False
        print("── Quality Gates ──")
        print("  [FAIL] full evaluation required")
        print()

    # ── Human scoring template ──
    if run_rag and "scoring_template" in results.get("rag", {}):
        scoring_rows = results["rag"]["scoring_template"]
        if scoring_rows and results.get("machine_quality_gate_passed") and not args.machine_only:
            scoring_path = args.scoring_output or Path("eval_ai_human_scoring.md")
            _write_scoring_template(scoring_path, scoring_rows, report_env)
            print(f"  Human scoring template written to {scoring_path}")
            print(
                "  Please score each answer 1-5, then rerun with --report-input and --scoring-input."
            )

    # ── Write output ──
    if args.output:
        _write_report(args.output, results)
        print(f"  Full report written to {args.output}")

    # ── Summary ──
    print()
    print("=" * 60)
    if all_gates_passed:
        print("  ALL QUALITY GATES PASSED")
    else:
        print("  SOME QUALITY GATES FAILED — review output above")
    print("=" * 60)
    return 0 if all_gates_passed else 1


def _write_scoring_template(path: Path, rows: list[dict], env: dict) -> None:
    """Write human scoring template as Markdown."""
    lines = [
        "# RAG Answer Human Scoring",
        "",
        f"> Date: {env['date']} | Git: {env['git_sha']}",
        "",
        "Scoring criteria (1-5):",
        "- 5: Answer is accurate and complete, citations are appropriate",
        "- 4: Answer is mostly correct, minor issues only",
        "- 3: Partially relevant, missing important information",
        "- 2: Marginally relevant, major errors",
        "- 1: Completely irrelevant or fabricated",
        "",
        "| # | Question | Expected | Expected Source IDs | Bot Answer | Citation Post IDs | Citation Details | Hit Rate | Score |",
        "|---|---|---|---|---|---|---|---|---|",
    ]

    for i, row in enumerate(rows, 1):

        def cell(value: Any, limit: int) -> str:
            return str(value)[:limit].replace("\n", " ").replace("|", "\\|")

        question = cell(row["question"], 60)
        expected = cell(row.get("expected_answer", ""), 80)
        expected_ids = cell(row.get("expected_source_post_ids", []), 40)
        bot_answer = cell(row.get("bot_answer", ""), 120)
        citations = cell(row.get("citation_post_ids", []), 40)
        citation_details = cell(
            json.dumps(row.get("citation_details", []), ensure_ascii=False), 180
        )
        hit_rate = row.get("source_hit_rate", "")
        score = row.get("relevance_score", "")
        lines.append(
            f"| {i} | {question} | {expected} | {expected_ids} | {bot_answer} | {citations} | {citation_details} | {hit_rate} | {score} |"
        )

    lines.extend(
        [
            "",
            f"Total: {len(rows)} questions",
            "Average score: ___ / 5.0",
            "Evaluator: _______________  Date: _______________",
        ]
    )

    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("\n".join(lines), encoding="utf-8")


if __name__ == "__main__":
    raise SystemExit(main())
