#!/usr/bin/env python3
"""Exercise an isolated ShareO OrbStack pilot with synthetic data.

The script deliberately talks to the pilot through its public/internal HTTP
API. Direct database writes are limited to promoting a newly registered
synthetic admin (there is no admin bootstrap API), inserting synthetic
conversation fixtures for the query baseline, and reading ACL state. It also
writes a short-lived synthetic presence key and temporarily changes Redis ACL
rules; both are restored in finally blocks.
"""

from __future__ import annotations

import argparse
import hashlib
import io
import json
import os
import statistics
import subprocess
import time
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit

import httpx
from PIL import Image, ImageDraw
from websockets.sync.client import connect


workspace = Path(__file__).resolve().parents[1]
demo_llm_base_url = "http://llm-fixture:8000/v1"
core_services = ("postgres", "redis", "minio", "app", "ai-service")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "phase",
        choices=["init", "prime", "search", "faults", "presence", "baseline", "all"],
    )
    parser.add_argument("--base-url", default="http://127.0.0.1:18091")
    parser.add_argument("--project-name", default="shareo-orbstack-reliable-20261123")
    parser.add_argument("--state-dir")
    parser.add_argument("--compose-file", default="deploy/compose.orbstack.yaml")
    parser.add_argument("--db-service", default="postgres")
    parser.add_argument("--ai-service", default="ai-service")
    parser.add_argument("--app-service", default="app")
    parser.add_argument("--redis-service", default="redis")
    parser.add_argument("--presence-command", default="EXISTS")
    parser.add_argument("--requests-per-size", type=int, default=20)
    parser.add_argument(
        "--demo",
        action="store_true",
        help="require the local fixed-response LLM fixture and demo phases",
    )
    return parser.parse_args()


args = parse_args()
base = args.base_url.rstrip("/")
state_dir = (
    workspace / args.state_dir
    if args.state_dir
    else workspace / ".local" / "shareo" / args.project_name
).resolve()
env_file = state_dir / ".env"
state_file = state_dir / "smoke-state.json"
credentials_file = state_dir / "pilot-credentials.json"
compose_file = (workspace / args.compose_file).resolve()
target_validated = False
client: httpx.Client | None = None


def fail(message: str) -> None:
    raise SystemExit(f"target validation failed: {message}")


def parse_loopback_port(value: str) -> int:
    parsed = urlsplit(value)
    if parsed.scheme != "http" or parsed.path not in ("", "/") or parsed.query:
        fail(f"base URL must be plain loopback HTTP without path/query: {value!r}")
    if parsed.hostname not in ("127.0.0.1", "::1"):
        fail(f"base URL must use 127.0.0.1 or ::1: {value!r}")
    try:
        port = parsed.port
    except ValueError:
        fail(f"base URL has an invalid port: {value!r}")
    if port is None:
        fail(f"base URL must include an explicit port: {value!r}")
    return port


def run_compose(*parts: str, extra_env: dict[str, str] | None = None) -> str:
    env = os.environ.copy()
    if extra_env:
        env.update(extra_env)
    command = [
        "docker",
        "--context",
        "orbstack",
        "compose",
        "--project-name",
        args.project_name,
        "--env-file",
        str(env_file),
        "-f",
        str(compose_file),
        *parts,
    ]
    result = subprocess.run(
        command,
        cwd=workspace,
        env=env,
        text=True,
        capture_output=True,
        check=True,
        timeout=240,
    )
    return result.stdout.strip()


def compose_config() -> dict[str, Any]:
    try:
        output = run_compose(
            "--profile",
            "fixture",
            "config",
            "--format",
            "json",
        )
    except subprocess.CalledProcessError as exc:
        fail(f"Compose config failed: {exc.stderr.strip()[:400]}")
    return json.loads(output)


