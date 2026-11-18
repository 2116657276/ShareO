#!/usr/bin/env bash
# Warm local embedding models and verify all AI readiness endpoints.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

env_value() {
    local key="$1" line value
    while IFS= read -r line; do
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
    done < .env 2>/dev/null || true
}

AI_PORT="${SHAREO_AI_PORT:-$(env_value SHAREO_AI_PORT)}"
AI_PORT="${AI_PORT:-8000}"
AI_URL="${SHAREO_AI_BASE_URL:-$(env_value SHAREO_AI_BASE_URL)}"
AI_URL="${AI_URL:-http://127.0.0.1:$AI_PORT}"

INTERNAL_TOKEN="${SHAREO_INTERNAL_TOKEN:-$(env_value SHAREO_INTERNAL_TOKEN)}"
if [ "$INTERNAL_TOKEN" = "replace-with-a-random-value" ]; then
    INTERNAL_TOKEN=""
fi
if [ -z "$INTERNAL_TOKEN" ] && [ -r "${SHAREO_LOCAL_STATE_DIR:-/tmp/shareo-local}/internal-token" ]; then
    INTERNAL_TOKEN="$(<"${SHAREO_LOCAL_STATE_DIR:-/tmp/shareo-local}/internal-token")"
fi
case "$INTERNAL_TOKEN" in
    shareo-local-internal|replace-with-a-random-value)
        INTERNAL_TOKEN=""
        ;;
esac
if [ -z "$INTERNAL_TOKEN" ]; then
    echo "[FAIL] SHAREO_INTERNAL_TOKEN 未配置，且本机状态目录没有生成的令牌" >&2
    exit 1
fi
headers=(-H "X-Internal-Token: $INTERNAL_TOKEN" -H 'Content-Type: application/json')

wait_http() {
    local label="$1" url="$2" attempts="${3:-180}" status
    for _ in $(seq 1 "$attempts"); do
        status="$(curl --noproxy '*' --silent --show-error --max-time 15 -o /dev/null -w '%{http_code}' "${headers[@]}" "$url" || printf '000')"
        if [ "$status" = "200" ]; then
            echo "[PASS] $label"
            return 0
        fi
        sleep 1
    done
    echo "[FAIL] $label (last HTTP status: $status)" >&2
    return 1
}

echo "Warming image model..."
image_warmed=0
for _ in $(seq 1 180); do
    if curl --noproxy '*' --silent --max-time 30 --fail \
        "${headers[@]}" -X POST "$AI_URL/v1/search/images" \
        -d '{"query":"夜景照片","limit":1}' >/dev/null; then
        echo "[PASS] image model warmup request"
        image_warmed=1
        break
    fi
    sleep 1
done
if [ "$image_warmed" -ne 1 ]; then
    echo "[FAIL] image model warmup request timed out" >&2
    exit 1
fi

echo "Warming text model and checking Provider..."
rag_warmed=0
for _ in $(seq 1 180); do
    if curl --noproxy '*' --silent --max-time 60 --fail \
        "${headers[@]}" -X POST "$AI_URL/v1/rag/answer" \
        -d '{"question":"夜景怎么拍？","history":[],"top_k":8}' >/dev/null; then
        echo "[PASS] RAG warmup request"
        rag_warmed=1
        break
    fi
    sleep 1
done
if [ "$rag_warmed" -ne 1 ]; then
    echo "[FAIL] RAG warmup request timed out" >&2
    exit 1
fi

wait_http "image-search readiness" "$AI_URL/readyz/image-search"
wait_http "RAG readiness" "$AI_URL/readyz/rag"
wait_http "Agent readiness" "$AI_URL/readyz/agent"
echo "[PASS] AI models and Agent dependencies are ready"
