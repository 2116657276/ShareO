#!/usr/bin/env bash
# Read-only smoke checks for the native Go/Python + Homebrew PostgreSQL runtime.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

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
    return 0
}

APP_PORT="$(env_value SHAREO_APP_PORT)"
APP_PORT="${APP_PORT:-8080}"
AI_PORT="$(env_value SHAREO_AI_PORT)"
AI_PORT="${AI_PORT:-8000}"
INTERNAL_TOKEN="$(env_value SHAREO_INTERNAL_TOKEN)"
INTERNAL_TOKEN="${INTERNAL_TOKEN:-shareo-local-internal}"
AI_URL="http://127.0.0.1:$AI_PORT"

check_http() {
    local label="$1" url="$2" status
    status="$(curl --noproxy '*' --silent --show-error --max-time 5 -o /dev/null -w '%{http_code}' "$url" || true)"
    [ "$status" = "200" ] || { echo "[FAIL] $label returned HTTP $status" >&2; return 1; }
    echo "[PASS] $label"
}

check_http "Go app health" "http://127.0.0.1:$APP_PORT/healthz"
check_http "AI service health" "$AI_URL/healthz"
check_http "MinIO health" "http://127.0.0.1:9000/minio/health/live"

if command -v redis-cli >/dev/null 2>&1 && [ "$(redis-cli -h 127.0.0.1 -p 6379 ping 2>/dev/null || true)" = "PONG" ]; then
    echo "[PASS] Redis ping"
else
    echo "[FAIL] Redis ping" >&2
    exit 1
fi

for endpoint in image-search rag agent; do
    status="$(curl --noproxy '*' --silent --show-error --max-time 8 -o /dev/null -w '%{http_code}' \
        -H "X-Internal-Token: $INTERNAL_TOKEN" "$AI_URL/readyz/$endpoint" || true)"
    if [ "$status" = "200" ]; then
        echo "[PASS] $endpoint readiness"
    elif [ "${SHAREO_LOCAL_STRICT:-0}" = "1" ]; then
        echo "[FAIL] $endpoint readiness returned HTTP $status" >&2
        exit 1
    else
        echo "[WARN] $endpoint readiness returned HTTP $status"
    fi
done

echo "[PASS] native stack smoke checks"
