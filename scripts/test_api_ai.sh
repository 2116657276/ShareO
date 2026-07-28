#!/usr/bin/env bash
# 本机聊天、RAG、Agent 和内部鉴权回归。
# 只通过 Go/AI HTTP 接口操作，不直接写 MySQL、Redis、MinIO 或 Qdrant。
set -euo pipefail

env_value() {
    local key="$1" line value
    if [ -n "${!key-}" ]; then
        printf '%s' "${!key}"
        return 0
    fi
    [ -f .env ] || return 0
    while IFS= read -r line || [ -n "$line" ]; do
        case "$line" in
            "$key="*)
                value="${line#*=}"
                value="${value#\"}"
                value="${value%\"}"
                value="${value#\'}"
                value="${value%\'}"
                printf '%s' "$value"
                return 0
                ;;
        esac
    done < .env
}

BASE="$(env_value SHAREO_BASE_URL)"
BASE="${BASE:-http://127.0.0.1:8080}"
AI_BASE="$(env_value SHAREO_AI_BASE_URL)"
AI_BASE="${AI_BASE:-http://127.0.0.1:8000}"
INTERNAL_TOKEN="$(env_value SHAREO_INTERNAL_TOKEN)"
INTERNAL_TOKEN="${INTERNAL_TOKEN:-shareo-local-internal}"
ADMIN_USER="$(env_value SHAREO_TEST_ADMIN_USER)"
ADMIN_USER="${ADMIN_USER:-demoadmin}"
ADMIN_PASSWORD="$(env_value SHAREO_TEST_ADMIN_PASSWORD)"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123}"
TEST_PASSWORD="$(env_value SHAREO_TEST_PASSWORD)"
TEST_PASSWORD="${TEST_PASSWORD:-shareo-api-test-pass}"
SUFFIX="$(python3 -c 'import time; print(time.time_ns())')"
SHORT_SUFFIX="${SUFFIX: -10}"
USER1="api-ai-${SHORT_SUFFIX}"
USER2="api-ai-peer-${SHORT_SUFFIX}"
PASS=0
FAIL=0
USER1_ID=""
USER2_ID=""
ADMIN_TOKEN=""

# 本机 API 请求不得经过宿主机代理。
curl_local() { command curl --noproxy '*' --silent --show-error --max-time "${SHAREO_API_TIMEOUT:-20}" "$@"; }

green() { printf '\033[32m  PASS: %s\033[0m\n' "$1"; PASS=$((PASS + 1)); }
red() { printf '\033[31m  FAIL: %s\033[0m\n' "$1" >&2; FAIL=$((FAIL + 1)); }

expect_code() {
    local label="$1" expected="$2" actual
    shift 2
    actual="$(command curl --noproxy '*' --silent --show-error --max-time 20 -o /dev/null -w '%{http_code}' "$@" || true)"
    if [ "$actual" = "$expected" ]; then
        green "$label (HTTP $actual)"
    else
        red "$label (expected HTTP $expected, got $actual)"
    fi
}

json_value() {
    local expression="$1"
    python3 -c "import json,sys; value=json.load(sys.stdin); print($expression)"
}

cleanup() {
    set +e
    if [ -n "$ADMIN_TOKEN" ]; then
        for user_id in "$USER1_ID" "$USER2_ID"; do
            if [ -n "$user_id" ]; then
                curl_local -X PUT "$BASE/api/v1/admin/users/$user_id/status" \
                    -b "token=$ADMIN_TOKEN" -H 'Content-Type: application/json' \
                    -d '{"status":0}' >/dev/null
            fi
        done
    fi
}
trap cleanup EXIT

echo "============================================"
echo "  ShareO 本机聊天与 AI API 回归"
echo "============================================"

if [ -z "$INTERNAL_TOKEN" ]; then
    red "SHAREO_INTERNAL_TOKEN 未配置"
    exit 1
fi

echo "=== 1. 内部鉴权与 readiness ==="
expect_code "AI readiness 缺少内部 Token" 401 "$AI_BASE/readyz/agent"
expect_code "AI readiness 使用错误 Token" 401 -H 'X-Internal-Token: invalid-test-token' "$AI_BASE/readyz/agent"
expect_code "Go 内部接口缺少内部 Token" 401 "$BASE/internal/health"
expect_code "Go 内部接口使用错误 Token" 401 -H 'X-Internal-Token: invalid-test-token' "$BASE/internal/health"
expect_code "Go 未登录访问身份接口" 401 "$BASE/api/v1/auth/me"

AI_READY="$(curl_local -H "X-Internal-Token: $INTERNAL_TOKEN" "$AI_BASE/readyz/agent")"
printf '%s' "$AI_READY" | python3 -c 'import json,sys; data=json.load(sys.stdin); assert data.get("status") in {"ready","ok"}'
green "Agent readiness"

echo "=== 2. 测试身份 ==="
ADMIN_LOGIN="$(curl_local -X POST "$BASE/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASSWORD\"}")"
ADMIN_TOKEN="$(printf '%s' "$ADMIN_LOGIN" | json_value 'value["data"]["token"]')"
green "管理员登录"

REGISTER1="$(curl_local -X POST "$BASE/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$USER1\",\"password\":\"$TEST_PASSWORD\"}")"
USER1_ID="$(printf '%s' "$REGISTER1" | json_value 'value["data"]["user"]["id"]')"
TOKEN1="$(printf '%s' "$REGISTER1" | json_value 'value["data"]["token"]')"
green "注册聊天测试用户"

