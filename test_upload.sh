#!/usr/bin/env zsh
# ShareO 图片上传+发帖脚本 (纯上传，无互动)
# 12张图片 → 12个用户 → 12篇帖子
set +e

BASE="http://localhost:8080"
RESOURCES="/Users/zhoujianlin/GoLandProjects/ShareO/resources/static/pictures"
PASS=0
FAIL=0

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BLUE='\033[0;34m'
NC='\033[0m'

# 帖子配文（简单）
typeset -A POST_TEXT
POST_TEXT=(
  "IMG_0102.jpeg"  "古镇街角"
  "IMG_0805.jpeg"  "午后阳光"
  "IMG_1216.jpeg"  "林间小径"
  "IMG_1219.jpeg"  "湖边倒影"
  "IMG_1222.jpeg"  "山巅远眺"
  "IMG_1226.jpeg"  "城市剪影"
  "IMG_1230.jpeg"  "夜色灯影"
  "IMG_1480.jpeg"  "花间一瞥"
  "IMG_6122.jpeg"  "海边落日"
  "IMG_7640.jpeg"  "巷弄深处"
  "IMG_8045.jpeg"  "窗前绿意"
  "IMG_9007.jpeg"  "雨后小巷"
)

log_section() {
  echo ""
  echo -e "${YELLOW}============================================================${NC}"
  echo -e "${YELLOW}  $1${NC}"
  echo -e "${YELLOW}============================================================${NC}"
}

log_pass() { echo -e "  ${GREEN}✓${NC} $1"; PASS=$((PASS+1)); }
log_fail() { echo -e "  ${RED}✗${NC} $1"; echo -e "    ${RED}$2${NC}"; FAIL=$((FAIL+1)); }

# 登录或注册
do_auth() {
  local uname="$1" pass="$2"
  local token=""
  # 先尝试登录
  local resp=$(curl -s -X POST "$BASE/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$uname\",\"password\":\"$pass\"}")
  local code=$(echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin).get('code',1))" 2>/dev/null || echo "1")
  if [[ "$code" == "0" ]]; then
    echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])" 2>/dev/null
    return 0
  fi
  # 注册
  resp=$(curl -s -X POST "$BASE/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$uname\",\"password\":\"$pass\"}")
  code=$(echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin).get('code',1))" 2>/dev/null || echo "1")
  if [[ "$code" == "0" ]]; then
    token=$(echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])" 2>/dev/null || echo "")
    if [[ -n "$token" && "$token" != "null" ]]; then
      echo "$token"; return 0
    fi
    # fallback: login
    resp=$(curl -s -X POST "$BASE/api/v1/auth/login" \
      -H 'Content-Type: application/json' \
      -d "{\"username\":\"$uname\",\"password\":\"$pass\"}")
    echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])" 2>/dev/null
    return 0
  fi
  local msg=$(echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin).get('message','?'))" 2>/dev/null)
  echo "FAIL:$msg"
  return 1
}

