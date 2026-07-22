#!/usr/bin/env bash
# Disposable semantic image-search E2E using the single supported Compose stack.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PROJECT_NAME="shareo-image-e2e"
APP_PORT="${SHAREO_E2E_APP_PORT:-18090}"
AI_PORT="${SHAREO_E2E_AI_PORT:-18010}"
INTERNAL_TOKEN="shareo-e2e-internal"
ADMIN_HASH='$2a$10$vbn5lSmj3e6arCkbXzKNqutbx5B/iqtnk6tHoYvETGh1qdnG/5Rd2'
POST_ID_FILE="$(mktemp /tmp/shareo-image-e2e-post.XXXXXX)"

if docker compose version >/dev/null 2>&1; then
    compose=(docker compose -p "$PROJECT_NAME")
else
    compose=(docker-compose -p "$PROJECT_NAME")
fi

export SHAREO_MYSQL_PORT="${SHAREO_E2E_MYSQL_PORT:-13316}"
export SHAREO_REDIS_PORT="${SHAREO_E2E_REDIS_PORT:-16389}"
export SHAREO_MINIO_PORT="${SHAREO_E2E_MINIO_PORT:-19010}"
export SHAREO_MINIO_CONSOLE_PORT="${SHAREO_E2E_MINIO_CONSOLE_PORT:-19011}"
export SHAREO_QDRANT_HTTP_PORT="${SHAREO_E2E_QDRANT_HTTP_PORT:-16343}"
export SHAREO_QDRANT_GRPC_PORT="${SHAREO_E2E_QDRANT_GRPC_PORT:-16344}"
export SHAREO_APP_PORT="$APP_PORT"
export SHAREO_AI_PORT="$AI_PORT"
export SHAREO_INTERNAL_TOKEN="$INTERNAL_TOKEN"
export SHAREO_DB_PASSWORD="shareo_pass"
export SHAREO_AI_MODEL_CACHE_SOURCE="${SHAREO_E2E_MODEL_CACHE_SOURCE:-hf_cache}"

cleanup() {
    status=$?
    if [ "$status" -ne 0 ]; then
        "${compose[@]}" logs --no-color --tail=150 app ai-service >&2 || true
    fi
    "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
    rm -f -- "$POST_ID_FILE"
}
trap cleanup EXIT INT TERM

cd "$PROJECT_DIR"
"${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
"${compose[@]}" up -d --build --wait

"${compose[@]}" exec -T mysql mysql -uroot -pshareo_pass shareo -e \
    "INSERT INTO users (username, password_hash, email, role, status) VALUES ('admin', '$ADMIN_HASH', 'admin-e2e@shareo.local', 'admin', 1);"

APP_URL="http://127.0.0.1:$APP_PORT"
AI_URL="http://127.0.0.1:$AI_PORT"
for _ in $(seq 1 900); do
    if curl --noproxy '*' --fail --silent \
        -H "X-Internal-Token: $INTERNAL_TOKEN" \
        "$AI_URL/readyz/image-search" >/dev/null; then
        break
    fi
    sleep 1
done
curl --noproxy '*' --fail --silent \
    -H "X-Internal-Token: $INTERNAL_TOKEN" \
    "$AI_URL/readyz/image-search" >/dev/null

# First post remains approved while duplicate delivery and worker recovery are checked.
SHAREO_BASE_URL="$APP_URL" SHAREO_TEST_SKIP_DELETE=1 \
    SHAREO_TEST_POST_ID_FILE="$POST_ID_FILE" bash scripts/test_image_search.sh
POST_ID="$(sed -n '1p' "$POST_ID_FILE")"

"${compose[@]}" exec -T redis redis-cli XADD shareo:stream:index_post '*' action upsert post_id "$POST_ID" >/dev/null
"${compose[@]}" exec -T redis redis-cli XADD shareo:stream:index_post '*' action upsert post_id "$POST_ID" >/dev/null
for _ in $(seq 1 60); do
    pending="$("${compose[@]}" exec -T redis redis-cli --raw XPENDING shareo:stream:index_post ai-workers | sed -n '1p')"
    lag="$("${compose[@]}" exec -T redis redis-cli --raw XINFO GROUPS shareo:stream:index_post | awk '$0 == "lag" { getline; print; exit }')"
    [ "$pending" = "0" ] && [ "$lag" = "0" ] && break
    sleep 1
done
[ "$pending" = "0" ] && [ "$lag" = "0" ] || { echo "duplicate deliveries did not converge" >&2; exit 1; }
echo "PASS duplicate index deliveries converged"

# Put one delivery into a dead consumer's pending list, then prove the restarted
# FastAPI process reclaims it after the 30-second idle threshold.
"${compose[@]}" stop ai-service >/dev/null
"${compose[@]}" exec -T redis redis-cli XADD shareo:stream:index_post '*' action upsert post_id "$POST_ID" >/dev/null
"${compose[@]}" exec -T redis redis-cli XREADGROUP GROUP ai-workers abandoned COUNT 1 STREAMS shareo:stream:index_post '>' >/dev/null
"${compose[@]}" start ai-service >/dev/null
RECOVERED=0
for _ in $(seq 1 45); do
    pending="$("${compose[@]}" exec -T redis redis-cli --raw XPENDING shareo:stream:index_post ai-workers | sed -n '1p')"
    if [ "$pending" = "0" ]; then
        RECOVERED=1
        break
    fi
    sleep 1
done
[ "$RECOVERED" = "1" ] || { echo "AI restart did not reclaim pending index task" >&2; exit 1; }
echo "PASS FastAPI restart reclaimed pending index task within 45 seconds"

# A second post exercises the normal delete event and ten-second visibility goal.
SHAREO_BASE_URL="$APP_URL" bash scripts/test_image_search.sh

AI_LOG="$("${compose[@]}" logs --no-color ai-service)"
for marker in \
    "image model load complete" \
    "image index payload fetch complete" \
    "image download complete" \
    "image embedding complete" \
    "image qdrant delete complete" \
    "image qdrant upsert complete"; do
    if ! grep -q "$marker" <<<"$AI_LOG"; then
        echo "missing staged AI log marker: $marker" >&2
        exit 1
    fi
done
echo "PASS staged image indexing timings were logged"
echo "PASS semantic image-search Compose E2E"
