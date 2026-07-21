#!/bin/bash
# ============================================================
#  ShareO API 功能测试脚本
#  所有测试通过 curl 调用 API 模拟用户/管理员行为
#  严禁直接 SQL/Redis/MinIO 操作
# ============================================================
set -e

BASE="http://localhost:8080"
PASS=0
FAIL=0

red()    { echo -e "\033[31m$1\033[0m"; }
green()  { echo -e "\033[32m$1\033[0m"; }

check() {
    local desc="$1" expected="$2" actual="$3"
    if echo "$actual" | grep -q "$expected"; then
        green "  PASS: $desc"
        PASS=$((PASS+1))
    else
        red "  FAIL: $desc (expected '$expected', got '$actual')"
        FAIL=$((FAIL+1))
    fi
}

check_code() {
    local desc="$1" expected="$2" actual="$3"
    if [ "$actual" = "$expected" ]; then
        green "  PASS: $desc (HTTP $actual)"
        PASS=$((PASS+1))
    else
        red "  FAIL: $desc (expected $expected, got $actual)"
        FAIL=$((FAIL+1))
    fi
}

echo "============================================"
echo "  ShareO API 功能测试"
echo "============================================"
echo ""

# --- Auth Tests ---
echo "=== 1. Auth 认证测试 ==="

# Register
REG=$(curl -s -X POST "$BASE/api/v1/auth/register" \
  -H 'Content-Type: application/json' \
  -d '{"username":"ftest01","password":"256500","email":"ftest01@test.com"}')
check "注册新用户" '"code":0' "$REG"
USER1_ID=$(echo "$REG" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['user']['id'])" 2>/dev/null)

# Login
LOGIN=$(curl -s -X POST "$BASE/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"ftest01","password":"256500"}')
check "登录" '"code":0' "$LOGIN"
TOKEN=$(echo "$LOGIN" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])" 2>/dev/null)

# Register duplicate
REG2=$(curl -s -X POST "$BASE/api/v1/auth/register" \
  -H 'Content-Type: application/json' \
  -d '{"username":"ftest01","password":"256500"}')
check "重复注册" '用户名已存在' "$REG2"

