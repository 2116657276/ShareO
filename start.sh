#!/usr/bin/env bash
# Default ShareO launcher: native Homebrew PostgreSQL/pgvector/Go/Python runtime.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$PROJECT_DIR"

exec bash scripts/start_local.sh
