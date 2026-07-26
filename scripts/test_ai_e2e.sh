#!/usr/bin/env bash
# Full private-message Bot E2E using the disposable mock LLM provider.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PROJECT_NAME="shareo-ai-e2e"
APP_PORT="${SHAREO_E2E_APP_PORT:-18091}"
AI_PORT="${SHAREO_E2E_AI_PORT:-18011}"
INTERNAL_TOKEN="shareo-ai-e2e-internal"
ADMIN_HASH='$2a$10$vbn5lSmj3e6arCkbXzKNqutbx5B/iqtnk6tHoYvETGh1qdnG/5Rd2'
NO_BUILD="${SHAREO_E2E_NO_BUILD:-0}"
APP_IMAGE="${SHAREO_E2E_APP_IMAGE:-shareo-app:latest}"
AI_IMAGE="${SHAREO_E2E_AI_IMAGE:-shareo-ai-service:latest}"

if docker compose version >/dev/null 2>&1; then
    compose=(docker compose -p "$PROJECT_NAME" -f compose.yaml -f compose.test.yaml --profile test)
else
    compose=(docker-compose -p "$PROJECT_NAME" -f compose.yaml -f compose.test.yaml --profile test)
fi
MODEL_CACHE_VOLUME="${SHAREO_E2E_MODEL_CACHE_VOLUME:-}"
COMPOSE_OVERRIDE_FILE=''
if [ "$NO_BUILD" = "1" ] || [ -n "$MODEL_CACHE_VOLUME" ]; then
    COMPOSE_OVERRIDE_FILE="$(mktemp -t shareo-ai-e2e-compose)"
    {
        printf '%s\n' \
        'services:' \
        '  app:' \
        "    image: $APP_IMAGE" \
        '    build: null' \
        '  ai-service:' \
        "    image: $AI_IMAGE" \
        '    build: null' \
        '    environment:' \
        '      NO_PROXY: app,redis,qdrant,mysql,mock-llm,localhost,127.0.0.1,::1' \
        '      no_proxy: app,redis,qdrant,mysql,mock-llm,localhost,127.0.0.1,::1' \
        '  mock-llm:' \
        "    image: $AI_IMAGE" \
        '    build: null'
        if [ -n "$MODEL_CACHE_VOLUME" ]; then
            printf '%s\n' \
            '    volumes:' \
            '      - hf_cache:/models'
        fi
        if [ -n "$MODEL_CACHE_VOLUME" ]; then
            printf '%s\n' \
            'volumes:' \
            '  hf_cache:' \
            '    external: true' \
            "    name: $MODEL_CACHE_VOLUME"
        fi
    } >"$COMPOSE_OVERRIDE_FILE"
    compose+=(-f "$COMPOSE_OVERRIDE_FILE")
fi

export SHAREO_MYSQL_PORT="${SHAREO_E2E_MYSQL_PORT:-13317}"
export SHAREO_REDIS_PORT="${SHAREO_E2E_REDIS_PORT:-16390}"
export SHAREO_MINIO_PORT="${SHAREO_E2E_MINIO_PORT:-19012}"
export SHAREO_MINIO_CONSOLE_PORT="${SHAREO_E2E_MINIO_CONSOLE_PORT:-19013}"
export SHAREO_QDRANT_HTTP_PORT="${SHAREO_E2E_QDRANT_HTTP_PORT:-16345}"
export SHAREO_QDRANT_GRPC_PORT="${SHAREO_E2E_QDRANT_GRPC_PORT:-16346}"
export SHAREO_APP_PORT="$APP_PORT"
export SHAREO_AI_PORT="$AI_PORT"
export SHAREO_INTERNAL_TOKEN="$INTERNAL_TOKEN"
export SHAREO_DB_PASSWORD="shareo_pass"
export SHAREO_AI_MODEL_CACHE_SOURCE="${SHAREO_E2E_MODEL_CACHE_SOURCE:-hf_cache}"

cleanup() {
    status=$?
    if [ "$status" -ne 0 ]; then
        "${compose[@]}" logs --no-color --tail=180 app ai-service mock-llm >&2 || true
    fi
    "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
    if [ -n "$COMPOSE_OVERRIDE_FILE" ]; then
        rm -f -- "$COMPOSE_OVERRIDE_FILE"
    fi
    exit "$status"
}
trap cleanup EXIT INT TERM

cd "$PROJECT_DIR"
"${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
if [ "$NO_BUILD" = "1" ]; then
    "${compose[@]}" up -d --no-build --wait
else
    "${compose[@]}" up -d --build --wait
fi

"${compose[@]}" exec -T mysql mysql -uroot -pshareo_pass shareo -e \
    "INSERT INTO users (username, password_hash, email, role, status) VALUES ('admin', '$ADMIN_HASH', 'admin-e2e@shareo.local', 'admin', 1);"