REGISTER2="$(curl_local -X POST "$BASE/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$USER2\",\"password\":\"$TEST_PASSWORD\"}")"
USER2_ID="$(printf '%s' "$REGISTER2" | json_value 'value["data"]["user"]["id"]')"
TOKEN2="$(printf '%s' "$REGISTER2" | json_value 'value["data"]["token"]')"
green "注册第二聊天测试用户"

echo "=== 3. 普通私聊与 Agent 边界 ==="
PEER_CONV="$(curl_local -X POST "$BASE/api/v1/conversations" -b "token=$TOKEN1" \
    -H 'Content-Type: application/json' -d "{\"user_id\":$USER2_ID}")"
PEER_CONV_ID="$(printf '%s' "$PEER_CONV" | json_value 'value["data"]["id"]')"
PEER_MESSAGE="$(curl_local -X POST "$BASE/api/v1/conversations/$PEER_CONV_ID/messages" \
    -b "token=$TOKEN1" -H 'Content-Type: application/json' \
    -d '{"content":"普通私聊回归消息"}')"
PEER_MESSAGE_ID="$(printf '%s' "$PEER_MESSAGE" | json_value 'value["data"]["id"]')"
if [ "$PEER_MESSAGE_ID" -gt 0 ]; then
    green "普通私聊消息落库"
else
    red "普通私聊消息落库"
fi
PEER_MESSAGES="$(curl_local "$BASE/api/v1/conversations/$PEER_CONV_ID/messages?limit=30" -b "token=$TOKEN1")"
printf '%s' "$PEER_MESSAGES" | python3 -c '
import json,sys
messages=json.load(sys.stdin)["data"]["messages"]
assert messages
assert all(not item.get("sender",{}).get("is_bot") for item in messages)
'
green "普通私聊不产生 Bot/Agent 回复"

expect_code "普通用户不能以 Agent 模式私聊普通用户" 400 \
    -X POST "$BASE/api/v1/conversations/$PEER_CONV_ID/messages" \
    -b "token=$TOKEN1" -H 'Content-Type: application/json' \
    -d '{"content":"不应进入 Agent","ai_mode":"agent"}'

BOT_ID="$(curl_local "$BASE/api/v1/users/search?q=shareo_bot&limit=20" -b "token=$TOKEN1" | \
    python3 -c 'import json,sys; users=json.load(sys.stdin)["data"]; print(next(u["id"] for u in users if u.get("is_bot") and u.get("username")=="shareo_bot"))')"
BOT_CONV="$(curl_local -X POST "$BASE/api/v1/conversations" -b "token=$TOKEN1" \
    -H 'Content-Type: application/json' -d "{\"user_id\":$BOT_ID}")"
BOT_CONV_ID="$(printf '%s' "$BOT_CONV" | json_value 'value["data"]["id"]')"
green "创建固定 Bot 私聊"

message_for_source() {
    local source_id="$1"
    local messages
    messages="$(curl_local "$BASE/api/v1/conversations/$BOT_CONV_ID/messages?limit=100" -b "token=$TOKEN1")"
    printf '%s' "$messages" | python3 -c '
import json
import sys

source_id = int(sys.argv[1])
try:
    messages = json.load(sys.stdin)["data"]["messages"]
except (ValueError, KeyError, TypeError):
    raise SystemExit(0)
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

send_and_wait() {
    local payload="$1" source message=""
    source="$(curl_local -X POST "$BASE/api/v1/conversations/$BOT_CONV_ID/messages" \
        -b "token=$TOKEN1" -H 'Content-Type: application/json' -d "$payload" \
        | json_value 'value["data"]["id"]')"
    for _ in $(seq 1 "${SHAREO_BOT_WAIT_SECONDS:-180}"); do
        message="$(message_for_source "$source" || true)"
        if [ -n "$message" ]; then
            printf '%s\n' "$message"
            return 0
        fi
        sleep 1
    done
    return 1
}

echo "=== 4. 默认 RAG ==="
RAG_MESSAGE="$(send_and_wait '{"content":"西湖夜景有什么特点？"}')" || {
    red "默认 RAG 在等待时间内未返回"
    exit 1
}
printf '%s' "$RAG_MESSAGE" | python3 -c '
import json,sys
message=json.load(sys.stdin)
meta=json.loads(message["meta"])
assert meta.get("ai_mode","rag") == "rag"
assert "agent_trace" not in meta
assert isinstance(meta.get("citations",[]), list)
'
green "默认 RAG 返回且不产生 Agent 轨迹"

echo "=== 5. 显式 Agent ==="
AGENT_MESSAGE="$(send_and_wait '{"content":"请比较西湖白天和夜晚的景观特色，并引用相关帖子。","ai_mode":"agent"}')" || {
    red "显式 Agent 在等待时间内未返回"
    exit 1
}
printf '%s' "$AGENT_MESSAGE" | python3 -c '
import json,sys
allowed={"semantic_search_posts","keyword_search_posts","read_posts","search_images"}
message=json.load(sys.stdin)
meta=json.loads(message["meta"])
assert meta.get("ai_mode") == "agent"
trace=meta.get("agent_trace")
assert trace and trace.get("status") == "completed"
assert trace.get("steps")
assert all(step.get("tool") in allowed for step in trace["steps"])
assert isinstance(meta.get("citations",[]), list)
assert "思维链" not in message.get("content","")
'
green "显式 Agent 返回受控轨迹和引用"

echo "=== 6. 回归结果 ==="
if [ "$FAIL" -gt 0 ]; then
    printf '%s\n' "结果: $PASS PASS, $FAIL FAIL" >&2
    exit 1
fi
printf '%s\n' "结果: $PASS PASS, $FAIL FAIL"
