#!/bin/bash
# Rebuild only a local ShareO database (or an explicitly named *_test database).
set -euo pipefail

if [ "${CONFIRM:-}" != "YES" ]; then
    echo "Refusing reset: run with CONFIRM=YES" >&2
    exit 2
fi

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
CONFIG_PATH="${SHAREO_CONFIG:-$PROJECT_DIR/config.yaml}"
READ_CONFIG="$PROJECT_DIR/scripts/read_config.py"

DB_HOST="$(python3 "$READ_CONFIG" database host "$CONFIG_PATH")"
DB_PORT="$(python3 "$READ_CONFIG" database port "$CONFIG_PATH")"
DB_USER="$(python3 "$READ_CONFIG" database user "$CONFIG_PATH")"
DB_PASS="$(python3 "$READ_CONFIG" database password "$CONFIG_PATH")"
DB_NAME="$(python3 "$READ_CONFIG" database dbname "$CONFIG_PATH")"

if [[ ! "$DB_NAME" =~ ^[A-Za-z0-9_]+$ ]]; then
    echo "Refusing reset: invalid database name" >&2
    exit 2
fi
if [[ "$DB_HOST" != "127.0.0.1" && "$DB_HOST" != "localhost" && ! "$DB_NAME" =~ _test$ ]]; then
    echo "Refusing reset: target must be local or use a *_test database" >&2
    exit 2
fi

export MYSQL_PWD="$DB_PASS"
mysql_args=(-h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER")
mysql "${mysql_args[@]}" -e "DROP DATABASE IF EXISTS \`$DB_NAME\`; CREATE DATABASE \`$DB_NAME\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
MYSQL_HOST="$DB_HOST" MYSQL_PORT="$DB_PORT" MYSQL_USER="$DB_USER" MYSQL_PASS="$DB_PASS" MYSQL_DATABASE="$DB_NAME" \
    bash "$PROJECT_DIR/scripts/migrate_schema.sh"

echo "Rebuilt database $DB_NAME on $DB_HOST:$DB_PORT"
