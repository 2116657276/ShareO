#!/bin/bash
# Initialize schema only. Demo seed (002), cleanup (004), and the default
# administrator row at the end of 001 are deliberately excluded.
# The official image sources this file into its entrypoint. Do not enable
# nounset here: it would leak into the parent script and break optional
# MYSQL_* variables after this initializer returns.
set -eo pipefail

export MYSQL_PWD="${MYSQL_ROOT_PASSWORD}"

sed '/^-- 默认管理员账号/,$d' /migrations/001_init.sql | mysql -uroot

schema_migrations=(
    /migrations/003_triggers.sql
    /migrations/005_repost.sql
    /migrations/006_fulltext.sql
    /migrations/007_notifications.sql
    /migrations/008_reply_to_uid_index.sql
    /migrations/009_chat.sql
    /migrations/010_chat_hardening.sql
)

for migration in "${schema_migrations[@]}"; do
    mysql -uroot shareo < "$migration"
done
