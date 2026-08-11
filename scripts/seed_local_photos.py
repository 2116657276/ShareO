"""Seed one deterministic local post for every image in the photo directory.

The command intentionally performs destructive MinIO cleanup, so it requires
``CONFIRM=YES`` and only accepts a localhost Go endpoint.  Captions are kept
outside git and are keyed by image SHA-256 to make reruns stable when files do
not change.
"""

from __future__ import annotations

import hashlib
import json
import mimetypes
import os
import pathlib
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timezone
from typing import Any


ROOT = pathlib.Path(__file__).resolve().parents[1]
PHOTO_DIR = ROOT / "resources/static/pictures"
STATE_DIR = pathlib.Path(
    os.environ.get("SHAREO_LOCAL_PHOTO_STATE_DIR", ROOT / ".local/shareo/local-photo-seed")
)
CAPTIONS_FILE = pathlib.Path(
    os.environ.get("SHAREO_LOCAL_PHOTO_CAPTIONS", STATE_DIR / "captions.json")
)
MANIFEST_FILE = STATE_DIR / "manifest.json"
OBJECT_SNAPSHOT_FILE = STATE_DIR / "old-minio-objects.json"
BASE_URL = os.environ.get("SHAREO_BASE_URL", "http://127.0.0.1:8080").rstrip("/")
AI_URL = os.environ.get("SHAREO_AI_BASE_URL", "http://127.0.0.1:8000").rstrip("/")
EXPECTED_PHOTOS = int(os.environ.get("SHAREO_LOCAL_PHOTO_EXPECTED_COUNT", "47"))
# Stay below the Go upload (20/minute) and post (30/minute) write limits by
# default. An explicit larger delay remains supported for slower MinIO hosts.
SLEEP_SECONDS = max(3.2, float(os.environ.get("SHAREO_LOCAL_PHOTO_SLEEP_SECONDS", "3.2")))
IMAGE_EXTENSIONS = {".jpg", ".jpeg", ".png", ".webp", ".gif"}
INTERACTION_TABLES = (
    "comments",
    "comment_likes",
    "likes",
    "favorites",
    "follows",
    "notifications",
    "conversations",
    "conversation_members",
    "messages",
    "bot_replies",
    "bot_task_outbox",
)


def env_file_value(key: str) -> str:
    env_file = ROOT / ".env"
    if not env_file.exists():
        return ""
    for raw_line in env_file.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        name, value = line.split("=", 1)
        if name.strip() == key:
            return value.strip().strip("\"'")
    return ""


def configured(key: str, default: str = "") -> str:
    return os.environ.get(key) or env_file_value(key) or default


DB_HOST = configured("SHAREO_PG_HOST", "127.0.0.1")
DB_PORT = configured("SHAREO_PG_PORT", "5432")
DB_NAME = configured("SHAREO_PG_DATABASE", "shareo")
DB_USER = configured("SHAREO_DB_USER", "shareo_app")
DB_PASSWORD = configured("SHAREO_DB_PASSWORD")
AI_DB_USER = configured("SHAREO_AI_DB_USER", "shareo_ai")
AI_DB_PASSWORD = configured("SHAREO_AI_DB_PASSWORD")
TEST_USERNAME = configured("SHAREO_TEST_USERNAME", "shareo_test")
TEST_PASSWORD = configured("SHAREO_TEST_PASSWORD")
INTERNAL_TOKEN = configured("SHAREO_INTERNAL_TOKEN", "shareo-local-internal")
APP_PORT = configured("SHAREO_APP_PORT", "8080")
MINIO_ENDPOINT = configured("SHAREO_MINIO_ENDPOINT", "127.0.0.1:9000")
MINIO_ACCESS_KEY = configured("SHAREO_MINIO_ACCESS_KEY", "minioadmin")
MINIO_SECRET_KEY = configured("SHAREO_MINIO_SECRET_KEY", "minioadmin")
MINIO_BUCKET = configured("SHAREO_MINIO_BUCKET", "shareo")
MINIO_ALIAS = "shareo-local"
MC_CONFIG_DIR = STATE_DIR / "mc"