APP_URL="http://127.0.0.1:$APP_PORT"
AI_URL="http://127.0.0.1:$AI_PORT"
QDRANT_URL="http://127.0.0.1:$SHAREO_QDRANT_HTTP_PORT"
for _ in $(seq 1 900); do
    if curl --noproxy '*' --fail --silent \
        -H "X-Internal-Token: $INTERNAL_TOKEN" \
        -H 'Content-Type: application/json' \
        -d '{"question":"夜景怎么拍？","history":[],"top_k":8}' \
        "$AI_URL/v1/rag/answer" >/dev/null; then
        break
    fi
    sleep 1
done
curl --noproxy '*' --fail --silent \
    -H "X-Internal-Token: $INTERNAL_TOKEN" "$AI_URL/readyz/rag" >/dev/null

TEXT_AUTHOR="text-e2e-$(date +%s%N)"
POST_ID="$("${compose[@]}" exec -T mysql mysql -uroot -pshareo_pass -N -s shareo -e \
    "INSERT INTO users (username, password_hash, email, role, status) VALUES ('$TEXT_AUTHOR', '$ADMIN_HASH', 'text-e2e@shareo.local', 'user', 1); INSERT INTO posts (user_id, content, status) VALUES (LAST_INSERT_ID(), '夜景拍摄建议：使用三脚架固定手机，降低曝光并等待光线稳定。', 'approved'); SELECT LAST_INSERT_ID();")"
"${compose[@]}" exec -T redis redis-cli XADD shareo:stream:index_post '*' \
    action upsert post_id "$POST_ID" >/dev/null

for _ in $(seq 1 120); do
    TEXT_POINTS="$(curl --noproxy '*' --fail --silent -X POST \
        "$QDRANT_URL/collections/post_chunks/points/scroll" \
        -H 'Content-Type: application/json' \
        -d "{\"filter\":{\"must\":[{\"key\":\"post_id\",\"match\":{\"value\":$POST_ID}}]},\"limit\":10,\"with_payload\":true}")"
    if grep -q "\"chunk_id\":\"$POST_ID:0\"" <<<"$TEXT_POINTS"; then
        break
    fi
    sleep 1
done
grep -q "\"chunk_id\":\"$POST_ID:0\"" <<<"$TEXT_POINTS"
echo "PASS approved post text indexed post=$POST_ID"

register_user() {
    local suffix username response
    suffix="$(date +%s%N)"
    username="bot-e2e-${suffix: -12}"
    response="$(curl --noproxy '*' --fail --silent -X POST "$APP_URL/api/v1/auth/register" \
        -H 'Content-Type: application/json' \
        -d "{\"username\":\"$username\",\"password\":\"shareo-test-pass\"}")"
    BOT_E2E_USERNAME="$username"
    BOT_E2E_TOKEN="$(printf '%s' "$response" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["token"])')"
}

register_user
USER_TOKEN="$BOT_E2E_TOKEN"
CHAT_PAGE="$(curl --noproxy '*' --fail --silent "$APP_URL/chat" -b "token=$USER_TOKEN")"
grep -q "AI 助手" <<<"$CHAT_PAGE"
grep -q "citationList" <<<"$CHAT_PAGE"
echo "PASS chat page renders Bot entry and citation renderer"
BOT_ID="$(curl --noproxy '*' --fail --silent "$APP_URL/api/v1/users/search?q=shareo_bot&limit=20" \
    -b "token=$USER_TOKEN" | python3 -c \
    'import json,sys; users=json.load(sys.stdin)["data"]; print(next(u["id"] for u in users if u.get("is_bot") and u.get("username")=="shareo_bot"))')"
