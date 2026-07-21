#!/usr/bin/env bash
# Disposable Phase 2 E2E runner. It only creates and removes the fixed *_e2e resources below.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
MYSQL_HOST="127.0.0.1"
MYSQL_PORT="3306"
MYSQL_DATABASE="shareo_e2e"
REDIS_URL="redis://127.0.0.1:6379/15"
MINIO_ENDPOINT="127.0.0.1:9000"
MINIO_BUCKET="shareo-e2e"
QDRANT_URL="http://127.0.0.1:6333"
QDRANT_COLLECTION="images-e2e"
GO_URL="http://127.0.0.1:18080"
AI_URL="http://127.0.0.1:18000"
INTERNAL_TOKEN="shareo-e2e-internal"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASS="${MYSQL_PASS:-shareo_pass}"
MINIO_ACCESS_KEY="${SHAREO_MINIO_ACCESS_KEY:-minioadmin}"
MINIO_SECRET_KEY="${SHAREO_MINIO_SECRET_KEY:-minioadmin}"
MODEL_CACHE_DIR="${SHAREO_AI_MODEL_CACHE_DIR:-$HOME/.cache/shareo/models}"
TMP_DIR="$(mktemp -d /tmp/shareo-image-e2e.XXXXXX)"
MC_CONFIG_DIR="$TMP_DIR/mc"
CONFIG_PATH="$TMP_DIR/config.yaml"
GO_PID=""
AI_PID=""
WORKER_PID=""

export MYSQL_PWD="$MYSQL_PASS"
export MC_CONFIG_DIR

stop_processes() {
    for pid in "$WORKER_PID" "$AI_PID" "$GO_PID"; do
        if [ -n "$pid" ]; then
            kill "$pid" 2>/dev/null || true
            wait "$pid" 2>/dev/null || true
        fi
    done
}

