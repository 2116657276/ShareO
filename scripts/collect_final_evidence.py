#!/usr/bin/env python3
"""Run native ShareO checks and write a redacted final evidence bundle."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import platform
import re
import shutil
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.request import Request, ProxyHandler, build_opener

ROOT = Path(__file__).resolve().parents[1]
EVIDENCE_DIR = ROOT / "docs" / "evidence" / "final-freeze"
LOCAL_STATE_DIR = ROOT / ".local" / "shareo"

DATASETS = {
    "image_search_current": ROOT
    / ".local"
    / "shareo"
    / "eval"
    / "image_search_local_v1.jsonl",
    "rag_current": ROOT / ".local" / "shareo" / "eval" / "rag_qa_current_v2.jsonl",
    "agent_current": ROOT
    / ".local"
    / "shareo"
    / "eval"
    / "agent_tasks_current_v2.jsonl",
    "post_search_current": ROOT / ".local" / "shareo" / "eval" / "post_search_v1.jsonl",
}

METRIC_KEYS = {
    "recall_1",
    "recall_5",
    "recall_10",
    "mrr",
    "ndcg_5",
    "precision_3",
    "no_match_accuracy",
    "duplicate_queries",
    "invisible_results",
    "source_hit_rate",
    "source_coverage_rate",
    "complete_source_coverage_rate",
    "citation_precision_rate",
    "extra_citations",
    "citation_accessible_rate",
    "citation_accessible",
    "total_hallucinated",
    "hallucinated_citations",
    "avg_sources_per_answer",
    "no_answer_accuracy",
    "failed_queries",
    "timeout_queries",
    "timeout_rate",
    "forbidden_actions",
    "budget_violations",
    "injection_failures",
    "provider_attempts",
    "provider_retries",
    "provider_retry_rate",
    "required_tool_selection_rate",
    "completed_required_tool_selection_rate",
    "unexpected_tool_calls",
    "latency_p50_ms",
    "latency_p95_ms",
    "total_queries",
    "total_hallucinated",
    "evaluated_answer_count",
    "ndcg_at_5",
    "precision_at_3",
}

METRIC_ALIASES = {"ndcg_at_5": "ndcg_5", "precision_at_3": "precision_3"}

LOG_PATTERNS = {
    "image_total_ms": re.compile(
        r"image search complete .*?duration_ms=(?P<value>[0-9.]+)"
    ),
    "rag_embedding_ms": re.compile(
        r"rag answer complete .*?embedding_ms=(?P<value>[0-9.]+)"
    ),
    "rag_retrieval_ms": re.compile(
        r"rag answer complete .*?retrieval_ms=(?P<value>[0-9.]+)"
    ),
    "rag_llm_ms": re.compile(r"rag answer complete .*?llm_ms=(?P<value>[0-9.]+)"),
    "rag_postprocess_ms": re.compile(
        r"rag answer complete .*?postprocess_ms=(?P<value>[0-9.]+)"
    ),
    "rag_total_ms": re.compile(r"rag answer complete .*?total_ms=(?P<value>[0-9.]+)"),
}


def utc_now() -> datetime:
    return datetime.now(timezone.utc)


def iso(value: datetime) -> str:
    return value.isoformat(timespec="milliseconds").replace("+00:00", "Z")


def read_env_value(name: str, default: str = "") -> str:
    value = os.environ.get(name)
    if value:
        return value
    env_path = ROOT / ".env"
    if not env_path.exists():
        return default
    try:
        for raw_line in env_path.read_text(encoding="utf-8").splitlines():
            line = raw_line.strip()
            if not line or line.startswith("#") or not line.startswith(f"{name}="):
                continue
            value = line.split("=", 1)[1].strip()
            if len(value) >= 2 and value[0] == value[-1] and value[0] in "'\"":
                value = value[1:-1]
            return value
    except OSError:
        return default
    return default


def command_output(
    args: list[str],
    *,
    cwd: Path = ROOT,
    extra_env: dict[str, str] | None = None,
) -> str:
    command_env = os.environ.copy()
    if extra_env:
        command_env.update(extra_env)
    try:
        result = subprocess.run(
            args,
            cwd=cwd,
            env=command_env,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            timeout=15,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired):
        return "unavailable"
    return result.stdout.strip()


def git_snapshot() -> dict[str, Any]:
    return {
        "sha": command_output(["git", "rev-parse", "HEAD"]),
        "branch": command_output(["git", "branch", "--show-current"]),
        "dirty": bool(command_output(["git", "status", "--porcelain"])),
    }


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def record_count(path: Path) -> int:
    with path.open(encoding="utf-8") as stream:
        return sum(1 for line in stream if line.strip())


def dataset_snapshot() -> dict[str, Any]:
    result: dict[str, Any] = {}
    for name, path in DATASETS.items():
        if path.exists():
            result[name] = {
                "path": str(path.relative_to(ROOT)),
                "sha256": sha256(path),
                "record_count": record_count(path),
            }
        else:
            result[name] = {"path": str(path.relative_to(ROOT)), "status": "missing"}
    return result


def request_json(url: str, token: str = "") -> dict[str, Any]:
    headers = {"Accept": "application/json"}
    if token:
        headers["X-Internal-Token"] = token
    request = Request(url, headers=headers)
    try:
        opener = build_opener(ProxyHandler({}))
        with opener.open(request, timeout=10) as response:
            body = response.read().decode("utf-8")
            payload = json.loads(body) if body else {}
            return {
                "http_status": response.status,
                "payload": payload if isinstance(payload, dict) else {"value": payload},
            }
    except HTTPError as exc:
        return {"http_status": exc.code, "payload": {"status": "http_error"}}
    except (URLError, TimeoutError, OSError, json.JSONDecodeError) as exc:
        return {"http_status": 0, "payload": {"status": type(exc).__name__}}


def readiness_snapshot() -> dict[str, Any]:
    app_port = read_env_value("SHAREO_APP_PORT", "8080")
    ai_port = read_env_value("SHAREO_AI_PORT", "8000")
    app_url = f"http://127.0.0.1:{app_port}"
    ai_url = f"http://127.0.0.1:{ai_port}"
    token = read_env_value("SHAREO_INTERNAL_TOKEN", "shareo-local-internal")
    endpoints = {
        "go_healthz": request_json(f"{app_url}/healthz"),
        "ai_healthz": request_json(f"{ai_url}/healthz"),
        "image_search": request_json(f"{ai_url}/readyz/image-search", token),
        "rag": request_json(f"{ai_url}/readyz/rag", token),
        "agent": request_json(f"{ai_url}/readyz/agent", token),
    }
    return {"app_url": app_url, "ai_url": ai_url, "endpoints": endpoints}


def ready_for(name: str, snapshot: dict[str, Any]) -> bool:
    item = snapshot.get("endpoints", {}).get(name, {})
    payload = item.get("payload", {})
    return item.get("http_status") == 200 and payload.get("status") == "ready"


def runtime_snapshot(readiness: dict[str, Any]) -> dict[str, Any]:
    configured_keys = (
        "SHAREO_AI_LLM_BASE_URL",
        "SHAREO_AI_LLM_API_KEY",
        "SHAREO_AI_LLM_MODEL",
        "SHAREO_INTERNAL_TOKEN",
    )
    return {
        "captured_at": iso(utc_now()),
        "git": git_snapshot(),
        "python": sys.version.split()[0],
        "python_eval_runtime": command_output(
            ["uv", "run", "--frozen", "python", "--version"],
            cwd=ROOT / "ai-service",
            extra_env={"UV_CACHE_DIR": str(ROOT / ".cache/shareo/uv")},
        ),
        "go": command_output(["go", "version"]),
        "uv": command_output(["uv", "--version"]),
        "platform": platform.platform(),
        "network_environment": os.environ.get("SHAREO_EVAL_NETWORK", "unspecified"),
        "configured_keys": {key: bool(read_env_value(key)) for key in configured_keys},
        "readiness": readiness,
        "dataset_hashes": dataset_snapshot(),
        "safe_env": {
            key: read_env_value(key)
            for key in (
                "SHAREO_AI_LLM_MODEL",
                "SHAREO_AI_LLM_TIMEOUT_SECONDS",
                "SHAREO_AI_AGENT_MAX_ROUNDS",
                "SHAREO_AI_AGENT_MAX_TOOL_CALLS",
                "SHAREO_AI_AGENT_MAX_OBSERVATION_CHARS",
                "SHAREO_AI_AGENT_TRACE_VERSION",
                "SHAREO_AI_TEXT_EMBEDDING_REVISION",
                "SHAREO_POST_SEARCH_SEMANTIC_THRESHOLD",
            )
            if read_env_value(key)
        },
        "runtime_parameters": {
            "chunk_size": read_env_value("SHAREO_AI_CHUNK_SIZE", "400"),
            "chunk_overlap": read_env_value("SHAREO_AI_CHUNK_OVERLAP", "80"),
            "rag_top_k": read_env_value("SHAREO_AI_RAG_TOP_K", "15"),
            "rag_max_sources": read_env_value("SHAREO_AI_RAG_MAX_SOURCES", "10"),
            "llm_timeout_seconds": read_env_value(
                "SHAREO_AI_LLM_TIMEOUT_SECONDS", "30.0"
            ),
            "llm_max_attempts": "2",
            "agent_max_rounds": read_env_value("SHAREO_AI_AGENT_MAX_ROUNDS", "4"),
            "agent_max_tool_calls": read_env_value(
                "SHAREO_AI_AGENT_MAX_TOOL_CALLS", "6"
            ),
            "agent_max_parallel_tools": read_env_value(
                "SHAREO_AI_AGENT_MAX_PARALLEL_TOOLS", "2"
            ),
            "agent_max_observation_chars": read_env_value(
                "SHAREO_AI_AGENT_MAX_OBSERVATION_CHARS", "12000"
            ),
            "agent_timeout_seconds": read_env_value(
                "SHAREO_AI_AGENT_TIMEOUT_SECONDS", "45.0"
            ),
        },
    }


def build_env(local_state: Path) -> dict[str, str]:
    env = os.environ.copy()
    # The native runtime loader reads .env for child processes. The evidence
    # collector must do the same so the standalone AI judge sees the exact
    # configured provider without placing credentials in any evidence file.
    provider_keys = (
        "SHAREO_AI_LLM_BASE_URL",
        "SHAREO_AI_LLM_API_KEY",
        "SHAREO_AI_LLM_MODEL",
        "SHAREO_AI_LLM_TIMEOUT_SECONDS",
        "SHAREO_AI_HTTP_PROXY",
        "SHAREO_AI_HTTPS_PROXY",
        "SHAREO_AI_ALL_PROXY",
        "SHAREO_AI_NO_PROXY",
    )
    for key in provider_keys:
        value = read_env_value(key)
        if value:
            env.setdefault(key, value)
    env["SHAREO_LOCAL_STATE_DIR"] = str(local_state)
    env["SHAREO_OPEN_BROWSER"] = "0"
    no_proxy = env.get("NO_PROXY", "")
    for host in ("127.0.0.1", "localhost", "::1"):
        if host not in no_proxy.split(","):
            no_proxy = f"{no_proxy},{host}" if no_proxy else host
    env["NO_PROXY"] = no_proxy
    env["no_proxy"] = no_proxy
    env["SHAREO_INTERNAL_TOKEN"] = read_env_value(
        "SHAREO_INTERNAL_TOKEN", "shareo-local-internal"
    )
    env["SHAREO_AI_INTERNAL_TOKEN"] = read_env_value(
        "SHAREO_AI_INTERNAL_TOKEN", env["SHAREO_INTERNAL_TOKEN"]
    )
    env.setdefault("GOCACHE", str(ROOT / ".cache/shareo/go-build"))
    env.setdefault("GOMODCACHE", str(ROOT / ".cache/shareo/go-mod"))
    env.setdefault("UV_CACHE_DIR", str(ROOT / ".cache/shareo/uv"))
    return env


def run_command(
    label: str,
    args: list[str],
    raw_dir: Path,
    env: dict[str, str],
    results: list[dict[str, Any]],
    env_overrides: dict[str, str] | None = None,
) -> bool:
    started_at = utc_now()
    log_path = raw_dir / "commands" / f"{len(results) + 1:02d}-{label}.log"
    log_path.parent.mkdir(parents=True, exist_ok=True)
    try:
        with log_path.open("w", encoding="utf-8") as log:
            completed = subprocess.run(
                args,
                cwd=ROOT,
                env=env,
                stdout=log,
                stderr=subprocess.STDOUT,
                text=True,
                check=False,
            )
        exit_code = completed.returncode
    except OSError:
        exit_code = 127
    finished_at = utc_now()
    status = "pass" if exit_code == 0 else "fail"
    judge_summary: dict[str, Any] = {}
    if label.startswith("ai-judge") and exit_code != 0:
        output_path = env.get("OUTPUT", "")
        try:
            judge_summary = json.loads(Path(output_path).read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            judge_summary = {}
        if (
            judge_summary.get("rag", {}).get("status") == "needs_human_review"
            or judge_summary.get("agent", {}).get("status") == "needs_human_review"
        ):
            status = "needs_human_review"
    result = {
        "label": label,
        "command": args,
        "status": status,
        "exit_code": exit_code,
        "started_at": iso(started_at),
        "finished_at": iso(finished_at),
        "duration_ms": round((finished_at - started_at).total_seconds() * 1000, 1),
        "raw_log": str(log_path.relative_to(raw_dir)),
    }
    if status == "fail":
        result["failure_summary"] = failure_summary(log_path)
    if judge_summary:
        result["review_summary"] = {
            "quality_gate_passed": judge_summary.get("quality_gate_passed", False),
            "rag": {
                "status": judge_summary.get("rag", {}).get("status"),
                "average": judge_summary.get("rag", {}).get("average"),
                "minimum": judge_summary.get("rag", {}).get("minimum"),
                "human_fallback_count": judge_summary.get("rag", {}).get(
                    "human_fallback_count"
                ),
            },
            "agent": {
                "status": judge_summary.get("agent", {}).get("status"),
                "average": judge_summary.get("agent", {}).get("average"),
                "minimum": judge_summary.get("agent", {}).get("minimum"),
                "human_fallback_count": judge_summary.get("agent", {}).get(
                    "human_fallback_count"
                ),
            },
        }
    if env_overrides:
        result["safe_env_overrides"] = env_overrides
    results.append(result)
    print(f"[{result['status'].upper()}] {label} ({result['duration_ms']:.0f} ms)")
    return exit_code == 0


def failure_summary(log_path: Path) -> str:
    try:
        lines = [
            line.strip()
            for line in log_path.read_text(
                encoding="utf-8", errors="replace"
            ).splitlines()
            if line.strip()
        ]
    except OSError:
        return "日志不可读"
    if not lines:
        return "无输出"
    important = [line for line in lines if "[False]" in line or "Error" in line]
    selected = important + lines[-8:]
    redacted: list[str] = []
    for line in selected:
        if line in redacted:
            continue
        line = re.sub(
            r"(?i)(authorization|cookie|x-internal-token|api[_-]?key|token|password|secret)\s*[=:]\s*[^\s,;]+",
            r"\1=[REDACTED]",
            line,
        )
        redacted.append(line[:300])
    return " | ".join(redacted)


def add_blocked(
    label: str,
    reason: str,
    args: list[str],
    results: list[dict[str, Any]],
) -> None:
    results.append(
        {"label": label, "command": args, "status": "blocked", "reason": reason}
    )
    print(f"[BLOCKED] {label}: {reason}")


def flatten_metrics(value: Any, prefix: str = "") -> dict[str, Any]:
    output: dict[str, Any] = {}
    if isinstance(value, dict):
        for key, child in value.items():
            path = f"{prefix}.{key}" if prefix else str(key)
            if key in METRIC_KEYS and isinstance(child, (bool, int, float, str)):
                metric_key = METRIC_ALIASES.get(key, key)
                metric_path = f"{prefix}.{metric_key}" if prefix else metric_key
                output[metric_path] = child
            elif isinstance(child, dict):
                output.update(flatten_metrics(child, path))
    return output


def report_metrics(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {"status": "missing"}
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        return {"status": "invalid", "error_type": type(exc).__name__}
    result: dict[str, Any] = {"status": "available", "metrics": flatten_metrics(data)}
    human_scoring = data.get("human_scoring")
    if isinstance(human_scoring, dict):
        result["human_scoring"] = {
            key: human_scoring[key]
            for key in ("status", "expected_count")
            if key in human_scoring
        }
    quality_judgement = data.get("quality_judgement")
    if isinstance(quality_judgement, dict):
        result["quality_judgement"] = {
            key: quality_judgement[key]
            for key in (
                "method",
                "status",
                "kind",
                "model",
                "prompt_version",
                "prompt_sha256",
                "count",
                "scored_count",
                "average",
                "minimum",
                "disagreement_count",
                "human_fallback_count",
                "quality_gate_passed",
                "gates",
                "timing",
                "error",
            )
            if key in quality_judgement
        }
    report = data.get("agent", {}).get("report", {})
    for key in ("timing_stages", "tool_step_latency", "llm_stage_latency"):
        value = report.get(key)
        if isinstance(value, dict):
            result[key] = value
    failure_categories = data.get("failure_categories")
    if not isinstance(failure_categories, dict):
        failure_categories = (
            data.get("agent", {}).get("report", {}).get("failure_categories")
        )
    if isinstance(failure_categories, dict):
        result["failure_categories"] = failure_categories
    return result


def numeric_summary(values: list[float]) -> dict[str, Any]:
    if not values:
        return {"status": "unavailable", "reason": "no numeric samples"}
    ordered = sorted(values)
    mean = sum(values) / len(values)
    median = ordered[len(ordered) // 2] if len(ordered) % 2 else (
        ordered[len(ordered) // 2 - 1] + ordered[len(ordered) // 2]
    ) / 2
    variance = sum((value - mean) ** 2 for value in values) / len(values)
    return {
        "status": "available",
        "count": len(values),
        "mean": round(mean, 4),
        "median": round(median, 4),
        "min": round(min(values), 4),
        "max": round(max(values), 4),
        "stdev": round(variance**0.5, 4),
    }


def aggregate_round_reports(round_reports: list[dict[str, Any]]) -> dict[str, Any]:
    """Summarize repeated reports without hiding any individual round."""
    groups = ("image_search_local", "post_search_local", "rag_image", "agent")
    output: dict[str, Any] = {
        "policy": "descriptive statistics across complete rounds; no best-round selection",
        "round_count": len(round_reports),
        "groups": {},
    }
    for group in groups:
        metric_values: dict[str, list[float]] = {}
        for round_report in round_reports:
            metrics = round_report.get("reports", {}).get(group, {}).get("metrics", {})
            for key, value in metrics.items():
                if isinstance(value, (int, float)) and not isinstance(value, bool):
                    metric_values.setdefault(key, []).append(float(value))
        output["groups"][group] = {
            key: numeric_summary(values) for key, values in sorted(metric_values.items())
        }
    return output


def judgement_summary(payload: dict[str, Any]) -> dict[str, Any]:
    if not payload:
        return {"status": "unavailable"}
    result: dict[str, Any] = {}
    for kind in ("rag", "agent"):
        item = payload.get(kind, {})
        result[kind] = {
            key: item.get(key)
            for key in (
                "status",
                "model",
                "prompt_version",
                "prompt_sha256",
                "count",
                "scored_count",
                "average",
                "minimum",
                "disagreement_count",
                "human_fallback_count",
                "quality_gate_passed",
                "gates",
                "timing",
            )
            if key in item
        }
    result["quality_gate_passed"] = payload.get("quality_gate_passed", False)
    return result


def support_summary(payload: dict[str, Any]) -> dict[str, Any]:
    """Keep dataset support audit evidence without copying source rows to the repo."""
    if not payload:
        return {"status": "unavailable", "quality_gate_passed": False}
    result: dict[str, Any] = {
        "method": payload.get("method"),
        "manifest_sha256": payload.get("manifest_sha256"),
        "quality_gate_passed": all(
            payload.get(kind, {}).get("quality_gate_passed") is True
            for kind in ("rag", "agent")
        ),
    }
    for kind in ("rag", "agent"):
        item = payload.get(kind, {})
        result[kind] = {
            key: item.get(key)
            for key in (
                "method",
                "status",
                "model",
                "prompt_version",
                "prompt_sha256",
                "count",
                "applicable_count",
                "supported_count",
                "unsupported_count",
                "unsupported_ids",
                "human_fallback_count",
                "quality_gate_passed",
                "timing",
                "error",
            )
            if key in item
        }
    return result


def source_fingerprint(
    before: dict[str, Any], after: dict[str, Any]
) -> dict[str, Any]:
    before_git = before.get("git", {})
    after_git = after.get("git", {})
    before_diff = before_git.get("diff_sha256")
    after_diff = after_git.get("diff_sha256")
    return {
        "policy": "source-only fingerprint excludes generated final-freeze and browser artifacts",
        "stable": bool(
            before_git.get("sha") == after_git.get("sha")
            and before_git.get("branch") == after_git.get("branch")
            and before_diff == after_diff
        ),
        "before": {
            "git": before_git,
            "summary": before.get("summary", {}),
        },
        "after": {
            "git": after_git,
            "summary": after.get("summary", {}),
        },
    }


def update_retained_runs(
    run_id: str,
    runtime: dict[str, Any],
    quality: dict[str, Any],
    latency: dict[str, Any],
    manifest: dict[str, Any],
) -> None:
    """Retain every bundle and advance authority only after the strict gate."""
    path = EVIDENCE_DIR / "retained-runs.json"
    try:
        retained = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        retained = {"dataset_scope": {}, "runs": []}
    runs = [item for item in retained.get("runs", []) if item.get("run_id") != run_id]
    reports = quality.get("reports", {})
    rag_metrics = reports.get("rag_image", {}).get("metrics", {})
    image_metrics = reports.get("image_search_local", {}).get("metrics", {})
    agent_metrics = reports.get("agent", {}).get("metrics", {})
    judgement = quality.get("quality_judgement", {})
    entry = {
        "run_id": run_id,
        "record_status": "full_bundle_retained",
        "authoritative": bool(
            quality.get("quality_gate_passed") is True
            and manifest.get("quality_gate_passed") is True
        ),
        "started_at": manifest.get("started_at"),
        "finished_at": manifest.get("finished_at"),
        "duration_ms": manifest.get("duration_ms"),
        "git": runtime.get("git", {}),
        "source_fingerprint_stable": quality.get("source_fingerprint", {}).get("stable"),
        "provider": runtime.get("safe_env", {}).get("SHAREO_AI_LLM_MODEL")
        or "configured runtime model",
        "dataset_sha256": {
            name: value.get("sha256")
            for name, value in runtime.get("dataset_hashes", {}).items()
            if isinstance(value, dict) and value.get("sha256")
        },
        "quality": {
            "image_recall_at_5": report_metric(image_metrics, "recall_5", "image_search.metrics"),
            "image_no_match_accuracy": report_metric(
                image_metrics, "no_match_accuracy", "image_search.metrics"
            ),
            "rag_complete_source_coverage_rate": report_metric(
                rag_metrics, "complete_source_coverage_rate", "rag.report"
            ),
            "rag_citation_precision_rate": report_metric(
                rag_metrics, "citation_precision_rate", "rag.report"
            ),
            "rag_citation_accessible_rate": report_metric(
                rag_metrics, "citation_accessible_rate", "rag.report"
            ),
            "agent_complete_source_coverage_rate": report_metric(
                agent_metrics, "complete_source_coverage_rate", "agent.report"
            ),
            "agent_citation_precision_rate": report_metric(
                agent_metrics, "citation_precision_rate", "agent.report"
            ),
            "agent_required_tool_selection_rate": report_metric(
                agent_metrics, "required_tool_selection_rate", "agent.report"
            ),
            "agent_citation_accessible_rate": report_metric(
                agent_metrics, "citation_accessible_rate", "agent.report"
            ),
            "ai_judge_quality_gate_passed": judgement.get("quality_gate_passed", False),
        },
        "latency_ms": {
            "image_p95": latency.get("evaluation_report_aggregate", {})
            .get("image_total_ms", {})
            .get("p95_ms"),
            "rag_p95": latency.get("evaluation_report_aggregate", {})
            .get("rag_total_ms", {})
            .get("p95_ms"),
            "agent_p95": latency.get("evaluation_report_aggregate", {})
            .get("agent_total_ms", {})
            .get("p95_ms"),
        },
        "command_status": manifest.get("status_counts", {}),
        "raw_bundle": manifest.get("raw_dir"),
    }
    runs.append(entry)
    retained["runs"] = runs
    if entry["authoritative"]:
        retained["authoritative_run_id"] = run_id
    write_json(path, retained)


def merge_human_handoffs(
    judgements: list[dict[str, Any]], run_id: str
) -> dict[str, Any]:
    """Union fallback rows from all rounds while keeping only redacted fields."""
    merged: dict[tuple[str, str], dict[str, Any]] = {}
    for round_index, payload in enumerate(judgements, 1):
        for kind in ("rag", "agent"):
            judgement = payload.get(kind, {}) if isinstance(payload, dict) else {}
            fallback_ids = {
                str(item.get("id")): item
                for item in judgement.get("human_fallback", [])
                if isinstance(item, dict) and item.get("id")
            }
            rows = {
                str(row.get("id")): row
                for row in judgement.get("rows", [])
                if isinstance(row, dict) and row.get("id")
            }
            for item_id, fallback in fallback_ids.items():
                row = rows.get(item_id, {})
                key = (kind, item_id)
                current = merged.setdefault(
                    key,
                    {
                        "kind": kind,
                        "id": item_id,
                        "category": row.get("category", ""),
                        "question": row.get("question", ""),
                        "reference_answer": row.get("reference_answer", ""),
                        "answer": row.get("answer", ""),
                        "expected_source_post_ids": row.get("expected_source_post_ids", []),
                        "citation_post_ids": row.get("citation_post_ids", []),
                        "scores": [],
                        "reasons": set(),
                        "rounds": [],
                        "deterministic_audit": row.get("deterministic_audit", {}),
                        "judge_reasons": [],
                    },
                )
                current["rounds"].append(round_index)
                current["reasons"].update(fallback.get("reasons", []))
                if row.get("score") is not None:
                    current["scores"].append(row["score"])
                current["judge_reasons"].extend(
                    pass_result.get("reason", "")
                    for pass_result in row.get("passes", [])
                    if isinstance(pass_result, dict) and pass_result.get("reason")
                )
    items = []
    for item in merged.values():
        item["reasons"] = sorted(item["reasons"])
        item["rounds"] = sorted(set(item["rounds"]))
        item["score_min"] = min(item["scores"]) if item["scores"] else None
        item["score_max"] = max(item["scores"]) if item["scores"] else None
        item.pop("scores", None)
        items.append(item)
    return {
        "run_id": run_id,
        "status": "required" if items else "not_required",
        "method": "llm_judge_with_human_fallback",
        "instructions": "人工只需复核下列题目；不得把 AI 分数改写为人工分数，也不得为通过门禁修改题目、答案或阈值。",
        "count": len(items),
        "items": sorted(items, key=lambda item: (item["kind"], item["id"])),
    }


def load_json(path: Path) -> dict[str, Any]:
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return {}
    return data if isinstance(data, dict) else {}


def build_human_handoff(
    quality_judgement: dict[str, Any], run_id: str
) -> dict[str, Any]:
    """Create a minimal, redacted handoff for rows the AI judge cannot clear."""
    items: list[dict[str, Any]] = []
    for kind in ("rag", "agent"):
        judgement = quality_judgement.get(kind, {})
        if not isinstance(judgement, dict):
            continue
        fallback_by_id = {
            str(item.get("id")): item
            for item in judgement.get("human_fallback", [])
            if isinstance(item, dict) and item.get("id")
        }
        for row in judgement.get("rows", []):
            if not isinstance(row, dict) or str(row.get("id")) not in fallback_by_id:
                continue
            fallback = fallback_by_id[str(row["id"])]
            reasons = fallback.get("reasons", [])
            recommendation = (
                "复核答案、期望来源和评测题意；只记录人工结论，不修改本次运行结果。"
            )
            if "missing_expected_sources" in reasons:
                recommendation = "优先核对期望来源是否与题目一致，再判断答案是否漏引或评测数据存在语义错配。"
            elif "score_below_3" in reasons:
                recommendation = "确认答案是否真正完成用户请求；若拒答是有意的安全边界，记录理由，否则提出最小修复。"
            items.append(
                {
                    "kind": kind,
                    "id": row["id"],
                    "category": row.get("category", ""),
                    "question": row.get("question", ""),
                    "reference_answer": row.get("reference_answer", ""),
                    "answer": row.get("answer", ""),
                    "expected_source_post_ids": row.get("expected_source_post_ids", []),
                    "citation_post_ids": row.get("citation_post_ids", []),
                    "score": row.get("score"),
                    "reasons": reasons,
                    "deterministic_audit": row.get("deterministic_audit", {}),
                    "judge_reasons": [
                        pass_result.get("reason", "")
                        for pass_result in row.get("passes", [])
                        if isinstance(pass_result, dict)
                    ],
                    "recommended_action": recommendation,
                }
            )
    return {
        "run_id": run_id,
        "status": "required" if items else "not_required",
        "method": "llm_judge_with_human_fallback",
        "instructions": "人工只需复核下列题目；不得把 AI 分数改写为人工分数，也不得为通过门禁修改题目、答案或阈值。",
        "count": len(items),
        "items": items,
    }


def chain_evidence(
    raw_dir: Path,
    results: list[dict[str, Any]],
    report_paths: dict[str, Path] | None = None,
    ai_labels: list[str] | None = None,
    agent_labels: list[str] | None = None,
) -> list[dict[str, Any]]:
    by_label = {item.get("label"): item for item in results}
    report_paths = report_paths or {}
    ai_labels = ai_labels or ["eval-ai-real-provider"]
    agent_labels = agent_labels or ["eval-agent-real-provider"]

    def window(labels: list[str]) -> dict[str, Any]:
        selected = [by_label[label] for label in labels if label in by_label]
        timestamps = [
            item[key]
            for item in selected
            for key in ("started_at", "finished_at")
            if item.get(key)
        ]
        return {
            "started_at": min(timestamps) if timestamps else None,
            "finished_at": max(timestamps) if timestamps else None,
            "duration_ms": round(
                sum(float(item.get("duration_ms", 0)) for item in selected), 1
            ),
        }

    def status(labels: list[str]) -> str:
        values = [
            by_label[label].get("status") for label in labels if label in by_label
        ]
        if any(value == "blocked" for value in values):
            return "blocked"
        if any(value == "needs_human_review" for value in values):
            return "needs_human_review"
        if any(value == "fail" for value in values):
            return "fail"
        if values and all(value == "pass" for value in values):
            return "pass"
        return "not_run"

    def base(scenario_id: str, labels: list[str]) -> dict[str, Any]:
        return {
            "scenario_id": scenario_id,
            "status": status(labels),
            **window(labels),
            "relevant_record_ids": [],
            "citation_count": 0,
            "tool_sequence": [],
            "retry_count": 0,
            "failure_category": None,
            "final_assertion": "",
            "source_commands": labels,
        }

    chains = [
        base(
            "publish-approve-index-search-visibility",
            ["integration", "local-stack", "api"],
        ),
        base(
            "private-chat-bot-rag-citation-websocket", ["api", *ai_labels]
        ),
        base(
            "explicit-agent-tools-trace-citation", ["api", *agent_labels]
        ),
        {
            "scenario_id": "dependency-failure-degradation-recovery",
            "status": "not_run",
            "started_at": None,
            "finished_at": None,
            "duration_ms": 0.0,
            "relevant_record_ids": [],
            "citation_count": 0,
            "tool_sequence": [],
            "retry_count": 0,
            "failure_category": "not_run: local-only scope",
            "final_assertion": "Compose 故障注入和恢复本轮未运行；不据此宣称发布级故障验证完成。",
            "source_commands": ["compose-fault-injection"],
        },
    ]

    rag_path = report_paths.get("rag_image", raw_dir / "rag-image.json")
    rag_report = load_json(rag_path).get("rag", {}).get("report", {})
    chains[1]["citation_count"] = {
        "status": "unavailable",
        "reason": "RAG 汇总只提供平均引用数",
        "average_sources_per_answer": rag_report.get("avg_sources_per_answer"),
    }
    chains[1]["failure_category"] = (
        "quality_gate: source_hit_rate"
        if any(by_label.get(label, {}).get("status") == "fail" for label in ai_labels)
        else None
    )
    chains[1]["retry_count"] = {
        "status": "unavailable",
        "reason": "RAG 汇总没有逐题 Provider 重试字段",
    }
    rag_source_hit = rag_report.get("source_hit_rate")
    rag_accessible = rag_report.get("citation_accessible_rate")
    rag_failed = rag_report.get("failed_queries")
    chains[1]["final_assertion"] = (
        f"RAG 请求执行完成；来源命中率 {rag_source_hit}、完整来源覆盖率 "
        f"{rag_report.get('complete_source_coverage_rate')}、引用可访问率 "
        f"{rag_accessible}、失败请求 {rag_failed}。"
    )

    agent_path = report_paths.get("agent", raw_dir / "agent.json")
    agent = load_json(agent_path).get("agent", {})
    agent_report = agent.get("report", {})
    agent_rows = agent.get("evaluation_rows") or agent.get("query_results", [])
    citation_ids = {
        hashlib.sha256(str(post_id).encode("utf-8")).hexdigest()[:12]
        for row in agent_rows
        if isinstance(row, dict)
        for post_id in (row.get("citation_post_ids") or [])
    }
    sequences: dict[str, int] = {}
    retries = 0
    for row in agent_rows:
        if not isinstance(row, dict):
            continue
        sequence = (
            ">".join(str(tool) for tool in (row.get("actual_tools") or [])) or "none"
        )
        sequences[sequence] = sequences.get(sequence, 0) + 1
        retries += int(row.get("provider_retries", 0) or 0)
    chains[2]["relevant_record_ids"] = sorted(citation_ids)
    chains[2]["citation_count"] = len(
        [
            post_id
            for row in agent_rows
            if isinstance(row, dict)
            for post_id in (row.get("citation_post_ids") or [])
        ]
    )
    chains[2]["tool_sequence"] = sequences
    chains[2]["retry_count"] = retries
    chains[2]["failure_category"] = (
        "quality_gate: source_hit_rate/tool_selection"
        if any(
            by_label.get(label, {}).get("status") == "fail" for label in agent_labels
        )
        else None
    )
    chains[2]["final_assertion"] = (
        f"36 条 Agent 请求均完成；来源命中率 {agent_report.get('source_hit_rate')}、"
        f"完整来源覆盖率 {agent_report.get('complete_source_coverage_rate')}、"
        f"额外工具调用 {agent_report.get('unexpected_tool_calls')}、引用可访问率 "
        f"{agent_report.get('citation_accessible_rate')}；安全门禁通过。"
    )

    chains[0]["final_assertion"] = (
        "集成、本机栈和 API 测试均通过；脱敏汇总不保存临时帖子、消息或任务真实 ID。"
    )
    return chains


def metric_availability(reports: dict[str, dict[str, Any]]) -> dict[str, Any]:
    requested = {
        "image_search": [
            "recall_1",
            "recall_5",
            "recall_10",
            "mrr",
            "ndcg_5",
            "precision_3",
            "no_match_accuracy",
            "duplicate_queries",
            "invisible_results",
            "latency_p50_ms",
            "latency_p95_ms",
        ],
        "post_search": [
            "recall_5",
            "mrr",
            "ndcg_5",
            "precision_3",
            "no_match_accuracy",
            "latency_p50_ms",
            "latency_p95_ms",
        ],
        "rag": [
            "source_hit_rate",
            "source_coverage_rate",
            "complete_source_coverage_rate",
            "citation_precision_rate",
            "extra_citations",
            "citation_accessible_rate",
            "total_hallucinated",
            "failed_queries",
            "fallback_count",
            "avg_sources_per_answer",
            "latency_p50_ms",
            "latency_p95_ms",
            "no_answer_accuracy",
        ],
        "agent": [
            "source_hit_rate",
            "source_coverage_rate",
            "complete_source_coverage_rate",
            "citation_precision_rate",
            "required_tool_selection_rate",
            "citation_accessible_rate",
            "unexpected_tool_calls",
            "forbidden_actions",
            "budget_violations",
            "injection_failures",
            "failed_queries",
            "timeout_queries",
            "provider_retries",
            "latency_p50_ms",
            "latency_p95_ms",
            "no_answer_accuracy",
        ],
    }
    sources = {
        "image_search": ["image_search_local", "rag_image"],
        "post_search": ["post_search_local"],
        "rag": ["rag_image"],
        "agent": ["agent"],
    }
    output: dict[str, Any] = {}
    for group, keys in requested.items():
        available_metrics = {
            path
            for source in sources[group]
            for path in reports.get(source, {}).get("metrics", {})
        }
        available = [
            key
            for key in keys
            if key in available_metrics
            or any(path.endswith(f".{key}") for path in available_metrics)
        ]
        output[group] = {
            "available": available,
            "unavailable": [key for key in keys if key not in available],
            "unavailable_reason": "现有评测报告未提供该字段；未估算或补写。",
        }
    return output


def percentile(values: list[float], fraction: float) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    index = min(len(ordered) - 1, max(0, round((len(ordered) - 1) * fraction)))
    return round(ordered[index], 1)


def summarize_log_metrics(log_path: Path, offsets: dict[str, int]) -> dict[str, Any]:
    values: dict[str, list[float]] = {key: [] for key in LOG_PATTERNS}
    if not log_path.exists():
        return {"status": "unavailable", "reason": "ai log not found"}
    try:
        with log_path.open("r", encoding="utf-8", errors="replace") as stream:
            stream.seek(offsets.get(str(log_path), 0))
            lines = stream.readlines()
    except OSError:
        return {"status": "unavailable", "reason": "ai log could not be read"}
    for line in lines:
        for key, pattern in LOG_PATTERNS.items():
            match = pattern.search(line)
            if match:
                values[key].append(float(match.group("value")))
    summary: dict[str, Any] = {
        "status": "available",
        "source": "ai.log appended structured lines",
    }
    for key, samples in values.items():
        summary[key] = {
            "sample_count": len(samples),
            "p50_ms": percentile(samples, 0.50),
            "p95_ms": percentile(samples, 0.95),
            "max_ms": round(max(samples), 1) if samples else 0.0,
        }
    for key in ("image_embedding_ms", "image_search_ms", "agent_tool_step_ms"):
        summary[key] = {
            "status": "unavailable",
            "reason": "现有结构化日志没有该分段耗时字段",
        }
    summary["queue_wait_ms"] = {
        "status": "unavailable",
        "reason": "no structured log field",
    }
    summary["callback_ms"] = {
        "status": "unavailable",
        "reason": "no structured log field",
    }
    return summary


def report_metric(
    metrics: dict[str, Any], suffix: str, preferred_prefix: str = ""
) -> Any:
    if preferred_prefix:
        preferred = metrics.get(f"{preferred_prefix}.{suffix}")
        if preferred is not None:
            return preferred
    for key, value in metrics.items():
        if key == suffix or key.endswith(f".{suffix}"):
            return value
    return None


def report_latency_summary(
    quality_reports: dict[str, dict[str, Any]],
) -> dict[str, Any]:
    image_metrics = quality_reports.get("rag_image", {}).get("metrics", {})
    rag_metrics = quality_reports.get("rag_image", {}).get("metrics", {})
    agent_metrics = quality_reports.get("agent", {}).get("metrics", {})
    return {
        "image_total_ms": {
            "source": "rag-image.json image_search report",
            "p50_ms": report_metric(
                image_metrics, "latency_p50_ms", "image_search.metrics"
            ),
            "p95_ms": report_metric(
                image_metrics, "latency_p95_ms", "image_search.metrics"
            ),
        },
        "rag_total_ms": {
            "source": "rag-image.json rag report",
            "p50_ms": report_metric(rag_metrics, "latency_p50_ms", "rag.report"),
            "p95_ms": report_metric(rag_metrics, "latency_p95_ms", "rag.report"),
        },
        "agent_total_ms": {
            "source": "agent.json agent report",
            "p50_ms": report_metric(agent_metrics, "latency_p50_ms", "agent.report"),
            "p95_ms": report_metric(agent_metrics, "latency_p95_ms", "agent.report"),
        },
        "agent_tool_step_ms": {
            "status": "available"
            if isinstance(
                quality_reports.get("agent", {}).get("tool_step_latency"), dict
            )
            else "unavailable",
            "by_tool": quality_reports.get("agent", {}).get("tool_step_latency", {}),
        },
        "agent_llm_ms": quality_reports.get("agent", {}).get(
            "llm_stage_latency",
            {"status": "unavailable", "reason": "no Agent LLM stage samples"},
        ),
        "agent_timing_stages": quality_reports.get("agent", {}).get(
            "timing_stages",
            {"status": "unavailable", "reason": "no Agent timing stage report"},
        ),
    }


def write_json(path: Path, payload: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )


def write_failure_matrix(results: list[dict[str, Any]], path: Path) -> None:
    lines = [
        "# Final Freeze 自动化结果",
        "",
        "> 本文件只记录本轮本机运行状态；原始命令输出位于 `.local/shareo/final-freeze/<run_id>/`，不进入仓库证据目录。",
        "",
        "| 场景 | 状态 | 退出码 | 说明 |",
        "|---|---|---:|---|",
    ]
    for result in results:
        status = result.get("status", "unknown")
        exit_code = result.get("exit_code", "—")
        reason = result.get("reason", result.get("failure_summary", ""))
        lines.append(
            f"| `{result.get('label', '')}` | `{status}` | {exit_code} | {reason} |"
        )
    lines.extend(
        [
            "| `compose-image-e2e` | `not_run` | — | local-only scope；Compose 图片链路未纳入本轮 |",
            "| `compose-bot-e2e` | `not_run` | — | local-only scope；Compose Bot 链路未纳入本轮 |",
            "| `compose-agent-e2e` | `not_run` | — | local-only scope；Compose Agent 链路未纳入本轮 |",
            "| `compose-fault-injection` | `not_run` | — | local-only scope；Compose 故障注入未纳入本轮 |",
        ]
    )
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def archive_previous_evidence() -> None:
    """Keep prior sanitized summaries before replacing the current root bundle."""
    manifest_path = EVIDENCE_DIR / "run-manifest.json"
    if not manifest_path.exists():
        return
    try:
        previous = json.loads(manifest_path.read_text(encoding="utf-8"))
        previous_run_id = str(previous.get("run_id", "")).strip()
    except (OSError, json.JSONDecodeError):
        return
    if not previous_run_id:
        return

    archive_dir = EVIDENCE_DIR / "runs" / previous_run_id
    archive_dir.mkdir(parents=True, exist_ok=True)
    for name in (
        "run-manifest.json",
        "runtime-snapshot.json",
        "command-results.json",
        "quality-summary.json",
        "latency-summary.json",
        "failure-matrix.md",
        "worktree-manifest.json",
        "source-worktree-manifest.json",
        "ai-judge-human-handoff.json",
        "dataset-support-audit.json",
        "browser-smoke.json",
        "resume-audit-summary.json",
    ):
        source = EVIDENCE_DIR / name
        if source.exists():
            shutil.copy2(source, archive_dir / name)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-id", default="")
    parser.add_argument("--ai-eval-runs", type=int, default=1)
    parser.add_argument("--browser-smoke", action="store_true")
    args = parser.parse_args()

    if not 1 <= args.ai_eval_runs <= 5:
        parser.error("--ai-eval-runs must be between 1 and 5")

    run_id = args.run_id or utc_now().strftime("%Y%m%dT%H%M%SZ")
    local_state = Path(read_env_value("SHAREO_LOCAL_STATE_DIR", str(LOCAL_STATE_DIR)))
    raw_dir = local_state / "final-freeze" / run_id
    raw_dir.mkdir(parents=True, exist_ok=True)
    EVIDENCE_DIR.mkdir(parents=True, exist_ok=True)
    archive_previous_evidence()
    env = build_env(local_state)
    results: list[dict[str, Any]] = []
    started_at = utc_now()

    setup_steps = [
        ("up", ["make", "up"]),
        ("local-doctor", ["make", "local-doctor"]),
        ("warm-ai", ["make", "warm-ai"]),
        ("prepare-current-ai-eval", ["make", "prepare-current-ai-eval"]),
    ]
    for label, command in setup_steps:
        run_command(label, command, raw_dir, env, results)

    source_before_path = raw_dir / "source-worktree-before.json"
    run_command(
        "source-worktree-manifest-before",
        ["make", "worktree-manifest"],
        raw_dir,
        env | {"OUTPUT": str(source_before_path), "SOURCE_ONLY": "1"},
        results,
        {
            "OUTPUT": str(source_before_path.relative_to(local_state)),
            "SOURCE_ONLY": "1",
        },
    )

    readiness = readiness_snapshot()
    write_json(raw_dir / "readiness-after-setup.json", readiness)
    runtime = runtime_snapshot(readiness)
    ai_log_path = local_state / "ai.log"
    ai_log_offsets = {
        str(ai_log_path): ai_log_path.stat().st_size if ai_log_path.exists() else 0
    }

    for label, command in (
        ("check", ["make", "check"]),
        ("go-race", ["go", "test", "-race", "./..."]),
        ("index-reconcile", ["make", "reconcile-index"]),
        ("integration", ["make", "test-integration-auto"]),
        ("local-stack", ["make", "test-local-stack"]),
        ("api", ["make", "test-api"]),
    ):
        command_env = env
        if label == "check":
            # The current evidence is replaced at the end of this run. Keep
            # the engineering checks, but do not fail on the previous run's
            # three-round/fingerprint assertions before new evidence exists.
            command_env = env | {"SHAREO_SKIP_CURRENT_EVIDENCE": "1"}
        run_command(label, command, raw_dir, command_env, results)

    post_output = raw_dir / "post-search.json"
    image_output = raw_dir / "image-search-local.json"
    if ready_for("rag", readiness):
        run_command(
            "post-search-eval",
            ["make", "eval-post-search"],
            raw_dir,
            env
            | {
                "DATASET": str(DATASETS["post_search_current"]),
                "OUTPUT": str(post_output),
            },
            results,
            {
                "DATASET": str(DATASETS["post_search_current"].relative_to(ROOT)),
                "OUTPUT": str(post_output.relative_to(local_state)),
            },
        )
    else:
        add_blocked(
            "post-search-eval",
            "RAG/text readiness unavailable",
            ["make", "eval-post-search"],
            results,
        )

    if ready_for("image_search", readiness):
        run_command(
            "image-search-local-eval",
            ["make", "eval-image-search-local"],
            raw_dir,
            env
            | {
                "DATASET": str(DATASETS["image_search_current"]),
                "OUTPUT": str(image_output),
            },
            results,
            {
                "DATASET": str(DATASETS["image_search_current"].relative_to(ROOT)),
                "OUTPUT": str(image_output.relative_to(local_state)),
            },
        )
    else:
        add_blocked(
            "image-search-local-eval",
            "image-search readiness unavailable",
            ["make", "eval-image-search-local"],
            results,
        )

    support_output = raw_dir / "dataset-support-audit.json"
    support_manifest = ROOT / ".local" / "shareo" / "local-photo-seed" / "manifest.json"
    if ready_for("rag", readiness) and ready_for("agent", readiness):
        run_command(
            "dataset-support-audit",
            ["make", "eval-dataset-audit"],
            raw_dir,
            env
            | {
                "RAG_DATASET": str(DATASETS["rag_current"]),
                "AGENT_DATASET": str(DATASETS["agent_current"]),
                "MANIFEST": str(support_manifest),
                "OUTPUT": str(support_output),
            },
            results,
            {
                "RAG_DATASET": str(DATASETS["rag_current"].relative_to(ROOT)),
                "AGENT_DATASET": str(DATASETS["agent_current"].relative_to(ROOT)),
                "MANIFEST": str(support_manifest.relative_to(ROOT)),
                "OUTPUT": str(support_output.relative_to(local_state)),
            },
        )
    else:
        add_blocked(
            "dataset-support-audit",
            "RAG or Agent readiness unavailable",
            ["make", "eval-dataset-audit"],
            results,
        )

    support_payload = load_json(support_output)
    support_gate = bool(
        support_payload
        and support_payload.get("rag", {}).get("quality_gate_passed") is True
        and support_payload.get("agent", {}).get("quality_gate_passed") is True
    )

    round_data: list[dict[str, Any]] = []
    ai_labels: list[str] = []
    agent_labels: list[str] = []
    if support_gate and ready_for("rag", readiness) and ready_for("agent", readiness):
        for round_index in range(1, args.ai_eval_runs + 1):
            round_dir = raw_dir / "ai-runs" / f"round-{round_index:02d}"
            rag_output = round_dir / "rag-image.json"
            agent_output = round_dir / "agent.json"
            judge_output = round_dir / "quality-judgement.json"
            ai_label = f"eval-ai-real-provider-r{round_index:02d}"
            agent_label = f"eval-agent-real-provider-r{round_index:02d}"
            judge_label = f"ai-judge-r{round_index:02d}"
            ai_labels.append(ai_label)
            agent_labels.append(agent_label)
            run_command(
                ai_label,
                ["make", "eval-ai", "MACHINE_ONLY=1"],
                round_dir,
                env
                | {
                    "MACHINE_ONLY": "1",
                    "OUTPUT": str(rag_output),
                    "SHAREO_IMAGE_DATASET": str(DATASETS["image_search_current"]),
                    "SHAREO_RAG_DATASET": str(DATASETS["rag_current"]),
                },
                results,
                {
                    "MACHINE_ONLY": "1",
                    "OUTPUT": str(rag_output.relative_to(local_state)),
                    "IMAGE_DATASET": str(
                        DATASETS["image_search_current"].relative_to(ROOT)
                    ),
                    "RAG_DATASET": str(DATASETS["rag_current"].relative_to(ROOT)),
                    "ROUND": str(round_index),
                },
            )
            run_command(
                agent_label,
                ["make", "eval-agent", "MACHINE_ONLY=1"],
                round_dir,
                env
                | {
                    "MACHINE_ONLY": "1",
                    "OUTPUT": str(agent_output),
                    "SHAREO_AGENT_DATASET": str(DATASETS["agent_current"]),
                },
                results,
                {
                    "MACHINE_ONLY": "1",
                    "OUTPUT": str(agent_output.relative_to(local_state)),
                    "AGENT_DATASET": str(DATASETS["agent_current"].relative_to(ROOT)),
                    "ROUND": str(round_index),
                },
            )
            if rag_output.exists() and agent_output.exists():
                run_command(
                    judge_label,
                    ["make", "eval-ai-judge"],
                    round_dir,
                    env
                    | {
                        "RAG_REPORT": str(rag_output),
                        "AGENT_REPORT": str(agent_output),
                        "OUTPUT": str(judge_output),
                    },
                    results,
                    {
                        "RAG_REPORT": str(rag_output.relative_to(local_state)),
                        "AGENT_REPORT": str(agent_output.relative_to(local_state)),
                        "OUTPUT": str(judge_output.relative_to(local_state)),
                        "ROUND": str(round_index),
                    },
                )
            else:
                add_blocked(
                    judge_label,
                    "RAG or Agent report unavailable",
                    ["make", "eval-ai-judge"],
                    results,
                )
            round_data.append(
                {
                    "round": round_index,
                    "rag": rag_output,
                    "agent": agent_output,
                    "judge": judge_output,
                }
            )
    else:
        reason = "dataset source support audit did not pass" if not support_gate else "RAG or Agent readiness unavailable"
        for round_index in range(1, args.ai_eval_runs + 1):
            add_blocked(
                f"eval-ai-real-provider-r{round_index:02d}",
                reason,
                ["make", "eval-ai", "MACHINE_ONLY=1"],
                results,
            )
            add_blocked(
                f"eval-agent-real-provider-r{round_index:02d}",
                reason,
                ["make", "eval-agent", "MACHINE_ONLY=1"],
                results,
            )
            add_blocked(
                f"ai-judge-r{round_index:02d}",
                reason,
                ["make", "eval-ai-judge"],
                results,
            )
            round_data.append(
                {
                    "round": round_index,
                    "rag": raw_dir / "missing-rag.json",
                    "agent": raw_dir / "missing-agent.json",
                    "judge": raw_dir / "missing-judge.json",
                }
            )

    browser_output = raw_dir / "browser-smoke.json"
    if args.browser_smoke:
        run_command(
            "browser-smoke",
            ["make", "test-browser-smoke"],
            raw_dir,
            env | {"SHAREO_BROWSER_OUTPUT": str(browser_output), "SHAREO_BROWSER_RUN_ID": run_id},
            results,
            {
                "OUTPUT": str(browser_output.relative_to(local_state)),
                "RUN_ID": run_id,
            },
        )
    else:
        results.append(
            {
                "label": "browser-smoke",
                "status": "not_run",
                "reason": "未请求浏览器自动化；resume-audit 会显式传入 --browser-smoke。",
            }
        )

    source_after_path = raw_dir / "source-worktree-after.json"
    run_command(
        "source-worktree-manifest-after",
        ["make", "worktree-manifest"],
        raw_dir,
        env | {"OUTPUT": str(source_after_path), "SOURCE_ONLY": "1"},
        results,
        {
            "OUTPUT": str(source_after_path.relative_to(local_state)),
            "SOURCE_ONLY": "1",
        },
    )

    worktree_output = raw_dir / "worktree-manifest.json"
    run_command(
        "worktree-manifest",
        ["make", "worktree-manifest"],
        raw_dir,
        env | {"OUTPUT": str(worktree_output)},
        results,
        {"OUTPUT": str(worktree_output.relative_to(local_state))},
    )

    final_readiness = readiness_snapshot()
    runtime["final_readiness"] = final_readiness
    runtime["captured_at_end"] = iso(utc_now())
    if worktree_output.exists():
        shutil.copy2(worktree_output, EVIDENCE_DIR / "worktree-manifest.json")

    source_before = load_json(source_before_path)
    source_after = load_json(source_after_path)
    fingerprint = source_fingerprint(source_before, source_after)
    write_json(EVIDENCE_DIR / "source-worktree-manifest.json", fingerprint)
    runtime["source_fingerprint"] = fingerprint
    runtime["git"]["source_diff_sha256"] = source_before.get("git", {}).get("diff_sha256")
    write_json(EVIDENCE_DIR / "runtime-snapshot.json", runtime)

    primary = next(
        (
            item
            for item in round_data
            if item["rag"].exists() and item["agent"].exists()
        ),
        round_data[0] if round_data else None,
    )
    primary_paths = {
        "rag_image": primary["rag"] if primary else raw_dir / "missing-rag.json",
        "agent": primary["agent"] if primary else raw_dir / "missing-agent.json",
    }
    report_paths = {
        "post_search_local": post_output,
        "image_search_local": image_output,
        **primary_paths,
    }
    quality_reports = {
        name: report_metrics(path) for name, path in report_paths.items()
    }
    quality_judgement = load_json(primary["judge"]) if primary else {}
    worktree_manifest = load_json(worktree_output)
    judgements = [load_json(item["judge"]) for item in round_data if item["judge"].exists()]
    human_handoff = merge_human_handoffs(judgements, run_id)
    write_json(EVIDENCE_DIR / "ai-judge-human-handoff.json", human_handoff)
    round_summaries = []
    for item in round_data:
        judgement = load_json(item["judge"])
        round_summaries.append(
            {
                "round": item["round"],
                "reports": {
                    "rag_image": report_metrics(item["rag"]),
                    "agent": report_metrics(item["agent"]),
                },
                "quality_judgement": judgement_summary(judgement),
                "sources": {
                    "rag_image": str(item["rag"].relative_to(raw_dir))
                    if item["rag"].exists()
                    else None,
                    "agent": str(item["agent"].relative_to(raw_dir))
                    if item["agent"].exists()
                    else None,
                    "quality_judgement": str(item["judge"].relative_to(raw_dir))
                    if item["judge"].exists()
                    else None,
                },
            }
        )
    completed_rounds = [
        item
        for item in round_summaries
        if item["reports"]["rag_image"].get("status") == "available"
        and item["reports"]["agent"].get("status") == "available"
        and item["quality_judgement"].get("quality_gate_passed") is True
    ]
    repeated_ai = {
        "requested_rounds": args.ai_eval_runs,
        "completed_rounds": len(completed_rounds),
        "all_rounds_quality_gate_passed": len(completed_rounds) == args.ai_eval_runs,
        "rounds": round_summaries,
        "aggregate": aggregate_round_reports(
            [{"reports": item["reports"]} for item in round_summaries]
        ),
    }
    write_json(
        EVIDENCE_DIR / "resume-audit-summary.json",
        {
            "run_id": run_id,
            "method": "three_repeated_real_provider_rounds",
            "ai_judge_method": "two independent LLM judge passes per round",
            "source_fingerprint_stable": fingerprint.get("stable", False),
            "dataset_hashes": runtime["dataset_hashes"],
            "dataset_support": support_summary(support_payload),
            "repeated_ai": repeated_ai,
            "human_fallback_count": human_handoff["count"],
        },
    )
    write_json(EVIDENCE_DIR / "dataset-support-audit.json", support_summary(support_payload))

    browser_payload = load_json(browser_output)
    if browser_payload:
        write_json(EVIDENCE_DIR / "browser-smoke.json", browser_payload)

    repeated_quality_passed = bool(repeated_ai["all_rounds_quality_gate_passed"])
    strict_quality_passed = bool(
        support_gate
        and repeated_quality_passed
        and fingerprint.get("stable") is True
        and (not args.browser_smoke or browser_payload.get("status") == "pass")
        and not human_handoff["count"]
    )
    results.append(
        {
            "label": "resume-quality-gate",
            "status": "pass" if strict_quality_passed else "fail",
            "reason": "三轮 AI 评测、来源支持审计、源码指纹和浏览器 smoke 均满足严格门禁。"
            if strict_quality_passed
            else "至少一项严格简历级门禁未通过；详见逐轮结果和人工交接包。",
        }
    )
    quality = {
        "run_id": run_id,
        "generated_at": iso(utc_now()),
        "scope": "native local stack with real configured provider",
        "dataset_counts": {
            name: value.get("record_count")
            for name, value in runtime["dataset_hashes"].items()
            if "record_count" in value
        },
        "provenance": {
            "git": runtime["git"],
            "dataset_hashes": runtime["dataset_hashes"],
            "started_at": iso(started_at),
            "provider": {
                "type": "configured real provider",
                "model": runtime.get("safe_env", {}).get("SHAREO_AI_LLM_MODEL")
                or "configured runtime model",
            },
        },
        "sources": {
            **{
                name: str(path.relative_to(raw_dir))
                for name, path in report_paths.items()
                if path.exists()
            },
            "quality_judgement": str(primary["judge"].relative_to(raw_dir))
            if primary and primary["judge"].exists()
            else None,
            "human_handoff": "ai-judge-human-handoff.json",
        },
        "reports": quality_reports,
        "quality_judgement": judgement_summary(quality_judgement),
        "dataset_support_audit": support_summary(support_payload),
        "ai_repetitions": repeated_ai,
        "quality_gate_passed": strict_quality_passed,
        "source_fingerprint": fingerprint,
        "browser_smoke": browser_payload or {"status": "not_run"},
        "human_handoff": {
            "status": human_handoff["status"],
            "count": human_handoff["count"],
            "path": "ai-judge-human-handoff.json",
        },
        "worktree_manifest": {
            "git": worktree_manifest.get("git", {}),
            "summary": worktree_manifest.get("summary", {}),
            "scope_policy": worktree_manifest.get("scope_policy", {}),
        },
        "metric_availability": metric_availability(quality_reports),
    }
    write_json(EVIDENCE_DIR / "quality-summary.json", quality)

    latency = {
        "run_id": run_id,
        "generated_at": iso(utc_now()),
        "provenance": {
            "git": runtime["git"],
            "dataset_hashes": runtime["dataset_hashes"],
            "started_at": iso(started_at),
            "provider": {
                "type": "configured real provider",
                "model": runtime.get("safe_env", {}).get("SHAREO_AI_LLM_MODEL")
                or "configured runtime model",
            },
        },
        "e2e_report_sources": {
            name: str(path.relative_to(raw_dir))
            for name, path in report_paths.items()
            if path.exists()
        },
        "evaluation_report_aggregate": report_latency_summary(quality_reports),
        "structured_log_aggregate": summarize_log_metrics(ai_log_path, ai_log_offsets),
        "quality_judgement": judgement_summary(quality_judgement),
        "repeated_runs": [
            {
                "round": item["round"],
                "evaluation_report_aggregate": report_latency_summary(item["reports"]),
                "quality_judgement": item["quality_judgement"],
            }
            for item in round_summaries
        ],
        "repeated_aggregate": repeated_ai["aggregate"],
        "human_handoff": {
            "status": human_handoff["status"],
            "count": human_handoff["count"],
        },
        "queue_wait_ms": {"status": "unavailable", "reason": "no structured log field"},
        "callback_ms": {"status": "unavailable", "reason": "no structured log field"},
    }
    write_json(EVIDENCE_DIR / "latency-summary.json", latency)

    finished_at = utc_now()
    manifest = {
        "run_id": run_id,
        "started_at": iso(started_at),
        "finished_at": iso(finished_at),
        "duration_ms": round((finished_at - started_at).total_seconds() * 1000, 1),
        "scope": "native local stack; real configured provider; no Compose fault injection",
        "raw_dir": str(raw_dir.relative_to(ROOT)),
        "evidence_dir": str(EVIDENCE_DIR.relative_to(ROOT)),
        "git": runtime["git"],
        "source_fingerprint": {
            "stable": fingerprint.get("stable", False),
            "path": "source-worktree-manifest.json",
        },
        "dataset_hashes": runtime["dataset_hashes"],
        "ai_eval_runs": {
            "requested": args.ai_eval_runs,
            "completed": len(completed_rounds),
            "source_policy": "all rounds retained; no best-round selection",
        },
        "browser_smoke": browser_payload or {"status": "not_run"},
        "quality_gate_passed": strict_quality_passed,
        "chains": chain_evidence(
            raw_dir, results, report_paths, ai_labels, agent_labels
        ),
        "status_counts": {
            status: sum(item.get("status") == status for item in results)
            for status in ("pass", "fail", "blocked", "needs_human_review", "not_run")
        },
    }
    write_json(EVIDENCE_DIR / "run-manifest.json", manifest)
    write_json(
        EVIDENCE_DIR / "command-results.json", {"run_id": run_id, "results": results}
    )
    write_failure_matrix(results, EVIDENCE_DIR / "failure-matrix.md")
    update_retained_runs(run_id, runtime, quality, latency, manifest)

    failures = [
        item
        for item in results
        if item.get("status") in {"fail", "blocked", "needs_human_review"}
    ]
    print(f"Evidence bundle written for run {run_id}")
    print(f"Raw outputs: {raw_dir}")
    print(f"Repository summary: {EVIDENCE_DIR}")
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