def validate_target() -> None:
    global target_validated
    port = parse_loopback_port(args.base_url)
    if not args.project_name.startswith("shareo-orbstack-"):
        fail(f"project must be an isolated shareo-orbstack-* project: {args.project_name!r}")
    if args.project_name == "shareo-orbstack":
        fail("refusing the original shareo-orbstack project")
    if demo_phase() and not args.demo:
        fail(f"phase {args.phase!r} requires --demo")
    expected_state = (workspace / ".local" / "shareo" / args.project_name).resolve()
    if state_dir != expected_state:
        fail(f"state directory must be {expected_state}: {state_dir}")
    if not compose_file.is_file():
        fail(f"Compose file is missing: {compose_file}")

    config = compose_config()
    if config.get("name") != args.project_name:
        fail(f"Compose project resolves to {config.get('name')!r}")
    values = read_env_file()
    volume_prefix = values.get("SHAREO_ORBSTACK_VOLUME_PREFIX", "shareo-orbstack")
    if volume_prefix != args.project_name:
        fail(f"volume prefix must equal project name, got {volume_prefix!r}")
    app = config.get("services", {}).get(args.app_service, {})
    published = {
        int(item["published"])
        for item in app.get("ports", [])
        if item.get("host_ip") in ("127.0.0.1", "::1") and item.get("published")
    }
    if published != {port}:
        fail(f"Compose app publishes {sorted(published)}, base URL uses {port}")
    fixture = config.get("services", {}).get("llm-fixture", {})
    if fixture.get("ports"):
        fail("llm-fixture must not publish a host port")

    rows = raw_service_rows()
    app_rows = [row for row in rows if row.get("Service") == args.app_service]
    if len(app_rows) != 1:
        fail(f"expected one running {args.app_service} container, got {len(app_rows)}")
    labels = parse_labels(app_rows[0].get("Labels"))
    if labels.get("com.docker.compose.project") != args.project_name:
        fail(f"app container belongs to {labels.get('com.docker.compose.project')!r}")
    if labels.get("com.docker.compose.service") != args.app_service:
        fail("app container service label mismatch")
    inspected = json.loads(
        subprocess.run(
            [
                "docker",
                "--context",
                "orbstack",
                "inspect",
                app_rows[0]["ID"],
                "--format",
                "{{json .NetworkSettings.Ports}}",
            ],
            text=True,
            capture_output=True,
            check=True,
        ).stdout
    )
    bound = {
        int(binding["HostPort"])
        for binding in inspected.get("8080/tcp") or []
        if binding.get("HostIp") in ("127.0.0.1", "::1")
    }
    if bound != {port}:
        fail(f"running app publishes {sorted(bound)}, base URL uses {port}")

    if args.demo:
        ai_rows = [row for row in rows if row.get("Service") == args.ai_service]
        if len(ai_rows) != 1:
            fail(f"expected one running {args.ai_service} container")
        ai_labels = parse_labels(ai_rows[0].get("Labels"))
        if ai_labels.get("com.docker.compose.project") != args.project_name:
            fail(f"AI container belongs to {ai_labels.get('com.docker.compose.project')!r}")
        environment = app_environment(ai_rows[0]["ID"])
        llm = environment.get("SHAREO_AI_LLM_BASE_URL", "")
        if llm.rstrip("/") != demo_llm_base_url:
            fail(f"AI LLM must target the local fixture, got {llm!r}")
    target_validated = True
    print(
        f"PASS target isolation project={args.project_name} loopback={base} port={port}",
        flush=True,
    )


def parse_labels(raw: str | None) -> dict[str, str]:
    labels: dict[str, str] = {}
    for item in (raw or "").split(","):
        if "=" in item:
            key, value = item.split("=", 1)
            labels[key] = value
    return labels


def app_environment(container_id: str) -> dict[str, str]:
    output = subprocess.run(
        [
            "docker",
            "--context",
            "orbstack",
            "inspect",
            container_id,
            "--format",
            "{{range .Config.Env}}{{println .}}{{end}}",
        ],
        text=True,
        capture_output=True,
        check=True,
    ).stdout
    environment: dict[str, str] = {}
    for line in output.splitlines():
        if "=" in line:
            key, value = line.split("=", 1)
            environment[key] = value
    return environment


def demo_phase() -> bool:
    return args.phase in ("prime", "search", "faults", "all")