cleanup_resources() {
    stop_processes
    redis-cli -u "$REDIS_URL" FLUSHDB >/dev/null 2>&1 || true
    mc alias set shareo-e2e-local "http://$MINIO_ENDPOINT" "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" >/dev/null 2>&1 || true
    mc rb --force "shareo-e2e-local/$MINIO_BUCKET" >/dev/null 2>&1 || true
    curl --noproxy '*' --silent -X DELETE "$QDRANT_URL/collections/$QDRANT_COLLECTION" >/dev/null 2>&1 || true
    mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" \
        -e "DROP DATABASE IF EXISTS \`$MYSQL_DATABASE\`;" >/dev/null 2>&1 || true
    case "$TMP_DIR" in
        /tmp/shareo-image-e2e.*) rm -rf -- "$TMP_DIR" ;;
        *) echo "refusing to remove unexpected temporary path: $TMP_DIR" >&2 ;;
    esac
}
trap cleanup_resources EXIT INT TERM

for command in mysql redis-cli mc curl uv go; do
    command -v "$command" >/dev/null || { echo "missing command: $command" >&2; exit 2; }
done
curl --noproxy '*' --fail --silent "http://$MINIO_ENDPOINT/minio/health/live" >/dev/null
curl --noproxy '*' --fail --silent "$QDRANT_URL/readyz" >/dev/null

mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" \
    -e "DROP DATABASE IF EXISTS \`$MYSQL_DATABASE\`; CREATE DATABASE \`$MYSQL_DATABASE\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
MYSQL_HOST="$MYSQL_HOST" MYSQL_PORT="$MYSQL_PORT" MYSQL_USER="$MYSQL_USER" \
    MYSQL_PASS="$MYSQL_PASS" MYSQL_DATABASE="$MYSQL_DATABASE" bash "$PROJECT_DIR/scripts/migrate_schema.sh"
mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" "$MYSQL_DATABASE" <<'SQL'
INSERT INTO users (username, password_hash, email, role, status)
VALUES ('admin', '$2a$10$vbn5lSmj3e6arCkbXzKNqutbx5B/iqtnk6tHoYvETGh1qdnG/5Rd2', 'admin-e2e@shareo.local', 'admin', 1);
SQL
redis-cli -u "$REDIS_URL" FLUSHDB >/dev/null
mc alias set shareo-e2e-local "http://$MINIO_ENDPOINT" "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" >/dev/null
mc mb --ignore-existing "shareo-e2e-local/$MINIO_BUCKET" >/dev/null
curl --noproxy '*' --silent -X DELETE "$QDRANT_URL/collections/$QDRANT_COLLECTION" >/dev/null || true

cp "$PROJECT_DIR/config.e2e.yaml.example" "$CONFIG_PATH"

(cd "$PROJECT_DIR" && SHAREO_CONFIG="$CONFIG_PATH" SHAREO_DB_PASSWORD="$MYSQL_PASS" \
    SHAREO_MINIO_ACCESS_KEY="$MINIO_ACCESS_KEY" SHAREO_MINIO_SECRET_KEY="$MINIO_SECRET_KEY" \
    SHAREO_INTERNAL_TOKEN="$INTERNAL_TOKEN" \
    SHAREO_AI_BASE_URL="$AI_URL" go run ./cmd/server >"$TMP_DIR/go.log" 2>&1) &
GO_PID=$!

(cd "$PROJECT_DIR/ai-service" && \
    SHAREO_AI_REDIS_URL="$REDIS_URL" SHAREO_AI_QDRANT_URL="$QDRANT_URL" \
    SHAREO_AI_IMAGE_COLLECTION="$QDRANT_COLLECTION" SHAREO_AI_INTERNAL_TOKEN="$INTERNAL_TOKEN" \
    SHAREO_AI_GO_BASE_URL="$GO_URL" SHAREO_AI_MODEL_CACHE_DIR="$MODEL_CACHE_DIR" \
    uv run --frozen uvicorn app.main:app --host 127.0.0.1 --port 18000 >"$TMP_DIR/ai.log" 2>&1) &
AI_PID=$!

for _ in $(seq 1 180); do
    if curl --noproxy '*' --fail --silent "$GO_URL/healthz" >/dev/null && \
       curl --noproxy '*' --fail --silent -H "X-Internal-Token: $INTERNAL_TOKEN" "$AI_URL/readyz/search" >/dev/null; then
        break
    fi
    sleep 1
done
curl --noproxy '*' --fail --silent "$GO_URL/healthz" >/dev/null || { tail -50 "$TMP_DIR/go.log" >&2; exit 1; }
curl --noproxy '*' --fail --silent -H "X-Internal-Token: $INTERNAL_TOKEN" "$AI_URL/readyz/search" >/dev/null || { tail -50 "$TMP_DIR/ai.log" >&2; exit 1; }

start_worker() {
  (cd "$PROJECT_DIR/ai-service" && \
    SHAREO_AI_REDIS_URL="$REDIS_URL" SHAREO_AI_QDRANT_URL="$QDRANT_URL" \
    SHAREO_AI_IMAGE_COLLECTION="$QDRANT_COLLECTION" SHAREO_AI_INTERNAL_TOKEN="$INTERNAL_TOKEN" \
    SHAREO_AI_GO_BASE_URL="$GO_URL" SHAREO_AI_MODEL_CACHE_DIR="$MODEL_CACHE_DIR" \
    uv run --frozen python -m app.workers >>"$TMP_DIR/worker.log" 2>&1) &
  WORKER_PID=$!
}
start_worker

SHAREO_BASE_URL="$GO_URL" SHAREO_TEST_SKIP_DELETE=1 SHAREO_TEST_POST_ID_FILE="$TMP_DIR/post-id" \
    bash "$PROJECT_DIR/scripts/test_image_search.sh"
POST_ID="$(sed -n '1p' "$TMP_DIR/post-id")"

# Backfill is deliberately executed twice; Qdrant point IDs make both deliveries idempotent.
(cd "$PROJECT_DIR" && SHAREO_CONFIG="$CONFIG_PATH" SHAREO_DB_PASSWORD="$MYSQL_PASS" \
    SHAREO_INTERNAL_TOKEN="$INTERNAL_TOKEN" go run ./cmd/backfill-index >/dev/null)
(cd "$PROJECT_DIR" && SHAREO_CONFIG="$CONFIG_PATH" SHAREO_DB_PASSWORD="$MYSQL_PASS" \
    SHAREO_INTERNAL_TOKEN="$INTERNAL_TOKEN" go run ./cmd/backfill-index >/dev/null)

# Put one valid upsert into another consumer's pending list, then prove a restarted worker
# reclaims it after the fixed 30-second idle threshold and converges within 45 seconds.
kill "$WORKER_PID"
wait "$WORKER_PID" 2>/dev/null || true
WORKER_PID=""
redis-cli -u "$REDIS_URL" XADD shareo:stream:index_post '*' action upsert post_id "$POST_ID" >/dev/null
redis-cli -u "$REDIS_URL" XREADGROUP GROUP ai-workers abandoned COUNT 1 STREAMS shareo:stream:index_post '>' >/dev/null
start_worker
RECOVERED=0
for _ in $(seq 1 45); do
    PENDING="$(redis-cli -u "$REDIS_URL" --raw XPENDING shareo:stream:index_post ai-workers | sed -n '1p')"
    if [ "$PENDING" = "0" ]; then
        RECOVERED=1
        break
    fi
    sleep 1
done
if [ "$RECOVERED" != "1" ]; then
    echo "worker restart did not reclaim pending message within 45 seconds" >&2
    tail -50 "$TMP_DIR/worker.log" >&2
    exit 1
fi
echo "PASS worker restart reclaimed pending message within 45 seconds"

# A second post exercises the normal delete path and its 10-second visibility objective.
SHAREO_BASE_URL="$GO_URL" bash "$PROJECT_DIR/scripts/test_image_search.sh"

echo "PASS isolated Phase 2 E2E resources were used; cleanup will now remove them"
