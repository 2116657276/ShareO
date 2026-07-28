#!/usr/bin/env bash
# Read-only local environment and Compose preflight for the current Demo stack.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

if docker compose version >/dev/null 2>&1; then
    compose=(docker compose)
else
    compose=(docker-compose)
fi

env_value() {
    local key="$1" line value
    if [ ! -f .env ]; then
        return 0
    fi
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
    done < .env
}

status=0
pass() { echo "[PASS] $1"; }
warn() { echo "[WARN] $1"; }
fail() { echo "[FAIL] $1" >&2; status=1; }

if docker info >/dev/null 2>&1; then
    pass "Docker daemon is reachable"
else
    fail "Docker daemon is not reachable; start Colima/Docker first"
fi

if "${compose[@]}" config -q >/dev/null 2>&1; then
    pass "Compose configuration is valid"
else
    fail "Compose configuration is invalid"
fi

for key in SHAREO_AI_LLM_BASE_URL SHAREO_AI_LLM_API_KEY SHAREO_AI_LLM_MODEL; do
    if [ -n "$(env_value "$key")" ]; then
        pass "$key is configured"
    else
        fail "$key is missing from .env or the shell"
    fi
done

for key in SHAREO_AI_HF_ENDPOINT SHAREO_AI_HTTP_PROXY SHAREO_AI_HTTPS_PROXY SHAREO_AI_ALL_PROXY; do
    if [ -n "$(env_value "$key")" ]; then
        pass "$key is configured"
    else
        warn "$key is not configured; model download may require direct network access"
    fi
done

no_proxy="$(env_value SHAREO_AI_NO_PROXY)"
for host in localhost 127.0.0.1 app redis qdrant; do
    if [[ ",$no_proxy," == *",$host,"* ]]; then
        continue
    fi
    warn "SHAREO_AI_NO_PROXY does not explicitly contain $host"
done

if "${compose[@]}" ps >/dev/null 2>&1; then
    pass "Compose project is inspectable"
else
    warn "Compose project is not running; run make up before readiness checks"
fi

http_status() {
    curl --noproxy '*' --silent --show-error --max-time 5 -o /dev/null -w '%{http_code}' "$1" || printf '000'
}

app_status="$(http_status http://127.0.0.1:8080/healthz)"
if [ "$app_status" = "200" ]; then
    pass "Go app healthz is ready"
else
    warn "Go app healthz returned $app_status"
fi

ai_status="$(http_status http://127.0.0.1:8000/healthz)"
if [ "$ai_status" = "200" ]; then
    pass "AI service healthz is ready"
else
    warn "AI service healthz returned $ai_status"
fi

internal_token="$(env_value SHAREO_INTERNAL_TOKEN)"
if [ -z "$internal_token" ]; then
    internal_token="shareo-dev-internal"
fi
host_fingerprint="$(cd ai-service && python3 -m app.commands.source_fingerprint)"
runtime_payload="$(
    curl --noproxy '*' --silent --show-error --max-time 5 \
        -H "X-Internal-Token: $internal_token" \
        http://127.0.0.1:8000/readyz/agent 2>/dev/null || true
)"
runtime_fingerprint="$(
    python3 -c 'import json,sys
try:
    value=json.load(sys.stdin).get("source_fingerprint","")
    print(value if isinstance(value,str) else "")
except Exception:
    print("")' <<<"$runtime_payload"
)"
if [ -z "$runtime_fingerprint" ]; then
    warn "AI runtime source fingerprint is unavailable"
elif [ "$runtime_fingerprint" = "$host_fingerprint" ]; then
    pass "AI runtime source fingerprint matches the workspace"
elif [ "${SHAREO_DOCTOR_ALLOW_STALE_AI:-0}" = "1" ]; then
    warn "AI runtime source fingerprint is stale; automatic verification will rebuild it"
else
    fail "AI runtime source fingerprint differs from the workspace; run make reload-ai"
fi

warn "Go app image has no runtime source fingerprint; source consistency is not proven"

echo "Preflight complete. Readiness and model warmup are handled by make warm-ai."
exit "$status"
