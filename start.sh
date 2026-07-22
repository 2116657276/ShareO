#!/bin/bash
# Compatibility wrapper for the single supported runtime: Docker Compose.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$PROJECT_DIR"

if docker compose version >/dev/null 2>&1; then
    compose=(docker compose)
else
    compose=(docker-compose)
fi

"${compose[@]}" up -d --build --wait

echo "ShareO is ready: http://localhost:${SHAREO_APP_PORT:-8080}"
echo "MinIO console:  http://localhost:${SHAREO_MINIO_CONSOLE_PORT:-9001}"
