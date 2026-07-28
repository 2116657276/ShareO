#!/usr/bin/env bash
# Warm local embedding models and verify all AI readiness endpoints.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"
AI_URL="${SHAREO_AI_BASE_URL:-http://127.0.0.1:8000}"

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

INTERNAL_TOKEN="$(env_value SHAREO_INTERNAL_TOKEN)"
if [ -z "$INTERNAL_TOKEN" ]; then
    case "$AI_URL" in
        http://127.0.0.1:*|http://localhost:*)
            INTERNAL_TOKEN="shareo-local-internal"
            ;;
        *)
            INTERNAL_TOKEN="shareo-dev-internal"
            ;;
    esac
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
for _ in $(seq 1 180); do
    if curl --noproxy '*' --silent --show-error --max-time 30 --fail \
        "${headers[@]}" -X POST "$AI_URL/v1/search/images" \
        -d '{"query":"夜景照片","limit":1}' >/dev/null; then
        echo "[PASS] image model warmup request"
        break
    fi
    sleep 1
done

echo "Warming text model and checking Provider..."
for _ in $(seq 1 180); do
    if curl --noproxy '*' --silent --show-error --max-time 60 --fail \
        "${headers[@]}" -X POST "$AI_URL/v1/rag/answer" \
        -d '{"question":"夜景怎么拍？","history":[],"top_k":8}' >/dev/null; then
        echo "[PASS] RAG warmup request"
        break
    fi
    sleep 1
done

wait_http "image-search readiness" "$AI_URL/readyz/image-search"
wait_http "RAG readiness" "$AI_URL/readyz/rag"
wait_http "Agent readiness" "$AI_URL/readyz/agent"
echo "[PASS] AI models and Agent dependencies are ready"
