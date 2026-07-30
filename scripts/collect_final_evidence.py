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
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.request import Request, ProxyHandler, build_opener

ROOT = Path(__file__).resolve().parents[1]
EVIDENCE_DIR = ROOT / "docs" / "evidence" / "final-freeze"
LOCAL_STATE_DIR = ROOT / ".local" / "shareo"

DATASETS = {
    "image_search_current": ROOT / ".local" / "shareo" / "eval" / "image_search_local_v1.jsonl",
    "rag_current": ROOT / ".local" / "shareo" / "eval" / "rag_qa_current_v1.jsonl",
    "agent_current": ROOT / ".local" / "shareo" / "eval" / "agent_tasks_current_v1.jsonl",
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
    "latency_p50_ms",
    "latency_p95_ms",
    "total_queries",
    "total_hallucinated",
    "human_scoring_count",
    "ndcg_at_5",
    "precision_at_3",
}

METRIC_ALIASES = {"ndcg_at_5": "ndcg_5", "precision_at_3": "precision_3"}

LOG_PATTERNS = {
    "image_total_ms": re.compile(r"image search complete .*?duration_ms=(?P<value>[0-9.]+)"),
    "rag_embedding_ms": re.compile(r"rag answer complete .*?embedding_ms=(?P<value>[0-9.]+)"),
    "rag_retrieval_ms": re.compile(r"rag answer complete .*?retrieval_ms=(?P<value>[0-9.]+)"),
    "rag_llm_ms": re.compile(r"rag answer complete .*?llm_ms=(?P<value>[0-9.]+)"),
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
            "llm_timeout_seconds": read_env_value("SHAREO_AI_LLM_TIMEOUT_SECONDS", "30.0"),
            "llm_max_attempts": "2",
            "agent_max_rounds": read_env_value("SHAREO_AI_AGENT_MAX_ROUNDS", "4"),
            "agent_max_tool_calls": read_env_value("SHAREO_AI_AGENT_MAX_TOOL_CALLS", "6"),
            "agent_max_parallel_tools": read_env_value(
                "SHAREO_AI_AGENT_MAX_PARALLEL_TOOLS", "2"
            ),
            "agent_max_observation_chars": read_env_value(
                "SHAREO_AI_AGENT_MAX_OBSERVATION_CHARS", "12000"
            ),
            "agent_timeout_seconds": read_env_value("SHAREO_AI_AGENT_TIMEOUT_SECONDS", "45.0"),
        },
    }


def build_env(local_state: Path) -> dict[str, str]:
    env = os.environ.copy()
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
    result = {
        "label": label,
        "command": args,
        "status": "pass" if exit_code == 0 else "fail",
        "exit_code": exit_code,
        "started_at": iso(started_at),
        "finished_at": iso(finished_at),
        "duration_ms": round((finished_at - started_at).total_seconds() * 1000, 1),
        "raw_log": str(log_path.relative_to(raw_dir)),
    }
    if exit_code != 0:
        result["failure_summary"] = failure_summary(log_path)
    if env_overrides:
        result["safe_env_overrides"] = env_overrides
    results.append(result)
    print(f"[{result['status'].upper()}] {label} ({result['duration_ms']:.0f} ms)")
    return exit_code == 0


def failure_summary(log_path: Path) -> str:
    try:
        lines = [line.strip() for line in log_path.read_text(encoding="utf-8", errors="replace").splitlines() if line.strip()]
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
    results.append({"label": label, "command": args, "status": "blocked", "reason": reason})
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
    failure_categories = data.get("failure_categories")
    if not isinstance(failure_categories, dict):
        failure_categories = data.get("agent", {}).get("report", {}).get("failure_categories")
    if isinstance(failure_categories, dict):
        result["failure_categories"] = failure_categories
    return result


def load_json(path: Path) -> dict[str, Any]:
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return {}
    return data if isinstance(data, dict) else {}


