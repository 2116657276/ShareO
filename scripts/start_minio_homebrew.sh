#!/bin/bash
# Start the Homebrew-installed MinIO binary on the ShareO development defaults.
set -euo pipefail

DATA_DIR="${SHAREO_MINIO_DATA_DIR:-$HOME/minio_data}"
API_ADDR="${SHAREO_MINIO_ADDRESS:-:9000}"
CONSOLE_ADDR="${SHAREO_MINIO_CONSOLE_ADDRESS:-:9001}"
HEALTH_URL="${SHAREO_MINIO_HEALTH_URL:-http://127.0.0.1:9000/minio/health/live}"
LOG_FILE="${SHAREO_MINIO_LOG_FILE:-/tmp/shareo-minio.log}"
PID_FILE="${SHAREO_MINIO_PID_FILE:-/tmp/shareo-minio.pid}"

if curl --noproxy '*' --fail --silent --show-error --max-time 3 "$HEALTH_URL" >/dev/null 2>&1; then
    echo "MinIO is ready at http://127.0.0.1:9000 (data: $DATA_DIR)"
    exit 0
fi

MINIO_BIN="${SHAREO_MINIO_BIN:-}"
if [ -z "$MINIO_BIN" ] && command -v brew >/dev/null 2>&1; then
    HOMEBREW_MINIO_PREFIX="$(brew --prefix minio 2>/dev/null || true)"
    if [ -x "$HOMEBREW_MINIO_PREFIX/bin/minio" ]; then
        MINIO_BIN="$HOMEBREW_MINIO_PREFIX/bin/minio"
    fi
fi
if [ -z "$MINIO_BIN" ] && command -v minio >/dev/null 2>&1; then
    MINIO_BIN="$(command -v minio)"
fi
if [ -z "$MINIO_BIN" ] || [ ! -x "$MINIO_BIN" ]; then
    echo "MinIO binary not found. Install it with: brew install minio/stable/minio" >&2
    exit 1
fi

mkdir -p "$DATA_DIR"
export MINIO_ROOT_USER="${SHAREO_MINIO_ACCESS_KEY:-minioadmin}"
export MINIO_ROOT_PASSWORD="${SHAREO_MINIO_SECRET_KEY:-minioadmin}"

nohup "$MINIO_BIN" server "$DATA_DIR" \
    --address "$API_ADDR" \
    --console-address "$CONSOLE_ADDR" \
    >"$LOG_FILE" 2>&1 &
echo $! >"$PID_FILE"

for _ in $(seq 1 30); do
    if curl --noproxy '*' --fail --silent --show-error --max-time 2 "$HEALTH_URL" >/dev/null 2>&1; then
        echo "MinIO is ready at http://127.0.0.1:9000 (data: $DATA_DIR)"
        exit 0
    fi
    sleep 1
done

echo "MinIO failed to become ready; data=$DATA_DIR log=$LOG_FILE" >&2
exit 1
