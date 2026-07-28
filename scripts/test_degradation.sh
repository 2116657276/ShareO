#!/usr/bin/env bash
# Disposable Phase 7C degradation matrix verification.
# The test project never reuses the primary ShareO Compose project or its volumes.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PROJECT_NAME="${SHAREO_DEGRADATION_PROJECT:-shareo-degradation}"
APP_PORT="${SHAREO_DEGRADATION_APP_PORT:-28180}"
AI_PORT="${SHAREO_DEGRADATION_AI_PORT:-28181}"
MYSQL_PORT="${SHAREO_DEGRADATION_MYSQL_PORT:-28182}"
REDIS_PORT="${SHAREO_DEGRADATION_REDIS_PORT:-28183}"
MINIO_PORT="${SHAREO_DEGRADATION_MINIO_PORT:-28184}"
MINIO_CONSOLE_PORT="${SHAREO_DEGRADATION_MINIO_CONSOLE_PORT:-28185}"
QDRANT_HTTP_PORT="${SHAREO_DEGRADATION_QDRANT_HTTP_PORT:-28186}"
QDRANT_GRPC_PORT="${SHAREO_DEGRADATION_QDRANT_GRPC_PORT:-28187}"
INTERNAL_TOKEN="shareo-degradation-internal"
DB_PASSWORD="shareo_pass"
ADMIN_HASH='$2a$10$vbn5lSmj3e6arCkbXzKNqutbx5B/iqtnk6tHoYvETGh1qdnG/5Rd2'
EVIDENCE_FILE="${SHAREO_DEGRADATION_EVIDENCE_FILE:-$PROJECT_DIR/docs/evidence/phase7c/degradation-automated.log}"
NO_BUILD="${SHAREO_DEGRADATION_NO_BUILD:-0}"
MODEL_CACHE_VOLUME="${SHAREO_DEGRADATION_MODEL_CACHE_VOLUME:-}"

COMPOSE_FILES=(-f compose.yaml -f compose.test.yaml)
COMPOSE_OVERRIDE_FILE=''
if [ -n "$MODEL_CACHE_VOLUME" ]; then
    COMPOSE_OVERRIDE_FILE="$(mktemp /tmp/shareo-degradation-compose.XXXXXX.yaml)"
    {
        printf '%s\n' \
        'services:' \
        '  ai-service:' \
        '    environment:' \
        '      NO_PROXY: app,redis,qdrant,mysql,mock-llm,localhost,127.0.0.1,::1' \
        '      no_proxy: app,redis,qdrant,mysql,mock-llm,localhost,127.0.0.1,::1' \
        '      HF_HUB_OFFLINE: "1"' \
        '      TRANSFORMERS_OFFLINE: "1"' \
        '    volumes:' \
        '      - hf_cache:/models'
        printf '%s\n' \
        'volumes:' \
        '  hf_cache:' \
        '    external: true' \
        "    name: $MODEL_CACHE_VOLUME"
    } >"$COMPOSE_OVERRIDE_FILE"
    COMPOSE_FILES+=(-f "$COMPOSE_OVERRIDE_FILE")
fi

if docker compose version >/dev/null 2>&1; then
    compose=(docker compose -p "$PROJECT_NAME" "${COMPOSE_FILES[@]}" --profile test)
else
    compose=(docker-compose -p "$PROJECT_NAME" "${COMPOSE_FILES[@]}" --profile test)
fi

export SHAREO_MYSQL_PORT="$MYSQL_PORT"
export SHAREO_REDIS_PORT="$REDIS_PORT"
export SHAREO_MINIO_PORT="$MINIO_PORT"
export SHAREO_MINIO_CONSOLE_PORT="$MINIO_CONSOLE_PORT"
export SHAREO_QDRANT_HTTP_PORT="$QDRANT_HTTP_PORT"
export SHAREO_QDRANT_GRPC_PORT="$QDRANT_GRPC_PORT"
export SHAREO_APP_PORT="$APP_PORT"
export SHAREO_AI_PORT="$AI_PORT"
export SHAREO_INTERNAL_TOKEN="$INTERNAL_TOKEN"
export SHAREO_DB_PASSWORD="$DB_PASSWORD"
if [ -n "$MODEL_CACHE_VOLUME" ]; then
    export SHAREO_AI_MODEL_CACHE_SOURCE=hf_cache
else
    export SHAREO_AI_MODEL_CACHE_SOURCE="${SHAREO_DEGRADATION_MODEL_CACHE_SOURCE:-hf_cache}"
fi

