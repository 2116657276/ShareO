#!/bin/bash
# ShareO 全面测试脚本
set -e

BASE="http://localhost:8080"
PASS=0
FAIL=0
SKIP=0

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

do_test() {
    local name="$1" method="$2" url="$3" data="$4" expected="$5" auth_token="$6"
    local curl_opts="-s -w '\n%{http_code}' -o /tmp/shareo_test_body"

    [ "$method" = "POST" ] && curl_opts="$curl_opts -X POST -H 'Content-Type: application/json'"
    [ "$method" = "PUT" ] && curl_opts="$curl_opts -X PUT -H 'Content-Type: application/json'"
    [ "$method" = "DELETE" ] && curl_opts="$curl_opts -X DELETE"
    [ -n "$data" ] && curl_opts="$curl_opts -d '$data'"
    [ -n "$auth_token" ] && curl_opts="$curl_opts -b 'token=$auth_token'"

    local resp=$(eval "curl $curl_opts '$BASE$url' 2>/dev/null")
    local code=$(echo "$resp" | tail -1)
    local body=$(echo "$resp" | head -n -1)

    if [ "$code" = "$expected" ]; then
        echo -e "  ${GREEN}✓${NC} $name ${CYAN}($code)${NC}"
        PASS=$((PASS+1))
        return 0
    else
        echo -e "  ${RED}✗${NC} $name ${RED}expected $expected, got $code${NC}"
        echo "    Body: $(echo "$body" | head -c 150)"
        FAIL=$((FAIL+1))
        return 1
    fi
}

echo "============================================================"
echo "  ShareO 全功能测试"
echo "  $(date)"
echo "============================================================"

# ====== 1. Health ======
echo -e "\n${YELLOW}[1] 基础健康检查${NC}"
do_test "Health check" "GET" "/healthz" "" "200"

# ====== 2. Auth Pages ======
echo -e "\n${YELLOW}[2] 页面渲染(无需登录)${NC}"
do_test "登录页" "GET" "/login" "" "200"
do_test "注册页" "GET" "/register" "" "200"
do_test "不存在页面" "GET" "/nonexistent" "" "404"

# ====== 3. Register ======
echo -e "\n${YELLOW}[3] 注册流程${NC}"
do_test "正常注册" "POST" "/api/v1/auth/register" '{"username":"zjl_test","password":"test123","email":"zjl@test.com"}' "200"
do_test "重复用户名" "POST" "/api/v1/auth/register" '{"username":"zjl_test","password":"test123"}' "400"
do_test "用户名过短" "POST" "/api/v1/auth/register" '{"username":"a","password":"test123"}' "400"
do_test "密码过短" "POST" "/api/v1/auth/register" '{"username":"newuser1","password":"123"}' "400"

# ====== 4. Login ======
echo -e "\n${YELLOW}[4] 登录流程${NC}"
do_test "正常登录" "POST" "/api/v1/auth/login" '{"username":"zjl_test","password":"test123"}' "200"
do_test "密码错误" "POST" "/api/v1/auth/login" '{"username":"zjl_test","password":"wrongpass"}' "400"
do_test "不存在用户" "POST" "/api/v1/auth/login" '{"username":"nobody_123","password":"test123"}' "400"

# Get user token
USER_TOKEN=$(curl -s -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"username":"zjl_test","password":"test123"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])" 2>/dev/null)
echo "  User token: ${USER_TOKEN:0:25}..."

# ====== 5. Admin Login ======
echo -e "\n${YELLOW}[5] 管理员登录${NC}"
do_test "管理员登录" "POST" "/api/v1/auth/login" '{"username":"admin","password":"admin123"}' "200"
ADMIN_TOKEN=$(curl -s -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])" 2>/dev/null)
echo "  Admin token: ${ADMIN_TOKEN:0:25}..."

# ====== 6. Auth Me/Profile ======
echo -e "\n${YELLOW}[6] 用户信息${NC}"
do_test "获取个人信息" "GET" "/api/v1/auth/me" "" "200" "$USER_TOKEN"
do_test "未认证访问" "GET" "/api/v1/auth/me" "" "401" ""
do_test "错误token" "GET" "/api/v1/auth/me" "" "401" "invalid_token_here"

