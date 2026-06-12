#!/bin/bash
# ============================================================
#  ShareO 一键启动器
#  自动检测并拉起 MySQL / Redis / MinIO → 编译启动 → 打开浏览器
# ============================================================
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
MINIO_DATA_DIR="$HOME/minio_data"
MINIO_ACCESS_KEY="minioadmin"
MINIO_SECRET_KEY="minioadmin"
MINIO_ADDR=":9000"
MINIO_CONSOLE=":9001"
BROWSER_URL="http://localhost:8080"

log_info()  { echo -e "${CYAN}[INFO]${NC}  $1"; }
log_ok()    { echo -e "${GREEN}[ OK ]${NC}  $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
log_fail()  { echo -e "${RED}[FAIL]${NC}  $1"; }
log_step()  { echo -e "\n${BOLD}${YELLOW}▶ $1${NC}"; }

check_port() {
    lsof -i ":$1" -sTCP:LISTEN -t >/dev/null 2>&1
}

wait_for_port() {
    local port="$1" label="$2" max_wait="${3:-30}"
    log_info "等待 $label 端口 $port ..."
    for i in $(seq 1 $max_wait); do
        if check_port "$port"; then
            log_ok "$label 已就绪 (${i}s)"
            return 0
        fi
        sleep 1
    done
    log_fail "$label 启动超时"
    return 1
}

# ============================================================
# Step 1: MySQL
# ============================================================
log_step "1/4 检查 MySQL"
if check_port 3306; then
    log_ok "MySQL 已在运行 (port 3306)"
else
    log_warn "MySQL 未运行，尝试启动..."
    brew services start mysql@8.0 2>/dev/null || brew services start mysql 2>/dev/null || {
        mysql.server start 2>/dev/null || {
            log_fail "无法自动启动 MySQL，请手动启动: brew services start mysql@8.0"
            exit 1
        }
    }
    wait_for_port 3306 "MySQL" 30 || exit 1
fi

# Verify connectivity
if mysql -u root -p"${MYSQL_PASS}" -e "SELECT 1" >/dev/null 2>&1; then
    log_ok "MySQL 连接验证通过"
else
    log_fail "MySQL 连接失败，请检查 config.yaml 中的密码"
    exit 1
fi

# ============================================================
# Step 2: Redis
# ============================================================
log_step "2/4 检查 Redis"
if check_port 6379; then
    log_ok "Redis 已在运行 (port 6379)"
else
    log_warn "Redis 未运行，尝试启动..."
    brew services start redis 2>/dev/null || {
        redis-server --daemonize yes --port 6379 2>/dev/null || {
            log_fail "无法自动启动 Redis"
            exit 1
        }
    }
    wait_for_port 6379 "Redis" 10 || exit 1
fi

if redis-cli PING >/dev/null 2>&1; then
    log_ok "Redis 连接验证通过 (PONG)"
else
    log_fail "Redis 连接失败"
    exit 1
fi

# ============================================================
# Step 3: MinIO
# ============================================================
log_step "3/4 检查 MinIO"
if check_port 9000; then
    log_ok "MinIO 已在运行 (API:9000, Console:9001)"
else
    log_warn "MinIO 未运行，启动中..."
    mkdir -p "$MINIO_DATA_DIR"
    MINIO_ROOT_USER="$MINIO_ACCESS_KEY" \
    MINIO_ROOT_PASSWORD="$MINIO_SECRET_KEY" \
    minio server "$MINIO_DATA_DIR" --address "$MINIO_ADDR" --console-address "$MINIO_CONSOLE" > /tmp/minio.log 2>&1 &
    wait_for_port 9000 "MinIO API" 15 || exit 1
    log_info "MinIO Console: http://localhost:9001 (minioadmin / minioadmin)"
fi

# ============================================================
# Step 4: Build & Start ShareO
# ============================================================
log_step "4/4 编译并启动 ShareO"

cd "$PROJECT_DIR"

# Kill any existing instance
if check_port 8080; then
    log_warn "端口 8080 被占用，关闭旧进程..."
    lsof -ti :8080 | xargs kill -9 2>/dev/null
    sleep 1
fi

# Clear stale feed cache
redis-cli DEL feed:latest:page1 2>/dev/null || true

log_info "编译 ShareO..."
if go build -o bin/shareo cmd/server/main.go; then
    log_ok "编译成功"
else
    log_fail "编译失败，请检查代码"
    exit 1
fi

log_info "启动 ShareO 服务..."
./bin/shareo &
SHAREO_PID=$!

# Wait for server to be ready
wait_for_port 8080 "ShareO" 10 || {
    log_fail "ShareO 启动失败，查看日志"
    exit 1
}

# Verify health
if curl -s http://localhost:8080/healthz | grep -q "ok"; then
    log_ok "ShareO 服务健康检查通过"
else
    log_warn "健康检查异常，但服务可能仍可用"
fi

# ============================================================
# Done - Open Browser
# ============================================================
echo ""
echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BOLD}  🎉  ShareO 启动成功！${NC}"
echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "  ${CYAN}🌐  前端页面:${NC}  ${BOLD}http://localhost:8080${NC}"
echo -e "  ${CYAN}🔑  管理员:${NC}    ${BOLD}admin / admin123${NC}"
echo -e "  ${CYAN}👤  用户:${NC}      注册后登录"
echo -e "  ${CYAN}🖼️  MinIO:${NC}    http://localhost:9001 (minioadmin)"
echo -e "  ${CYAN}📋  PID:${NC}      $SHAREO_PID"
echo ""
echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Open browser
log_info "正在打开浏览器..."
sleep 1
open "$BROWSER_URL" 2>/dev/null || xdg-open "$BROWSER_URL" 2>/dev/null || log_warn "请手动打开: $BROWSER_URL"

# Keep script alive and show logs
echo ""
log_info "服务运行中 (Ctrl+C 停止所有服务)"
echo ""
wait $SHAREO_PID
