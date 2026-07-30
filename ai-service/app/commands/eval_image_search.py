"""Evaluate semantic image search with an independent graded query set."""

import argparse
import json
import math
import statistics
import time
from pathlib import Path

import httpx


def reciprocal_rank(ids: list[int], relevant: set[int]) -> float:
    for rank, post_id in enumerate(ids, 1):
        if post_id in relevant:
            return 1.0 / rank
    return 0.0


def recall_at(ids: list[int], relevant: set[int], k: int) -> float:
    return len(set(ids[:k]) & relevant) / len(relevant)


def relevance_map(item: dict) -> dict[int, int]:
    if isinstance(item.get("relevance"), dict):
        return {int(key): int(value) for key, value in item["relevance"].items()}
    return {int(value): 2 for value in item.get("relevant_post_ids", [])}


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


def load_queries(path: Path) -> list[dict]:
    queries = [json.loads(line) for line in path.read_text().splitlines() if line.strip()]
    unlabeled = [
        item["query"]
        for item in queries
        if not relevance_map(item) and not item.get("expect_no_results", False)
    ]
    if unlabeled:
        raise RuntimeError(
            f"evaluation set has {len(unlabeled)} unlabeled queries; label relevance first"
        )
    return queries


def evaluate(
    base_url: str,
    queries: list[dict],
    score_threshold: float | None = None,
) -> dict:
    metrics = {
        "recall_5": [],
        "recall_10": [],
        "mrr": [],
        "precision_3": [],
        "ndcg_5": [],
        "manual_relevance_10": [],
        "latency_ms": [],
        "no_match": [],
    }
    duplicate_queries = 0
    query_results = []
    with httpx.Client(base_url=base_url, timeout=10.0) as client:
        for item in queries:
            relevance = relevance_map(item)
            relevant = set(relevance)
            started = time.perf_counter()
            response = client.get(
                "/api/v1/search/images",
                params={"q": item["query"], "limit": 20},
            )
            response.raise_for_status()
            elapsed = (time.perf_counter() - started) * 1000
            results = response.json()["data"]["results"]
            if score_threshold is not None:
                results = [
                    result
                    for result in results
                    if float(result.get("score") or 0.0) >= score_threshold
                ]
            ids = [int(result["post_id"]) for result in results]
            if len(ids) != len(set(ids)):
                duplicate_queries += 1

            if item.get("expect_no_results", False):
                no_result_score = 1.0 if not ids else 0.0
                metrics["no_match"].append(no_result_score)
            else:
                metrics["recall_5"].append(recall_at(ids, relevant, 5))
                metrics["recall_10"].append(recall_at(ids, relevant, 10))
                metrics["mrr"].append(reciprocal_rank(ids, relevant))
                metrics["precision_3"].append(precision_at(ids, relevant, 3))
                metrics["ndcg_5"].append(ndcg_at(ids, relevance, 5))

            top_ten = ids[:10]
            if not item.get("expect_no_results", False):
                manual_relevance = len(set(top_ten) & relevant) / len(top_ten) if top_ten else 0.0
                metrics["manual_relevance_10"].append(manual_relevance)
            metrics["latency_ms"].append(elapsed)
            query_results.append(
                {
                    "id": item.get("id"),
                    "query": item["query"],
                    "category": item.get("category"),
                    "expected_post_ids": sorted(relevant),
                    "actual_post_ids": ids,
                    "top_scores": [float(result.get("score") or 0.0) for result in results[:10]],
                    "latency_ms": elapsed,
                }
            )

    semantic = {
        "recall_5": statistics.fmean(metrics["recall_5"]),
        "recall_10": statistics.fmean(metrics["recall_10"]),
        "mrr": statistics.fmean(metrics["mrr"]),
        "precision_at_3": statistics.fmean(metrics["precision_3"]),
        "ndcg_at_5": statistics.fmean(metrics["ndcg_5"]),
        "manual_relevance_at_10": statistics.fmean(metrics["manual_relevance_10"]),
        "no_match_accuracy": (
            statistics.fmean(metrics["no_match"]) if metrics["no_match"] else 1.0
        ),
        "latency_p50_ms": percentile(metrics["latency_ms"], 0.50),
        "latency_p95_ms": percentile(metrics["latency_ms"], 0.95),
    }
    return {
        "queries": len(queries),
        "score_threshold": score_threshold,
        "semantic_duplicate_queries": duplicate_queries,
        "semantic": semantic,
        "query_results": query_results,
        "quality_gate_passed": (
            semantic["recall_10"] >= 0.75
            and semantic["mrr"] >= 0.55
            and semantic["no_match_accuracy"] >= 0.80
            and duplicate_queries == 0
        ),
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://127.0.0.1:8080")
    parser.add_argument(
        "--dataset", default="../.local/shareo/eval/image_search_local_v1.jsonl", type=Path
    )
    parser.add_argument("--output", type=Path)
    parser.add_argument("--score-threshold", type=float)
    args = parser.parse_args()
    report = evaluate(
        args.base_url,
        load_queries(args.dataset),
        score_threshold=args.score_threshold,
    )
    rendered = json.dumps(report, ensure_ascii=False, indent=2)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered + "\n", encoding="utf-8")
    print(rendered)


if __name__ == "__main__":
    main()
