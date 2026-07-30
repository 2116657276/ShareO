#!/usr/bin/env bash
# Start the native ShareO runtime, wait for all required readiness checks, and open the web app.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

env_file_value() {
    local key="$1" line name value
    [ -f "$PROJECT_DIR/.env" ] || return 0
    while IFS= read -r line || [ -n "$line" ]; do
        line="${line%$'\r'}"
        [[ -z "$line" || "$line" == \#* ]] && continue
        name="${line%%=*}"
        [ "$name" = "$line" ] && continue
        name="${name//[[:space:]]/}"
        [ "$name" = "$key" ] || continue
        value="${line#*=}"
        value="${value#\"}"
        value="${value%\"}"
        value="${value#\'}"
        value="${value%\'}"
        printf '%s' "$value"
        return 0
    done < "$PROJECT_DIR/.env"
}

APP_PORT="${SHAREO_APP_PORT:-$(env_file_value SHAREO_APP_PORT)}"
APP_PORT="${APP_PORT:-8080}"
APP_URL="http://127.0.0.1:${APP_PORT}/home"

export NO_PROXY="${NO_PROXY:-127.0.0.1,localhost,::1}"
for host in 127.0.0.1 localhost ::1; do
    case ",$NO_PROXY," in
        *",$host,"*) ;;
        *) NO_PROXY="$NO_PROXY,$host" ;;
    esac
done
export no_proxy="${no_proxy:-$NO_PROXY}"

pass() { echo "[PASS] $1"; }
fail() { echo "[FAIL] $1" >&2; }

echo "Starting native ShareO services..."
# Always reload application code. Infrastructure and data remain untouched.
bash scripts/local_runtime.sh app-stop
bash scripts/local_runtime.sh dev-local

echo "Warming AI models and checking Provider/Qdrant readiness..."
bash scripts/warm_ai.sh

echo "Running final local readiness checks..."
SHAREO_LOCAL_STRICT=1 bash scripts/test_local_stack.sh

page_status="$(curl --noproxy '*' --silent --show-error --max-time 5 \
    -o /dev/null -w '%{http_code}' "$APP_URL" || true)"
case "$page_status" in
    2??|3??)
        pass "web page responds at $APP_URL (HTTP $page_status)"
        ;;
    *)
        fail "web page is not reachable at $APP_URL (HTTP $page_status)"
        exit 1
        ;;
esac

if [ "${SHAREO_OPEN_BROWSER:-1}" = "1" ] && command -v open >/dev/null 2>&1; then
    if open "$APP_URL"; then
        pass "opened $APP_URL in the default browser"
    else
        fail "could not open the default browser"
        exit 1
    fi
elif [ "${SHAREO_OPEN_BROWSER:-1}" != "1" ]; then
    echo "[INFO] browser opening skipped because SHAREO_OPEN_BROWSER is not 1"
else
    echo "[INFO] macOS 'open' command is unavailable; visit $APP_URL manually"
fi

echo "Native ShareO is ready. Use 'make local-stop' to stop only processes started by ShareO."