# ====== 获取图片列表 ======
IMAGES=($(ls "$RESOURCES"/*.jpeg 2>/dev/null | sort))
IMG_COUNT=${#IMAGES[@]}
echo -e "${CYAN}找到 $IMG_COUNT 张图片${NC}"

# 动态生成用户名 (从 test15 开始，避免与已有 test01-14 冲突)
USER_NUM=15
typeset -A IMG_USER
POST_IDS=()

# ====== 管理员登录 ======
log_section "[0] 管理员登录"
ADMIN_TOKEN=$(do_auth "admin" "admin123")
if [[ -n "$ADMIN_TOKEN" && "$ADMIN_TOKEN" != "null" && "$ADMIN_TOKEN" != FAIL:* ]]; then
  log_pass "管理员登录成功"
else
  log_fail "管理员登录失败" "$ADMIN_TOKEN"
  exit 1
fi

# ====== 上传+发帖 ======
log_section "[1] 上传图片+发帖 (共 $IMG_COUNT 张)"

for IMG_PATH in "${IMAGES[@]}"; do
  IMG_FILE=$(basename "$IMG_PATH")
  CONTENT="${POST_TEXT[$IMG_FILE]}"
  UNAME="test$(printf '%02d' $USER_NUM)"
  IMG_SIZE=$(du -h "$IMG_PATH" | cut -f1)
  IMG_BYTES=$(stat -f%z "$IMG_PATH")

  echo -e "\n${BLUE}--- $UNAME → $IMG_FILE ($IMG_SIZE, ${IMG_BYTES}bytes) ---${NC}"

  # 检查是否超限
  MAX_SIZE=52428800  # 50MB
  if [[ $IMG_BYTES -gt $MAX_SIZE ]]; then
    log_fail "$IMG_FILE 超过50MB限制 (${IMG_BYTES} > ${MAX_SIZE})" "跳过"
    continue
  fi

  # 1. 认证
  TOKEN=$(do_auth "$UNAME" "256500")
  if [[ "$TOKEN" == FAIL:* ]]; then
    log_fail "认证 $UNAME 失败" "${TOKEN#FAIL:}"
    USER_NUM=$((USER_NUM+1))
    continue
  fi
  log_pass "认证 $UNAME"

  # 2. 上传
  echo "  ⏳ 上传中 ($IMG_SIZE)..."
  UPLOAD_RESP=$(curl -s --max-time 120 -X POST "$BASE/api/v1/upload" \
    -b "token=$TOKEN" \
    -F "file=@$IMG_PATH")
  UPLOAD_CODE=$(echo "$UPLOAD_RESP" | python3 -c "import sys,json;print(json.load(sys.stdin).get('code',1))" 2>/dev/null || echo "1")
  IMG_URL=$(echo "$UPLOAD_RESP" | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('url',''))" 2>/dev/null || echo "")

  if [[ "$UPLOAD_CODE" == "0" && -n "$IMG_URL" ]]; then
    log_pass "上传成功 → $IMG_URL"
  else
    ERR_MSG=$(echo "$UPLOAD_RESP" | python3 -c "import sys,json;print(json.load(sys.stdin).get('message','?'))" 2>/dev/null)
    log_fail "上传失败: $ERR_MSG" ""
    USER_NUM=$((USER_NUM+1))
    continue
  fi

  # 3. 发帖
  CREATE_RESP=$(curl -s -X POST "$BASE/api/v1/posts" \
    -b "token=$TOKEN" \
    -H 'Content-Type: application/json' \
    -d "{\"content\":\"$CONTENT\",\"images\":[\"$IMG_URL\"]}")
  POST_ID=$(echo "$CREATE_RESP" | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('id',''))" 2>/dev/null || echo "")

  if [[ -n "$POST_ID" && "$POST_ID" != "null" ]]; then
    log_pass "发帖成功 → id=$POST_ID ($CONTENT)"
    POST_IDS+=("$POST_ID")
    IMG_USER[$IMG_FILE]="$UNAME"
  else
    ERR_MSG=$(echo "$CREATE_RESP" | python3 -c "import sys,json;print(json.load(sys.stdin).get('message','?'))" 2>/dev/null)
    log_fail "发帖失败: $ERR_MSG" ""
    USER_NUM=$((USER_NUM+1))
    continue
  fi

  # 4. 管理员审批
  APPROVE_RESP=$(curl -s -X POST "$BASE/api/v1/admin/posts/$POST_ID/approve" \
    -b "token=$ADMIN_TOKEN")
  APPROVE_CODE=$(echo "$APPROVE_RESP" | python3 -c "import sys,json;print(json.load(sys.stdin).get('code',1))" 2>/dev/null || echo "1")
  if [[ "$APPROVE_CODE" == "0" ]]; then
    log_pass "审批通过"
  else
    log_fail "审批失败" ""
  fi

  IMG_USER[$IMG_FILE]="$UNAME"
  USER_NUM=$((USER_NUM+1))
done

# ====== 验证 ======
log_section "[2] 验证结果"

echo -e "${CYAN}管理员统计:${NC}"
curl -s -b "token=$ADMIN_TOKEN" "$BASE/api/v1/admin/stats" | python3 -c "
import sys,json
s=json.load(sys.stdin).get('data',{})
print(f'  用户总数: {s.get(\"total_users\",\"?\")}')
print(f'  帖子总数: {s.get(\"total_posts\",\"?\")}')
print(f'  待审核:   {s.get(\"pending_posts\",\"?\")}')
"

echo -e "\n${CYAN}Feed可见性:${NC}"
FEED_TOTAL=$(curl -s -b "token=$ADMIN_TOKEN" "$BASE/api/v1/feed?sort=latest&page=1&page_size=30" | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('total',0))" 2>/dev/null || echo "0")
log_pass "Feed帖子总数=$FEED_TOTAL"

echo -e "\n${CYAN}帖子明细:${NC}"
for PID in "${POST_IDS[@]}"; do
  curl -s -b "token=$ADMIN_TOKEN" "$BASE/api/v1/posts/$PID" | python3 -c "
import sys,json
d=json.load(sys.stdin).get('data',{})
print(f'  帖子$PID: status={d.get(\"status\",\"?\")}, author={d.get(\"user\",{}).get(\"username\",\"?\")}, cover={d.get(\"cover_image\",\"?\")[:60]}...')
" 2>/dev/null
done

# ====== 汇总 ======
echo ""
echo -e "${YELLOW}============================================================${NC}"
echo -e "${YELLOW}  上传测试汇总${NC}"
echo -e "${YELLOW}============================================================${NC}"
echo -e "  ${GREEN}通过: $PASS${NC}"
echo -e "  ${RED}失败: $FAIL${NC}"

echo ""
echo "| 图片 | 用户 | 帖子ID | 配文 |"
echo "|------|------|--------|------|"
for IMG_FILE in $(ls "$RESOURCES"/*.jpeg 2>/dev/null | sort | while read f; do basename "$f"; done); do
  UNAME="${IMG_USER[$IMG_FILE]}"
  TEXT="${POST_TEXT[$IMG_FILE]}"
  echo "| $IMG_FILE | $UNAME | - | $TEXT |"
done

echo ""
[[ $FAIL -eq 0 ]] && echo -e "  ${GREEN}✅ 全部上传成功!${NC}" || echo -e "  ${RED}⚠️  有 $FAIL 个失败${NC}"
echo -e "${YELLOW}============================================================${NC}"
