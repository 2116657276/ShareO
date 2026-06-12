#!/usr/bin/env zsh
# ShareO 普通用户发帖测试 (不经过管理员审核，帖子保持 pending 状态)
# 12张图片 → 12个新用户 → 12篇帖子 (全部pending，等待管理员审核)
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

log_pass() { echo -e "  ${GREEN}✓${NC} $1"; PASS=$((PASS+1)); }
log_fail() { echo -e "  ${RED}✗${NC} $1"; echo -e "    ${RED}$2${NC}"; FAIL=$((FAIL+1)); }

do_auth() {
  local uname="$1" pass="$2"
  local resp=$(curl -s -X POST "$BASE/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$uname\",\"password\":\"$pass\"}")
  local code=$(echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin).get('code',1))" 2>/dev/null)
  if [[ "$code" == "0" ]]; then
    echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])" 2>/dev/null
    return 0
  fi
  resp=$(curl -s -X POST "$BASE/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$uname\",\"password\":\"$pass\"}")
  code=$(echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin).get('code',1))" 2>/dev/null)
  if [[ "$code" == "0" ]]; then
    local token=$(echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])" 2>/dev/null)
    if [[ -n "$token" && "$token" != "null" ]]; then
      echo "$token"; return 0
    fi
    resp=$(curl -s -X POST "$BASE/api/v1/auth/login" \
      -H 'Content-Type: application/json' \
      -d "{\"username\":\"$uname\",\"password\":\"$pass\"}")
    echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])" 2>/dev/null
    return 0
  fi
  echo "FAIL:$(echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin).get('message','?'))" 2>/dev/null)"
  return 1
}

echo -e "${YELLOW}============================================================${NC}"
echo -e "${YELLOW}  ShareO 普通用户发帖 (不审批，帖子保持 pending)${NC}"
echo -e "${YELLOW}============================================================${NC}"