def require_target() -> None:
    if not target_validated:
        raise SystemExit("target validation must run before any pilot side effect")


def ensure_state_dir() -> None:
    require_target()
    state_dir.mkdir(parents=True, exist_ok=True)
    os.chmod(state_dir, 0o700)


def read_env_file() -> dict[str, str]:
    values: dict[str, str] = {}
    if not env_file.exists():
        return values
    for line in env_file.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        values[key.strip()] = value.strip().strip("\"'")
    return values


def compose(*parts: str, extra_env: dict[str, str] | None = None) -> str:
    require_target()
    return run_compose(*parts, extra_env=extra_env)


def sql(query: str) -> str:
    values = read_env_file()
    admin_user = values.get("SHAREO_PG_ADMIN_USER", "shareo_admin")
    database = values.get("SHAREO_PG_DATABASE", "shareo")
    return compose(
        "exec",
        "-T",
        args.db_service,
        "psql",
        "-U",
        admin_user,
        "-d",
        database,
        "-qAt",
        "-v",
        "ON_ERROR_STOP=1",
        "-c",
        query,
    )


def redis(*parts: str) -> str:
    return compose("exec", "-T", args.redis_service, "redis-cli", "--raw", *parts)


def redis_json(*parts: str) -> Any:
    return json.loads(
        compose("exec", "-T", args.redis_service, "redis-cli", "--json", *parts) or "null"
    )


def request(
    method: str,
    path: str,
    token: str | None = None,
    *,
    expect: int | tuple[int, ...] = 200,
    **kwargs: Any,
) -> httpx.Response:
    headers = dict(kwargs.pop("headers", {}) or {})
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if client is None:
        raise RuntimeError("HTTP client is not initialized")
    response = client.request(method, base + path, headers=headers, **kwargs)
    expected = (expect,) if isinstance(expect, int) else expect
    if response.status_code not in expected:
        raise AssertionError(f"{method} {path}: HTTP {response.status_code}: {response.text[:400]}")
    return response


def api(method: str, path: str, token: str | None = None, **kwargs: Any) -> Any:
    body = request(method, "/api/v1" + path, token, **kwargs).json()
    if body.get("code") != 0:
        raise AssertionError(f"{method} {path}: {body}")
    return body.get("data")


def wait_for(check, *, timeout: float = 120) -> Any:
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        result = check()
        if result:
            return result
        time.sleep(0.8)
    raise AssertionError(f"timeout when checking {getattr(check, '__name__', check)}")


def json_value(value: Any, expression: str) -> Any:
    return eval(expression, {"__builtins__": {}}, {"value": value})


def load_state() -> dict[str, Any]:
    if not state_file.exists():
        return {}
    return json.loads(state_file.read_text(encoding="utf-8"))


def write_state(state: dict[str, Any]) -> None:
    require_target()
    state_file.write_text(json.dumps(state), encoding="utf-8")
    os.chmod(state_file, 0o600)


def password() -> str:
    value = os.getenv("SHAREO_PILOT_TEST_PASSWORD", "").strip()
    if value:
        return value
    state = load_state()
    if state.get("password"):
        return state["password"]
    value = os.urandom(24).hex()
    state["password"] = value
    write_state(state)
    return value


def credentials() -> dict[str, str]:
    if credentials_file.exists():
        return json.loads(credentials_file.read_text(encoding="utf-8"))
    return {}


def login(username: str) -> str:
    return api(
        "POST",
        "/auth/login",
        json={"username": username, "password": password()},
    )["token"]


def register(username: str) -> dict[str, Any]:
    return api(
        "POST",
        "/auth/register",
        json={"username": username, "password": password()},
    )


def redis_write(command: str, *parts: str) -> str:
    require_target()
    return redis(command, *parts)


def image_bytes() -> bytes:
    image = Image.new("RGB", (256, 256), (22, 73, 154))
    drawing = ImageDraw.Draw(image)
    drawing.ellipse((60, 60, 196, 196), fill=(226, 253, 134))
    png = io.BytesIO()
    image.save(png, format="PNG")
    return png.getvalue()


