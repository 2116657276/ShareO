#!/bin/bash
# Destructively reset the Homebrew-backed ShareO development data only.
set -euo pipefail

if [ "${CONFIRM:-}" != "YES" ]; then
    echo "Refusing to delete data. Re-run with CONFIRM=YES." >&2
    exit 2
fi

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
CONFIG_PATH="${SHAREO_CONFIG:-$PROJECT_DIR/config.yaml}"
DB_HOST="${MYSQL_HOST:-127.0.0.1}"
DB_PORT="${MYSQL_PORT:-3306}"
DB_USER="${MYSQL_USER:-root}"
DB_NAME="${MYSQL_DATABASE:-shareo}"
REDIS_HOST="${REDIS_HOST:-127.0.0.1}"
REDIS_PORT="${REDIS_PORT:-6379}"
REDIS_DB="${REDIS_DB:-0}"
MINIO_ENDPOINT="${SHAREO_MINIO_ENDPOINT:-127.0.0.1:9000}"
MINIO_BUCKET="${SHAREO_MINIO_BUCKET:-shareo}"
MINIO_DATA_DIR="${SHAREO_MINIO_DATA_DIR:-$HOME/minio_data}"

[ "$DB_HOST" = "127.0.0.1" ] && [ "$DB_PORT" = "3306" ] && [ "$DB_NAME" = "shareo" ] || {
    echo "Refusing unexpected MySQL target: $DB_USER@$DB_HOST:$DB_PORT/$DB_NAME" >&2
    exit 2
}
[ "$REDIS_HOST" = "127.0.0.1" ] && [ "$REDIS_PORT" = "6379" ] && [ "$REDIS_DB" = "0" ] || {
    echo "Refusing unexpected Redis target: $REDIS_HOST:$REDIS_PORT db=$REDIS_DB" >&2
    exit 2
}
[ "$MINIO_ENDPOINT" = "127.0.0.1:9000" ] && [ "$MINIO_BUCKET" = "shareo" ] || {
    echo "Refusing unexpected MinIO target: $MINIO_ENDPOINT/$MINIO_BUCKET" >&2
    exit 2
}
[ -d "$MINIO_DATA_DIR" ] || {
    echo "MinIO data directory does not exist: $MINIO_DATA_DIR" >&2
    exit 2
}

MYSQL_PASS_VALUE="${MYSQL_PASS:-}"
if [ -z "$MYSQL_PASS_VALUE" ] && [ -f "$CONFIG_PATH" ]; then
    MYSQL_PASS_VALUE="$(python3 "$PROJECT_DIR/scripts/read_config.py" database password "$CONFIG_PATH" 2>/dev/null || true)"
fi
export MYSQL_PASS="$MYSQL_PASS_VALUE"
export MYSQL_PWD="$MYSQL_PASS_VALUE"

bash "$PROJECT_DIR/scripts/start_minio_homebrew.sh"

MC_BIN="${MC_BIN:-}"
if [ -z "$MC_BIN" ] && command -v mc >/dev/null 2>&1; then
    MC_BIN="$(command -v mc)"
fi
if [ -z "$MC_BIN" ] && command -v brew >/dev/null 2>&1; then
    MC_PREFIX="$(brew --prefix minio/stable/mc 2>/dev/null || true)"
    if [ -x "$MC_PREFIX/bin/mc" ]; then
        MC_BIN="$MC_PREFIX/bin/mc"
    fi
fi
if [ -z "$MC_BIN" ] || [ ! -x "$MC_BIN" ]; then
    echo "MinIO Client not found. Install it with: brew install minio/stable/mc" >&2
    exit 1
fi

ALIAS="shareo-local"
MINIO_ACCESS_KEY="${SHAREO_MINIO_ACCESS_KEY:-minioadmin}"
MINIO_SECRET_KEY="${SHAREO_MINIO_SECRET_KEY:-minioadmin}"
"$MC_BIN" alias set "$ALIAS" "http://$MINIO_ENDPOINT" "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" >/dev/null
"$MC_BIN" rm --recursive --force "$ALIAS/$MINIO_BUCKET" >/dev/null
"$MC_BIN" mb --ignore-existing "$ALIAS/$MINIO_BUCKET" >/dev/null

mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -e "DROP DATABASE IF EXISTS \`$DB_NAME\`; CREATE DATABASE \`$DB_NAME\`;"
bash "$PROJECT_DIR/scripts/migrate_schema.sh"
redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -n "$REDIS_DB" FLUSHDB >/dev/null

USERS="$(mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -N -B -e "SELECT COUNT(*) FROM $DB_NAME.users;")"
POST_IMAGES="$(mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -N -B -e "SELECT COUNT(*) FROM $DB_NAME.post_images;")"
REDIS_KEYS="$(redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -n "$REDIS_DB" DBSIZE | tr -d '\r')"
OBJECTS="$("$MC_BIN" ls --recursive "$ALIAS/$MINIO_BUCKET" 2>/dev/null | wc -l | tr -d ' ')"

[ "$USERS" = "0" ] && [ "$POST_IMAGES" = "0" ] && [ "$REDIS_KEYS" = "0" ] && [ "$OBJECTS" = "0" ] || {
    echo "Reset verification failed: users=$USERS post_images=$POST_IMAGES redis_keys=$REDIS_KEYS objects=$OBJECTS" >&2
    exit 1
}
echo "Homebrew ShareO data reset complete: users=0 post_images=0 redis_keys=0 objects=0"