APP_URL="http://127.0.0.1:$APP_PORT"
AI_URL="http://127.0.0.1:$AI_PORT"

mkdir -p "$(dirname "$EVIDENCE_FILE")"
exec >"$EVIDENCE_FILE" 2>&1

cleanup() {
    status=$?
    if [ "$status" -ne 0 ]; then
        "${compose[@]}" logs --no-color --tail=160 app ai-service mock-llm >&2 || true
    fi
    "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
    if [ -n "$COMPOSE_OVERRIDE_FILE" ]; then
        rm -f -- "$COMPOSE_OVERRIDE_FILE"
    fi
    exit "$status"
}
trap cleanup EXIT INT TERM

http_status() {
    local method="$1" url="$2"
    shift 2
    curl --noproxy '*' --silent --show-error -o /dev/null -w '%{http_code}' \
        -X "$method" "$url" "$@" || printf '000'
}

expect_status() {
    local label="$1" expected="$2" method="$3" url="$4"
    shift 4
    local actual
    actual="$(http_status "$method" "$url" "$@")"
    if [ "$actual" != "$expected" ]; then
        echo "FAIL $label expected=$expected actual=$actual" >&2
        return 1
    fi
    echo "PASS $label status=$actual"
}

expect_not_200() {
    local label="$1" method="$2" url="$3"
    shift 3
    local actual
    actual="$(http_status "$method" "$url" "$@")"
    if [ "$actual" = "200" ]; then
        echo "FAIL $label unexpectedly returned HTTP 200" >&2
        return 1
    fi
    echo "PASS $label status=$actual"
}

wait_status() {
    local label="$1" expected="$2" method="$3" url="$4" attempts="$5"
    shift 5
    local actual='000'
    for _ in $(seq 1 "$attempts"); do
        actual="$(http_status "$method" "$url" "$@")"
        if [ "$actual" = "$expected" ]; then
            echo "PASS $label status=$actual"
            return 0
        fi
        sleep 1
    done
    echo "FAIL $label expected=$expected actual=$actual" >&2
    return 1
}

warm_and_check_ready() {
    local warmed=0
    for _ in $(seq 1 180); do
        if curl --noproxy '*' --fail --silent --max-time 10 \
            -H "X-Internal-Token: $INTERNAL_TOKEN" \
            -H 'Content-Type: application/json' \
            -d '{"query":"夜景照片","limit":1}' "$AI_URL/v1/search/images" >/dev/null 2>&1 \
            && curl --noproxy '*' --fail --silent --max-time 10 \
            -H "X-Internal-Token: $INTERNAL_TOKEN" \
            -H 'Content-Type: application/json' \
            -d '{"question":"夜景怎么拍？","top_k":8}' "$AI_URL/v1/rag/answer" >/dev/null 2>&1; then
            warmed=1
            break
        fi
        sleep 1
    done
    if [ "$warmed" -ne 1 ]; then
        echo 'FAIL AI readiness warmup did not complete' >&2
        return 1
    fi
    wait_status 'image readiness' 200 GET "$AI_URL/readyz/image-search" 180 \
        -H "X-Internal-Token: $INTERNAL_TOKEN"
    wait_status 'RAG readiness' 200 GET "$AI_URL/readyz/rag" 180 \
        -H "X-Internal-Token: $INTERNAL_TOKEN"
}

compose_service() {
    "${compose[@]}" "$@"
}

register_user() {
    local username response
    username="degradation-$(date +%s%N | tail -c 13)"
    response="$(curl --noproxy '*' --fail --silent -X POST "$APP_URL/api/v1/auth/register" \
        -H 'Content-Type: application/json' \
        -d "{\"username\":\"$username\",\"password\":\"shareo-test-pass\"}")"
    printf '%s' "$response" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["token"])'
}

send_message() {
    local token="$1" conversation_id="$2" content="$3"
    curl --noproxy '*' --fail --silent -X POST \
        "$APP_URL/api/v1/conversations/$conversation_id/messages" \
        -b "token=$token" -H 'Content-Type: application/json' \
        -d "$(python3 -c 'import json,sys; print(json.dumps({"content":sys.argv[1]},ensure_ascii=False))' "$content")"
}