def fail(message: str) -> None:
    raise RuntimeError(message)


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
        detail = result.stderr.strip() or result.stdout.strip()
        fail(f"{' '.join(args)} failed with exit code {result.returncode}: {detail[:500]}")
    return result.stdout


def psql_query(query: str, *, ai: bool = False) -> list[str]:
    password = AI_DB_PASSWORD if ai else DB_PASSWORD
    user = AI_DB_USER if ai else DB_USER
    if not password:
        fail("database password is missing; set SHAREO_DB_PASSWORD and SHAREO_AI_DB_PASSWORD")
    env = os.environ.copy()
    env["PGPASSWORD"] = password
    output = run_command(
        [
            "psql",
            "--no-psqlrc",
            "--host",
            DB_HOST,
            "--port",
            DB_PORT,
            "--username",
            user,
            "--dbname",
            DB_NAME,
            "--tuples-only",
            "--no-align",
            "--set",
            "ON_ERROR_STOP=1",
            "--command",
            query,
        ],
        env=env,
    )
    return [line.strip() for line in output.splitlines() if line.strip()]


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
        headers["X-Internal-Token" if internal_token else "Cookie"] = (
            token if internal_token else f"token={token}"
        )
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


def json_body(status: int, raw: bytes, label: str) -> dict[str, Any]:
    if status < 200 or status >= 300:
        fail(f"{label} failed with HTTP {status}: {raw[:300].decode('utf-8', errors='replace')}")
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


def require_api_success(status: int, raw: bytes, label: str) -> dict[str, Any]:
    return json_body(status, raw, label)


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def enumerate_images() -> list[pathlib.Path]:
    if not PHOTO_DIR.is_dir():
        fail(f"photo directory missing: {PHOTO_DIR}")
    photos = sorted(
        path
        for path in PHOTO_DIR.iterdir()
        if path.is_file() and path.suffix.lower() in IMAGE_EXTENSIONS
    )
    if len(photos) != EXPECTED_PHOTOS:
        fail(f"expected {EXPECTED_PHOTOS} photos, found {len(photos)}")
    return photos