def messages(conversation: int, token: str) -> list[dict[str, Any]]:
    return api("GET", f"/conversations/{conversation}/messages", token)["messages"]


def bot_replies(
    conversation: int, source: int, token: str
) -> list[tuple[dict[str, Any], dict[str, Any]]]:
    result = []
    for message in messages(conversation, token):
        raw = message.get("meta")
        if not raw:
            continue
        meta = raw if isinstance(raw, dict) else json.loads(raw)
        if meta.get("source_message_id") == source:
            result.append((message, meta))
    return result


def service_rows() -> list[dict[str, Any]]:
    require_target()
    return raw_service_rows()


def raw_service_rows() -> list[dict[str, Any]]:
    try:
        text = run_compose("ps", "--all", "--format", "json")
    except subprocess.CalledProcessError as exc:
        fail(f"Compose ps failed: {exc.stderr.strip()[:400]}")
    try:
        parsed = json.loads(text)
        return parsed if isinstance(parsed, list) else [parsed]
    except json.JSONDecodeError:
        return [json.loads(line) for line in text.splitlines() if line.strip()]


def verify_init() -> None:
    rows = service_rows()
    by_service = {row.get("Service") or row.get("Name"): row for row in rows}
    for service in core_services:
        row = by_service.get(service, {})
        state = str(row.get("State", "")).lower()
        health = str(row.get("Health", "")).lower()
        if state != "running" or health != "healthy":
            raise AssertionError(f"{service} is not healthy: {row}")
    init_row = by_service.get("postgres-init", {})
    if not init_row or str(init_row.get("State", "")).lower() != "exited":
        raise AssertionError(f"postgres-init did not exit: {init_row}")
    exit_code = init_row.get("ExitCode")
    if str(exit_code) != "0":
        raise AssertionError(f"postgres-init failed: {init_row}")
    if args.demo:
        fixture = by_service.get("llm-fixture", {})
        if not fixture or str(fixture.get("Health", "")).lower() != "healthy":
            raise AssertionError(f"llm-fixture is not healthy: {fixture}")
    print("PASS five core services healthy and postgres-init completed", flush=True)

    users = int(sql("SELECT count(*) FROM users;"))
    non_bot = int(sql("SELECT count(*) FROM users WHERE username <> 'shareo_bot';"))
    counts = {
        "posts": int(sql("SELECT count(*) FROM posts;")),
        "messages": int(sql("SELECT count(*) FROM messages;")),
        "outbox": int(sql("SELECT count(*) FROM bot_task_outbox;")),
        "image_vectors": int(sql("SELECT count(*) FROM ai.image_embeddings;")),
        "text_vectors": int(sql("SELECT count(*) FROM ai.post_chunk_embeddings;")),
    }
    if users != 1 or non_bot != 0 or any(counts.values()):
        raise AssertionError(f"pilot is not empty: users={users} counts={counts}")
    print(f"PASS empty database users={users} {counts}", flush=True)

    acl = sql(
        "SELECT "
        "has_schema_privilege('shareo_app','ai','CREATE'),"
        "has_table_privilege('shareo_app','ai.post_chunk_embeddings','INSERT'),"
        "has_schema_privilege('shareo_ai','public','CREATE'),"
        "has_table_privilege('shareo_ai','public.posts','INSERT');"
    )
    if acl != "f|f|f|f":
        raise AssertionError(f"unexpected role privileges: {acl}")
    print("PASS application and AI roles are confined to their schemas", flush=True)


