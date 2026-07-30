"""Evaluate keyword, semantic, and hybrid post search against graded labels."""

import argparse
import json
import math
import os
import statistics
import time
from pathlib import Path
from typing import Any

import httpx


def relevance_map(item: dict[str, Any]) -> dict[int, int]:
    if isinstance(item.get("relevance"), dict):
        return {int(key): int(value) for key, value in item["relevance"].items()}
    return {int(value): 2 for value in item.get("relevant_post_ids", [])}


def recall_at(ids: list[int], relevant: set[int], k: int) -> float:
    if not relevant:
        return 1.0 if not ids else 0.0
    return len(set(ids[:k]) & relevant) / len(relevant)


def reciprocal_rank(ids: list[int], relevant: set[int]) -> float:
    if not relevant:
        return 1.0 if not ids else 0.0
    for rank, post_id in enumerate(ids, 1):
        if post_id in relevant:
            return 1.0 / rank
    return 0.0


def precision_at(ids: list[int], relevant: set[int], k: int) -> float:
    if not relevant:
        return 1.0 if not ids else 0.0
    return len(set(ids[:k]) & relevant) / k


def ndcg_at(ids: list[int], relevance: dict[int, int], k: int) -> float:
    if not relevance:
        return 1.0 if not ids else 0.0
    dcg = sum(
        (2 ** relevance.get(post_id, 0) - 1) / math.log2(rank + 1)
        for rank, post_id in enumerate(ids[:k], 1)
    )
    ideal = sorted(relevance.values(), reverse=True)[:k]
    idcg = sum((2**grade - 1) / math.log2(rank + 1) for rank, grade in enumerate(ideal, 1))
    return dcg / idcg if idcg else 0.0


def percentile(values: list[float], fraction: float) -> float:
    ordered = sorted(values)
    index = min(len(ordered) - 1, max(0, round((len(ordered) - 1) * fraction)))
    return ordered[index]


def load_queries(path: Path) -> list[dict[str, Any]]:
    queries = [json.loads(line) for line in path.read_text().splitlines() if line.strip()]
    if not queries:
        raise RuntimeError("post search evaluation set is empty")
    for item in queries:
        if not str(item.get("query", "")).strip():
            raise RuntimeError("every post search row needs a query")
        grades = relevance_map(item)
        if item.get("expect_no_results", False):
            if grades:
                raise RuntimeError("no-result query cannot contain relevance labels")
        elif not grades:
            raise RuntimeError(f"query is unlabeled: {item['query']}")
        if any(value not in (1, 2) for value in grades.values()):
            raise RuntimeError("graded relevance values must be 1 or 2")
    return queries


def _ids(mode: str, body: dict[str, Any]) -> list[int]:
    data = body.get("data", body)
    items = data.get("list", []) if mode == "hybrid" else data.get("results", data.get("items", []))
    return [int(item.get("post_id", item.get("id"))) for item in items]


def evaluate(
    base_url: str,
    ai_url: str,
    internal_token: str,
    queries: list[dict[str, Any]],
) -> dict[str, Any]:
    metrics = {
        mode: {
            "recall_5": [],
            "mrr": [],
            "ndcg_5": [],
            "precision_3": [],
            "no_match": [],
            "latency_ms": [],
        }
        for mode in ("keyword", "semantic", "hybrid")
    }
    rows: list[dict[str, Any]] = []
    headers = {"X-Internal-Token": internal_token}
    with (
        httpx.Client(base_url=base_url, timeout=15.0, trust_env=False) as go,
        httpx.Client(base_url=ai_url, timeout=15.0, trust_env=False) as ai,
    ):
        for item in queries:
            grades = relevance_map(item)
            relevant = set(grades)
            query_row = {"id": item.get("id", ""), "query": item["query"], "modes": {}}
            requests = (
                (
                    "keyword",
                    go,
                    "GET",
                    "/internal/posts/agent/search",
                    {"params": {"q": item["query"], "limit": 10}, "headers": headers},
                ),
                (
                    "semantic",
                    ai,
                    "POST",
                    "/v1/search/posts",
                    {"json": {"query": item["query"], "limit": 20}, "headers": headers},
                ),
                (
                    "hybrid",
                    go,
                    "GET",
                    "/api/v1/search",
                    {"params": {"q": item["query"], "page": 1, "page_size": 20}},
                ),
            )
            for mode, client, method, path, kwargs in requests:
                started = time.perf_counter()
                response = client.request(method, path, **kwargs)
                response.raise_for_status()
                latency = (time.perf_counter() - started) * 1000
                ids = _ids(mode, response.json())
                values = metrics[mode]
                values["recall_5"].append(recall_at(ids, relevant, 5))
                values["mrr"].append(reciprocal_rank(ids, relevant))
                values["ndcg_5"].append(ndcg_at(ids, grades, 5))
                values["precision_3"].append(precision_at(ids, relevant, 3))
                if item.get("expect_no_results", False):
                    values["no_match"].append(1.0 if not ids else 0.0)
                values["latency_ms"].append(latency)
                query_row["modes"][mode] = {"post_ids": ids, "latency_ms": latency}
            rows.append(query_row)
    report: dict[str, Any] = {"queries": len(queries), "results": rows}
    for mode, values in metrics.items():
        no_match = values["no_match"]
        report[mode] = {
            "recall_5": statistics.fmean(values["recall_5"]),
            "mrr": statistics.fmean(values["mrr"]),
            "ndcg_5": statistics.fmean(values["ndcg_5"]),
            "precision_3": statistics.fmean(values["precision_3"]),
            "no_match_accuracy": statistics.fmean(no_match) if no_match else None,
            "latency_p50_ms": percentile(values["latency_ms"], 0.50),
            "latency_p95_ms": percentile(values["latency_ms"], 0.95),
        }
    hybrid = report["hybrid"]
    report["quality_gates"] = {
        "recall_5 >= 0.80": hybrid["recall_5"] >= 0.80,
        "mrr >= 0.65": hybrid["mrr"] >= 0.65,
        "ndcg_5 >= 0.70": hybrid["ndcg_5"] >= 0.70,
        "no_match_accuracy >= 0.80": (
            hybrid["no_match_accuracy"] is not None and hybrid["no_match_accuracy"] >= 0.80
        ),
    }
    report["quality_gate_passed"] = all(report["quality_gates"].values())
    return report


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://127.0.0.1:8080")
    parser.add_argument("--ai-url", default="http://127.0.0.1:8000")
    parser.add_argument(
        "--dataset",
        type=Path,
        default=Path("../.local/shareo/eval/post_search_v1.jsonl"),
    )
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    token = os.environ.get("SHAREO_INTERNAL_TOKEN") or os.environ.get(
        "SHAREO_AI_INTERNAL_TOKEN", ""
    )
    if not token:
        raise SystemExit("SHAREO_INTERNAL_TOKEN is required")
    report = evaluate(args.base_url, args.ai_url, token, load_queries(args.dataset))
    rendered = json.dumps(report, ensure_ascii=False, indent=2)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered + "\n", encoding="utf-8")
    print(rendered)
    raise SystemExit(0 if report["quality_gate_passed"] else 1)


if __name__ == "__main__":
    main()