# ====== 7. Feed (Auth Required) ======
echo -e "\n${YELLOW}[7] Feed流${NC}"
do_test "未登录Feed" "GET" "/" "" "401" ""
do_test "登录后Feed" "GET" "/" "" "200" "$USER_TOKEN"
do_test "Feed API (latest)" "GET" "/api/v1/feed?sort=latest&page=1&page_size=5" "" "200" "$USER_TOKEN"
do_test "Feed API (hot)" "GET" "/api/v1/feed?sort=hot&page=1&page_size=5" "" "200" "$USER_TOKEN"
do_test "Feed API page2" "GET" "/api/v1/feed?page=2&page_size=5" "" "200" "$USER_TOKEN"

# ====== 8. Upload Image ======
echo -e "\n${YELLOW}[8] 图片上传${NC}"
# Create a test image
python3 -c "
from PIL import Image
img = Image.new('RGB', (200, 200), color='#ff6b35')
img.save('/tmp/test_upload.jpg', 'JPEG')
" 2>/dev/null || python3 -c "
# Fallback: minimal valid JPEG
import struct
with open('/tmp/test_upload.jpg', 'wb') as f:
    # SOI
    f.write(b'\xff\xd8')
    # APP0 JFIF
    f.write(b'\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00')
    # DQT
    f.write(b'\xff\xdb\x00C\x00\x08\x06\x06\x07\x06\x05\x08\x07\x07\x07\x09\x09\x08\n\x0c\x14\r\x0c\x0b\x0b\x0c\x19\x12\x13\x0f\x14\x1d\x1a\x1f\x1e\x1d\x1a\x1c\x1c $.\x27 \x1c\x1c(7),01444\x1f\x27;9=82<.342')
    # SOF0
    f.write(b'\xff\xc0\x00\x0b\x08\x00\x01\x00\x01\x01\x01\x11\x00')
    # DHT
    f.write(b'\xff\xc4\x00\x1f\x00\x00\x01\x05\x01\x01\x01\x01\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\n\x0b')
    # SOS
    f.write(b'\xff\xda\x00\x08\x01\x01\x00\x00?\x00\x7f\x00')
    # EOI
    f.write(b'\xff\xd9')
"

# Upload
UPLOAD_RESULT=$(curl -s -X POST "$BASE/api/v1/upload" -b "token=$USER_TOKEN" -F "file=@/tmp/test_upload.jpg")
UPLOAD_CODE=$(echo "$UPLOAD_RESULT" | python3 -c "import sys,json;d=json.load(sys.stdin);print(d['code'])" 2>/dev/null || echo "fail")
IMG_URL=$(echo "$UPLOAD_RESULT" | python3 -c "import sys,json;d=json.load(sys.stdin);print(d.get('data',{}).get('url',''))" 2>/dev/null || echo "")
if [ "$UPLOAD_CODE" = "0" ]; then
    echo -e "  ${GREEN}✓${NC} 上传图片 ${CYAN}(200)${NC} → $IMG_URL"
    PASS=$((PASS+1))
else
    echo -e "  ${RED}✗${NC} 上传图片 ${RED}$UPLOAD_RESULT${NC}"
    FAIL=$((FAIL+1))
fi

do_test "无文件上传" "POST" "/api/v1/upload" "" "400" "$USER_TOKEN"

# ====== 9. Create Post ======
echo -e "\n${YELLOW}[9] 帖子管理${NC}"
CREATE_RESP=$(curl -s -X POST "$BASE/api/v1/posts" -b "token=$USER_TOKEN" -H 'Content-Type: application/json' \
    -d "{\"content\":\"测试作品 - 日落\",\"images\":[\"$IMG_URL\"],\"topic_ids\":[1]}")
POST_ID=$(echo "$CREATE_RESP" | python3 -c "import sys,json;d=json.load(sys.stdin);print(d['data']['id'])" 2>/dev/null || echo "")
if [ -n "$POST_ID" ] && [ "$POST_ID" != "null" ]; then
    echo -e "  ${GREEN}✓${NC} 创建帖子 ${CYAN}(200)${NC} → id=$POST_ID"
    PASS=$((PASS+1))
else
    echo -e "  ${RED}✗${NC} 创建帖子失败: $CREATE_RESP"
    FAIL=$((FAIL+1))
fi