def prime() -> None:
    request("GET", "/healthz")
    if args.demo:
        environment = app_environment(service_container_id(args.ai_service))
        llm = environment.get("SHAREO_AI_LLM_BASE_URL", "")
        if llm.rstrip("/") != demo_llm_base_url:
            raise AssertionError(f"demo AI LLM must use the local fixture, got {llm!r}")
    suffix = str(time.time_ns())[-9:]
    admin_name = f"pilot-admin-{suffix}"
    writer_name = f"pilot-writer-{suffix}"
    peer_name = f"pilot-peer-{suffix}"
    state = load_state()
    state.update(
        {
            "admin": admin_name,
            "writer": writer_name,
            "peer": peer_name,
            "password": password(),
        }
    )
    register(admin_name)
    sql("UPDATE users SET role = 'admin' WHERE username = " + quote_sql(admin_name) + ";")
    admin = login(admin_name)
    writer = register(writer_name)
    peer = register(peer_name)
    writer_token = writer["token"]
    uploaded = api(
        "POST",
        "/upload",
        writer_token,
        files={"file": ("pilot.png", image_bytes(), "image/png")},
    )
    image = uploaded["url"]
    post = api(
        "POST",
        "/posts",
        writer_token,
        json={
            "content": "轨道蓝点。拍摄夜景时用三脚架固定相机，避免长曝光画面晃动。",
            "images": [image],
        },
    )["id"]
    public_before = request("GET", f"/api/v1/posts/{post}", expect=(403, 404))
    results_before = api("GET", "/search", params={"q": "轨道蓝点"})
    if any(row["id"] == post for row in results_before["list"]):
        raise AssertionError("pending post leaked into public search")
    request("GET", image, expect=(403, 404))
    print(
        f"PASS pending post hidden before approval post={post} http={public_before.status_code}",
        flush=True,
    )

    api("POST", f"/admin/posts/{post}/approve", admin)
    assert api("GET", f"/posts/{post}")["id"] == post
    assert request("GET", image).content
    print(f"PASS approval and public image read post={post}", flush=True)

    peer_id = peer["user"]["id"]
    dm = api(
        "POST",
        "/conversations",
        writer_token,
        json={"user_id": peer_id},
    )["id"]
    with connect(
        f"{base.replace('http://', 'ws://')}/ws",
        origin=base,
        proxy=None,
        additional_headers={"Authorization": f"Bearer {peer['token']}"},
    ) as socket:
        message = api(
            "POST",
            f"/conversations/{dm}/messages",
            writer_token,
            json={"content": "合成私聊消息"},
        )
        for _ in range(8):
            frame = json.loads(socket.recv(timeout=5))
            if frame.get("type") == "new_message":
                assert frame["data"]["message"]["id"] == message["id"], frame
                break
        else:
            raise AssertionError("no WebSocket new_message")
    assert any(row["id"] == message["id"] for row in messages(dm, peer["token"]))
    print("PASS DM persistence and WebSocket delivery", flush=True)

    bot = int(sql("SELECT id FROM users WHERE username = 'shareo_bot';"))
    conversation = api(
        "POST",
        "/conversations",
        writer_token,
        json={"user_id": bot},
    )["id"]
    state.update(
        {
            "post": post,
            "image": image,
            "image_sha256": hashlib.sha256(request("GET", image).content).hexdigest(),
            "conversation": conversation,
            "peer_conversation": dm,
            "writer_token": writer_token,
            "peer_token": peer["token"],
            "writer_id": writer["user"]["id"],
            "peer_id": peer_id,
        }
    )
    write_state(state)
    credentials_file.write_text(
        json.dumps(
            {
                "admin": admin_name,
                "writer": writer_name,
                "peer": peer_name,
                "password": password(),
            }
        ),
        encoding="utf-8",
    )
    os.chmod(credentials_file, 0o600)
    print(
        f"PASS synthetic API seed post={post} bot_conversation={conversation}",
        flush=True,
    )