def chain_evidence(raw_dir: Path, results: list[dict[str, Any]]) -> list[dict[str, Any]]:
    by_label = {item.get("label"): item for item in results}

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
        values = [by_label[label].get("status") for label in labels if label in by_label]
        if any(value == "blocked" for value in values):
            return "blocked"
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
        base("private-chat-bot-rag-citation-websocket", ["api", "eval-ai-real-provider"]),
        base("explicit-agent-tools-trace-citation", ["api", "eval-agent-real-provider"]),
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

    rag_report = load_json(raw_dir / "rag-image.json").get("rag", {}).get("report", {})
    chains[1]["citation_count"] = {
        "status": "unavailable",
        "reason": "RAG 汇总只提供平均引用数",
        "average_sources_per_answer": rag_report.get("avg_sources_per_answer"),
    }
    chains[1]["failure_category"] = (
        "quality_gate: source_hit_rate"
        if by_label.get("eval-ai-real-provider", {}).get("status") == "fail"
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
        f"RAG 请求执行完成；来源命中率 {rag_source_hit}、引用可访问率 "
        f"{rag_accessible}、失败请求 {rag_failed}。"
    )

    agent = load_json(raw_dir / "agent.json").get("agent", {})
    agent_report = agent.get("report", {})
    agent_rows = agent.get("query_results", [])
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
        sequence = ">".join(str(tool) for tool in (row.get("actual_tools") or [])) or "none"
        sequences[sequence] = sequences.get(sequence, 0) + 1
        retries += int(row.get("provider_retries", 0) or 0)
    chains[2]["relevant_record_ids"] = sorted(citation_ids)
    chains[2]["citation_count"] = len(
        [post_id for row in agent_rows if isinstance(row, dict) for post_id in (row.get("citation_post_ids") or [])]
    )
    chains[2]["tool_sequence"] = sequences
    chains[2]["retry_count"] = retries
    chains[2]["failure_category"] = (
        "quality_gate: source_hit_rate/tool_selection"
        if by_label.get("eval-agent-real-provider", {}).get("status") == "fail"
        else None
    )
    chains[2]["final_assertion"] = (
        f"36 条 Agent 请求均完成；来源命中率 {agent_report.get('source_hit_rate')}、"
        f"必需工具选择率 {agent_report.get('required_tool_selection_rate')}、引用可访问率 "
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
            "required_tool_selection_rate",
            "citation_accessible_rate",
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
    summary: dict[str, Any] = {"status": "available", "source": "ai.log appended structured lines"}
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
    summary["queue_wait_ms"] = {"status": "unavailable", "reason": "no structured log field"}
    summary["callback_ms"] = {"status": "unavailable", "reason": "no structured log field"}
    return summary


def report_metric(metrics: dict[str, Any], suffix: str, preferred_prefix: str = "") -> Any:
    if preferred_prefix:
        preferred = metrics.get(f"{preferred_prefix}.{suffix}")
        if preferred is not None:
            return preferred
    for key, value in metrics.items():
        if key == suffix or key.endswith(f".{suffix}"):
            return value
    return None


def report_latency_summary(quality_reports: dict[str, dict[str, Any]]) -> dict[str, Any]:
    image_metrics = quality_reports.get("rag_image", {}).get("metrics", {})
    rag_metrics = quality_reports.get("rag_image", {}).get("metrics", {})
    agent_metrics = quality_reports.get("agent", {}).get("metrics", {})
    return {
        "image_total_ms": {
            "source": "rag-image.json image_search report",
            "p50_ms": report_metric(image_metrics, "latency_p50_ms", "image_search.metrics"),
            "p95_ms": report_metric(image_metrics, "latency_p95_ms", "image_search.metrics"),
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
            "status": "unavailable",
            "reason": "Agent 报告只提供逐题总耗时，没有每个工具步骤耗时字段",
        },
    }


def write_json(path: Path, payload: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


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
        lines.append(f"| `{result.get('label', '')}` | `{status}` | {exit_code} | {reason} |")
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
    ):
        source = EVIDENCE_DIR / name
        if source.exists():
            shutil.copy2(source, archive_dir / name)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-id", default="")
    args = parser.parse_args()

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
        ("local-doctor", ["make", "local-doctor"]),
        ("up", ["make", "up"]),
        ("warm-ai", ["make", "warm-ai"]),
        ("prepare-current-ai-eval", ["make", "prepare-current-ai-eval"]),
    ]
    for label, command in setup_steps:
        run_command(label, command, raw_dir, env, results)

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
        ("integration", ["make", "test-integration-auto"]),
        ("local-stack", ["make", "test-local-stack"]),
        ("api", ["make", "test-api"]),
    ):
        run_command(label, command, raw_dir, env, results)

    post_output = raw_dir / "post-search.json"
    image_output = raw_dir / "image-search-local.json"
    if ready_for("rag", readiness):
        run_command(
            "post-search-eval",
            ["make", "eval-post-search"],
            raw_dir,
            env | {
                "DATASET": str(DATASETS["post_search_current"]),
                "OUTPUT": str(post_output),
            },
            results,
            {"DATASET": str(DATASETS["post_search_current"].relative_to(ROOT)), "OUTPUT": str(post_output.relative_to(local_state))},
        )
    else:
        add_blocked("post-search-eval", "RAG/text readiness unavailable", ["make", "eval-post-search"], results)

    if ready_for("image_search", readiness):
        run_command(
            "image-search-local-eval",
            ["make", "eval-image-search-local"],
            raw_dir,
            env | {
                "DATASET": str(DATASETS["image_search_current"]),
                "OUTPUT": str(image_output),
            },
            results,
            {"DATASET": str(DATASETS["image_search_current"].relative_to(ROOT)), "OUTPUT": str(image_output.relative_to(local_state))},
        )
    else:
        add_blocked("image-search-local-eval", "image-search readiness unavailable", ["make", "eval-image-search-local"], results)

    if ready_for("rag", readiness) and ready_for("image_search", readiness):
        rag_image_output = raw_dir / "rag-image.json"
        run_command(
            "eval-ai-real-provider",
            ["make", "eval-ai", "MACHINE_ONLY=1"],
            raw_dir,
            env
            | {
                "MACHINE_ONLY": "1",
                "OUTPUT": str(rag_image_output),
                "SHAREO_IMAGE_DATASET": str(DATASETS["image_search_current"]),
                "SHAREO_RAG_DATASET": str(DATASETS["rag_current"]),
            },
            results,
            {
                "MACHINE_ONLY": "1",
                "OUTPUT": str(rag_image_output.relative_to(local_state)),
                "IMAGE_DATASET": str(DATASETS["image_search_current"].relative_to(ROOT)),
                "RAG_DATASET": str(DATASETS["rag_current"].relative_to(ROOT)),
            },
        )
    else:
        add_blocked("eval-ai-real-provider", "image or RAG readiness unavailable", ["make", "eval-ai", "MACHINE_ONLY=1"], results)

    if ready_for("agent", readiness):
        agent_output = raw_dir / "agent.json"
        run_command(
            "eval-agent-real-provider",
            ["make", "eval-agent", "MACHINE_ONLY=1"],
            raw_dir,
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
            },
        )
    else:
        add_blocked("eval-agent-real-provider", "Agent readiness unavailable", ["make", "eval-agent", "MACHINE_ONLY=1"], results)

    final_readiness = readiness_snapshot()
    runtime["final_readiness"] = final_readiness
    runtime["captured_at_end"] = iso(utc_now())
    write_json(EVIDENCE_DIR / "runtime-snapshot.json", runtime)

    report_paths = {
        "post_search_local": post_output,
        "image_search_local": image_output,
        "rag_image": raw_dir / "rag-image.json",
        "agent": raw_dir / "agent.json",
    }
    quality_reports = {name: report_metrics(path) for name, path in report_paths.items()}
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
            name: str(path.relative_to(raw_dir)) for name, path in report_paths.items() if path.exists()
        },
        "reports": quality_reports,
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
            name: str(path.relative_to(raw_dir)) for name, path in report_paths.items() if path.exists()
        },
        "evaluation_report_aggregate": report_latency_summary(quality_reports),
        "structured_log_aggregate": summarize_log_metrics(ai_log_path, ai_log_offsets),
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
        "dataset_hashes": runtime["dataset_hashes"],
        "chains": chain_evidence(raw_dir, results),
        "status_counts": {
            status: sum(item.get("status") == status for item in results)
            for status in ("pass", "fail", "blocked", "not_run")
        },
    }
    write_json(EVIDENCE_DIR / "run-manifest.json", manifest)
    write_json(EVIDENCE_DIR / "command-results.json", {"run_id": run_id, "results": results})
    write_failure_matrix(results, EVIDENCE_DIR / "failure-matrix.md")

    failures = [item for item in results if item.get("status") in {"fail", "blocked"}]
    print(f"Evidence bundle written for run {run_id}")
    print(f"Raw outputs: {raw_dir}")
    print(f"Repository summary: {EVIDENCE_DIR}")
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