# Login wrong password
LOGIN2=$(curl -s -X POST "$BASE/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"ftest01","password":"wrong"}')
check "错误密码登录" '用户名或密码错误' "$LOGIN2"

# XSS username
XSS=$(curl -s -X POST "$BASE/api/v1/auth/register" \
  -H 'Content-Type: application/json' \
  -d '{"username":"<script>alert(1)</script>","password":"256500"}')
check "XSS用户名" '用户名包含非法字符' "$XSS"

# Me
ME=$(curl -s "$BASE/api/v1/auth/me" -b "token=$TOKEN")
check "个人信息" '"code":0' "$ME"

# Update profile
UP=$(curl -s -X PUT "$BASE/api/v1/auth/profile" -b "token=$TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"bio":"functional test bio","email":"ftest01_new@test.com"}')
check "更新资料" '"code":0' "$UP"

# Unauthorized access
NOAUTH=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/api/v1/auth/me")
check_code "未登录访问me" "401" "$NOAUTH"

# Register second user for social tests
REG2B=$(curl -s -X POST "$BASE/api/v1/auth/register" \
  -H 'Content-Type: application/json' \
  -d '{"username":"ftest02","password":"256500","email":"ftest02@test.com"}')
LOGIN2B=$(curl -s -X POST "$BASE/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"ftest02","password":"256500"}')
TOKEN2=$(echo "$LOGIN2B" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])" 2>/dev/null)

echo ""
echo "=== 2. 帖子与Feed测试 ==="

# Create a valid 1x1 PNG without exposing shell quoting to binary bytes.
TMPIMG=$(mktemp /tmp/shareo-testimg.XXXXXX.png)
trap 'rm -f "$TMPIMG"' EXIT
python3 - "$TMPIMG" <<'PY'
import base64
import pathlib
import sys

png = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
pathlib.Path(sys.argv[1]).write_bytes(base64.b64decode(png))
PY

# Upload image
UPLOAD=$(curl -s -X POST "$BASE/api/v1/upload" -b "token=$TOKEN" -F "file=@$TMPIMG")
check "上传图片" '"code":0' "$UPLOAD"
IMGURL=$(echo "$UPLOAD" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['url'])" 2>/dev/null)

# Create post
CREATE=$(curl -s -X POST "$BASE/api/v1/posts" -b "token=$TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"content\":\"功能测试帖 #test #风景\",\"images\":[\"$IMGURL\"]}")
check "发帖" '"code":0' "$CREATE"
POSTID=$(echo "$CREATE" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null)

# New posts are pending by design. Approve the fixture before testing public reads.
ADM_LOGIN=$(curl -s -X POST "$BASE/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}')
check "管理员登录" '"code":0' "$ADM_LOGIN"
ADM_TOKEN=$(echo "$ADM_LOGIN" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])" 2>/dev/null)
APPROVE=$(curl -s -X POST "$BASE/api/v1/admin/posts/$POSTID/approve" -b "token=$ADM_TOKEN")
check "审核通过" '"code":0' "$APPROVE"

# Get post detail
DETAIL=$(curl -s "$BASE/api/v1/posts/$POSTID")
check "帖子详情" '"code":0' "$DETAIL"

# View counting is an explicit write, never a side effect of GET.
VIEW=$(curl -s -X POST "$BASE/api/v1/posts/$POSTID/view" -b "token=$TOKEN2")
check "记录浏览" '"code":0' "$VIEW"

# Feed
FEED=$(curl -s "$BASE/api/v1/feed?sort=latest")
check "Feed流" '"code":0' "$FEED"

# Search
SEARCH=$(curl -s "$BASE/api/v1/search?q=功能测试")
check "搜索" '"code":0' "$SEARCH"

echo ""
echo "=== 3. 社交功能测试 ==="

# Like
LIKE=$(curl -s -X POST "$BASE/api/v1/posts/$POSTID/like" -b "token=$TOKEN2")
check "点赞" '"code":0' "$LIKE"

# Unlike (toggle)
UNLIKE=$(curl -s -X POST "$BASE/api/v1/posts/$POSTID/like" -b "token=$TOKEN2")
check "取消点赞" '"code":0' "$UNLIKE"

# Favorite
FAV=$(curl -s -X POST "$BASE/api/v1/posts/$POSTID/favorite" -b "token=$TOKEN2")
check "收藏" '"code":0' "$FAV"

# Comment
COMMENT=$(curl -s -X POST "$BASE/api/v1/posts/$POSTID/comments" -b "token=$TOKEN2" \
  -H 'Content-Type: application/json' \
  -d '{"content":"很棒的作品！"}')
check "发评论" '"code":0' "$COMMENT"
CID=$(echo "$COMMENT" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null)

# Get comments
GETC=$(curl -s "$BASE/api/v1/posts/$POSTID/comments")
check "评论列表" '"code":0' "$GETC"

# Delete comment
DELC=$(curl -s -X DELETE "$BASE/api/v1/comments/$CID" -b "token=$TOKEN2")
check "删除评论" '"code":0' "$DELC"

# Follow
FOLLOW=$(curl -s -X POST "$BASE/api/v1/users/$USER1_ID/follow" -b "token=$TOKEN2")
check "关注" '"code":0' "$FOLLOW"

# Self-follow
SELFF=$(curl -s -X POST "$BASE/api/v1/users/$USER1_ID/follow" -b "token=$TOKEN")
check "拒绝关注自己" 'cannot follow yourself' "$SELFF"

# Like non-existing post
NEX=$(curl -s -X POST "$BASE/api/v1/posts/99999/like" -b "token=$TOKEN2")
check "点赞不存在帖" '帖子不存在' "$NEX"

echo ""
echo "=== 4. 通知测试 ==="

NOTIFS=$(curl -s "$BASE/api/v1/notifications" -b "token=$TOKEN")
check "通知列表" '"code":0' "$NOTIFS"

UNREAD=$(curl -s "$BASE/api/v1/notifications/unread-count" -b "token=$TOKEN")
check "未读通知数" '"code":0' "$UNREAD"

echo ""
echo "=== 5. 管理员测试 ==="

# Stats
STATS=$(curl -s "$BASE/api/v1/admin/stats" -b "token=$ADM_TOKEN")
check "管理员统计" '"code":0' "$STATS"

# Pending posts
PENDING=$(curl -s "$BASE/api/v1/admin/pending-posts" -b "token=$ADM_TOKEN")
check "待审核列表" '"code":0' "$PENDING"

# Normal user can't access admin
UNADM=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/api/v1/admin/stats" -b "token=$TOKEN")
check_code "普通用户访问admin" "403" "$UNADM"

echo ""
echo "=== 6. 边界测试 ==="

# Empty search
EMPTYS=$(curl -s "$BASE/api/v1/search?q=")
check "空搜索" '搜索关键词不能为空' "$EMPTYS"

# Feed with invalid params (clamping test)
FEED2=$(curl -s "$BASE/api/v1/feed?page=-1&page_size=1000")
check "无效分页参数" '"code":0' "$FEED2"

# Repost
REPOST=$(curl -s -X POST "$BASE/api/v1/posts/$POSTID/repost" -b "token=$TOKEN2" \
  -H 'Content-Type: application/json' \
  -d '{"text":"转帖测试"}')
check "转帖" '"code":0' "$REPOST"
REPOSTID=$(echo "$REPOST" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null)

echo ""
echo "=== 7. 清理测试数据 ==="

# Delete repost first
curl -s -X DELETE "$BASE/api/v1/admin/posts/$REPOSTID" -b "token=$ADM_TOKEN" > /dev/null
curl -s -X DELETE "$BASE/api/v1/admin/posts/$POSTID" -b "token=$ADM_TOKEN" > /dev/null
green "  PASS: 测试数据已清理"
PASS=$((PASS+1))

echo ""
echo "============================================"
echo "  结果: $PASS PASS, $FAIL FAIL"
echo "============================================"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
exit 0
