#!/bin/bash
# ============================================================
# ShareO v2 结构清理脚本（2026-07-19 评审配套，幂等可重跑）
# 作用: 删除构建产物 / example 冗余 / 归位根目录文档到 docs/
#       / 移出个人照片 / 整理模板目录 / 暂存新增资产
# 跑完后请 git status 检查，再按 README 建议分块提交
# ============================================================
set -e
cd "$(dirname "$0")/.."

echo "[1/4] 构建产物与 example 冗余"
rm -rf bin
[ -f Makefile.example ] && git rm -q Makefile.example || true
[ -f start.sh.example ] && git rm -q start.sh.example || true
# 用户已删除的旧文档，登记删除
for f in CLAUDE.md PROJECT.md TEST_CHECKLIST.md; do
    git ls-files --error-unmatch "$f" >/dev/null 2>&1 && git rm -q "$f" || true
done

echo "[2/4] 根目录文档归位 docs/"
mkdir -p docs/reviews
[ -f FEATURES.md ]    && git mv FEATURES.md docs/features.md || true
[ -f CODE_REVIEW.md ] && git mv CODE_REVIEW.md docs/reviews/2026-06-19-code-review.md || true
[ -f FEEDBACK.md ]    && git mv FEEDBACK.md docs/reviews/2026-06-19-fix-log.md || true

echo "[3/4] 个人照片移出版本库（文件保留在磁盘）+ 脚本与资产整理"
git ls-files resources/ | grep -q . && git rm -rq --cached resources/ || true
rm -f resources/static/pictures/.DS_Store
[ -f test_api.sh ] && mv test_api.sh scripts/test_api.sh || true
git add scripts/ web/static/img/placeholder.svg .gitignore

echo "[4/4] 模板目录整理（模板名不变，LoadHTMLFiles 按 basename 注册）"
[ -f web/templates/404.html ]           && git mv web/templates/404.html web/templates/layout/404.html || true
[ -f web/templates/notifications.html ] && git mv web/templates/notifications.html web/templates/user/notifications.html || true
[ -f web/templates/topic.html ]         && git mv web/templates/topic.html web/templates/feed/topic.html || true

echo "== 验证 =="
go build ./... && go vet ./...
echo "CLEANUP_DONE — 请 git status 检查后分块提交"
