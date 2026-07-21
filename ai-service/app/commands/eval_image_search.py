"""Evaluate semantic image search against keyword search using labeled queries."""

import argparse
import json
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


def percentile(values: list[float], fraction: float) -> float:
    ordered = sorted(values)
    index = min(len(ordered) - 1, max(0, round((len(ordered) - 1) * fraction)))
    return ordered[index]


def load_queries(path: Path) -> list[dict]:
    queries = [json.loads(line) for line in path.read_text().splitlines() if line.strip()]
    unlabeled = [
        item["query"]
        for item in queries
        if not item.get("relevant_post_ids") and not item.get("expect_no_results", False)
    ]
    if unlabeled:
        raise RuntimeError(
            f"evaluation set has {len(unlabeled)} unlabeled queries; label relevant_post_ids first"
        )
    return queries


def evaluate(base_url: str, queries: list[dict]) -> dict:
    metrics = {
        "semantic": {
            "recall_5": [],
            "recall_10": [],
            "mrr": [],
            "manual_relevance_10": [],
            "latency_ms": [],
        },
        "keyword": {
            "recall_5": [],
            "recall_10": [],
            "mrr": [],
            "manual_relevance_10": [],
            "latency_ms": [],
        },
    }
    duplicate_queries = 0
    with httpx.Client(base_url=base_url, timeout=10.0) as client:
        for item in queries:
            relevant = {int(value) for value in item["relevant_post_ids"]}
            for mode, path, params in (
                ("semantic", "/api/v1/search/images", {"q": item["query"], "limit": 20}),
                ("keyword", "/api/v1/search", {"q": item["query"], "page_size": 20}),
            ):
                started = time.perf_counter()
                response = client.get(path, params=params)
                response.raise_for_status()
                elapsed = (time.perf_counter() - started) * 1000
                data = response.json()["data"]
                results = data["results"] if mode == "semantic" else data["list"]
                ids = [int(result.get("post_id", result.get("id"))) for result in results]
                if mode == "semantic" and len(ids) != len(set(ids)):
                    duplicate_queries += 1
                if item.get("expect_no_results", False):
                    no_result_score = 1.0 if not ids else 0.0
                    metrics[mode]["recall_5"].append(no_result_score)
                    metrics[mode]["recall_10"].append(no_result_score)
                    metrics[mode]["mrr"].append(no_result_score)
                else:
                    metrics[mode]["recall_5"].append(recall_at(ids, relevant, 5))
                    metrics[mode]["recall_10"].append(recall_at(ids, relevant, 10))
                    metrics[mode]["mrr"].append(reciprocal_rank(ids, relevant))
                top_ten = ids[:10]
                if item.get("expect_no_results", False):
                    manual_relevance = 1.0 if not top_ten else 0.0
                else:
                    manual_relevance = (
                        len(set(top_ten) & relevant) / len(top_ten) if top_ten else 0.0
                    )
                metrics[mode]["manual_relevance_10"].append(manual_relevance)
                metrics[mode]["latency_ms"].append(elapsed)

    report = {"queries": len(queries), "semantic_duplicate_queries": duplicate_queries}
    for mode, values in metrics.items():
        report[mode] = {
            "recall_5": statistics.fmean(values["recall_5"]),
            "recall_10": statistics.fmean(values["recall_10"]),
            "mrr": statistics.fmean(values["mrr"]),
            "manual_relevance_at_10": statistics.fmean(values["manual_relevance_10"]),
            "latency_p50_ms": percentile(values["latency_ms"], 0.50),
            "latency_p95_ms": percentile(values["latency_ms"], 0.95),
        }
    report["quality_gate_passed"] = (
        report["semantic"]["recall_10"] >= 0.75
        and report["semantic"]["mrr"] >= 0.55
        and duplicate_queries == 0
    )
    return report


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://127.0.0.1:8080")
    parser.add_argument("--dataset", default="../docs/eval/image_search_v1.jsonl", type=Path)
    args = parser.parse_args()
    print(
        json.dumps(
            evaluate(args.base_url, load_queries(args.dataset)),
            ensure_ascii=False,
            indent=2,
        )
    )


if __name__ == "__main__":
    main()