bot_message_for_source() {
    local token="$1" conversation_id="$2" source_id="$3"
    curl --noproxy '*' --fail --silent \
        "$APP_URL/api/v1/conversations/$conversation_id/messages?limit=100" \
        -b "token=$token" | python3 -c '
import json, sys
source_id = int(sys.argv[1])
for message in json.load(sys.stdin)["data"]["messages"]:
    if not message.get("sender", {}).get("is_bot") or not message.get("meta"):
        continue
    try:
        meta = json.loads(message["meta"])
    except (TypeError, ValueError):
        continue
    if meta.get("source_message_id") == source_id:
        print(message.get("content", ""))
        break
' "$source_id"
}

cd "$PROJECT_DIR"
echo "Phase 7C degradation verification project=$PROJECT_NAME"
compose_service down -v --remove-orphans >/dev/null 2>&1 || true
if [ "$NO_BUILD" = "1" ]; then
    compose_service up -d --no-build --wait
else
    compose_service up -d --build --wait
fi
compose_service exec -T mysql mysql -uroot -p"$DB_PASSWORD" shareo -e \
    "INSERT INTO users (username, password_hash, email, role, status) VALUES ('admin', '$ADMIN_HASH', 'admin-degradation@shareo.local', 'admin', 1);"
expect_status 'Go health baseline' 200 GET "$APP_URL/healthz"
expect_status 'AI health baseline' 200 GET "$AI_URL/healthz"
warm_and_check_ready

USER_TOKEN="$(register_user)"
OTHER_TOKEN="$(register_user)"
BOT_ID="$(curl --noproxy '*' --fail --silent "$APP_URL/api/v1/users/search?q=shareo_bot&limit=20" \
    -b "token=$USER_TOKEN" | python3 -c \
    'import json,sys; print(next(u["id"] for u in json.load(sys.stdin)["data"] if u.get("is_bot") and u.get("username")=="shareo_bot"))')"
OTHER_ID="$(curl --noproxy '*' --fail --silent "$APP_URL/api/v1/auth/me" -b "token=$OTHER_TOKEN" | \
    python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')"
