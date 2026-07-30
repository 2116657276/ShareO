#!/usr/bin/env bash
# Run real integration tests against a disposable database in the current Demo MySQL.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

local_mode=0
if command -v mysql >/dev/null 2>&1 && curl --noproxy '*' --silent --fail \
    http://127.0.0.1:8080/healthz >/dev/null 2>&1; then
    local_mode=1
elif docker compose version >/dev/null 2>&1; then
    compose=(docker compose)
elif docker-compose version >/dev/null 2>&1; then
    compose=(docker-compose)
else
    echo "[FAIL] neither the native stack nor Docker Compose is available" >&2
    exit 2
fi

env_value() {
    local key="$1" line value
    while IFS= read -r line; do
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
    done < .env 2>/dev/null || true
}

config_database_password() {
    [ -f config.yaml ] || return 0
    awk '
        /^database:[[:space:]]*$/ { inside=1; next }
        /^[^[:space:]]/ { inside=0 }
        inside && /^[[:space:]]+password:/ {
            value=$0
            sub(/^[[:space:]]+password:[[:space:]]*/, "", value)
            gsub(/^\"|\"$/, "", value)
            gsub(/^\x27|\x27$/, "", value)
            print value
            exit
        }
    ' config.yaml
}

db_password="$(env_value SHAREO_DB_PASSWORD)"
if [ -z "$db_password" ] && [ "$local_mode" -eq 1 ]; then
    db_password="$(config_database_password)"
fi
db_password="${db_password:-shareo_pass}"
mysql_port="$(env_value SHAREO_MYSQL_PORT)"
mysql_port="${mysql_port:-3306}"
redis_port="$(env_value SHAREO_REDIS_PORT)"
redis_port="${redis_port:-6379}"
qdrant_port="$(env_value SHAREO_QDRANT_HTTP_PORT)"
qdrant_port="${qdrant_port:-6333}"
test_db="shareo_test"

export GOCACHE="${GOCACHE:-$PROJECT_DIR/.cache/shareo/go-build}"
export GOMODCACHE="${GOMODCACHE:-$PROJECT_DIR/.cache/shareo/go-mod}"
export UV_CACHE_DIR="${UV_CACHE_DIR:-$PROJECT_DIR/.cache/shareo/uv}"

mysql_exec() {
    if [ "$local_mode" -eq 1 ]; then
        MYSQL_PWD="$db_password" mysql --protocol=tcp -h 127.0.0.1 -P "$mysql_port" -u root "$@"
    else
        "${compose[@]}" exec -T mysql mysql "-p$db_password" "$@"
    fi
}

cleanup() {
    mysql_exec -e \
        "DROP DATABASE IF EXISTS $test_db" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

mysql_exec -e \
    "CREATE DATABASE IF NOT EXISTS $test_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci" >/dev/null
for migration in migrations/*.sql; do
    mysql_exec "$test_db" < "$migration"
done

export SHAREO_TEST_MYSQL_DSN="root:${db_password}@tcp(127.0.0.1:${mysql_port})/${test_db}?charset=utf8mb4&parseTime=True&loc=Local"
export SHAREO_TEST_REDIS_URL="redis://127.0.0.1:${redis_port}/0"
export SHAREO_TEST_QDRANT_URL="http://127.0.0.1:${qdrant_port}"
local_no_proxy="127.0.0.1,localhost,::1"
export NO_PROXY="${NO_PROXY:+$NO_PROXY,}$local_no_proxy"
export no_proxy="${no_proxy:+$no_proxy,}$local_no_proxy"

go test -count=1 -tags=integration ./...
(
    cd ai-service
    uv run --frozen pytest -m integration
)
if [ "$local_mode" -eq 1 ]; then
    echo "[PASS] native MySQL/Redis/Qdrant integration tests"
else
    echo "[PASS] current Demo MySQL/Redis/Qdrant integration tests"
fi
