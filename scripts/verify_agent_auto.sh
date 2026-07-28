#!/usr/bin/env bash
# One-command Phase 8A verification on the current Demo Compose project.
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"
MAKE_CMD="${MAKE:-make}"
REUSE_CURRENT="${SHAREO_VERIFY_REUSE_CURRENT:-0}"
E2E_APP_IMAGE="${SHAREO_E2E_APP_IMAGE:-shareo-app:latest}"
E2E_AI_IMAGE="${SHAREO_E2E_AI_IMAGE:-shareo-ai-service:latest}"
MODEL_CACHE_VOLUME="${SHAREO_E2E_MODEL_CACHE_VOLUME:-shareo_hf_cache}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
LOG_DIR="$PROJECT_DIR/docs/evidence/phase8-agent/auto"
LOG_FILE="$LOG_DIR/phase8a-${STAMP}.log"
mkdir -p "$LOG_DIR"
exec > >(tee -a "$LOG_FILE") 2>&1

run_stage() {
    local name="$1"
    shift
    echo
    echo "===== START $name ====="
    if "$@"; then
        echo "===== PASS $name ====="
    else
        echo "===== FAIL $name =====" >&2
        echo "脱敏阶段日志：$LOG_FILE" >&2
        echo "请先处理该阶段，再重试 make verify-agent-auto。" >&2
        exit 1
    fi
}

skip_stage() {
    local name="$1"
    echo
    echo "===== SKIP $name ====="
    echo "原因：SHAREO_VERIFY_REUSE_CURRENT=1，保留当前 Demo 容器和数据卷。"
}

run_stage "doctor before rebuild" env SHAREO_DOCTOR_ALLOW_STALE_AI=1 "$MAKE_CMD" doctor
if [ "$REUSE_CURRENT" = "1" ]; then
    skip_stage "rebuild Go app image"
else
    run_stage "rebuild current Demo services" "$MAKE_CMD" up
fi
run_stage "force rebuild and recreate AI service" "$MAKE_CMD" reload-ai
run_stage "doctor after rebuild" "$MAKE_CMD" doctor
run_stage "unit and static checks" "$MAKE_CMD" check
run_stage "base Agent/RAG/image E2E" env \
    SHAREO_E2E_NO_BUILD=1 \
    SHAREO_E2E_APP_IMAGE="$E2E_APP_IMAGE" \
    SHAREO_E2E_AI_IMAGE="$E2E_AI_IMAGE" \
    SHAREO_E2E_MODEL_CACHE_VOLUME="$MODEL_CACHE_VOLUME" \
    SHAREO_AGENT_E2E_NO_BUILD=1 \
    SHAREO_AGENT_E2E_MODEL_CACHE_VOLUME="$MODEL_CACHE_VOLUME" \
    "$MAKE_CMD" test-agent-base
run_stage "current-stack integration" "$MAKE_CMD" test-integration-auto
run_stage "degradation matrix" env \
    SHAREO_DEGRADATION_NO_BUILD=1 \
    SHAREO_DEGRADATION_MODEL_CACHE_VOLUME="$MODEL_CACHE_VOLUME" \
    "$MAKE_CMD" test-degradation
run_stage "AI warmup and readiness" "$MAKE_CMD" warm-ai
run_stage "RAG machine evaluation" "$MAKE_CMD" eval-ai-machine
run_stage "Agent machine evaluation" "$MAKE_CMD" eval-agent-machine

echo
if [ "$REUSE_CURRENT" = "1" ]; then
    echo "===== PARTIAL Phase 8A automatic verification ====="
    echo "AI 源码指纹已核对；Go app 镜像与工作区源码一致性未证明（SKIP）。"
else
    echo "===== PASS Phase 8A automatic verification ====="
fi
echo "脱敏阶段日志：$LOG_FILE"
