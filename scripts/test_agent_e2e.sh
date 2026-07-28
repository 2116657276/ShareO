#!/usr/bin/env bash
# Cross-service Agent E2E using the deterministic mock provider.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PROJECT_NAME="shareo-agent-e2e"
APP_PORT="${SHAREO_AGENT_E2E_APP_PORT:-18092}"
AI_PORT="${SHAREO_AGENT_E2E_AI_PORT:-18012}"
INTERNAL_TOKEN="shareo-agent-e2e-internal"
ADMIN_HASH='$2a$10$vbn5lSmj3e6arCkbXzKNqutbx5B/iqtnk6tHoYvETGh1qdnG/5Rd2'
NO_BUILD="${SHAREO_AGENT_E2E_NO_BUILD:-0}"
APP_IMAGE="${SHAREO_AGENT_E2E_APP_IMAGE:-shareo-agent-e2e-app:latest}"
AI_IMAGE="${SHAREO_AGENT_E2E_AI_IMAGE:-shareo-agent-e2e-ai-service:latest}"
MOCK_LLM_IMAGE="${SHAREO_AGENT_E2E_MOCK_LLM_IMAGE:-shareo-agent-e2e-mock-llm:latest}"
COMPOSE_OVERRIDE_FILE=''

if docker compose version >/dev/null 2>&1; then
    compose=(docker compose -p "$PROJECT_NAME" -f compose.yaml -f compose.test.yaml --profile test)
else
    compose=(docker-compose -p "$PROJECT_NAME" -f compose.yaml -f compose.test.yaml --profile test)
fi

if [ "$NO_BUILD" = "1" ]; then
    COMPOSE_OVERRIDE_FILE="$(mktemp -t shareo-agent-e2e-compose)"
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
            '      HF_HUB_OFFLINE: "1"' \
            '      TRANSFORMERS_OFFLINE: "1"' \
            '    volumes:' \
            '      - ./ai-service/app:/app/app:ro' \
            '      - agent-e2e-model-cache:/models'
        printf '%s\n' \
            '  mock-llm:' \
            "    image: $MOCK_LLM_IMAGE" \
            '    build: null' \
            '    volumes:' \
            '      - ./ai-service/app:/app/app:ro'
        printf '%s\n' \
            'volumes:' \
            '  agent-e2e-model-cache:' \
            '    external: true' \
            "    name: ${SHAREO_AGENT_E2E_MODEL_CACHE_VOLUME:-shareo_hf_cache}"
    } >"$COMPOSE_OVERRIDE_FILE"
    compose+=(-f "$COMPOSE_OVERRIDE_FILE")
fi
export SHAREO_MYSQL_PORT="${SHAREO_AGENT_E2E_MYSQL_PORT:-13318}"
export SHAREO_REDIS_PORT="${SHAREO_AGENT_E2E_REDIS_PORT:-16391}"
export SHAREO_MINIO_PORT="${SHAREO_AGENT_E2E_MINIO_PORT:-19014}"
export SHAREO_MINIO_CONSOLE_PORT="${SHAREO_AGENT_E2E_MINIO_CONSOLE_PORT:-19015}"
export SHAREO_QDRANT_HTTP_PORT="${SHAREO_AGENT_E2E_QDRANT_HTTP_PORT:-16347}"
export SHAREO_QDRANT_GRPC_PORT="${SHAREO_AGENT_E2E_QDRANT_GRPC_PORT:-16348}"
export SHAREO_APP_PORT="$APP_PORT"
export SHAREO_AI_PORT="$AI_PORT"
export SHAREO_INTERNAL_TOKEN="$INTERNAL_TOKEN"
export SHAREO_DB_PASSWORD="shareo_pass"
export SHAREO_AI_MODEL_CACHE_SOURCE="${SHAREO_AGENT_E2E_MODEL_CACHE_SOURCE:-hf_cache}"

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
    "INSERT INTO users (username, password_hash, email, role, status) VALUES ('admin', '$ADMIN_HASH', 'admin-agent-e2e@shareo.local', 'admin', 1);"

APP_URL="http://127.0.0.1:$APP_PORT"
AI_URL="http://127.0.0.1:$AI_PORT"
QDRANT_URL="http://127.0.0.1:$SHAREO_QDRANT_HTTP_PORT"
for _ in $(seq 1 900); do
    if curl --noproxy '*' --fail --silent -H "X-Internal-Token: $INTERNAL_TOKEN" \
        "$AI_URL/readyz/agent" >/dev/null; then
        break
    fi
    sleep 1
done
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
curl --noproxy '*' --fail --silent -H "X-Internal-Token: $INTERNAL_TOKEN" \
    "$AI_URL/readyz/rag" >/dev/null
