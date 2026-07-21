#!/bin/bash
# Apply structure migrations only. Seed and cleanup migrations are opt-in.
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

mysql_args=(-h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER")

sed \
    -e "s/^CREATE DATABASE IF NOT EXISTS shareo$/CREATE DATABASE IF NOT EXISTS $DB_NAME/" \
    -e "s/^USE shareo;$/USE $DB_NAME;/" \
    -e '/^-- 默认管理员账号/,$d' \
    "$PROJECT_DIR/migrations/001_init.sql" | mysql "${mysql_args[@]}"

schema_migrations=(
    003_triggers.sql
    005_repost.sql
    006_fulltext.sql
    007_notifications.sql
    008_reply_to_uid_index.sql
    009_chat.sql
    010_chat_hardening.sql
)

for filename in "${schema_migrations[@]}"; do
    mysql "${mysql_args[@]}" "$DB_NAME" < "$PROJECT_DIR/migrations/$filename"
done