IMAGES=($(ls "$RESOURCES"/*.jpeg 2>/dev/null | sort))
echo -e "\n${CYAN}找到 ${#IMAGES[@]} 张图片${NC}"

USER_NUM=31
typeset -A IMG_POST_ID

for IMG_PATH in "${IMAGES[@]}"; do
  IMG_FILE=$(basename "$IMG_PATH")
  CONTENT="${POST_TEXT[$IMG_FILE]}"
  UNAME="uploader$(printf '%02d' $((USER_NUM - 30)))"
  IMG_SIZE=$(du -h "$IMG_PATH" | cut -f1)
  IMG_BYTES=$(stat -f%z "$IMG_PATH")

  echo -e "\n${BLUE}--- $UNAME → $IMG_FILE ($IMG_SIZE) ---${NC}"

  # 超限检查
  if [[ $IMG_BYTES -gt 52428800 ]]; then
    log_fail "$IMG_FILE 超过50MB限制" "跳过"
    USER_NUM=$((USER_NUM+1)); continue
  fi

  # 1. 认证
  TOKEN=$(do_auth "$UNAME" "256500")
  if [[ "$TOKEN" == FAIL:* ]]; then
    log_fail "认证 $UNAME 失败" "${TOKEN#FAIL:}"
    USER_NUM=$((USER_NUM+1)); continue
  fi
  log_pass "认证 $UNAME"

  # 2. 上传图片
  echo "  ⏳ 上传中 ($IMG_SIZE)..."
  UPLOAD_RESP=$(curl -s --max-time 120 -X POST "$BASE/api/v1/upload" \
    -b "token=$TOKEN" -F "file=@$IMG_PATH")
  UPLOAD_CODE=$(echo "$UPLOAD_RESP" | python3 -c "import sys,json;print(json.load(sys.stdin).get('code',1))" 2>/dev/null)
  IMG_URL=$(echo "$UPLOAD_RESP" | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('url',''))" 2>/dev/null)

  if [[ "$UPLOAD_CODE" == "0" && -n "$IMG_URL" ]]; then
    log_pass "上传成功 → $IMG_URL"
  else
    log_fail "上传失败" "$(echo "$UPLOAD_RESP" | python3 -c "import sys,json;print(json.load(sys.stdin).get('message','?'))" 2>/dev/null)"
    USER_NUM=$((USER_NUM+1)); continue
  fi

  # 3. 发帖 — 不审批！保持 pending
  CREATE_RESP=$(curl -s -X POST "$BASE/api/v1/posts" \
    -b "token=$TOKEN" \
    -H 'Content-Type: application/json' \
    -d "{\"content\":\"$CONTENT\",\"images\":[\"$IMG_URL\"]}")
  POST_ID=$(echo "$CREATE_RESP" | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('id',''))" 2>/dev/null)

  if [[ -n "$POST_ID" && "$POST_ID" != "null" ]]; then
    log_pass "发帖成功 → id=$POST_ID ($CONTENT)"
    IMG_POST_ID[$IMG_FILE]="$POST_ID"
  else
    log_fail "发帖失败" "$(echo "$CREATE_RESP" | python3 -c "import sys,json;print(json.load(sys.stdin).get('message','?'))" 2>/dev/null)"
    USER_NUM=$((USER_NUM+1)); continue
  fi

  USER_NUM=$((USER_NUM+1))
done

# ====== 验证 ======
echo ""
echo -e "${YELLOW}============================================================${NC}"
echo -e "${YELLOW}  验证结果${NC}"
echo -e "${YELLOW}============================================================${NC}"

ADMIN_TOKEN=$(curl -s -X POST "$BASE/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])" 2>/dev/null)

echo -e "\n${CYAN}管理员统计:${NC}"
curl -s -b "token=$ADMIN_TOKEN" "$BASE/api/v1/admin/stats" | python3 -c "
import sys,json
s=json.load(sys.stdin).get('data',{})
print(f'  用户总数: {s.get(\"total_users\",\"?\")}')
print(f'  帖子总数: {s.get(\"total_posts\",\"?\")}')
print(f'  待审核:   {s.get(\"pending_posts\",\"?\")}')
"

echo -e "\n${CYAN}Feed 可见性 (应为0，因为帖子都是pending):${NC}"
FEED_TOTAL=$(curl -s -b "token=$ADMIN_TOKEN" "$BASE/api/v1/feed?sort=latest&page=1&page_size=20" | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('total',0))" 2>/dev/null)
if [[ "$FEED_TOTAL" == "0" ]]; then
  log_pass "Feed帖子数=$FEED_TOTAL (正确: pending帖不在Feed中展示)"
else
  log_fail "Feed帖子数=$FEED_TOTAL (预期0，pending帖不应可见)"
fi

echo -e "\n${CYAN}待审核帖子列表:${NC}"
curl -s -b "token=$ADMIN_TOKEN" "$BASE/api/v1/admin/pending-posts?page=1&page_size=20" | python3 -c "
import sys,json
d=json.load(sys.stdin)
data=d.get('data',{})
posts=data.get('list',[])
print(f'待审核数: {len(posts)}')
for p in posts:
    print(f'  帖子{p[\"id\"]}: author={p.get(\"username\",\"?\")}, content={p.get(\"content\",\"?\")[:30]}, status={p.get(\"status\",\"?\")}')
"

# ====== 汇总 ======
echo ""
echo -e "${YELLOW}============================================================${NC}"
echo -e "${YELLOW}  发帖明细 (全部 pending，等待管理员审核)${NC}"
echo -e "${YELLOW}============================================================${NC}"
echo ""
echo "| 帖子ID | 用户 | 图片 | 配文 | 状态 |"
echo "|--------|------|------|------|------|"
for IMG_FILE in $(ls "$RESOURCES"/*.jpeg 2>/dev/null | sort | while read f; do basename "$f"; done); do
  PID="${IMG_POST_ID[$IMG_FILE]}"
  TEXT="${POST_TEXT[$IMG_FILE]}"
  U_NUM=$(echo "$IMG_FILE" | python3 -c "import sys; [sys.stdout.write(f'uploader{i+1:02d}') for i,f in enumerate(sorted(sys.stdin.read().strip().split('\n'))) if '$IMG_FILE' in f]" 2>/dev/null)
  echo "| $PID | - | $IMG_FILE | $TEXT | pending |"
done

echo ""
echo -e "  ${GREEN}通过: $PASS${NC}  ${RED}失败: $FAIL${NC}"
[[ $FAIL -eq 0 ]] && echo -e "  ${GREEN}✅ 全部发帖成功，等待管理员审核!${NC}" || echo -e "  ${RED}⚠️  有 $FAIL 个失败${NC}"
echo -e "${YELLOW}============================================================${NC}"
echo ""
echo -e "${CYAN}提示: 管理员账号 admin/admin123，访问 /admin/review 审核帖子${NC}"