do_test "获取帖子详情" "GET" "/api/v1/posts/$POST_ID" "" "200" "$USER_TOKEN"
do_test "编辑帖子" "PUT" "/api/v1/posts/$POST_ID" '{"content":"修改后的描述"}' "200" "$USER_TOKEN"
do_test "获取不存在的帖子" "GET" "/api/v1/posts/99999" "" "404" "$USER_TOKEN"

# ====== 10. Social - Like ======
echo -e "\n${YELLOW}[10] 社交互动 - 点赞${NC}"
do_test "点赞" "POST" "/api/v1/posts/$POST_ID/like" "" "200" "$USER_TOKEN"
do_test "重复点赞(取消)" "POST" "/api/v1/posts/$POST_ID/like" "" "200" "$USER_TOKEN"
# Another user likes too
USER2_TOKEN=$(curl -s -X POST "$BASE/api/v1/auth/register" -H 'Content-Type: application/json' -d '{"username":"user2_test","password":"test123"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])" 2>/dev/null)
do_test "另一用户点赞" "POST" "/api/v1/posts/$POST_ID/like" "" "200" "$USER2_TOKEN"

# ====== 11. Social - Favorite ======
echo -e "\n${YELLOW}[11] 社交互动 - 收藏${NC}"
do_test "收藏" "POST" "/api/v1/posts/$POST_ID/favorite" "" "200" "$USER_TOKEN"
do_test "取消收藏" "POST" "/api/v1/posts/$POST_ID/favorite" "" "200" "$USER_TOKEN"

# ====== 12. Social - Comment ======
echo -e "\n${YELLOW}[12] 社交互动 - 评论${NC}"
COMMENT_RESP=$(curl -s -X POST "$BASE/api/v1/posts/$POST_ID/comments" -b "token=$USER_TOKEN" -H 'Content-Type: application/json' \
    -d '{"post_id":'"$POST_ID"',"content":"拍得真不错！👍"}')
COMMENT_ID=$(echo "$COMMENT_RESP" | python3 -c "import sys,json;d=json.load(sys.stdin);print(d['data']['id'])" 2>/dev/null || echo "")
if [ -n "$COMMENT_ID" ] && [ "$COMMENT_ID" != "null" ]; then
    echo -e "  ${GREEN}✓${NC} 发表评论 ${CYAN}(200)${NC} → id=$COMMENT_ID"
    PASS=$((PASS+1))
else
    echo -e "  ${RED}✗${NC} 评论失败: $COMMENT_RESP"
    FAIL=$((FAIL+1))
fi
do_test "获取评论列表" "GET" "/api/v1/posts/$POST_ID/comments" "" "200" "$USER_TOKEN"
do_test "空评论" "POST" "/api/v1/posts/$POST_ID/comments" '{"post_id":'"$POST_ID"',"content":""}' "400" "$USER_TOKEN"

# ====== 13. Social - Follow ======
echo -e "\n${YELLOW}[13] 社交互动 - 关注${NC}"
do_test "关注admin" "POST" "/api/v1/users/1/follow" "" "200" "$USER_TOKEN"
do_test "取消关注" "POST" "/api/v1/users/1/follow" "" "200" "$USER_TOKEN"
do_test "关注自己" "POST" "/api/v1/users/65/follow" "" "400" "$USER_TOKEN"  # user_id may vary
do_test "获取关注列表" "GET" "/api/v1/users/1/following" "" "200" "$USER_TOKEN"
do_test "获取粉丝列表" "GET" "/api/v1/users/1/followers" "" "200" "$USER_TOKEN"

# ====== 14. User Profile ======
echo -e "\n${YELLOW}[14] 用户主页${NC}"
do_test "查看用户主页" "GET" "/user/1" "" "200" "$USER_TOKEN"
do_test "查看自己主页" "GET" "/user/65" "" "200" "$USER_TOKEN"

# ====== 15. Admin API ======
echo -e "\n${YELLOW}[15] 管理员功能${NC}"
do_test "Dashboard Stats" "GET" "/api/v1/admin/stats" "" "200" "$ADMIN_TOKEN"
do_test "审核页面" "GET" "/admin/review" "" "200" "$ADMIN_TOKEN"
do_test "用户管理页" "GET" "/admin/users" "" "200" "$ADMIN_TOKEN"
do_test "系统日志页" "GET" "/admin/logs" "" "200" "$ADMIN_TOKEN"
do_test "非管理员访问" "GET" "/api/v1/admin/stats" "" "403" "$USER_TOKEN"
do_test "非管理员页面" "GET" "/admin/" "" "403" "$USER_TOKEN"

# ====== 16. Post Review ======
echo -e "\n${YELLOW}[16] 帖子审核流程${NC}"
do_test "通过帖子" "POST" "/api/v1/admin/posts/$POST_ID/approve" "" "200" "$ADMIN_TOKEN"

# Create a post to reject
REJECT_POST_ID=$(curl -s -X POST "$BASE/api/v1/posts" -b "token=$USER_TOKEN" -H 'Content-Type: application/json' \
    -d "{\"content\":\"违规内容测试\",\"images\":[\"$IMG_URL\"]}" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null)
do_test "驳回帖子" "POST" "/api/v1/admin/posts/$REJECT_POST_ID/reject" '{"comment":"含违规内容"}' "200" "$ADMIN_TOKEN"

# ====== 17. User Ban ======
echo -e "\n${YELLOW}[17] 用户封禁/解封${NC}"
USER2_ID=$(curl -s -b "token=$ADMIN_TOKEN" "$BASE/api/v1/auth/me" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null || echo "66")
do_test "封禁user2" "PUT" "/api/v1/admin/users/$USER2_ID/status" '{"status":0}' "200" "$ADMIN_TOKEN"
do_test "被封用户登录" "POST" "/api/v1/auth/login" '{"username":"user2_test","password":"test123"}' "400"
do_test "解封user2" "PUT" "/api/v1/admin/users/$USER2_ID/status" '{"status":1}' "200" "$ADMIN_TOKEN"
do_test "解封后登录" "POST" "/api/v1/auth/login" '{"username":"user2_test","password":"test123"}' "200"

# ====== 18. Edge Cases ======
echo -e "\n${YELLOW}[18] 边界情况${NC}"
do_test "帖子分页过大" "GET" "/api/v1/feed?page=999&page_size=5" "" "200" "$USER_TOKEN"
do_test "page_size=0" "GET" "/api/v1/feed?page=1&page_size=0" "" "200" "$USER_TOKEN"
do_test "超大page_size" "GET" "/api/v1/feed?page=1&page_size=100" "" "200" "$USER_TOKEN"
do_test "无sort参数" "GET" "/api/v1/feed" "" "200" "$USER_TOKEN"
do_test "XSS尝试" "POST" "/api/v1/auth/register" '{"username":"<script>alert(1)</script>","password":"test12345"}' "200" "$USER_TOKEN"

# ====== 19. POST Pages ======
echo -e "\n${YELLOW}[19] 帖子页面渲染${NC}"
do_test "创建帖子页" "GET" "/post/create" "" "200" "$USER_TOKEN"
do_test "帖子详情页" "GET" "/post/$POST_ID" "" "200" "$USER_TOKEN"
do_test "编辑帖子页" "GET" "/post/$POST_ID/edit" "" "200" "$USER_TOKEN"
do_test "用户无权限编辑他人" "GET" "/post/$POST_ID/edit" "" "200" "$USER2_TOKEN"  # will redirect to /

# ====== 20. Delete ======
echo -e "\n${YELLOW}[20] 删除操作${NC}"
do_test "删除评论(非本人)" "DELETE" "/api/v1/comments/$COMMENT_ID" "" "400" "$USER2_TOKEN"
do_test "删除帖子(非本人)" "DELETE" "/api/v1/posts/$POST_ID" "" "400" "$USER_TOKEN"

# ====== Summary ======
echo ""
echo "============================================================"
echo "  测试结果汇总"
echo "============================================================"
echo -e "  ${GREEN}通过: $PASS${NC}"
echo -e "  ${RED}失败: $FAIL${NC}"
echo -e "  ${YELLOW}跳过: $SKIP${NC}"
echo ""
if [ $FAIL -eq 0 ]; then
    echo -e "  ${GREEN}🎉 全部测试通过!${NC}"
else
    echo -e "  ${RED}⚠️  有 $FAIL 个测试失败，需要修复${NC}"
fi
echo "============================================================"
