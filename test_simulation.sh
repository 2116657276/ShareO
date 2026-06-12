#!/usr/bin/env zsh
# ShareO 普通用户模拟测试脚本
# 流程: test01-test09 各发一帖(一图一帖) → test10-test14 浏览+随机点赞
set +e  # Don't exit on error, we handle failures ourselves

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

typeset -A IMG_MAP
IMG_MAP=(
  test01  "IMG_0102.jpeg"
  test02  "IMG_0203.jpeg"
  test03  "IMG_0224.jpeg"
  test04  "IMG_0284.jpeg"
  test05  "IMG_0285.jpeg"
  test06  "IMG_0432.jpeg"
  test07  "IMG_6122.jpeg"
  test08  "IMG_9008.jpeg"
  test09  "IMG_9009.jpeg"
)

typeset -A POST_DESC
POST_DESC=(
  test01  "清晨古镇石板路 🌅 #街拍"
  test02  "雨后山间云雾 ⛰️ #山景"
  test03  "城市霓虹夜景 🏙️ #夜景"
  test04  "周末山顶徒步 🥾 #户外"
  test05  "晒太阳的小猫 🐱 #宠物"
  test06  "春日花卉微距 🌸 #花卉"
  test07  "海边日落 🌊 #海边"
  test08  "老建筑记忆 📷 #建筑"
  test09  "咖啡厅午后 ☕ #生活"
)

typeset -A U_TOKEN
typeset -A U_POST_ID
POST_IDS=()
USERS=(test01 test02 test03 test04 test05 test06 test07 test08 test09)

log_section() {
  echo ""
  echo -e "${YELLOW}============================================================${NC}"
  echo -e "${YELLOW}  $1${NC}"
  echo -e "${YELLOW}============================================================${NC}"
}

log_pass() {
  echo -e "  ${GREEN}✓${NC} $1"
  PASS=$((PASS+1))
}

log_fail() {
  echo -e "  ${RED}✗${NC} $1"
  echo -e "    ${RED}$2${NC}"
  FAIL=$((FAIL+1))
}

# 注册或登录用户，返回token
# 用法: do_login <username> <password>
do_login() {
  local uname="$1" pass="$2"
  local token=""

  # 先尝试登录
  local login_resp=$(curl -s -X POST "$BASE/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$uname\",\"password\":\"$pass\"}")
  local login_code=$(echo "$login_resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('code',1))" 2>/dev/null || echo "1")

  if [[ "$login_code" == "0" ]]; then
    token=$(echo "$login_resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['data']['token'])" 2>/dev/null || echo "")
    echo "$token"
    return 0
  fi

  # 登录失败，尝试注册
  local reg_resp=$(curl -s -X POST "$BASE/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$uname\",\"password\":\"$pass\"}")
  local reg_code=$(echo "$reg_resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('code',1))" 2>/dev/null || echo "1")

  if [[ "$reg_code" == "0" ]]; then
    # 注册成功，自动返回token (register API返回token)
    token=$(echo "$reg_resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['data']['token'])" 2>/dev/null || echo "")
    if [[ -n "$token" && "$token" != "null" ]]; then
      echo "$token"
      return 0
    fi
    # 如果注册响应没有token，再登录一次
    local login2=$(curl -s -X POST "$BASE/api/v1/auth/login" \
      -H 'Content-Type: application/json' \
      -d "{\"username\":\"$uname\",\"password\":\"$pass\"}")
    token=$(echo "$login2" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['data']['token'])" 2>/dev/null || echo "")
    echo "$token"
    return 0
  fi

  # 注册也失败
  local reg_msg=$(echo "$reg_resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('message','?'))" 2>/dev/null)
  echo "FAIL:$reg_msg"
  return 1
}

# macOS-compatible random selection from array
random_pick() {
  local n="$1"
  shift
  local arr=("$@")
  # Use python3 for cross-platform random selection
  python3 -c "
import random, sys
items = [$(printf '"%s",' "${arr[@]}" | sed 's/,$//')]
random.shuffle(items)
for i in items[:$n]:
    print(i)
"
}