def quote_sql(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def app_container_id() -> str:
    rows = service_rows()
    matches = [row for row in rows if row.get("Service") == args.app_service]
    if len(matches) != 1:
        raise AssertionError(f"expected one {args.app_service} container: {rows}")
    return str(matches[0]["ID"])


def service_container_id(service: str) -> str:
    rows = service_rows()
    matches = [row for row in rows if row.get("Service") == service]
    if len(matches) != 1:
        raise AssertionError(f"expected one {service} container: {rows}")
    return str(matches[0]["ID"])


def search() -> None:
    state = load_state()
    post = state["post"]

    def text_indexed() -> bool:
        return (
            int(sql(f"SELECT count(*) FROM ai.post_chunk_embeddings WHERE post_id = {post};")) > 0
        )

    wait_for(text_indexed, timeout=250)
    results = api("GET", "/search", params={"q": "轨道蓝点"})
    assert any(row["id"] == post for row in results["list"]), results
    print("PASS pgvector text index and public text search", flush=True)

    def image_indexed() -> bool:
        return int(sql(f"SELECT count(*) FROM ai.image_embeddings WHERE post_id = {post};")) > 0

    wait_for(image_indexed, timeout=250)
    image_results = api("GET", "/search/images", params={"q": "蓝色背景上的圆形", "limit": 5})
    assert any(int(row["post_id"]) == post for row in image_results["results"]), image_results
    print("PASS pgvector image index and public semantic image search", flush=True)

    writer = login(state["writer"])
    conversation = state["conversation"]
    source = api(
        "POST",
        f"/conversations/{conversation}/messages",
        writer,
        json={"content": "拍摄夜景应该怎样固定相机？"},
    )["id"]
    replies = wait_for(
        lambda: bot_replies(conversation, source, writer),
        timeout=180,
    )
    reply, meta = replies[0]
    assert any(citation["post_id"] == post for citation in meta["citations"]), (reply, meta)
    state["citation_source"] = source
    write_state(state)
    print("PASS fixed Provider Bot reply with approved citation", flush=True)


def in_outbox(source: int) -> str:
    return sql(
        f"SELECT status || ':' || attempts FROM bot_task_outbox WHERE message_id = {source};"
    )


def pending_stream_entries(source: int) -> list[dict[str, str]]:
    entries = redis_json(
        "XPENDING",
        "shareo:stream:bot_tasks",
        "ai-workers",
        "-",
        "+",
        "1000",
    )
    pending_ids = {entry[0] for entry in entries or []}
    if not pending_ids:
        return []
    stream = redis_json(
        "XRANGE",
        "shareo:stream:bot_tasks",
        "-",
        "+",
        "COUNT",
        "1000",
    )
    result = []
    for stream_id, data in stream or []:
        if stream_id not in pending_ids:
            continue
        if data and isinstance(data[0], list):
            data = [item for pair in data for item in pair]
        values = dict(zip(data[::2], data[1::2], strict=False))
        if values.get("message_id") == str(source):
            result.append(values)
    return result


def faults() -> None:
    state = load_state()
    writer = login(state["writer"])
    conversation = state["conversation"]
    values = read_env_file()
    internal_token = values["SHAREO_INTERNAL_TOKEN"]

    assert redis("ACL", "SETUSER", "default", "-xadd") == "OK"
    try:
        source = api(
            "POST",
            f"/conversations/{conversation}/messages",
            writer,
            json={"content": "三脚架如何固定相机？"},
        )["id"]
        wait_for(
            lambda: (
                in_outbox(source).startswith("pending:")
                and int(in_outbox(source).split(":", 1)[1]) > 0
            ),
            timeout=25,
        )
        assert not bot_replies(conversation, source, writer)
        print(
            f"PASS outbox retained pending task while Redis XADD was disabled source={source}",
            flush=True,
        )
    finally:
        assert redis("ACL", "SETUSER", "default", "+xadd") == "OK"
    wait_for(lambda: bot_replies(conversation, source, writer), timeout=60)
    print("PASS outbox replayed after Redis XADD recovered", flush=True)

    reply, _ = bot_replies(conversation, source, writer)[0]
    response = request(
        "POST",
        "/internal/bot/reply",
        headers={
            "X-Internal-Token": internal_token,
            "Content-Type": "application/json",
        },
        json={
            "source_message_id": source,
            "conversation_id": conversation,
            "content": "重复回调测试",
        },
    )
    body = response.json()
    duplicate = body.get("data") or {}
    duplicate_row, _ = bot_replies(conversation, source, writer)[0]
    assert body.get("code") == 0, body
    assert duplicate.get("id") == reply["id"], (duplicate, reply)
    assert duplicate.get("content") == reply["content"], (duplicate, reply)
    assert duplicate_row["id"] == reply["id"]
    assert sql(f"SELECT count(*) FROM bot_replies WHERE source_message_id = {source};") == "1"
    print(
        "PASS repeated callback HTTP body and database both returned one existing reply",
        flush=True,
    )

    state["after_outbox_source"] = source
    write_state(state)
    try:
        bad_env = {"SHAREO_AI_INTERNAL_TOKEN_OVERRIDE": "pilot-invalid-internal-token"}
        compose(
            "up",
            "-d",
            "--no-deps",
            "--force-recreate",
            "--wait",
            args.ai_service,
            extra_env=bad_env,
        )
        source401 = api(
            "POST",
            f"/conversations/{conversation}/messages",
            writer,
            json={"content": "三脚架夜景拍摄如何防晃？"},
        )["id"]
        wait_for(lambda: pending_stream_entries(source401), timeout=25)
        time.sleep(2.2)
        assert not bot_replies(conversation, source401, writer)
        print(
            f"PASS internal 401 kept source={source401} pending with no reply",
            flush=True,
        )
    finally:
        assert redis("ACL", "SETUSER", "default", "+xadd") == "OK"
        compose(
            "up",
            "-d",
            "--no-deps",
            "--force-recreate",
            "--wait",
            args.ai_service,
        )
    replies = wait_for(lambda: bot_replies(conversation, source401, writer), timeout=180)
    assert replies[0][1].get("citations"), replies
    wait_for(
        lambda: not pending_stream_entries(source401),
        timeout=60,
    )
    assert sql(f"SELECT count(*) FROM bot_replies WHERE source_message_id = {source401};") == "1"
    print(
        f"PASS reclaimed source={source401} acknowledged and committed one cited reply",
        flush=True,
    )


def presence_command_calls(command: str) -> int:
    needle = f"cmdstat_{command.lower()}:"
    for line in redis("INFO", "commandstats").splitlines():
        if line.startswith(needle):
            return int(line.split("calls=", 1)[1].split(",", 1)[0])
    return 0


def presence() -> None:
    state = load_state()
    writer = login(state["writer"])
    peer_name = f"pilot-presence-peer-{str(time.time_ns())[-9:]}"
    peer = register(peer_name)
    peer_id = peer["user"]["id"]
    conversation = api("POST", "/conversations", writer, json={"user_id": peer_id})["id"]

    presence_key = f"ws:online:{peer_id}"
    try:
        assert redis_write("SET", presence_key, str(int(time.time())), "EX", "120") == "OK"
        before = presence_command_calls(args.presence_command)
        listed = api("GET", "/conversations", writer)
        after = presence_command_calls(args.presence_command)
        row = next(item for item in listed if item["id"] == conversation)
        assert row["online"] is True, row
        print(
            f"PASS Redis-only presence visible without a local WebSocket "
            f"redis_{args.presence_command.lower()}={after - before}",
            flush=True,
        )
        order_before = [item["id"] for item in listed]

        command_flag = "-" + args.presence_command.lower()
        assert redis("ACL", "SETUSER", "default", command_flag) == "OK"
        try:
            failed = api("GET", "/conversations", writer)
            assert [item["id"] for item in failed] == order_before
            failed_row = next(item for item in failed if item["id"] == conversation)
            assert failed_row["online"] is False, failed_row
            print(
                "PASS Redis presence failure keeps HTTP 200, marks non-local peer offline "
                "and preserves order",
                flush=True,
            )
        finally:
            assert redis("ACL", "SETUSER", "default", "+" + args.presence_command.lower()) == "OK"
        recovered = api("GET", "/conversations", writer)
        recovered_row = next(item for item in recovered if item["id"] == conversation)
        assert recovered_row["online"] is True, recovered_row
        print("PASS presence recovered after Redis ACL restoration", flush=True)
    finally:
        redis_write("DEL", presence_key)

    try:
        with connect(
            f"{base.replace('http://', 'ws://')}/ws",
            origin=base,
            proxy=None,
            additional_headers={"Authorization": f"Bearer {peer['token']}"},
        ):
            wait_for(lambda: online_for(conversation, writer), timeout=5)
            before = presence_command_calls(args.presence_command)
            listed = api("GET", "/conversations", writer)
            after = presence_command_calls(args.presence_command)
            row = next(item for item in listed if item["id"] == conversation)
            assert row["online"] is True, row
            print(
                f"PASS WebSocket hub hit online=true "
                f"redis_{args.presence_command.lower()}={after - before}",
                flush=True,
            )
    finally:
        wait_for(
            lambda: not online_for(conversation, writer),
            timeout=10,
        )
    print("PASS WebSocket disconnect clears the local presence flag", flush=True)


def online_for(conversation: int, token: str) -> bool:
    listed = api("GET", "/conversations", token)
    return any(item["id"] == conversation and item["online"] is True for item in listed)


def baseline() -> None:
    results = []
    for size in (1, 10, 100):
        name = f"pilot-list-{size}-{str(time.time_ns())[-9:]}"
        user = register(name)
        owner = user["user"]["id"]
        peers = "\n".join(
            f"({quote_sql(name + '-p' + str(index))}, 'synthetic-unused'),"
            for index in range(1, size + 1)
        ).rstrip(",")
        sql(
            f"WITH peers AS ("
            f"INSERT INTO users(username, password_hash) VALUES {peers} RETURNING id"
            f"), conversations_created AS ("
            f"INSERT INTO conversations(dm_key) "
            f"SELECT '{owner}:' || id FROM peers RETURNING id, "
            f"split_part(dm_key, ':', 2)::bigint AS peer"
            f"), members_created AS ("
            f"INSERT INTO conversation_members(conversation_id, user_id) "
            f"SELECT id, peer FROM conversations_created "
            f"UNION ALL SELECT id, {owner} FROM conversations_created"
            f") "
            f"INSERT INTO messages(conversation_id, sender_id, content) "
            f"SELECT id, peer, '合成查询基线消息' FROM conversations_created;"
        )
        # Keep the HTTP client warm without including the warm-up in the samples.
        api("GET", "/conversations", user["token"])
        before_exists = presence_command_calls("EXISTS")
        before_mget = presence_command_calls("MGET")
        started_at = time.perf_counter()
        latencies = []
        for _ in range(args.requests_per_size):
            started = time.perf_counter()
            listed = api("GET", "/conversations", user["token"])
            latencies.append((time.perf_counter() - started) * 1000)
        elapsed = (time.perf_counter() - started_at) * 1000
        after_exists = presence_command_calls("EXISTS")
        after_mget = presence_command_calls("MGET")
        assert len(listed) == size, (size, len(listed))
        ordered = sorted(latencies)
        p95 = ordered[min(len(ordered) - 1, max(0, int((len(ordered) - 1) * 0.95)))]
        row = {
            "conversations": size,
            "redis_exists": after_exists - before_exists,
            "redis_mget": after_mget - before_mget,
            "redis_rtt": (after_exists - before_exists) + (after_mget - before_mget),
            "window_ms": round(elapsed, 3),
            "median_ms": round(statistics.median(latencies), 3),
            "p95_ms": round(p95, 3),
            "samples": len(latencies),
        }
        results.append(row)
        print("QUERY_RESULT " + json.dumps(row, ensure_ascii=False), flush=True)
    print("QUERY_RESULTS " + json.dumps(results, ensure_ascii=False), flush=True)


def main() -> None:
    global client
    validate_target()
    ensure_state_dir()
    client = httpx.Client(trust_env=False, timeout=60)
    try:
        if args.phase in ("init", "all"):
            verify_init()
        if args.phase in ("prime", "all"):
            prime()
        if args.phase in ("search", "all"):
            search()
        if args.phase in ("faults", "all"):
            faults()
        if args.phase in ("presence", "all"):
            presence()
        if args.phase in ("baseline", "all"):
            baseline()
    finally:
        client.close()


if __name__ == "__main__":
    main()
