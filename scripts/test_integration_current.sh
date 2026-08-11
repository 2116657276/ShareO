#!/usr/bin/env bash
# Run integration tests against a disposable native PostgreSQL database.
# The database is intentionally left in place for inspection; no container
# runtime or legacy database/vector service is consulted.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

env_value() {
    local key="$1" line value
    if [ -n "${!key-}" ]; then
        printf '%s' "${!key}"
        return 0
    fi
    [ -f .env ] || return 0
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
    done < .env
    return 0
}

if [ "$(env_value SHAREO_RUNTIME)" = "compose" ]; then
    echo "[FAIL] SHAREO_RUNTIME=compose is retired; integration tests use native PostgreSQL/Redis" >&2
    exit 2
fi

pg_host="$(env_value SHAREO_PG_HOST)"
pg_host="${pg_host:-127.0.0.1}"
pg_port="$(env_value SHAREO_PG_PORT)"
pg_port="${pg_port:-5432}"
admin_user="$(env_value SHAREO_PG_ADMIN_USER)"
admin_user="${admin_user:-$(id -un)}"
admin_password="$(env_value SHAREO_PG_ADMIN_PASSWORD)"
redis_port="$(env_value SHAREO_REDIS_PORT)"
redis_port="${redis_port:-6379}"

export GOCACHE="${GOCACHE:-$PROJECT_DIR/.cache/shareo/go-build}"
export GOMODCACHE="${GOMODCACHE:-$PROJECT_DIR/.cache/shareo/go-mod}"
export UV_CACHE_DIR="${UV_CACHE_DIR:-$PROJECT_DIR/.cache/shareo/uv}"
local_no_proxy="127.0.0.1,localhost,::1"
export NO_PROXY="${NO_PROXY:+$NO_PROXY,}$local_no_proxy"
export no_proxy="${no_proxy:+$no_proxy,}$local_no_proxy"

for command in psql createdb pg_isready redis-cli; do
    command -v "$command" >/dev/null 2>&1 || {
        echo "[FAIL] required native command is missing: $command" >&2
        exit 2
    }
done

if ! pg_isready --host "$pg_host" --port "$pg_port" >/dev/null 2>&1; then
    echo "[FAIL] PostgreSQL is not accepting connections at $pg_host:$pg_port" >&2
    echo "[INFO] start the local stack with: SHAREO_OPEN_BROWSER=0 make up" >&2
    exit 2
fi
if [ "$(redis-cli -h 127.0.0.1 -p "$redis_port" PING 2>/dev/null || true)" != "PONG" ]; then
    echo "[FAIL] Redis is not responding on 127.0.0.1:$redis_port" >&2
    exit 2
fi

if [ -n "$admin_password" ]; then
    export PGPASSWORD="$admin_password"
fi

stamp="$(date +%Y%m%d%H%M%S)"
test_db="shareo_it_${stamp}_test"
createdb --host "$pg_host" --port "$pg_port" --username "$admin_user" "$test_db"
for migration in migrations/postgres/*.sql; do
    psql --no-psqlrc --set ON_ERROR_STOP=1 \
        --host "$pg_host" --port "$pg_port" --username "$admin_user" --dbname "$test_db" \
        < "$migration" >/dev/null
done

admin_user_escaped="$(python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=""))' "$admin_user")"
admin_password_escaped=""
if [ -n "$admin_password" ]; then
    admin_password_escaped=":$(python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=""))' "$admin_password")"
fi
admin_dsn="postgresql://${admin_user_escaped}${admin_password_escaped}@${pg_host}:${pg_port}/${test_db}?sslmode=disable"
export SHAREO_TEST_POSTGRES_DSN="${SHAREO_TEST_POSTGRES_DSN:-$admin_dsn}"
export SHAREO_TEST_AI_DATABASE_URL="${SHAREO_TEST_AI_DATABASE_URL:-$admin_dsn}"
export SHAREO_TEST_REDIS_URL="${SHAREO_TEST_REDIS_URL:-redis://127.0.0.1:${redis_port}/0}"

echo "[INFO] integration database: $test_db (left in place for inspection)"
echo "[INFO] running Go PostgreSQL integration tests"
env GOCACHE="$GOCACHE" GOMODCACHE="$GOMODCACHE" go test -count=1 -tags=integration ./...
echo "[INFO] running Python Redis/pgvector integration tests"
(
    cd ai-service
    uv run --frozen pytest -m integration
)
echo "[PASS] native PostgreSQL 17/pgvector and Redis integration tests"