BOT_CONVERSATION_ID="$(curl --noproxy '*' --fail --silent -X POST "$APP_URL/api/v1/conversations" \
    -b "token=$USER_TOKEN" -H 'Content-Type: application/json' \
    -d "{\"user_id\":$BOT_ID}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')"
ORDINARY_CONVERSATION_ID="$(curl --noproxy '*' --fail --silent -X POST "$APP_URL/api/v1/conversations" \
    -b "token=$USER_TOKEN" -H 'Content-Type: application/json' \
    -d "{\"user_id\":$OTHER_ID}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')"
expect_status 'full-text search baseline' 200 GET "$APP_URL/api/v1/search?q=夜景&limit=10"
expect_status 'ordinary private message baseline' 200 POST "$APP_URL/api/v1/conversations/$ORDINARY_CONVERSATION_ID/messages" \
    -b "token=$USER_TOKEN" -H 'Content-Type: application/json' -d '{"content":"baseline"}'

echo '--- AI service stopped ---'
compose_service stop ai-service >/dev/null
expect_status 'community health with AI stopped' 200 GET "$APP_URL/healthz"
expect_status 'full-text search with AI stopped' 200 GET "$APP_URL/api/v1/search?q=夜景&limit=10"
expect_status 'semantic search with AI stopped' 503 GET "$APP_URL/api/v1/search/images?q=夜景&limit=5"
compose_service start ai-service >/dev/null
wait_status 'AI service restore' 200 GET "$AI_URL/healthz" 120
warm_and_check_ready

echo '--- Qdrant stopped ---'
compose_service stop qdrant >/dev/null
expect_status 'community health with Qdrant stopped' 200 GET "$APP_URL/healthz"
expect_status 'full-text search with Qdrant stopped' 200 GET "$APP_URL/api/v1/search?q=夜景&limit=10"
expect_status 'semantic search with Qdrant stopped' 503 GET "$APP_URL/api/v1/search/images?q=夜景&limit=5"
expect_status 'RAG with Qdrant stopped' 503 POST "$AI_URL/v1/rag/answer" \
    -H "X-Internal-Token: $INTERNAL_TOKEN" -H 'Content-Type: application/json' \
    -d '{"question":"夜景怎么拍？","top_k":8}'
compose_service start qdrant >/dev/null
wait_status 'Qdrant restore' 200 GET "$AI_URL/healthz" 120
warm_and_check_ready

echo '--- MinIO stopped ---'
POST_ID_FILE="$(mktemp /tmp/shareo-degradation-post.XXXXXX)"
SHAREO_BASE_URL="$APP_URL" SHAREO_TEST_ADMIN_USER=admin SHAREO_TEST_ADMIN_PASSWORD=admin123 \
    SHAREO_TEST_SKIP_DELETE=1 SHAREO_TEST_POST_ID_FILE="$POST_ID_FILE" bash scripts/test_image_search.sh >/dev/null
POST_ID="$(sed -n '1p' "$POST_ID_FILE")"
rm -f -- "$POST_ID_FILE"
IMAGE_URL="$(compose_service exec -T mysql mysql -uroot -p"$DB_PASSWORD" -N -s shareo \
    -e "SELECT image_url FROM post_images WHERE post_id = $POST_ID LIMIT 1;" | tr -d '\r')"
case "$IMAGE_URL" in
    http://*|https://*) IMAGE_ENDPOINT="$IMAGE_URL" ;;
    /*) IMAGE_ENDPOINT="$APP_URL$IMAGE_URL" ;;
    *) IMAGE_ENDPOINT="$APP_URL/api/v1/images/$IMAGE_URL" ;;
esac
compose_service stop minio >/dev/null
expect_status 'full-text search with MinIO stopped' 200 GET "$APP_URL/api/v1/search?q=夜景&limit=10"
expect_not_200 'image proxy with MinIO stopped' GET "$IMAGE_ENDPOINT"
compose_service start minio >/dev/null
wait_status 'MinIO restore' 200 GET "$APP_URL/healthz" 120
expect_status 'image proxy after MinIO restore' 200 GET "$IMAGE_ENDPOINT"

echo '--- Redis stopped ---'
compose_service stop redis >/dev/null
expect_status 'community health with Redis stopped' 200 GET "$APP_URL/healthz"
expect_status 'full-text search with Redis stopped' 200 GET "$APP_URL/api/v1/search?q=夜景&limit=10"
expect_status 'ordinary message with Redis stopped' 200 POST \
    "$APP_URL/api/v1/conversations/$ORDINARY_CONVERSATION_ID/messages" \
    -b "token=$USER_TOKEN" -H 'Content-Type: application/json' -d '{"content":"redis outage"}'
compose_service start redis >/dev/null
wait_status 'Redis restore' 200 GET "$APP_URL/healthz" 120
warm_and_check_ready

echo '--- LLM provider stopped ---'
compose_service stop mock-llm >/dev/null
LLM_USER_TOKEN="$(register_user)"
LLM_OTHER_TOKEN="$(register_user)"
LLM_OTHER_ID="$(curl --noproxy '*' --fail --silent "$APP_URL/api/v1/auth/me" -b "token=$LLM_OTHER_TOKEN" | \
    python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')"
LLM_ORDINARY_CONVERSATION_ID="$(curl --noproxy '*' --fail --silent -X POST "$APP_URL/api/v1/conversations" \
    -b "token=$LLM_USER_TOKEN" -H 'Content-Type: application/json' \
    -d "{\"user_id\":$LLM_OTHER_ID}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')"
expect_status 'ordinary message with LLM stopped' 200 POST \
    "$APP_URL/api/v1/conversations/$LLM_ORDINARY_CONVERSATION_ID/messages" \
    -b "token=$LLM_USER_TOKEN" -H 'Content-Type: application/json' -d '{"content":"llm outage does not block ordinary chat"}'
LLM_BOT_CONVERSATION_ID="$(curl --noproxy '*' --fail --silent -X POST "$APP_URL/api/v1/conversations" \
    -b "token=$LLM_USER_TOKEN" -H 'Content-Type: application/json' \
    -d "{\"user_id\":$BOT_ID}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')"
SOURCE_ID="$(send_message "$LLM_USER_TOKEN" "$LLM_BOT_CONVERSATION_ID" '测试超时' | \
    python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')"
BOT_CONTENT=''
for _ in $(seq 1 90); do
    BOT_CONTENT="$(bot_message_for_source "$LLM_USER_TOKEN" "$LLM_BOT_CONVERSATION_ID" "$SOURCE_ID")"
    if [ -n "$BOT_CONTENT" ]; then
        break
    fi
    sleep 1
done
grep -q 'AI 当前暂不可用，请稍后重试。' <<<"$BOT_CONTENT"
echo 'PASS Bot fixed fallback with LLM provider stopped'
compose_service start mock-llm >/dev/null
wait_status 'LLM provider restore' 200 GET "$AI_URL/healthz" 120
warm_and_check_ready

expect_status 'final Go health baseline' 200 GET "$APP_URL/healthz"
expect_status 'final full-text search baseline' 200 GET "$APP_URL/api/v1/search?q=夜景&limit=10"
echo 'PASS Phase 7C degradation matrix'
