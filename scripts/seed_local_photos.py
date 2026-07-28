"""Rebuild the local manual-test corpus from all photos in the local photo directory."""

from __future__ import annotations

import json
import mimetypes
import os
import pathlib
import subprocess
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone


ROOT = pathlib.Path(__file__).resolve().parents[1]
PHOTO_DIR = ROOT / "resources/static/pictures"
STATE_DIR = pathlib.Path(
    os.environ.get("SHAREO_LOCAL_PHOTO_STATE_DIR", ROOT / ".local/shareo/local-photo-seed")
)
CAPTIONS_FILE = pathlib.Path(
    os.environ.get("SHAREO_LOCAL_PHOTO_CAPTIONS", STATE_DIR / "captions.tsv")
)
MANIFEST_FILE = STATE_DIR / "manifest.json"
BACKUP_DIR = pathlib.Path(
    os.environ.get("SHAREO_LOCAL_PHOTO_BACKUP_DIR", "/tmp/shareo-local/local-photo-seed")
)
BASE_URL = os.environ.get("SHAREO_BASE_URL", "http://127.0.0.1:8080").rstrip("/")
AI_URL = os.environ.get("SHAREO_AI_BASE_URL", "http://127.0.0.1:8000").rstrip("/")
EXPECTED_PHOTOS = int(os.environ.get("SHAREO_LOCAL_PHOTO_EXPECTED_COUNT", "47"))
SLEEP_SECONDS = float(os.environ.get("SHAREO_LOCAL_PHOTO_SLEEP_SECONDS", "3"))


def env_file_value(key: str) -> str:
    env_file = ROOT / ".env"
    if not env_file.exists():
        return ""
    for raw_line in env_file.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        name, value = line.split("=", 1)
        if name.strip() != key:
            continue
        return value.strip().strip("\"'")
    return ""


def configured(key: str, default: str = "") -> str:
    return os.environ.get(key) or env_file_value(key) or default


DB_PASSWORD = configured("SHAREO_DB_PASSWORD")
if not DB_PASSWORD:
    config_file = ROOT / "config.yaml"
    if config_file.exists():
        for line in config_file.read_text(encoding="utf-8").splitlines():
            if line.strip().startswith("password:"):
                DB_PASSWORD = line.split(":", 1)[1].strip().strip("\"'")
                break
DB_PASSWORD = DB_PASSWORD or "shareo_pass"
INTERNAL_TOKEN = configured("SHAREO_INTERNAL_TOKEN", "shareo-local-internal")
APP_PORT = configured("SHAREO_APP_PORT", "8080")


def fail(message: str) -> None:
    raise RuntimeError(message)


def http_request(
    method: str,
    url: str,
    payload: object | None = None,
    token: str = "",
    file_path: pathlib.Path | None = None,
    internal_token: bool = False,
) -> tuple[int, bytes]:
    headers = {"Accept": "application/json"}
    if token:
        if internal_token:
            headers["X-Internal-Token"] = token
        else:
            headers["Cookie"] = f"token={token}"
    if file_path is not None:
        boundary = b"----ShareOLocalPhotoSeed"
        content_type = mimetypes.guess_type(file_path.name)[0] or "application/octet-stream"
        data = file_path.read_bytes()
        body = b"--" + boundary + b"\r\n"
        body += (
            b'Content-Disposition: form-data; name="file"; filename="'
            + file_path.name.encode()
            + b'"\r\nContent-Type: '
            + content_type.encode()
            + b"\r\n\r\n"
            + data
            + b"\r\n--"
            + boundary
            + b"--\r\n"
        )
        headers["Content-Type"] = f"multipart/form-data; boundary={boundary.decode()}"
    elif payload is not None:
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        headers["Content-Type"] = "application/json"
    else:
        body = None

    request = urllib.request.Request(url, data=body, headers=headers, method=method)
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        with opener.open(request, timeout=90) as response:
            return response.status, response.read()
    except urllib.error.HTTPError as exc:
        return exc.code, exc.read()
    except urllib.error.URLError as exc:
        return 0, str(exc.reason).encode("utf-8", errors="replace")


def json_body(status: int, raw: bytes, label: str) -> dict:
    if status < 200 or status >= 300:
        fail(f"{label} failed with HTTP {status}")
    try:
        value = json.loads(raw.decode("utf-8"))
    except json.JSONDecodeError as exc:
        fail(f"{label} returned invalid JSON: {exc.msg}")
    if not isinstance(value, dict):
        fail(f"{label} returned an unexpected JSON shape")
    if value.get("code") not in (None, 0):
        fail(f"{label} returned business code {value.get('code')}")
    return value