# ====== 管理员登录 ======
log_section "[0] 管理员登录"
ADMIN_RESP=$(curl -s -X POST "$BASE/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}')
ADMIN_TOKEN=$(echo "$ADMIN_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['data']['token'])" 2>/dev/null || echo "")
if [[ -n "$ADMIN_TOKEN" && "$ADMIN_TOKEN" != "null" ]]; then
  log_pass "管理员登录成功"
else
  log_fail "管理员登录失败" "$ADMIN_RESP"
  exit 1
fi

# ====== Phase 1: 用户注册/登录、上传、发帖 ======
log_section "[1] 用户注册/登录+上传+发帖 (test01-test09，一图一帖)"

for UNAME in "${USERS[@]}"; do
  IMG_FILE="${IMG_MAP[$UNAME]}"
  CONTENT="${POST_DESC[$UNAME]}"
  IMG_PATH="$RESOURCES/$IMG_FILE"

  if [[ ! -f "$IMG_PATH" ]]; then
    log_fail "图片不存在: $IMG_PATH" ""
    continue
  fi
  IMG_SIZE=$(du -h "$IMG_PATH" | cut -f1)

  echo -e "\n${BLUE}--- $UNAME ($IMG_FILE, $IMG_SIZE) ---${NC}"

  # 1.1 登录或注册
  TOKEN=$(do_login "$UNAME" "256500")
  if [[ "$TOKEN" == FAIL:* ]]; then
    log_fail "登录/注册 $UNAME 失败" "${TOKEN#FAIL:}"
    continue
  fi
  if [[ -n "$TOKEN" && "$TOKEN" != "null" ]]; then
    log_pass "认证 $UNAME"
    U_TOKEN[$UNAME]="$TOKEN"
  else
    log_fail "获取 $UNAME token 失败" ""
    continue
  fi

  # 1.2 上传图片
  echo "  ⏳ 上传中 ($IMG_SIZE)..."
  UPLOAD_RESP=$(curl -s -X POST "$BASE/api/v1/upload" \
    -b "token=$TOKEN" \
    -F "file=@$IMG_PATH")
  UPLOAD_CODE=$(echo "$UPLOAD_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('code',1))" 2>/dev/null || echo "1")
  IMG_URL=$(echo "$UPLOAD_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('url',''))" 2>/dev/null || echo "")
  if [[ "$UPLOAD_CODE" == "0" ]]; then
    log_pass "上传 $IMG_FILE → $IMG_URL"
  else
    log_fail "上传 $IMG_FILE 失败" "$UPLOAD_RESP"
    continue
  fi

  # 1.3 创建帖子
  CREATE_RESP=$(curl -s -X POST "$BASE/api/v1/posts" \
    -b "token=$TOKEN" \
    -H 'Content-Type: application/json' \
    -d "{\"content\":\"$CONTENT\",\"images\":[\"$IMG_URL\"],\"topic_ids\":[1]}")
  POST_ID=$(echo "$CREATE_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['data']['id'])" 2>/dev/null || echo "")
  if [[ -n "$POST_ID" && "$POST_ID" != "null" ]]; then
    log_pass "发帖: $CONTENT → id=$POST_ID"
    U_POST_ID[$UNAME]="$POST_ID"
    POST_IDS+=("$POST_ID")
  else
    log_fail "发帖失败" "$CREATE_RESP"
    continue
  fi

  # 1.4 管理员审批通过
  APPROVE_RESP=$(curl -s -X POST "$BASE/api/v1/admin/posts/$POST_ID/approve" \
    -b "token=$ADMIN_TOKEN")
  APPROVE_CODE=$(echo "$APPROVE_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('code',1))" 2>/dev/null || echo "1")
  if [[ "$APPROVE_CODE" == "0" ]]; then
    log_pass "审批通过 帖子$POST_ID"
  else
    log_fail "审批 帖子$POST_ID 失败" "$APPROVE_RESP"
  fi
done

echo ""
echo -e "${CYAN}帖子ID列表: ${POST_IDS[*]}${NC}"
echo -e "${CYAN}共 ${#POST_IDS[@]} 个帖子${NC}"

# ====== Phase 2: 新用户浏览 + 随机点赞 ======
log_section "[2] 新用户浏览Feed + 随机点赞 (test10-test14)"

typeset -A LIKES_FOR_USER
LIKES_FOR_USER[test10]="1 2 3 8"
LIKES_FOR_USER[test11]="2 1 4 6"
LIKES_FOR_USER[test12]="3 4 5 6"
LIKES_FOR_USER[test13]="7 8 9"
LIKES_FOR_USER[test14]="9 4 2 1"

BROWSER_USERS=(test10 test11 test12 test13 test14)

for UNAME in "${BROWSER_USERS[@]}"; do
  echo -e "\n${BLUE}--- $UNAME ---${NC}"

  # 2.1 登录或注册
  TOKEN=$(do_login "$UNAME" "256500")
  if [[ "$TOKEN" == FAIL:* ]]; then
    log_fail "登录/注册 $UNAME 失败" "${TOKEN#FAIL:}"
    continue
  fi
  if [[ -n "$TOKEN" && "$TOKEN" != "null" ]]; then
    log_pass "认证 $UNAME"
  else
    log_fail "获取 $UNAME token 失败" ""
    continue
  fi

  # 2.2 浏览Feed (最新)
  FEED_RESP=$(curl -s -b "token=$TOKEN" "$BASE/api/v1/feed?sort=latest&page=1&page_size=9")
  FEED_TOTAL=$(echo "$FEED_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('total',0))" 2>/dev/null || echo "0")
  log_pass "浏览最新Feed (total=$FEED_TOTAL)"

  # 2.3 浏览Feed (热门)
  HOT_RESP=$(curl -s -b "token=$TOKEN" "$BASE/api/v1/feed?sort=hot&page=1&page_size=9")
  HOT_TOTAL=$(echo "$HOT_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('total',0))" 2>/dev/null || echo "0")
  log_pass "浏览热门Feed (total=$HOT_TOTAL)"

  # 2.4 随机查看3个帖子详情
  if [[ ${#POST_IDS[@]} -gt 0 ]]; then
    PICKED=$(random_pick 3 "${POST_IDS[@]}")
    while IFS= read -r PID; do
      DETAIL_RESP=$(curl -s -b "token=$TOKEN" "$BASE/api/v1/posts/$PID")
      DETAIL_CODE=$(echo "$DETAIL_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('code',1))" 2>/dev/null || echo "1")
      if [[ "$DETAIL_CODE" == "0" ]]; then
        POST_AUTHOR=$(echo "$DETAIL_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['data'].get('username','?'))" 2>/dev/null)
        log_pass "查看帖子$PID (作者: $POST_AUTHOR)"
      else
        log_fail "查看帖子$PID 失败" ""
      fi
    done <<< "$PICKED"
  fi

  # 2.5 按照预设点赞 (1-based索引 -> 实际post id)
  LIKE_INDEXES="${LIKES_FOR_USER[$UNAME]}"
  for IDX in ${(z)LIKE_INDEXES}; do
    if [[ $IDX -lt 1 || $IDX -gt ${#POST_IDS[@]} ]]; then
      log_fail "点赞索引$IDX 超出范围(1-${#POST_IDS[@]})" ""
      continue
    fi
    PID="${POST_IDS[$((IDX-1))]}"
    LIKE_RESP=$(curl -s -X POST "$BASE/api/v1/posts/$PID/like" \
      -b "token=$TOKEN")
    LIKE_CODE=$(echo "$LIKE_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('code',1))" 2>/dev/null || echo "1")
    if [[ "$LIKE_CODE" == "0" ]]; then
      log_pass "点赞帖子$PID"
    else
      LIKE_MSG=$(echo "$LIKE_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('message','?'))" 2>/dev/null)
      log_fail "点赞帖子$PID" "$LIKE_MSG"
    fi
  done
done

# ====== Phase 3: 验证数据 ======
log_section "[3] 验证测试结果"

STATS_RESP=$(curl -s -b "token=$ADMIN_TOKEN" "$BASE/api/v1/admin/stats")
echo -e "${CYAN}管理员统计:${NC}"
echo "$STATS_RESP" | python3 -c "
import sys,json
d=json.load(sys.stdin)
s=d.get('data',{})
print(f'  用户总数: {s.get(\"total_users\",\"?\")}')
print(f'  帖子总数: {s.get(\"total_posts\",\"?\")}')
print(f'  待审核:   {s.get(\"pending_posts\",\"?\")}')
print(f'  总点赞:   {s.get(\"total_likes\",\"?\")}')
print(f'  总评论:   {s.get(\"total_comments\",\"?\")}')
" 2>/dev/null || echo "  获取统计失败"

FEED_CHECK=$(curl -s -b "token=$ADMIN_TOKEN" "$BASE/api/v1/feed?sort=latest&page=1&page_size=20")
FEED_CHECK_TOTAL=$(echo "$FEED_CHECK" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('total',0))" 2>/dev/null || echo "0")
if [[ "$FEED_CHECK_TOTAL" == "9" ]]; then
  log_pass "Feed中帖子总数=$FEED_CHECK_TOTAL (预期9)"
else
  log_fail "Feed帖子数: $FEED_CHECK_TOTAL (预期9)"
fi

echo -e "\n${CYAN}帖子状态检查:${NC}"
for PID in "${POST_IDS[@]}"; do
  POST_DETAIL=$(curl -s -b "token=$ADMIN_TOKEN" "$BASE/api/v1/posts/$PID")
  STATUS=$(echo "$POST_DETAIL" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['data'].get('status','?'))" 2>/dev/null || echo "?")
  LIKES=$(echo "$POST_DETAIL" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['data'].get('like_count',0))" 2>/dev/null || echo "0")
  AUTHOR=$(echo "$POST_DETAIL" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['data'].get('username','?'))" 2>/dev/null || echo "?")
  if [[ "$STATUS" == "approved" ]]; then
    echo -e "  ${GREEN}✓${NC} 帖子$PID: status=$STATUS, likes=$LIKES, author=$AUTHOR"
    PASS=$((PASS+1))
  else
    echo -e "  ${RED}✗${NC} 帖子$PID: status=$STATUS (expected approved)"
    FAIL=$((FAIL+1))
  fi
done

echo -e "\n${CYAN}测试用户验证:${NC}"
ALL_USERS=("${USERS[@]}" "${BROWSER_USERS[@]}")
for UNAME in "${ALL_USERS[@]}"; do
  LOGIN_CHECK=$(curl -s -X POST "$BASE/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$UNAME\",\"password\":\"256500\"}")
  LOGIN_CHECK_CODE=$(echo "$LOGIN_CHECK" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('code',1))" 2>/dev/null || echo "1")
  if [[ "$LOGIN_CHECK_CODE" == "0" ]]; then
    echo -e "  ${GREEN}✓${NC} $UNAME 可正常登录"
    PASS=$((PASS+1))
  else
    echo -e "  ${RED}✗${NC} $UNAME 登录失败"
    FAIL=$((FAIL+1))
  fi
done

# ====== 汇总 ======
echo ""
echo -e "${YELLOW}============================================================${NC}"
echo -e "${YELLOW}  模拟测试汇总${NC}"
echo -e "${YELLOW}============================================================${NC}"
echo -e "  ${GREEN}通过: $PASS${NC}"
echo -e "  ${RED}失败: $FAIL${NC}"
echo ""

echo -e "${YELLOW}============================================================${NC}"
echo -e "${YELLOW}  发帖明细汇总${NC}"
echo -e "${YELLOW}============================================================${NC}"
echo ""
echo "| 帖子ID | 用户 | 图片 | 内容 |"
echo "|--------|------|------|------|"
for UNAME in "${USERS[@]}"; do
  IMG="${IMG_MAP[$UNAME]}"
  DESC="${POST_DESC[$UNAME]}"
  PID="${U_POST_ID[$UNAME]}"
  echo "| $PID | $UNAME | $IMG | $DESC |"
done

echo ""
if [[ $FAIL -eq 0 ]]; then
  echo -e "  ${GREEN}🎉 全部模拟测试通过!${NC}"
else
  echo -e "  ${RED}⚠️  有 $FAIL 个步骤失败，请检查${NC}"
fi
echo -e "${YELLOW}============================================================${NC}"
