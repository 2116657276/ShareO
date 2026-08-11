#!/usr/bin/env bash
# Manage ShareO's native development runtime without Docker.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
STATE_DIR="${SHAREO_LOCAL_STATE_DIR:-/tmp/shareo-local}"
mkdir -p "$STATE_DIR"

pass() { echo "[PASS] $1"; }
warn() { echo "[WARN] $1"; }
fail() { echo "[FAIL] $1" >&2; }

env_file_value() {
    local key="$1" line name value
    [ -f "$PROJECT_DIR/.env" ] || return 0
    while IFS= read -r line || [ -n "$line" ]; do
        line="${line%$'\r'}"
        [[ -z "$line" || "$line" == \#* ]] && continue
        name="${line%%=*}"
        [ "$name" = "$line" ] && continue
        name="${name//[[:space:]]/}"
        [ "$name" = "$key" ] || continue
        value="${line#*=}"
        value="${value#\"}"
        value="${value%\"}"
        value="${value#\'}"
        value="${value%\'}"
        printf '%s' "$value"
        return 0
    done < "$PROJECT_DIR/.env"
    return 0
}

value_for() {
    local key="$1" value
    value="${!key-}"
    if [ -n "$value" ]; then
        printf '%s' "$value"
        return 0
    fi
    env_file_value "$key"
}

load_allowed_env() {
    local key value
    for key in \
        SHAREO_INTERNAL_TOKEN SHAREO_DB_PASSWORD SHAREO_JWT_SECRET \
        SHAREO_MINIO_ACCESS_KEY SHAREO_MINIO_SECRET_KEY SHAREO_REDIS_PASSWORD \
        SHAREO_TRUSTED_ORIGINS SHAREO_AI_BASE_URL SHAREO_AI_REDIS_URL \
        SHAREO_AI_DATABASE_URL SHAREO_AI_GO_BASE_URL SHAREO_AI_INTERNAL_TOKEN \
        SHAREO_DB_USER SHAREO_DB_PASSWORD SHAREO_DB_SSLMODE SHAREO_DB_TIMEZONE \
        SHAREO_AI_DB_USER SHAREO_AI_DB_PASSWORD \
        SHAREO_PG_HOST SHAREO_PG_PORT SHAREO_PG_DATABASE SHAREO_PG_ADMIN_USER SHAREO_PG_ADMIN_PASSWORD \
        SHAREO_REDIS_PORT SHAREO_MINIO_ENDPOINT SHAREO_MINIO_BUCKET \
        SHAREO_TEST_USERNAME SHAREO_TEST_PASSWORD \
        SHAREO_IMAGE_SEARCH_SCORE_THRESHOLD \
        SHAREO_AI_MODEL_CACHE_DIR SHAREO_AI_TEXT_MODEL_CACHE_DIR SHAREO_AI_EMBEDDING_REVISION \
        SHAREO_AI_HF_ENDPOINT SHAREO_AI_LLM_BASE_URL SHAREO_AI_LLM_API_KEY \
        SHAREO_AI_LLM_MODEL SHAREO_AI_LLM_TIMEOUT_SECONDS SHAREO_AI_AGENT_ENABLED \
        SHAREO_AI_AGENT_MAX_ROUNDS SHAREO_AI_AGENT_MAX_TOOL_CALLS \
        SHAREO_AI_AGENT_MAX_PARALLEL_TOOLS SHAREO_AI_AGENT_MAX_OBSERVATION_CHARS \
        SHAREO_AI_AGENT_TRACE_VERSION \
        SHAREO_MINIO_DATA_DIR SHAREO_RUNTIME HTTP_PROXY HTTPS_PROXY ALL_PROXY NO_PROXY \
        http_proxy https_proxy all_proxy no_proxy HF_ENDPOINT HF_HOME HF_HUB_OFFLINE \
        TRANSFORMERS_OFFLINE; do
        value="$(value_for "$key")"
        if [ -n "$value" ]; then
            export "$key=$value"
        fi
    done
}

resolve_path() {
    local path="$1"
    case "$path" in
        /*) printf '%s' "$path" ;;
        *) printf '%s/%s' "$PROJECT_DIR" "$path" ;;
    esac
}

pid_alive() {
    local pid_file="$1" pid
    [ -f "$pid_file" ] || return 1
    pid="$(sed -n '1p' "$pid_file" 2>/dev/null || true)"
    [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

stop_pid_file() {
    local pid_file="$1" label pid
    label="$2"
    [ -f "$pid_file" ] || return 0
    pid="$(sed -n '1p' "$pid_file" 2>/dev/null || true)"
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
        kill "$pid" 2>/dev/null || true
        for _ in $(seq 1 20); do
            kill -0 "$pid" 2>/dev/null || break
            sleep 0.2
        done
        if kill -0 "$pid" 2>/dev/null; then
            warn "$label did not exit after SIGTERM; leaving it for manual cleanup (pid=$pid)"
        else
            pass "$label stopped"
        fi
    fi
    rm -f -- "$pid_file"
}

url_status() {
    local url="$1"
    curl --noproxy '*' --silent --show-error --max-time 4 -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true
}

wait_url() {
    local label="$1" url="$2" attempts="${3:-60}" status
    for _ in $(seq 1 "$attempts"); do
        status="$(url_status "$url")"
        if [ "$status" = "200" ]; then
            pass "$label"
            return 0
        fi
        sleep 1
    done
    fail "$label (last HTTP status: $status)"
    return 1
}

port_open() {
    local host="$1" port="$2"
    if command -v nc >/dev/null 2>&1; then
        nc -z -w 2 "$host" "$port" >/dev/null 2>&1
        return
    fi
    if command -v lsof >/dev/null 2>&1; then
        lsof -nP -iTCP:"$port" -sTCP:LISTEN 2>/dev/null | grep LISTEN >/dev/null
        return
    fi
    curl --noproxy '*' --silent --show-error --max-time 2 "http://$host:$port" -o /dev/null >/dev/null 2>&1
}

port_owner() {
    local port="$1" owner
    if command -v lsof >/dev/null 2>&1; then
        owner="$(lsof -nP -iTCP:"$port" -sTCP:LISTEN 2>/dev/null |
            awk 'NR == 2 { printf "%s (pid=%s)", $1, $2; exit }')"
        if [ -n "$owner" ]; then
            printf '%s' "$owner"
            return 0
        fi
    fi
    printf 'unknown process'
}

report_unhealthy_port() {
    local label="$1" port="$2" url="$3" owner
    owner="$(port_owner "$port")"
    fail "$label port $port is occupied but $url is not healthy (owner: $owner); stop the conflicting process or choose another port"
    return 1
}

discover_proxy() {
    local configured port candidate
    proxy_reachable() {
        local proxy="$1" endpoint status
        endpoint="$(value_for SHAREO_AI_HF_ENDPOINT)"
        endpoint="${endpoint:-https://huggingface.co}"
        status="$(curl --proxy "$proxy" --noproxy '' --silent --show-error --max-time 5 \
            -o /dev/null -w '%{http_code}' "$endpoint" 2>/dev/null || true)"
        case "$status" in
            2*|3*) return 0 ;;
            *) return 1 ;;
        esac
    }

    if command -v lsof >/dev/null 2>&1; then
        for port in 7890 7891; do
            if lsof -nP -iTCP:"$port" -sTCP:LISTEN 2>/dev/null | grep LISTEN >/dev/null; then
                candidate="http://127.0.0.1:$port"
                if proxy_reachable "$candidate"; then
                    printf '%s' "$candidate"
                    return 0
                fi
            fi
        done
    fi
    for configured in \
        "$(value_for SHAREO_AI_HTTPS_PROXY)" \
        "$(value_for HTTPS_PROXY)" \
        "$(value_for SHAREO_AI_HTTP_PROXY)" \
        "$(value_for HTTP_PROXY)"; do
        if [ -n "$configured" ] && proxy_reachable "$configured"; then
            printf '%s' "$configured"
            return 0
        fi
    done
}

proxy_check() {
    local proxy
    proxy="$(discover_proxy)"
    if [ -z "$proxy" ]; then
        warn "no local HTTP proxy detected; cached dependencies are still usable"
        return 0
    fi
    pass "proxy reaches Hugging Face endpoint (listener configured)"
}

start_brew_service() {
    local service="$1"
    if ! command -v brew >/dev/null 2>&1; then
        fail "Homebrew is not installed"
        return 1
    fi
    if brew services start "$service" >/dev/null 2>&1; then
        pass "Homebrew service requested: $service"
        return 0
    fi
    warn "could not request Homebrew service: $service"
    return 1
}

start_direct_minio() {
    local pid_file data_dir minio_bin user secret
    pid_file="$STATE_DIR/minio.pid"
    if pid_alive "$pid_file"; then
        wait_url "MinIO started by ShareO" "http://127.0.0.1:9000/minio/health/live" 20
        return
    fi
    minio_bin="$(command -v minio || true)"
    [ -n "$minio_bin" ] || { fail "MinIO binary is unavailable"; return 1; }
    data_dir="$(value_for SHAREO_MINIO_DATA_DIR)"
    data_dir="${data_dir:-.local/shareo/minio}"
    data_dir="$(resolve_path "$data_dir")"
    mkdir -p "$data_dir"
    user="$(value_for SHAREO_MINIO_ACCESS_KEY)"
    secret="$(value_for SHAREO_MINIO_SECRET_KEY)"
    user="${user:-minioadmin}"
    secret="${secret:-minioadmin}"
    (
        cd "$PROJECT_DIR"
        MINIO_ROOT_USER="$user" MINIO_ROOT_PASSWORD="$secret" \
            nohup "$minio_bin" server "$data_dir" --address 127.0.0.1:9000 \
            --console-address 127.0.0.1:9001 >"$STATE_DIR/minio.log" 2>&1 < /dev/null &
        printf '%s\n' "$!" > "$pid_file"
    )
    wait_url "MinIO started with local data directory" "http://127.0.0.1:9000/minio/health/live" 30
}

check_postgres() {
    local port
    port="$(value_for SHAREO_PG_PORT)"
    port="${port:-5432}"
    if command -v pg_isready >/dev/null 2>&1 && pg_isready -h 127.0.0.1 -p "$port" >/dev/null 2>&1; then
        pass "PostgreSQL 17 responds on 127.0.0.1:$port"
        return 0
    fi
    if port_open 127.0.0.1 "$port"; then
        pass "PostgreSQL port is open on 127.0.0.1:$port (credentials not printed)"
        return 0
    fi
    fail "PostgreSQL is not reachable on 127.0.0.1:$port"
    return 1
}

check_redis() {
    local port
    port="$(value_for SHAREO_REDIS_PORT)"
    port="${port:-6379}"
    if command -v redis-cli >/dev/null 2>&1 && [ "$(redis-cli -h 127.0.0.1 -p "$port" ping 2>/dev/null || true)" = "PONG" ]; then
        pass "Redis responds on 127.0.0.1:$port"
        return 0
    fi
    fail "Redis is not reachable on 127.0.0.1:$port"
    return 1
}

check_minio() {
    wait_url "MinIO health" "http://127.0.0.1:9000/minio/health/live" 2
}

start_infra() {
    local minio_ready=0
    load_allowed_env
    start_brew_service postgresql@17 || true
    start_brew_service redis || true
    start_brew_service minio || true
    for _ in $(seq 1 30); do check_postgres && break || sleep 1; done
    check_postgres
    for _ in $(seq 1 30); do check_redis && break || sleep 1; done
    check_redis
    for _ in $(seq 1 5); do
        if [ "$(url_status http://127.0.0.1:9000/minio/health/live)" = "200" ]; then
            minio_ready=1
            pass "MinIO already healthy on 127.0.0.1:9000"
            break
        fi
        sleep 1
    done
    if [ "$minio_ready" -eq 0 ] && port_open 127.0.0.1 9000; then
        report_unhealthy_port "MinIO" 9000 "http://127.0.0.1:9000/minio/health/live"
        return 1
    fi
    if [ "$minio_ready" -eq 0 ]; then
        start_direct_minio
    fi
    if [ -n "$(value_for SHAREO_DB_PASSWORD)" ] && [ -n "$(value_for SHAREO_AI_DB_PASSWORD)" ]; then
        "$PROJECT_DIR/scripts/bootstrap_postgres.sh"
    else
        fail "SHAREO_DB_PASSWORD and SHAREO_AI_DB_PASSWORD are required before starting ShareO"
        return 1
    fi
    pass "native infrastructure is ready"
}

stop_infra() {
    stop_pid_file "$STATE_DIR/minio.pid" MinIO
    if [ "${SHAREO_STOP_BREW_SERVICES:-0}" = "1" ]; then
        brew services stop postgresql@17 >/dev/null 2>&1 || true
        brew services stop redis >/dev/null 2>&1 || true
        brew services stop minio >/dev/null 2>&1 || true
        pass "Homebrew services stopped by explicit SHAREO_STOP_BREW_SERVICES=1"
    else
        warn "Homebrew services were left running; set SHAREO_STOP_BREW_SERVICES=1 to stop them"
    fi
}

start_local_processes() {
    local app_pid="$STATE_DIR/app.pid" ai_pid="$STATE_DIR/ai.pid"
    local app_port ai_port app_url ai_url app_ready ai_ready
    local config_path model_dir proxy no_proxy cache_root image_snapshot
    load_allowed_env
    app_port="$(value_for SHAREO_APP_PORT)"; app_port="${app_port:-8080}"
    ai_port="$(value_for SHAREO_AI_PORT)"; ai_port="${ai_port:-8000}"
    app_url="http://127.0.0.1:$app_port/healthz"
    ai_url="http://127.0.0.1:$ai_port/healthz"
    app_ready=0
    ai_ready=0
    config_path="$(value_for SHAREO_CONFIG)"; config_path="${config_path:-$PROJECT_DIR/config.yaml}"
    config_path="$(resolve_path "$config_path")"
    [ -f "$config_path" ] || { fail "Go config is missing: $config_path"; return 1; }

    # Keep an older ignored config.yaml from selecting the retired MySQL
    # endpoint. These defaults are also used by bootstrap_postgres.sh and can
    # still be overridden explicitly through SHAREO_PG_* / SHAREO_DB_USER.
    export SHAREO_PG_HOST="${SHAREO_PG_HOST:-127.0.0.1}"
    export SHAREO_PG_PORT="${SHAREO_PG_PORT:-5432}"
    export SHAREO_PG_DATABASE="${SHAREO_PG_DATABASE:-shareo}"
    export SHAREO_DB_USER="${SHAREO_DB_USER:-shareo_app}"

    # A TCP listener is not enough to reuse a service: an unrelated application
    # can leave a port open while the expected HTTP endpoint is unavailable.
    if [ "$(url_status "$app_url")" = "200" ]; then
        app_ready=1
    elif port_open 127.0.0.1 "$app_port"; then
        report_unhealthy_port "Go app" "$app_port" "$app_url"
        return 1
    fi
    if [ "$(url_status "$ai_url")" = "200" ]; then
        ai_ready=1
    elif port_open 127.0.0.1 "$ai_port"; then
        report_unhealthy_port "AI service" "$ai_port" "$ai_url"
        return 1
    fi

    model_dir="$(value_for SHAREO_AI_MODEL_CACHE_DIR)"
    if [ -z "$model_dir" ]; then
        if [ -d "${HOME:-}/.cache/shareo/models" ] && find -L "${HOME:-}/.cache/shareo/models" \
            -type f \( -name 'config.json' -o -name 'preprocessor_config.json' \) \
            -print -quit 2>/dev/null | grep -q .; then
            model_dir="${HOME}/.cache/shareo/models"
            pass "using complete host model cache"
        else
            model_dir=".cache/shareo/models"
        fi
    fi
    model_dir="${model_dir}"
    model_dir="$(resolve_path "$model_dir")"
    mkdir -p "$model_dir"
    image_snapshot="$model_dir/models--OFA-Sys--chinese-clip-vit-base-patch16/snapshots/${SHAREO_AI_EMBEDDING_REVISION:-36e679e65c2a2fead755ae21162091293ad37834}"
    cache_root="$PROJECT_DIR/.cache/shareo"
    mkdir -p "$cache_root/go-build" "$cache_root/go-mod" "$cache_root/uv"
    proxy="$(discover_proxy)"
    no_proxy="$(value_for NO_PROXY)"
    no_proxy="${no_proxy:-127.0.0.1,localhost,::1}"
    export NO_PROXY="$no_proxy"
    export no_proxy="$no_proxy"
    if [ -n "$proxy" ]; then
        export HTTP_PROXY="${HTTP_PROXY:-$proxy}"
        export HTTPS_PROXY="${HTTPS_PROXY:-$proxy}"
        export ALL_PROXY="${ALL_PROXY:-$proxy}"
    fi
    export SHAREO_CONFIG="$config_path"
    export SHAREO_APP_PORT="$app_port"
    export SHAREO_AI_BASE_URL="${SHAREO_AI_BASE_URL:-http://127.0.0.1:$ai_port}"
    export SHAREO_AI_REDIS_URL="${SHAREO_AI_REDIS_URL:-redis://127.0.0.1:6379/0}"
    export SHAREO_AI_DATABASE_URL="${SHAREO_AI_DATABASE_URL:-postgresql://${SHAREO_AI_DB_USER:-shareo_ai}@127.0.0.1:5432/${SHAREO_PG_DATABASE:-shareo}?sslmode=disable}"
    export SHAREO_AI_GO_BASE_URL="${SHAREO_AI_GO_BASE_URL:-http://127.0.0.1:$app_port}"
    export SHAREO_AI_MODEL_CACHE_DIR="$model_dir"
    export SHAREO_AI_TEXT_MODEL_CACHE_DIR="${SHAREO_AI_TEXT_MODEL_CACHE_DIR:-$model_dir}"
    export SHAREO_AI_INTERNAL_TOKEN="${SHAREO_AI_INTERNAL_TOKEN:-${SHAREO_INTERNAL_TOKEN:-shareo-local-internal}}"
    export SHAREO_INTERNAL_TOKEN="${SHAREO_INTERNAL_TOKEN:-shareo-local-internal}"
    export HF_ENDPOINT="${HF_ENDPOINT:-${SHAREO_AI_HF_ENDPOINT:-https://huggingface.co}}"
    export HF_HOME="${HF_HOME:-$model_dir}"
    export GOCACHE="${GOCACHE:-$cache_root/go-build}"
    export GOMODCACHE="${GOMODCACHE:-$cache_root/go-mod}"
    export UV_CACHE_DIR="${UV_CACHE_DIR:-$cache_root/uv}"
    if [ -f "$image_snapshot/preprocessor_config.json" ] && \
        [ -f "$image_snapshot/config.json" ] && [ -f "$image_snapshot/pytorch_model.bin" ]; then
        export HF_HUB_OFFLINE=1
        export TRANSFORMERS_OFFLINE=1
        pass "complete image model cache found; offline model loading enabled"
    fi

    if [ "$app_ready" -eq 0 ]; then
        go build -o "$STATE_DIR/shareo-app" ./cmd/server
        nohup "$STATE_DIR/shareo-app" >"$STATE_DIR/app.log" 2>&1 < /dev/null &
        printf '%s\n' "$!" > "$app_pid"
    else
        pass "Go app already healthy on 127.0.0.1:$app_port"
    fi
    if [ "$ai_ready" -eq 0 ]; then
        (
            cd "$PROJECT_DIR/ai-service"
            exec uv run --frozen uvicorn app.main:app --host 127.0.0.1 --port "$ai_port"
        ) >"$STATE_DIR/ai.log" 2>&1 < /dev/null &
        printf '%s\n' "$!" > "$ai_pid"
    else
        pass "AI service already healthy on 127.0.0.1:$ai_port"
    fi
    if [ "$app_ready" -eq 0 ]; then
        wait_url "Go app health" "$app_url" 45
    fi
    if [ "$ai_ready" -eq 0 ]; then
        wait_url "AI service health" "$ai_url" 45
    fi
    pass "native Go and AI processes started; run make warm-ai for model readiness"
}

doctor() {
    local status=0 cache_dir proxy app_port ai_port
    load_allowed_env
    command -v brew >/dev/null 2>&1 && pass "Homebrew available" || { fail "Homebrew unavailable"; status=1; }
    command -v go >/dev/null 2>&1 && pass "Go available" || { fail "Go unavailable"; status=1; }
    command -v uv >/dev/null 2>&1 && pass "uv available" || { fail "uv unavailable"; status=1; }
    command -v python3 >/dev/null 2>&1 && pass "Python available" || { fail "Python unavailable"; status=1; }
    cache_dir="$(value_for SHAREO_AI_MODEL_CACHE_DIR)"; cache_dir="${cache_dir:-.cache/shareo/models}"
    cache_dir="$(resolve_path "$cache_dir")"
    if [ -d "$cache_dir" ] && find -L "$cache_dir" -type f -print -quit 2>/dev/null | grep -q .; then
        pass "local model cache contains files"
    else
        warn "local model cache is empty: $cache_dir"
    fi
    check_postgres || true
    check_redis || true
    check_minio || true
    app_port="$(value_for SHAREO_APP_PORT)"; app_port="${app_port:-8080}"
    ai_port="$(value_for SHAREO_AI_PORT)"; ai_port="${ai_port:-8000}"
    if [ "$(url_status "http://127.0.0.1:$app_port/healthz")" = "200" ]; then
        pass "Go app healthz"
    elif port_open 127.0.0.1 "$app_port"; then
        warn "Go app port $app_port is occupied by $(port_owner "$app_port") but healthz is unavailable"
    else
        warn "Go app is not running"
    fi
    if [ "$(url_status "http://127.0.0.1:$ai_port/healthz")" = "200" ]; then
        pass "AI service healthz"
    elif port_open 127.0.0.1 "$ai_port"; then
        warn "AI service port $ai_port is occupied by $(port_owner "$ai_port") but healthz is unavailable"
    else
        warn "AI service is not running"
    fi
    proxy="$(discover_proxy)"
    [ -n "$proxy" ] && pass "local proxy detected (address withheld)" || warn "no local proxy listener detected"
    proxy_check
    echo "[INFO] native state directory: $STATE_DIR"
    return "$status"
}

stop_local() {
    stop_pid_file "$STATE_DIR/app.pid" "Go app"
    stop_pid_file "$STATE_DIR/ai.pid" "AI service"
    stop_pid_file "$STATE_DIR/minio.pid" MinIO
}

stop_local_apps() {
    stop_pid_file "$STATE_DIR/app.pid" "Go app"
    stop_pid_file "$STATE_DIR/ai.pid" "AI service"
}

main() {
    local action="${1:-doctor}"
    cd "$PROJECT_DIR"
    case "$action" in
        doctor) doctor ;;
        infra-up) start_infra ;;
        infra-down) stop_infra ;;
        dev-local) start_infra; start_local_processes ;;
        app-stop) stop_local_apps ;;
        local-stop|stop) stop_local ;;
        *) echo "usage: $0 {doctor|infra-up|infra-down|dev-local|app-stop|local-stop}" >&2; return 2 ;;
    esac
}

main "$@"
