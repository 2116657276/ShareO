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
import json
import os
import statistics
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

import httpx

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
                scoring_rows.append(
                    {
                        "question": question,
                        "category": category,
                        "expected_answer": item.get("reference_answer", ""),
                        "bot_answer": "SEND_FAILED",
                        "citations": [],
                        "citation_post_ids": [],
                        "relevance_score": "",
                    }
                )
                continue

            # Wait for bot reply
            bot_message = _wait_for_bot_reply(client, token, conv_id, source_id, timeout=120)
            elapsed = (time.perf_counter() - started) * 1000
            metrics["total_latency_ms"].append(elapsed)

            if not bot_message:
                scoring_rows.append(
                    {
                        "question": question,
                        "category": category,
                        "expected_answer": item.get("reference_answer", ""),
                        "bot_answer": "NO_REPLY",
                        "citations": [],
                        "citation_post_ids": [],
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
                if not citation_post_ids and not bot_content.strip():
                    no_answer_correct += 1
                hits = 1.0 if not citation_post_ids else 0.0
            elif expected_ids:
                hit_ids = set(citation_post_ids) & expected_ids
                hits = len(hit_ids) / len(expected_ids) if expected_ids else 1.0
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
                    "bot_answer": bot_content[:500],
                    "citations": [c.get("post_id") for c in citations],
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
    }


# ── Quality gate checks ──


def check_quality_gates(
    image_report: dict,
    rag_metrics: dict,
    human_avg_score: float | None = None,
) -> dict[str, bool | str]:
    gates: dict[str, bool | str] = {}
    sem = image_report.get("semantic", {})

    # Image search gates
    gates["recall_5 >= 0.70"] = sem.get("recall_5", 0) >= 0.70
    gates["mrr >= 0.55"] = sem.get("mrr", 0) >= 0.55
    gates["no duplicate posts"] = image_report.get("semantic_duplicate_queries", 0) == 0

    # RAG gates
    gates["source_hit_rate >= 0.80"] = rag_metrics.get("source_hit_rate", 0) >= 0.80
    gates["citation_accessible 100%"] = rag_metrics.get("citation_accessible_rate", 0) >= 1.0
    gates["hallucinated citations 0"] = rag_metrics.get("total_hallucinated", 0) == 0

    # Human scoring
    if human_avg_score is not None:
        gates["human_relevance_avg >= 4.0"] = human_avg_score >= 4.0

    return gates


# ── Main command ──


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--base-url", default=os.environ.get("SHAREO_BASE_URL", "http://127.0.0.1:8080")
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
        help="Write human scoring template to file (default: eval_ai_human_scoring.md in cwd)",
    )
    args = parser.parse_args()

    run_image = not args.rag_only
    run_rag = not args.image_only

    # Environment info
    report_env = {
        "date": datetime.now(timezone.utc).strftime("%Y-%m-%d"),
        "base_url": args.base_url,
        "image_dataset": str(args.image_dataset),
        "rag_dataset": str(args.rag_dataset),
        "python_version": os.environ.get("PYTHON_VERSION", ""),
    }

    # Collect git info if available
    import subprocess

    try:
        report_env["git_sha"] = subprocess.check_output(
            ["git", "rev-parse", "HEAD"], text=True
        ).strip()
    except Exception:
        report_env["git_sha"] = "unknown"

    print("=" * 60)
    print("  ShareO AI Evaluation — Phase 7B")
    print("=" * 60)
    print(f"  Base URL:  {args.base_url}")
    print(f"  Git SHA:   {report_env['git_sha']}")
    print(f"  Date:      {report_env['date']}")
    print()

    results: dict[str, Any] = {"environment": report_env}
    all_gates_passed = True

    # ── Image search evaluation ──
    if run_image:
        print("── Image Search Evaluation ──")
        image_queries = load_image_queries(args.image_dataset)
        print(f"  Loaded {len(image_queries)} queries")
        image_report = evaluate_image_search(args.base_url, image_queries)
        image_metrics = _export_image_metrics(image_report)
        results["image_search"] = {
            "report": image_report,
            "metrics": image_metrics,
        }

        print(f"  Recall@5:      {image_metrics['recall_5']:.4f}  (target ≥ 0.70)")
        print(f"  Recall@10:     {image_metrics['recall_10']:.4f}")
        print(f"  MRR:           {image_metrics['mrr']:.4f}  (target ≥ 0.55)")
        print(f"  Duplicates:    {image_metrics['duplicate_queries']}  (target 0)")
        print(f"  Latency P50:   {image_metrics['latency_p50_ms']:.0f} ms")
        print(f"  Latency P95:   {image_metrics['latency_p95_ms']:.0f} ms")
        print()

    # ── RAG evaluation ──
    if run_rag:
        print("── RAG Bot Evaluation ──")
        if not args.rag_dataset.exists():
            print(f"  SKIP: dataset not found at {args.rag_dataset}")
            results["rag"] = {"error": "dataset not found"}
        else:
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
            print(f"  Avg Sources/Answer: {rag_metrics.get('avg_sources_per_answer', 0):.1f}")
            print(f"  Latency P50:        {rag_metrics.get('latency_p50_ms', 0):.0f} ms")
            print(f"  Latency P95:        {rag_metrics.get('latency_p95_ms', 0):.0f} ms")
            print()

    # ── Quality gates ──
    if run_image and run_rag and "error" not in results.get("rag", {}):
        rag_m = results.get("rag", {}).get("metrics", {})
        img_r = results.get("image_search", {}).get("report", {})
        gates = check_quality_gates(img_r, rag_m)
        results["quality_gates"] = gates

        print("── Quality Gates ──")
        for gate_name, passed in gates.items():
            status = "PASS" if passed else "FAIL"
            print(f"  [{status}] {gate_name}")
            if not passed:
                all_gates_passed = False
        print()

    # ── Human scoring template ──
    if run_rag and "scoring_template" in results.get("rag", {}):
        scoring_rows = results["rag"]["scoring_template"]
        if scoring_rows:
            scoring_path = args.scoring_output or Path("eval_ai_human_scoring.md")
            _write_scoring_template(scoring_path, scoring_rows, report_env)
            print(f"  Human scoring template written to {scoring_path}")
            print("  Please score each answer 1-5 and update relevance_score.")

    # ── Write output ──
    if args.output:
        # Strip scoring template for the JSON output (too verbose)
        output_data = {k: v for k, v in results.items()}
        if "rag" in output_data and "scoring_template" in output_data["rag"]:
            del output_data["rag"]["scoring_template"]
        args.output.write_text(json.dumps(output_data, ensure_ascii=False, indent=2))
        print(f"  Full report written to {args.output}")

    # ── Summary ──
    print()
    print("=" * 60)
    if all_gates_passed:
        print("  ALL QUALITY GATES PASSED")
    else:
        print("  SOME QUALITY GATES FAILED — review output above")
    print("=" * 60)


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
        "| # | Question | Expected | Bot Answer | Citations | Hit Rate | Score |",
        "|---|---|---|---|---|---|---|",
    ]

    for i, row in enumerate(rows, 1):
        question = row["question"][:60]
        expected = row.get("expected_answer", "")[:80]
        bot_answer = row.get("bot_answer", "")[:100].replace("\n", " ").replace("|", "\\|")
        citations = ", ".join(str(c) for c in row.get("citation_post_ids", []))
        hit_rate = row.get("source_hit_rate", "")
        score = row.get("relevance_score", "")
        lines.append(
            f"| {i} | {question} | {expected} | {bot_answer} | {citations} | {hit_rate} | {score} |"
        )

    lines.extend(
        [
            "",
            f"Total: {len(rows)} questions",
            "Average score: ___ / 5.0",
            "Evaluator: _______________  Date: _______________",
        ]
    )

    path.write_text("\n".join(lines))


if __name__ == "__main__":
    main()
