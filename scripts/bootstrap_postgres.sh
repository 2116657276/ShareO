#!/usr/bin/env bash
# Provision the local PostgreSQL roles/database and apply the checked-in schema.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

env_file_value() {
    local key="$1" line value
    [ -f "$PROJECT_DIR/.env" ] || return 0
    while IFS= read -r line || [ -n "$line" ]; do
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
    done < "$PROJECT_DIR/.env"
    return 0
}

value_for() {
    local key="$1" default="${2:-}" value
    value="${!key-}"
    if [[ -n "$value" ]]; then
        printf '%s' "$value"
    else
        value="$(env_file_value "$key")"
        printf '%s' "${value:-$default}"
    fi
}

PG_HOST="$(value_for SHAREO_PG_HOST 127.0.0.1)"
PG_PORT="$(value_for SHAREO_PG_PORT 5432)"
PG_DATABASE="$(value_for SHAREO_PG_DATABASE shareo)"
PG_ADMIN_USER="$(value_for SHAREO_PG_ADMIN_USER "$(id -un)")"
PG_ADMIN_PASSWORD="$(value_for SHAREO_PG_ADMIN_PASSWORD)"
APP_ROLE="$(value_for SHAREO_DB_USER shareo_app)"
AI_ROLE="$(value_for SHAREO_AI_DB_USER shareo_ai)"
APP_PASSWORD="$(value_for SHAREO_DB_PASSWORD)"
AI_PASSWORD="$(value_for SHAREO_AI_DB_PASSWORD)"

if [[ -z "$APP_PASSWORD" || -z "$AI_PASSWORD" ]]; then
    echo "SHAREO_DB_PASSWORD and SHAREO_AI_DB_PASSWORD are required" >&2
    exit 2
fi
if ! command -v psql >/dev/null 2>&1; then
    echo "psql is required (install Homebrew postgresql@17)" >&2
    exit 2
fi

admin_args=(--host "$PG_HOST" --port "$PG_PORT" --username "$PG_ADMIN_USER")
admin_env=(env)
if [[ -n "$PG_ADMIN_PASSWORD" ]]; then
    admin_env=(env PGPASSWORD="$PG_ADMIN_PASSWORD")
fi
if ! "${admin_env[@]}" psql "${admin_args[@]}" --dbname postgres --quiet --tuples-only --no-align \
    -c "SELECT 1" >/dev/null; then
    echo "cannot connect to PostgreSQL as $PG_ADMIN_USER on $PG_HOST:$PG_PORT" >&2
    exit 1
fi

db_name_literal="${PG_DATABASE//\'/\'\'}"
if ! "${admin_env[@]}" psql "${admin_args[@]}" --dbname postgres --quiet --tuples-only --no-align \
    -c "SELECT 1 FROM pg_database WHERE datname = '$db_name_literal'" | grep -qx 1; then
    "${admin_env[@]}" createdb "${admin_args[@]}" "$PG_DATABASE"
fi

"${admin_env[@]}" psql "${admin_args[@]}" --dbname "$PG_DATABASE" --quiet --set=ON_ERROR_STOP=1 \
    -v app_password="$APP_PASSWORD" -v ai_password="$AI_PASSWORD" \
    -v app_role="$APP_ROLE" -v ai_role="$AI_ROLE" <<'SQL' >/dev/null
SELECT format('CREATE ROLE %I LOGIN', :'app_role')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'app_role');
\gexec
SELECT format('CREATE ROLE %I LOGIN', :'ai_role')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'ai_role');
\gexec
SQL

"${admin_env[@]}" psql "${admin_args[@]}" --dbname "$PG_DATABASE" --quiet --set=ON_ERROR_STOP=1 \
    -v app_password="$APP_PASSWORD" -v ai_password="$AI_PASSWORD" \
    -v app_role="$APP_ROLE" -v ai_role="$AI_ROLE" <<'SQL' >/dev/null
SELECT format('ALTER ROLE %I LOGIN PASSWORD %L', :'app_role', :'app_password');
\gexec
SELECT format('ALTER ROLE %I LOGIN PASSWORD %L', :'ai_role', :'ai_password');
\gexec
SQL

"${admin_env[@]}" psql "${admin_args[@]}" --dbname "$PG_DATABASE" --set=ON_ERROR_STOP=1 \
    -f "$PROJECT_DIR/migrations/postgres/001_schema.sql" \
    -f "$PROJECT_DIR/migrations/postgres/002_vector.sql"

"${admin_env[@]}" psql "${admin_args[@]}" --dbname "$PG_DATABASE" --quiet --set=ON_ERROR_STOP=1 \
    -v db_name="$PG_DATABASE" -v app_role="$APP_ROLE" -v ai_role="$AI_ROLE" <<'SQL' >/dev/null
SELECT format('GRANT CONNECT ON DATABASE %I TO %I, %I', :'db_name', :'app_role', :'ai_role');
\gexec
SELECT format('GRANT USAGE ON SCHEMA public TO %I', :'app_role');
\gexec
SELECT format('GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO %I', :'app_role');
\gexec
SELECT format('GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO %I', :'app_role');
\gexec
SELECT format('ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %I', :'app_role');
\gexec
SELECT format('ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO %I', :'app_role');
\gexec

SELECT format('GRANT USAGE ON SCHEMA ai TO %I', :'ai_role');
\gexec
SELECT format('GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA ai TO %I', :'ai_role');
\gexec
SELECT format('GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA ai TO %I', :'ai_role');
\gexec
SELECT format('ALTER DEFAULT PRIVILEGES IN SCHEMA ai GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %I', :'ai_role');
\gexec
SELECT format('ALTER DEFAULT PRIVILEGES IN SCHEMA ai GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO %I', :'ai_role');
\gexec
SQL

echo "PostgreSQL roles and ShareO schemas are ready on $PG_HOST:$PG_PORT/$PG_DATABASE"
