#!/bin/bash
# Apply the clean-slate lightweight schema.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
DB_HOST="${MYSQL_HOST:-127.0.0.1}"
DB_PORT="${MYSQL_PORT:-3306}"
DB_USER="${MYSQL_USER:-root}"
DB_NAME="${MYSQL_DATABASE:-shareo}"
export MYSQL_PWD="${MYSQL_PASS:-shareo_pass}"

if [[ ! "$DB_NAME" =~ ^[A-Za-z0-9_]+$ ]]; then
    echo "MYSQL_DATABASE must contain only letters, digits, or underscores" >&2
    exit 2
fi

mysql_args=(-h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" "$DB_NAME")
mysql "${mysql_args[@]}" < "$PROJECT_DIR/migrations/001_init.sql"