CONVERSATION_ID="$(curl --noproxy '*' --fail --silent -X POST "$APP_URL/api/v1/conversations" \
    -b "token=$USER_TOKEN" -H 'Content-Type: application/json' \
    -d "{\"user_id\":$BOT_ID}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')"

send_message() {
    local content="$1"
    curl --noproxy '*' --fail --silent -X POST \
        "$APP_URL/api/v1/conversations/$CONVERSATION_ID/messages" \
        -b "token=$USER_TOKEN" -H 'Content-Type: application/json' \
        -d "$(python3 -c 'import json,sys; print(json.dumps({"content":sys.argv[1]},ensure_ascii=False))' "$content")"
}

bot_message_for_source() {
    local source_id="$1"
    curl --noproxy '*' --fail --silent \
        "$APP_URL/api/v1/conversations/$CONVERSATION_ID/messages?limit=100" \
        -b "token=$USER_TOKEN" | python3 -c '
import json, sys
source_id = int(sys.argv[1])
messages = json.load(sys.stdin)["data"]["messages"]
for message in messages:
    if not message.get("sender", {}).get("is_bot") or not message.get("meta"):
        continue
    try:
        meta = json.loads(message["meta"])
    except (TypeError, ValueError):
        continue
    if meta.get("source_message_id") == source_id:
        print(json.dumps(message, ensure_ascii=False))
        break
' "$source_id"
}

SOURCE_ID="$(send_message '夜景怎么拍？' | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')"
BOT_MESSAGE=""
for _ in $(seq 1 90); do
    BOT_MESSAGE="$(bot_message_for_source "$SOURCE_ID")"
    [ -n "$BOT_MESSAGE" ] && break
    sleep 1
done
[ -n "$BOT_MESSAGE" ]
python3 - "$BOT_MESSAGE" "$POST_ID" <<'PY'
import json
import sys

message = json.loads(sys.argv[1])
post_id = int(sys.argv[2])
meta = json.loads(message["meta"])
assert meta["source_message_id"] > 0
assert any(item["post_id"] == post_id for item in meta["citations"])
assert message["sender"]["is_bot"] == 1
PY
echo "PASS Bot reply contains visible citation post=$POST_ID"

# Re-deliver the same source message after a successful callback.
"${compose[@]}" exec -T redis redis-cli XADD shareo:stream:bot_tasks '*' \
    conversation_id "$CONVERSATION_ID" message_id "$SOURCE_ID" >/dev/null
sleep 3
BOT_COUNT="$(curl --noproxy '*' --fail --silent \
    "$APP_URL/api/v1/conversations/$CONVERSATION_ID/messages?limit=100" \
    -b "token=$USER_TOKEN" | python3 -c \
    'import json,sys; print(sum(1 for m in json.load(sys.stdin)["data"]["messages"] if m.get("sender",{}).get("is_bot")))')"
[ "$BOT_COUNT" = "1" ]
echo "PASS duplicate Bot task produced one reply"

# A timeout is reclaimed quickly only in this disposable test profile.
TIMEOUT_SOURCE_ID="$(send_message '测试超时' | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')"
TIMEOUT_MESSAGE=""
for _ in $(seq 1 60); do
    TIMEOUT_MESSAGE="$(bot_message_for_source "$TIMEOUT_SOURCE_ID")"
    if grep -q 'AI 当前暂不可用，请稍后重试。' <<<"$TIMEOUT_MESSAGE"; then
        break
    fi
    sleep 1
done
grep -q 'AI 当前暂不可用，请稍后重试。' <<<"$TIMEOUT_MESSAGE"
echo "PASS LLM timeout produced one fixed fallback"

# Ordinary private messages must not enter bot_tasks or receive a Bot reply.
register_user
OTHER_TOKEN="$BOT_E2E_TOKEN"
OTHER_ID="$(curl --noproxy '*' --fail --silent "$APP_URL/api/v1/auth/me" -b "token=$OTHER_TOKEN" | \
    python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')"
ORDINARY_CONVERSATION_ID="$(curl --noproxy '*' --fail --silent -X POST "$APP_URL/api/v1/conversations" \
    -b "token=$USER_TOKEN" -H 'Content-Type: application/json' \
    -d "{\"user_id\":$OTHER_ID}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')"
curl --noproxy '*' --fail --silent -X POST \
    "$APP_URL/api/v1/conversations/$ORDINARY_CONVERSATION_ID/messages" \
    -b "token=$USER_TOKEN" -H 'Content-Type: application/json' \
    -d '{"content":"普通私聊"}' >/dev/null
sleep 3
ORDINARY_COUNT="$(curl --noproxy '*' --fail --silent \
    "$APP_URL/api/v1/conversations/$ORDINARY_CONVERSATION_ID/messages?limit=100" \
    -b "token=$USER_TOKEN" | python3 -c \
    'import json,sys; print(len(json.load(sys.stdin)["data"]["messages"]))')"
[ "$ORDINARY_COUNT" = "1" ]
echo "PASS ordinary private chat did not trigger Bot"

curl --noproxy '*' --fail --silent -X DELETE "$APP_URL/api/v1/admin/posts/$POST_ID" \
    -b "token=$(curl --noproxy '*' --fail --silent -X POST "$APP_URL/api/v1/auth/login" \
        -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' | \
        python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["token"])')" >/dev/null
for _ in $(seq 1 30); do
    TEXT_POINTS="$(curl --noproxy '*' --fail --silent -X POST \
        "$QDRANT_URL/collections/post_chunks/points/scroll" \
        -H 'Content-Type: application/json' \
        -d "{\"filter\":{\"must\":[{\"key\":\"post_id\",\"match\":{\"value\":$POST_ID}}]},\"limit\":10,\"with_payload\":true}")"
    if ! grep -q "\"chunk_id\":\"$POST_ID:" <<<"$TEXT_POINTS"; then
        break
    fi
    sleep 1
done
if grep -q "\"chunk_id\":\"$POST_ID:" <<<"$TEXT_POINTS"; then
    echo "deleted post still appears in post_chunks post=$POST_ID" >&2
    exit 1
fi
DELETED_SOURCE_ID="$(send_message '删除后的帖子还能回答吗？' | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')"
DELETED_MESSAGE=""
for _ in $(seq 1 60); do
    DELETED_MESSAGE="$(bot_message_for_source "$DELETED_SOURCE_ID")"
    [ -n "$DELETED_MESSAGE" ] && break
    sleep 1
done
python3 - "$DELETED_MESSAGE" "$POST_ID" <<'PY'
import json
import sys

message = json.loads(sys.argv[1])
post_id = int(sys.argv[2])
meta = json.loads(message["meta"])
assert all(item["post_id"] != post_id for item in meta.get("citations", []))
PY
echo "PASS deleted post is absent from new Bot citations"
echo "PASS private-message Bot E2E"