def load_captions(photos: list[pathlib.Path]) -> dict[str, dict[str, str]]:
    if not CAPTIONS_FILE.is_file():
        fail(
            f"caption manifest missing: {CAPTIONS_FILE}; inspect each image and write captions keyed by SHA-256"
        )
    try:
        document = json.loads(CAPTIONS_FILE.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        fail(f"invalid caption manifest: {exc.msg}")
    captions = document.get("captions") if isinstance(document, dict) else None
    if not isinstance(captions, dict):
        fail("caption manifest must contain a captions object keyed by image SHA-256")

    result: dict[str, dict[str, str]] = {}
    for photo in photos:
        digest = sha256_file(photo)
        entry = captions.get(digest)
        if not isinstance(entry, dict):
            fail(f"caption missing for {photo.name} ({digest})")
        filename = str(entry.get("filename", ""))
        caption = str(entry.get("caption", "")).strip()
        if filename != photo.name:
            fail(f"caption SHA-256 entry {digest} is bound to {filename!r}, expected {photo.name!r}")
        if not 5 <= len(caption) <= 30:
            fail(f"caption for {photo.name} must be 5-30 characters")
        if any(value in caption.lower() for value in ("token", "api", "http", "authorization")):
            fail(f"caption for {photo.name} contains operational text")
        result[digest] = {"filename": filename, "caption": caption}
    if set(result) != set(captions):
        fail("caption manifest must exactly match the current image SHA-256 set")
    return result


def load_manifest() -> dict[str, Any]:
    if not MANIFEST_FILE.exists():
        return {}
    try:
        value = json.loads(MANIFEST_FILE.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        fail(f"invalid local photo manifest: {exc.msg}")
    if not isinstance(value, dict):
        fail("local photo manifest must be an object")
    return value


def save_manifest(manifest: dict[str, Any]) -> None:
    manifest["updated_at"] = datetime.now(timezone.utc).isoformat()
    MANIFEST_FILE.parent.mkdir(parents=True, exist_ok=True)
    MANIFEST_FILE.write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    MANIFEST_FILE.chmod(0o600)


def validate_local_stack() -> None:
    checks = (
        ("Go health", f"{BASE_URL}/healthz"),
        ("AI health", f"{AI_URL}/healthz"),
        ("MinIO health", "http://127.0.0.1:9000/minio/health/live"),
    )
    for label, url in checks:
        status, _ = http_request("GET", url)
        if status != 200:
            fail(f"{label} returned HTTP {status}")
    if psql_query("SELECT 1") != ["1"]:
        fail("Go PostgreSQL readiness query failed")
    if psql_query("SELECT 1", ai=True) != ["1"]:
        fail("AI PostgreSQL readiness query failed")
    if run_command(["redis-cli", "-h", "127.0.0.1", "-p", "6379", "PING"]).strip() != "PONG":
        fail("Redis did not respond with PONG")
    status, raw = http_request("GET", f"{AI_URL}/readyz")
    ready = json_body(status, raw, "AI readiness")
    if ready.get("status") != "ready":
        fail(f"AI readiness is {ready.get('status')}")
    print("[PASS] local PostgreSQL, pgvector, Redis, MinIO and Go/AI readiness")


def database_counts() -> dict[str, int]:
    counts: dict[str, int] = {}
    for table in ("users", "posts", "post_images", "system_logs", *INTERACTION_TABLES):
        rows = psql_query(f"SELECT COUNT(*) FROM public.{table}")
        counts[table] = int(rows[0]) if rows else 0
    return counts


def ensure_pristine_database(manifest: dict[str, Any]) -> None:
    counts = database_counts()
    if manifest.get("status") == "completed":
        return
    allowed_test_username = TEST_USERNAME.replace("'", "''")
    unexpected_users = psql_query(
        "SELECT username FROM public.users WHERE username NOT IN "
        f"('shareo_bot', '{allowed_test_username}') ORDER BY username"
    )
    if unexpected_users:
        fail(f"target PostgreSQL contains unexpected users: {unexpected_users}")
    seed_ids = {int(item["post_id"]) for item in manifest.get("posts", []) if item.get("post_id")}
    if counts["posts"]:
        if not seed_ids:
            fail("target PostgreSQL already contains business data; refusing automatic deletion")
        found_ids = {
            int(value)
            for value in psql_query(
                "SELECT id FROM public.posts WHERE id IN ("
                + ",".join(str(post_id) for post_id in sorted(seed_ids))
                + ")"
            )
        }
        if found_ids != seed_ids or counts["posts"] != len(seed_ids):
            fail("target PostgreSQL contains posts outside the interrupted local seed; refusing deletion")
        if counts["post_images"] > len(seed_ids):
            fail("target PostgreSQL contains image rows outside the interrupted local seed")
    elif counts["post_images"]:
        fail("target PostgreSQL contains orphaned image rows; refusing automatic deletion")
    if counts["system_logs"] and not manifest.get("posts"):
        fail("target PostgreSQL already contains system logs; refusing automatic deletion")
    if any(counts[name] for name in INTERACTION_TABLES):
        fail("target PostgreSQL already contains interaction, chat or outbox data; refusing automatic deletion")
    print("[PASS] target PostgreSQL has no old posts, interactions, chat or outbox data")


def ensure_test_user() -> None:
    if not TEST_PASSWORD:
        fail("SHAREO_TEST_PASSWORD is required and is never read from the repository")
    hash_env = os.environ.copy()
    hash_env["SHAREO_GENHASH_PASSWORD"] = TEST_PASSWORD
    password_hash = run_command(["go", "run", "./cmd/genhash"], env=hash_env).strip()
    if not password_hash:
        fail("failed to generate test-user password hash")
    env = os.environ.copy()
    env["PGPASSWORD"] = DB_PASSWORD
    def sql_literal(value: str) -> str:
        return "'" + value.replace("'", "''") + "'"

    query = f"""
    INSERT INTO public.users (username, password_hash, email, bio, role, status, is_bot)
    VALUES ({sql_literal(TEST_USERNAME)}, {sql_literal(password_hash)}, 'shareo_test@shareo.local', '本机图片数据集测试用户', 'admin', 1, 0)
    ON CONFLICT (username) DO UPDATE SET
        password_hash = EXCLUDED.password_hash,
        role = 'admin', status = 1, is_bot = 0;
    """
    run_command(
        [
            "psql",
            "--no-psqlrc",
            "--host",
            DB_HOST,
            "--port",
            DB_PORT,
            "--username",
            DB_USER,
            "--dbname",
            DB_NAME,
            "--set",
            "ON_ERROR_STOP=1",
            "--command",
            query,
        ],
        env=env,
    )
    print(f"[PASS] fixed test user {TEST_USERNAME} is ready")


def mc_command(*args: str) -> str:
    if not shutil_which("mc"):
        fail("mc is required to snapshot and clear the MinIO bucket")
    env = os.environ.copy()
    env["MC_CONFIG_DIR"] = str(MC_CONFIG_DIR)
    return run_command(["mc", *args], env=env)


def shutil_which(command: str) -> str | None:
    return next(
        (
            candidate
            for candidate in os.environ.get("PATH", "").split(os.pathsep)
            if (pathlib.Path(candidate) / command).is_file()
            and os.access(pathlib.Path(candidate) / command, os.X_OK)
        ),
        None,
    )


def configure_minio_alias() -> None:
    MC_CONFIG_DIR.mkdir(parents=True, exist_ok=True)
    endpoint = MINIO_ENDPOINT if "://" in MINIO_ENDPOINT else f"http://{MINIO_ENDPOINT}"
    mc_command("alias", "set", MINIO_ALIAS, endpoint, MINIO_ACCESS_KEY, MINIO_SECRET_KEY, "--api", "S3v4")


def list_minio_objects() -> list[str]:
    output = mc_command("ls", "--recursive", "--json", f"{MINIO_ALIAS}/{MINIO_BUCKET}")
    objects: list[str] = []
    for line in output.splitlines():
        if not line.strip():
            continue
        try:
            item = json.loads(line)
        except json.JSONDecodeError:
            continue
        key = item.get("key") or item.get("Key")
        if key:
            objects.append(str(key))
    return sorted(set(objects))


def clear_minio_bucket() -> list[str]:
    objects = list_minio_objects()
    OBJECT_SNAPSHOT_FILE.parent.mkdir(parents=True, exist_ok=True)
    OBJECT_SNAPSHOT_FILE.write_text(json.dumps(objects, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    OBJECT_SNAPSHOT_FILE.chmod(0o600)
    if objects:
        mc_command("rm", "--recursive", "--force", f"{MINIO_ALIAS}/{MINIO_BUCKET}")
    if list_minio_objects():
        fail("MinIO bucket is not empty after the guarded cleanup")
    print(f"[PASS] snapshotted and cleared {len(objects)} old MinIO objects")
    return objects


def clear_shareo_redis() -> None:
    keys = run_command(["redis-cli", "-h", "127.0.0.1", "-p", "6379", "--scan", "--pattern", "shareo:*"]).splitlines()
    if keys:
        run_command(["redis-cli", "-h", "127.0.0.1", "-p", "6379", "DEL", *keys])
    print(f"[PASS] cleared {len(keys)} ShareO Redis keys")


def reconcile(apply: bool) -> dict[str, Any]:
    env = os.environ.copy()
    env.update(
        {
            "SHAREO_AI_INTERNAL_TOKEN": INTERNAL_TOKEN,
            "SHAREO_AI_GO_BASE_URL": BASE_URL,
            "SHAREO_AI_REDIS_URL": "redis://127.0.0.1:6379/0",
            "SHAREO_AI_DATABASE_URL": f"postgresql://{AI_DB_USER}@{DB_HOST}:{DB_PORT}/{DB_NAME}?sslmode=disable",
            "SHAREO_AI_DB_PASSWORD": AI_DB_PASSWORD,
        }
    )
    args = ["make", "-s", "reconcile-index"]
    if apply:
        args.append("APPLY=1")
    raw = run_command(args, env=env)
    try:
        return json.loads(raw)
    except json.JSONDecodeError as exc:
        fail(f"reconcile-index returned invalid JSON: {exc.msg}")


def object_key(image_url: str) -> str:
    marker = "/api/v1/images/"
    if marker not in image_url:
        fail(f"unexpected image URL: {image_url}")
    return image_url.split(marker, 1)[1].lstrip("/")


def delete_seed_posts(post_ids: set[int]) -> None:
    """Physically remove only manifest-owned posts and their cascaded rows."""

    if not post_ids:
        return
    values = ",".join(str(post_id) for post_id in sorted(post_ids))
    psql_query(f"DELETE FROM public.posts WHERE id IN ({values})")


def expected_object_keys(posts: list[dict[str, Any]]) -> set[str]:
    keys: set[str] = set()
    for post in posts:
        original = object_key(str(post["image_url"]))
        keys.add(original)
        day_parts = original.split("/")
        if len(day_parts) >= 5 and day_parts[1] == "original":
            date_path = "/".join(day_parts[2:5])
            identifier = day_parts[5].rsplit(".", 1)[0]
            keys.add(f"posts/thumb/{date_path}/{identifier}.jpg")
            keys.add(f"posts/medium/{date_path}/{identifier}.jpg")
    return keys


def verify_final(manifest: dict[str, Any], posts: list[dict[str, Any]]) -> None:
    total_posts = int(psql_query("SELECT COUNT(*) FROM public.posts")[0])
    visible = psql_query(
        "SELECT COUNT(*) FROM public.posts WHERE status = 'approved' AND is_deleted = 0"
    )
    image_count = psql_query("SELECT COUNT(*) FROM public.post_images")
    if (
        total_posts != EXPECTED_PHOTOS
        or int(visible[0]) != EXPECTED_PHOTOS
        or int(image_count[0]) != EXPECTED_PHOTOS
    ):
        fail(
            f"expected {EXPECTED_PHOTOS} total/visible posts and images, "
            f"got total={total_posts}, visible={visible}, images={image_count}"
        )
    one_image_posts = int(
        psql_query(
            "SELECT COUNT(*) FROM ("
            "SELECT post_id FROM public.post_images GROUP BY post_id HAVING COUNT(*) <> 1"
            ") AS invalid_posts"
        )[0]
    )
    if one_image_posts:
        fail(f"{one_image_posts} posts do not have exactly one image")
    owner_username = TEST_USERNAME.replace("'", "''")
    owner_rows = psql_query(
        "SELECT COUNT(*) FROM public.posts p "
        "JOIN public.users u ON u.id = p.user_id "
        f"WHERE u.username = '{owner_username}'"
    )
    if int(owner_rows[0]) != EXPECTED_PHOTOS:
        fail(f"expected all posts to belong to {TEST_USERNAME}, got {owner_rows}")
    manifest_ids = sorted(int(item["post_id"]) for item in posts)
    database_ids = sorted(
        int(value)
        for value in psql_query(
            "SELECT id FROM public.posts WHERE status = 'approved' AND is_deleted = 0 ORDER BY id"
        )
    )
    if database_ids != manifest_ids:
        fail(f"database post IDs do not match the seed manifest: {database_ids} != {manifest_ids}")
    for table in INTERACTION_TABLES:
        if int(psql_query(f"SELECT COUNT(*) FROM public.{table}")[0]) != 0:
            fail(f"fresh seed table {table} is not empty")

    ai_image_count = int(psql_query("SELECT COUNT(*) FROM ai.image_embeddings", ai=True)[0])
    if ai_image_count != EXPECTED_PHOTOS:
        fail(f"expected {EXPECTED_PHOTOS} image vectors, got {ai_image_count}")
    ai_text_count = int(psql_query("SELECT COUNT(*) FROM ai.post_chunk_embeddings", ai=True)[0])
    if ai_text_count != EXPECTED_PHOTOS:
        fail(f"expected {EXPECTED_PHOTOS} text vectors, got {ai_text_count}")

    objects = set(list_minio_objects())
    expected = expected_object_keys(posts)
    if not objects or not objects.issubset(expected):
        fail(f"MinIO contains unexpected objects: {sorted(objects - expected)[:10]}")
    if sum(1 for key in objects if "/original/" in key) != EXPECTED_PHOTOS:
        fail("MinIO does not contain exactly one original object per image")

    for sample in (posts[0], posts[len(posts) // 2], posts[-1]):
        status, _ = http_request("GET", f"{BASE_URL}{sample['image_url']}")
        if status != 200:
            fail(f"sample image proxy returned HTTP {status}")

    query = urllib.parse.quote(str(posts[0]["caption"])[:8])
    status, raw = http_request("GET", f"{BASE_URL}/api/v1/search/images?q={query}&limit=5")
    body = json_body(status, raw, "semantic image search")
    if not body.get("data", {}).get("results"):
        fail("semantic image search returned no sample result")
    result = reconcile(False)
    for collection in ("images", "post_chunks"):
        details = result.get(collection, {})
        if details.get("repair_posts") or details.get("stale_posts"):
            fail(f"reconcile-index found differences in {collection}")
    print("[PASS] database, pgvector, MinIO, image proxy, Feed and semantic image search match the corpus")


def publish_posts(photos: list[pathlib.Path], captions: dict[str, dict[str, str]], manifest: dict[str, Any], token: str) -> list[dict[str, Any]]:
    posts_by_filename = {item["filename"]: item for item in manifest.get("posts", [])}
    manifest["status"] = "publishing"
    for index, photo in enumerate(photos):
        digest = sha256_file(photo)
        caption = captions[digest]["caption"]
        existing = posts_by_filename.get(photo.name)
        if existing and existing.get("sha256") != digest:
            delete_seed_posts({int(existing["post_id"])})
            existing = None
            posts_by_filename.pop(photo.name, None)
        if existing:
            status, raw = http_request(
                "GET", f"{BASE_URL}/api/v1/posts/{existing['post_id']}", token=token
            )
            if status == 200:
                current = json.loads(raw.decode("utf-8")).get("data", {})
                if current.get("content") == caption and len(current.get("images", [])) == 1:
                    print(f"[PASS] reuse {photo.name} as post #{existing['post_id']}")
                    continue
            delete_seed_posts({int(existing["post_id"])})
            posts_by_filename.pop(photo.name, None)

        status, raw = http_request("POST", f"{BASE_URL}/api/v1/upload", token=token, file_path=photo)
        upload = json_body(status, raw, f"upload {photo.name}")
        urls = upload.get("data", {}).get("urls", {})
        image_url = str(urls.get("original") or upload.get("data", {}).get("url", ""))
        if not image_url:
            fail(f"upload {photo.name} returned no image URL")
        status, raw = http_request(
            "POST",
            f"{BASE_URL}/api/v1/posts",
            {"content": caption, "images": [image_url]},
            token=token,
        )
        created = require_api_success(status, raw, f"create post for {photo.name}")
        post_id = int(created.get("data", {}).get("id", 0))
        if not post_id:
            fail(f"create post for {photo.name} returned no post ID")
        item = {
            "filename": photo.name,
            "sha256": digest,
            "caption": caption,
            "post_id": post_id,
            "image_url": image_url,
        }
        posts_by_filename[photo.name] = item
        manifest["posts"] = sorted(posts_by_filename.values(), key=lambda value: value["filename"])
        save_manifest(manifest)
        status, raw = http_request(
            "POST", f"{BASE_URL}/api/v1/admin/posts/{post_id}/approve", token=token
        )
        require_api_success(status, raw, f"approve post #{post_id}")
        print(f"[PASS] published {photo.name} as {TEST_USERNAME}, post #{post_id}")
        if SLEEP_SECONDS > 0:
            time.sleep(SLEEP_SECONDS)
    return sorted(posts_by_filename.values(), key=lambda value: value["filename"])


def clear_seed_audit_logs() -> None:
    """Keep the fresh local dataset free of seed-only review audit rows."""

    psql_query("DELETE FROM public.system_logs")
    print("[PASS] cleared seed review audit logs")


def main() -> int:
    if os.environ.get("CONFIRM") != "YES":
        print("[FAIL] run with CONFIRM=YES; this clears the configured MinIO bucket", file=sys.stderr)
        return 2
    if not (BASE_URL.startswith("http://127.0.0.1:") or BASE_URL.startswith("http://localhost:")):
        print("[FAIL] local photo seed only permits localhost BASE_URL", file=sys.stderr)
        return 2
    local_hosts = {"127.0.0.1", "localhost", "::1"}
    if DB_HOST not in local_hosts:
        print("[FAIL] local photo seed only permits a localhost PostgreSQL host", file=sys.stderr)
        return 2
    minio_url = MINIO_ENDPOINT if "://" in MINIO_ENDPOINT else f"http://{MINIO_ENDPOINT}"
    if urllib.parse.urlsplit(minio_url).hostname not in local_hosts:
        print("[FAIL] local photo seed only permits a localhost MinIO endpoint", file=sys.stderr)
        return 2

    STATE_DIR.mkdir(parents=True, exist_ok=True)
    photos = enumerate_images()
    captions = load_captions(photos)
    manifest = load_manifest()
    validate_local_stack()
    ensure_pristine_database(manifest)
    ensure_test_user()
    configure_minio_alias()

    if manifest.get("status") != "completed":
        if manifest.get("posts"):
            # A previous interrupted run may have left pending seed posts. They
            # are the only posts allowed by the fresh-dataset guard.
            delete_seed_posts({int(item["post_id"]) for item in manifest["posts"]})
            manifest["posts"] = []
        clear_minio_bucket()
        clear_shareo_redis()
        manifest = {
            "version": 2,
            "status": "cleanup_done",
            "created_at": datetime.now(timezone.utc).isoformat(),
            "expected_images": EXPECTED_PHOTOS,
            "captions": captions,
            "posts": [],
        }
        save_manifest(manifest)
    else:
        print("[PASS] completed manifest found; preserving idempotent dataset")

    token = login(TEST_USERNAME, TEST_PASSWORD)
    posts = publish_posts(photos, captions, manifest, token)
    if len(posts) != EXPECTED_PHOTOS:
        fail(f"only {len(posts)}/{EXPECTED_PHOTOS} photos have posts")
    manifest["posts"] = posts
    manifest["status"] = "indexing"
    save_manifest(manifest)

    env = os.environ.copy()
    env.update({"SHAREO_CONFIG": str(ROOT / "config.yaml"), "SHAREO_DB_PASSWORD": DB_PASSWORD})
    run_command(["make", "-s", "backfill-index"], env=env)
    for _ in range(180):
        pending = run_command(
            ["redis-cli", "-h", "127.0.0.1", "-p", "6379", "XPENDING", "shareo:stream:index_post", "ai-workers"]
        ).splitlines()
        if pending and pending[0].strip() == "0":
            break
        time.sleep(1)
    else:
        fail("index stream did not drain within 180 seconds")

    clear_seed_audit_logs()
    verify_final(manifest, posts)
    manifest["status"] = "completed"
    manifest["post_ids"] = [int(item["post_id"]) for item in posts]
    save_manifest(manifest)
    print(f"[PASS] {EXPECTED_PHOTOS} local photo posts are approved and fully vectorized")
    print(f"[INFO] manifest: {MANIFEST_FILE}")
    print(f"[INFO] old MinIO object snapshot: {OBJECT_SNAPSHOT_FILE}")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except RuntimeError as exc:
        print(f"[FAIL] {exc}", file=sys.stderr)
        raise SystemExit(1) from exc