echo "PASS Agent and RAG readiness"

TEXT_AUTHOR="agent-e2e-$(date +%s%N)"
POST_ID="$("${compose[@]}" exec -T mysql mysql -uroot -pshareo_pass -N -s shareo -e \
    "INSERT INTO users (username, password_hash, email, role, status) VALUES ('$TEXT_AUTHOR', '$ADMIN_HASH', 'agent-e2e@shareo.local', 'user', 1); INSERT INTO posts (user_id, content, status) VALUES (LAST_INSERT_ID(), '夜景拍摄建议：使用三脚架固定相机，降低曝光并等待灯光稳定。', 'approved'); SELECT LAST_INSERT_ID();")"
"${compose[@]}" exec -T redis redis-cli XADD shareo:stream:index_post '*' action upsert post_id "$POST_ID" >/dev/null
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
echo "PASS approved post indexed post=$POST_ID"

suffix="$(date +%s%N)"
username="agent-e2e-${suffix: -12}"
register="$(curl --noproxy '*' --fail --silent -X POST "$APP_URL/api/v1/auth/register" \
    -H 'Content-Type: application/json' -d "{\"username\":\"$username\",\"password\":\"shareo-test-pass\"}")"
TOKEN="$(printf '%s' "$register" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["token"])')"
CHAT_PAGE="$(curl --noproxy '*' --fail --silent "$APP_URL/chat" -b "token=$TOKEN")"
grep -q 'agentMode' <<<"$CHAT_PAGE"
echo "PASS chat page exposes explicit Agent switch"
BOT_ID="$(curl --noproxy '*' --fail --silent "$APP_URL/api/v1/users/search?q=shareo_bot&limit=20" \
    -b "token=$TOKEN" | python3 -c 'import json,sys; print(next(u["id"] for u in json.load(sys.stdin)["data"] if u.get("is_bot") and u.get("username")=="shareo_bot"))')"
CONVERSATION_ID="$(curl --noproxy '*' --fail --silent -X POST "$APP_URL/api/v1/conversations" \
    -b "token=$TOKEN" -H 'Content-Type: application/json' -d "{\"user_id\":$BOT_ID}" | \
    python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')"

bot_message_for_source() {
    local source_id="$1"
    curl --noproxy '*' --fail --silent \
        "$APP_URL/api/v1/conversations/$CONVERSATION_ID/messages?limit=100" \
        -b "token=$TOKEN" | python3 -c '
import json,sys
source_id=int(sys.argv[1])
for message in json.load(sys.stdin)["data"]["messages"]:
    if not message.get("sender",{}).get("is_bot") or not message.get("meta"):
        continue
    try:
        meta=json.loads(message["meta"])
    except (TypeError,ValueError):
        continue
    if meta.get("source_message_id") == source_id:
        print(json.dumps(message,ensure_ascii=False))
        break
' "$source_id"
}

SOURCE_ID="$(curl --noproxy '*' --fail --silent -X POST \
    "$APP_URL/api/v1/conversations/$CONVERSATION_ID/messages" -b "token=$TOKEN" \
    -H 'Content-Type: application/json' -d '{"content":"夜景怎么拍？","ai_mode":"agent"}' | \
    python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')"
BOT_MESSAGE=""
for _ in $(seq 1 120); do
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
assert meta["ai_mode"] == "agent"
trace = meta["agent_trace"]
assert trace["status"] == "completed" and trace["steps"]
assert all(step["tool"] in {"semantic_search_posts", "keyword_search_posts", "read_posts", "search_images"} for step in trace["steps"])
assert any(item["post_id"] == post_id for item in meta["citations"])
assert "思维链" not in message["content"]
PY
echo "PASS Agent reply has bounded steps and approved citation post=$POST_ID"

RAG_SOURCE_ID="$(curl --noproxy '*' --fail --silent -X POST \
    "$APP_URL/api/v1/conversations/$CONVERSATION_ID/messages" -b "token=$TOKEN" \
    -H 'Content-Type: application/json' -d '{"content":"夜景怎么拍？"}' | \
    python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')"
RAG_MESSAGE=""
for _ in $(seq 1 90); do
    RAG_MESSAGE="$(bot_message_for_source "$RAG_SOURCE_ID")"
    [ -n "$RAG_MESSAGE" ] && break
    sleep 1
done
python3 - "$RAG_MESSAGE" <<'PY'
import json
import sys
message = json.loads(sys.argv[1])
meta = json.loads(message["meta"])
assert meta.get("ai_mode", "rag") == "rag"
assert "agent_trace" not in meta
PY
echo "PASS default private chat still uses RAG"
echo "PASS private-message Agent E2E"