def login(username: str, password: str) -> str:
    status, raw = http_request(
        "POST",
        f"{BASE_URL}/api/v1/auth/login",
        {"username": username, "password": password},
    )
    body = json_body(status, raw, f"login {username}")
    token = body.get("data", {}).get("token", "")
    if not token:
        fail(f"login {username} returned no token")
    return str(token)


def require_api_success(status: int, raw: bytes, label: str) -> dict:
    return json_body(status, raw, label)


def run_command(args: list[str], *, env: dict[str, str] | None = None) -> str:
    result = subprocess.run(
        args,
        cwd=ROOT,
        env=env,
        check=False,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        fail(f"{' '.join(args)} failed with exit code {result.returncode}")
    return result.stdout


def mysql_query(query: str) -> list[str]:
    env = os.environ.copy()
    env["MYSQL_PWD"] = DB_PASSWORD
    output = run_command(
        [
            "mysql",
            "--protocol=tcp",
            "-h",
            "127.0.0.1",
            "-P",
            configured("SHAREO_MYSQL_PORT", "3306"),
            "-u",
            "root",
            "shareo",
            "--batch",
            "--skip-column-names",
            "-e",
            query,
        ],
        env=env,
    )
    return [line.strip() for line in output.splitlines() if line.strip()]


def redis_command(*args: str) -> str:
    return run_command(["redis-cli", "-h", "127.0.0.1", "-p", "6379", *args]).strip()


def validate_local_stack() -> None:
    checks = [
        ("Go health", f"{BASE_URL}/healthz", ""),
        ("Qdrant health", "http://127.0.0.1:6333/healthz", ""),
        ("MinIO health", "http://127.0.0.1:9000/minio/health/live", ""),
    ]
    for label, url, token in checks:
        status, _ = http_request("GET", url, token=token)
        if status != 200:
            fail(f"{label} returned HTTP {status}")
    if redis_command("PING") != "PONG":
        fail("Redis did not respond with PONG")
    for endpoint in ("image-search", "rag", "agent"):
        status, _ = http_request(
            "GET",
            f"{AI_URL}/readyz/{endpoint}",
            token=INTERNAL_TOKEN,
            internal_token=True,
        )
        if status != 200:
            fail(f"AI readiness {endpoint} returned HTTP {status}")
    print("[PASS] local services and image/RAG/Agent readiness are ready")


def read_captions() -> tuple[list[pathlib.Path], dict[str, str]]:
    if not PHOTO_DIR.is_dir():
        fail(f"photo directory missing: {PHOTO_DIR}")
    if not CAPTIONS_FILE.is_file():
        fail(f"caption mapping missing: {CAPTIONS_FILE}")
    photos = sorted(
        path
        for path in PHOTO_DIR.iterdir()
        if path.is_file() and path.suffix.lower() in {".jpg", ".jpeg", ".png", ".webp"}
    )
    if len(photos) != EXPECTED_PHOTOS:
        fail(f"expected {EXPECTED_PHOTOS} photos, found {len(photos)}")
    captions: dict[str, str] = {}
    for line_no, line in enumerate(CAPTIONS_FILE.read_text(encoding="utf-8").splitlines(), 1):
        if not line.strip():
            continue
        parts = line.split("\t")
        if len(parts) != 2:
            fail(f"caption line {line_no} must contain filename and caption")
        filename, caption = (part.strip() for part in parts)
        if filename in captions:
            fail(f"duplicate caption mapping: {filename}")
        if not 5 <= len(caption) <= 20:
            fail(f"caption for {filename} must be 5-20 characters")
        if any(value in caption.lower() for value in ("token", "api", "http", "authorization")):
            fail(f"caption for {filename} contains operational text")
        captions[filename] = caption
    photo_names = {path.name for path in photos}
    if photo_names != set(captions):
        fail("caption mapping does not exactly match the local photo set")
    print(f"[PASS] validated {len(photos)} photos and 5-20 character captions")
    return photos, captions


def load_manifest() -> dict:
    if not MANIFEST_FILE.exists():
        return {}
    try:
        value = json.loads(MANIFEST_FILE.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        fail(f"invalid local photo manifest: {exc.msg}")
    if not isinstance(value, dict):
        fail("local photo manifest must be an object")
    return value


def save_manifest(manifest: dict) -> None:
    manifest["updated_at"] = datetime.now(timezone.utc).isoformat()
    MANIFEST_FILE.parent.mkdir(parents=True, exist_ok=True)
    MANIFEST_FILE.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def reconcile(apply: bool) -> dict:
    env = os.environ.copy()
    env.update(
        {
            "SHAREO_AI_INTERNAL_TOKEN": INTERNAL_TOKEN,
            "SHAREO_AI_GO_BASE_URL": BASE_URL,
            "SHAREO_AI_REDIS_URL": "redis://127.0.0.1:6379/0",
            "SHAREO_AI_QDRANT_URL": "http://127.0.0.1:6333",
        }
    )
    args = ["make", "-s", "reconcile-index"]
    if apply:
        args.append("APPLY=1")
    raw = run_command(args, env=env)
    try:
        result = json.loads(raw)
    except json.JSONDecodeError as exc:
        fail(f"reconcile-index returned invalid JSON: {exc.msg}")
    return result


def collection_points(collection: str) -> list[dict]:
    status, raw = http_request(
        "POST",
        f"http://127.0.0.1:6333/collections/{collection}/points/scroll",
        {"limit": 100, "with_payload": True, "with_vectors": False},
    )
    body = json_body(status, raw, f"Qdrant scroll {collection}")
    return body.get("result", {}).get("points", [])


def collection_count(collection: str) -> int:
    status, raw = http_request("GET", f"http://127.0.0.1:6333/collections/{collection}")
    body = json_body(status, raw, f"Qdrant metadata {collection}")
    return int(body.get("result", {}).get("points_count", 0))


def verify_final(manifest: dict, old_ids: list[int], new_ids: list[int]) -> None:
    old_deleted = int(mysql_query(f"SELECT COUNT(*) FROM posts WHERE id IN ({','.join(map(str, old_ids))}) AND is_deleted=1")[0])
    if old_deleted != len(old_ids):
        fail(f"only {old_deleted}/{len(old_ids)} old posts are soft-deleted")
    new_visible = int(mysql_query(f"SELECT COUNT(*) FROM posts WHERE id IN ({','.join(map(str, new_ids))}) AND status='approved' AND is_deleted=0")[0])
    if new_visible != len(new_ids):
        fail(f"only {new_visible}/{len(new_ids)} new posts are approved and visible")

    image_points = collection_points("images")
    text_points = collection_points("post_chunks")
    image_posts = {int(point.get("payload", {}).get("post_id", 0)) for point in image_points}
    text_posts = {int(point.get("payload", {}).get("post_id", 0)) for point in text_points}
    if collection_count("images") != len(new_ids) or image_posts != set(new_ids):
        fail("Qdrant images collection does not exactly match the 47 new posts")
    if collection_count("post_chunks") != len(new_ids) or text_posts != set(new_ids):
        fail("Qdrant post_chunks collection does not exactly match the 47 new posts")
    if redis_command("XPENDING", "shareo:stream:index_post", "ai-workers").splitlines()[0] != "0":
        fail("index stream still has pending messages")
    result = reconcile(False)
    for collection in ("images", "post_chunks"):
        details = result.get(collection, {})
        if details.get("repair_posts") or details.get("stale_posts"):
            fail(f"reconcile-index found differences in {collection}")

    samples = [manifest["posts"][0], manifest["posts"][len(manifest["posts"]) // 2], manifest["posts"][-1]]
    for sample in samples:
        status, _ = http_request("GET", f"{BASE_URL}{sample['image_url']}")
        if status != 200:
            fail(f"sample image proxy returned HTTP {status}")
    status, raw = http_request(
        "GET",
        f"{BASE_URL}/api/v1/search/images?q=%E6%B5%B7%E8%BE%B9%E6%97%A5%E8%90%BD&limit=5",
    )
    body = json_body(status, raw, "semantic image search")
    if not body.get("data", {}).get("results"):
        fail("semantic image search returned no sample result")
    print("[PASS] database, Qdrant, Redis, image proxy and semantic search match the new corpus")


def main() -> int:
    if os.environ.get("CONFIRM") != "YES":
        print("[FAIL] run with CONFIRM=YES; this soft-deletes all current posts", file=sys.stderr)
        return 2
    if not (BASE_URL.startswith("http://127.0.0.1:") or BASE_URL.startswith("http://localhost:")):
        print("[FAIL] local photo seed only permits localhost BASE_URL", file=sys.stderr)
        return 2

    STATE_DIR.mkdir(parents=True, exist_ok=True)
    BACKUP_DIR.mkdir(parents=True, exist_ok=True)
    photos, captions = read_captions()
    validate_local_stack()
    admin_token = login("demoadmin", "admin123")
    author_tokens = {
        "demo_alice": login("demo_alice", "alice2024"),
        "demo_bob": login("demo_bob", "bob2024"),
    }
    manifest = load_manifest()
    status = manifest.get("status", "")

    if not manifest:
        old_ids = [int(value) for value in mysql_query("SELECT id FROM posts ORDER BY id")]
        if len(old_ids) != 31:
            fail(f"expected 31 current posts, found {len(old_ids)}; refusing cleanup")
        (BACKUP_DIR / "old_post_ids.txt").write_text(
            "".join(f"{post_id}\n" for post_id in old_ids), encoding="utf-8"
        )
        manifest = {
            "version": 1,
            "status": "cleanup_pending",
            "created_at": datetime.now(timezone.utc).isoformat(),
            "old_post_ids": old_ids,
            "captions": captions,
            "posts": [],
        }
        save_manifest(manifest)
        print("[PASS] captured 31 old post IDs")
    else:
        old_ids = [int(value) for value in manifest.get("old_post_ids", [])]
        if manifest.get("captions") != captions:
            fail("caption mapping changed after the local photo seed started")
        print(f"[WARN] resuming local photo seed with status={status}")

    if status in ("", "cleanup_pending"):
        for post_id in old_ids:
            response_status, response_body = http_request(
                "DELETE", f"{BASE_URL}/api/v1/admin/posts/{post_id}", token=admin_token
            )
            require_api_success(response_status, response_body, f"soft-delete post #{post_id}")
        manifest["status"] = "cleanup_done"
        save_manifest(manifest)
        reconcile(True)
        print("[PASS] soft-deleted all 31 old posts and reconciled stale vectors")

    posts_by_filename = {item["filename"]: item for item in manifest.get("posts", [])}
    manifest["status"] = "publishing"
    save_manifest(manifest)
    for index, photo in enumerate(photos):
        filename = photo.name
        caption = captions[filename]
        author = "demo_alice" if index % 2 == 0 else "demo_bob"
        token = author_tokens[author]
        existing = posts_by_filename.get(filename)
        if existing:
            status_code, raw = http_request("GET", f"{BASE_URL}/api/v1/posts/{existing['post_id']}")
            if status_code == 200:
                current = json.loads(raw.decode("utf-8")).get("data", {})
                if current.get("content") == caption and len(current.get("images", [])) == 1:
                    print(f"[PASS] reuse {filename} as post #{existing['post_id']}")
                    continue

        status_code, raw = http_request("POST", f"{BASE_URL}/api/v1/upload", token=token, file_path=photo)
        upload = json_body(status_code, raw, f"upload {filename}")
        image_url = upload.get("data", {}).get("url", "")
        if not image_url:
            fail(f"upload {filename} returned no image URL")
        status_code, raw = http_request(
            "POST",
            f"{BASE_URL}/api/v1/posts",
            {"content": caption, "images": [image_url]},
            token=token,
        )
        created = require_api_success(status_code, raw, f"create post for {filename}")
        post_id = int(created.get("data", {}).get("id", 0))
        if not post_id:
            fail(f"create post for {filename} returned no post ID")
        status_code, raw = http_request(
            "POST", f"{BASE_URL}/api/v1/admin/posts/{post_id}/approve", token=admin_token
        )
        require_api_success(status_code, raw, f"approve post #{post_id}")
        item = {
            "filename": filename,
            "caption": caption,
            "author": author,
            "post_id": post_id,
            "image_url": image_url,
        }
        posts_by_filename[filename] = item
        manifest["posts"] = sorted(posts_by_filename.values(), key=lambda value: value["filename"])
        save_manifest(manifest)
        print(f"[PASS] published {filename} as {author}, post #{post_id}")
        time.sleep(SLEEP_SECONDS)

    if len(posts_by_filename) != len(photos):
        fail(f"only {len(posts_by_filename)}/{len(photos)} photos have posts")
    env = os.environ.copy()
    env.update({"SHAREO_CONFIG": str(ROOT / "config.yaml"), "SHAREO_DB_PASSWORD": DB_PASSWORD})
    run_command(["make", "-s", "backfill-index"], env=env)
    manifest["status"] = "indexing"
    save_manifest(manifest)

    for _ in range(180):
        pending = redis_command("XPENDING", "shareo:stream:index_post", "ai-workers").splitlines()[0]
        if pending == "0":
            break
        time.sleep(1)
    else:
        fail("index stream did not drain within 180 seconds")

    result = reconcile(False)
    reconcile_path = pathlib.Path("/tmp/shareo-local-photo-reconcile.json")
    reconcile_path.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    new_ids = sorted(int(item["post_id"]) for item in posts_by_filename.values())
    verify_final(manifest, old_ids, new_ids)
    manifest["status"] = "completed"
    manifest["post_ids"] = new_ids
    save_manifest(manifest)
    print("[PASS] 47 local photo posts are approved and fully vectorized")
    print(f"[INFO] manifest: {MANIFEST_FILE}")
    print("[INFO] old MinIO objects were intentionally retained")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except RuntimeError as exc:
        print(f"[FAIL] {exc}", file=sys.stderr)
        raise SystemExit(1) from exc
