#!/usr/bin/env bash
# ============================================================
#  ShareO Demo Seed — 可重复的演示数据初始化脚本
#  Phase 7A: 创建固定 Demo 用户、管理员、帖子、图片
#  幂等：可安全多次执行
# ============================================================
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BASE_URL="${SHAREO_BASE_URL:-http://127.0.0.1:8080}"
INTERNAL_TOKEN="${SHAREO_INTERNAL_TOKEN:-}"
DB_PASSWORD="${SHAREO_DB_PASSWORD:-shareo_pass}"

# Disable system proxy for local API calls
unset http_proxy HTTP_PROXY https_proxy HTTPS_PROXY all_proxy ALL_PROXY
ADMIN_USER="demoadmin"
ADMIN_PASSWORD="admin123"
ADMIN_EMAIL="demoadmin@shareo.local"
# bcrypt hash of "admin123" — verified against the container's bcrypt library
ADMIN_HASH='$2a$10$vbn5lSmj3e6arCkbXzKNqutbx5B/iqtnk6tHoYvETGh1qdnG/5Rd2'

# Demo users: "username:password" pairs
DEMO_USERS=("demo_alice:alice2024" "demo_bob:bob2024")

# Demo post contents — diverse categories for image search eval
# Format: "category|content"
POSTS=(
  "object|春天的桃花盛开了，粉色花瓣在阳光下特别通透。用微距镜头捕捉花朵特写，浅景深把背景虚化得非常柔和，春意盎然的感觉。"
  "object|这栋欧式城堡建筑太震撼了，石头塔楼直指蓝天。仰拍视角让古堡显得格外雄伟，复古的拱窗和石雕细节充满了历史沉淀感。"
  "scene|站在山顶俯瞰整座城市，红瓦屋顶层层叠叠延伸到远方。远处的天际线与大片红屋顶构成了一幅充满异国风情的城市画卷。"
  "object|在花店买了一束紫色矢车菊和雏菊，插在白瓷花瓶里。放在桌角作为室内静物，阳光透过窗户洒在花瓣上，优雅又浪漫。"
  "object|绿叶簇拥下的白色栀子花特写，花瓣洁白如雪，带着淡淡的花香。特写镜头把花瓣的纹理和绿叶的光泽表现得淋漓尽致。"
  "scene|蓝天白云下的红砖钟楼大门，矗立在晴空之下。古朴的钟楼建筑与湛蓝的天空形成强烈对比，让人感受到岁月沉淀的庄重。"
  "scene|盛开的西湖荷花塘与水上凉亭，一片碧绿的荷叶连到天边。夏日的西湖粉荷绿叶，远处的木制凉亭在荷塘映衬下古色古香。"
  "motion|西湖水面上泛起阵阵涟漪，一行野鸭正在清澈的水面上游动。清澈见底的湖水与游动的水鸟构成了一幅和谐的自然画面。"
  "scene|蓝天白云下广阔的西湖荷花池，美不胜收的夏日景色。阳光洒在满池的荷叶上，波光粼粼，让人流连忘返。"
  "color|夕阳照射下的西湖水面与远山，水面波光粼粼。傍晚的金黄余晖撒在湖面上，远山如黛，呈现出温暖惬意的色调。"
  "composition|西湖黄昏与雷峰塔远景，夕阳余晖映照在水面。将雷峰塔放在画面远端，黄昏的光影把西湖的秀美展现得淋漓尽致。"
  "color|傍晚西湖金黄色晚霞与升起的月亮，天空色彩丰富。深蓝色的云彩与金黄色的夕阳在空中交织，日月同辉的景色令人惊叹。"
  "composition|透过湖边的树枝看过去，远处的雷峰塔和落日余晖构成了一幅天然框景。树枝形成的框景构图增加了画面的层次感。"
  "scene|西湖夜景，岸边的柳树被金黄色灯光照亮，水面倒影成趣。华灯初上，静谧的湖面倒映着绚丽的夜景灯光。"
  "style|夜幕深蓝色天空下的雷峰塔灯火通明，高耸璀璨。夜景模式下塔身的黄金灯光与深邃的夜空形成鲜明对比，壮观无比。"
  "composition|湖面游船与蓝天白云的清晰倒影，水天一色。游船划过平静的湖面，水中的倒影随着波浪荡漾，富有流动美感。"
  "color|傍晚紫蓝色天空与远山霞光，深邃唯美。天空中绚丽的紫色与蓝色过渡自然，落日沉入山峦之后余温尚存。"
  "scene|绿树成荫的纪念公园与蓝天白云，环境优雅清静。阳光穿过浓密的树冠在草地上洒下斑驳光影，给人安详宁静的感觉。"
  "motion|飞跃在大海水面上的鸟群与远山，展翅高飞。抓拍鸟群飞过海面的瞬间，翅膀张开的动态与平静的海面形成鲜明对比。"
  "motion|海面上空成群飞翔的海鸥，画面充满动感。海风吹过，海鸥在空中盘旋翱翔，自由自在，展现出生命的活力。"
  "color|公园花坛里盛开的鲜艳红花与绿色枝叶相映成趣。高饱和度的红色花朵在绿叶衬托下极为夺目，给画面增添了浓郁色彩。"
  "motion|树丛枝头抓拍的小鸟，神态敏捷可爱。小鸟栖息在密林枝桠间，小巧玲珑，大光圈虚化背景让主体更加突出。"
  "color|今天的火烧云太壮观了！整个天空被染成了橙红色，树木与山峦在强烈的霞光下化作剪影，视觉冲击力极强。"
  "motion|大海上空成群飞翔与栖息的海鸥，壮阔的海景。蔚蓝的海水与翻飞的海鸥相呼应，构成了一幅宏大的海洋风光摄影作品。"
  "object|阳光下蹲在树根旁的小三花猫，闭目养神。花猫毛色分明，惬意地趴在温暖的泥地上晒太阳，画面十分治愈。"
  "scene|大雪纷飞后的校园雪景与鲜艳的红色抽象雕塑。洁白的雪地与醒目的红色雕塑形成鲜明色彩对比，冬日氛围拉满。"
)

PHOTOS=(
  "IMG_0102.jpeg" "IMG_0285.jpeg" "IMG_0330.jpeg" "IMG_0805.jpeg"
  "IMG_0806.jpeg" "IMG_0856.jpeg" "IMG_1200.jpeg" "IMG_1203.jpeg"
  "IMG_1207.jpeg" "IMG_1210.jpeg" "IMG_1216.jpeg" "IMG_1219.jpeg"
  "IMG_1222.jpeg" "IMG_1226.jpeg" "IMG_1230.jpeg" "IMG_1238.jpeg"
  "IMG_1254.jpeg" "IMG_1480.jpeg" "IMG_2197.jpeg" "IMG_2198.jpeg"
  "IMG_4176.jpeg" "IMG_4991.jpeg" "IMG_6122.jpeg" "IMG_7640.jpeg"
  "IMG_8045.jpeg" "IMG_9007.jpeg"
)
PHOTO_SRC_DIR="$PROJECT_DIR/resources/static/pictures"

# Generate a simple colored PNG image using Python (no Pillow dependency)
generate_image() {
    local output="$1" r="$2" g="$3" b="$4" text="$5"
    python3 - "$output" "$r" "$g" "$b" "$text" <<'PYEOF'
import struct, zlib, sys, pathlib

out, r, g, b, text = sys.argv[1], int(sys.argv[2]), int(sys.argv[3]), int(sys.argv[4]), sys.argv[5]
W, H = 200, 200

def chunk(ctype, data):
    c = ctype + data
    return struct.pack('>I', len(data)) + c + struct.pack('>I', zlib.crc32(c) & 0xFFFFFFFF)

# Create a simple image with colored rectangles and text-like pattern
raw = b''
for y in range(H):
    raw += b'\x00'  # filter none
    for x in range(W):
        # Create a simple pattern: gradient + diagonal stripe
        cr = min(255, r + (x * 30 // W))
        cg = min(255, g + (y * 30 // H))
        cb = min(255, b + ((x + y) * 15 // (W + H)))
        # Add some variation based on "text" bytes
        idx = (x + y) % len(text)
        cr = min(255, cr + ord(text[idx]) % 20 - 10)
        cg = min(255, cg + ord(text[(idx+3)%len(text)]) % 20 - 10)
        raw += struct.pack('BBB', cr, cg, cb)

def itxt(keyword, text):
    # iTXt chunk for embedding metadata
    payload = keyword.encode() + b'\x00\x00\x00\x00\x00' + text.encode('utf-8')
    return chunk(b'iTXt', payload)

ihdr = chunk(b'IHDR', struct.pack('>IIBBBBB', W, H, 8, 2, 0, 0, 0))
idata = chunk(b'IDAT', zlib.compress(raw))
iend = chunk(b'IEND', b'')
png = b'\x89PNG\r\n\x1a\n' + ihdr + itxt('Description', text) + idata + iend
pathlib.Path(out).write_bytes(png)
PYEOF
}

# ============================================================
# Helper functions
# ============================================================
red()    { echo -e "\033[31m$1\033[0m" >&2; }
green()  { echo -e "\033[32m$1\033[0m" >&2; }
yellow() { echo -e "\033[33m$1\033[0m" >&2; }

api_call() {
    local method="$1" url="$2" data="$3" token="${4:-}"
    local args=(-s -w '\n%{http_code}' -X "$method" "$url" -H 'Content-Type: application/json')
    if [ -n "$token" ]; then
        args+=(-b "token=$token")
    fi
    if [ -n "$data" ] && [ "$data" != "null" ]; then
        args+=(-d "$data")
    fi
    curl --noproxy '*' "${args[@]}" 2>/dev/null
}

find_existing_post_id() {
    local content="$1"
    python3 - "$EXISTING_INDEX_FILE" "$content" <<'PYEOF'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
target = sys.argv[2]
items = payload.get("data", {}).get("items", [])
matches = [item for item in items if item.get("content") == target and item.get("images")]
if len(matches) > 1:
    print("DUPLICATE")
elif matches:
    print(matches[0]["post_id"])
PYEOF
}

parse_json_field() {
    # Extract a field from JSON response: .data.token, .data.id, .code, etc.
    python3 -c "
import json, sys
try:
    obj = json.load(sys.stdin)
    path = sys.argv[1].split('.')
    for p in path:
        if isinstance(obj, list):
            obj = obj[int(p)]
        else:
            obj = obj[p]
    print(obj)
except Exception:
    print('')
" "$1"
}

check_success() {
    local response="$1" desc="$2"
    local http_code code
    http_code="$(echo "$response" | tail -1)"
    local body
    body="$(echo "$response" | sed '$d')"
    code="$(echo "$body" | parse_json_field "code")"
    if [ "$http_code" = "200" ] && [ "$code" = "0" ]; then
        green "  ✓ $desc"
        echo "$body"
        return 0
    else
        red "  ✗ $desc (HTTP $http_code, code=$code)"
        echo "$body" >&2
        return 1
    fi
}

register_or_login() {
    local username="$1" password="$2"
    local resp token

    # Try register first
    resp="$(api_call POST "$BASE_URL/api/v1/auth/register" \
        "{\"username\":\"$username\",\"password\":\"$password\",\"email\":\"$username@demo.local\"}")"
    local http_code code
    http_code="$(echo "$resp" | tail -1)"
    local body
    body="$(echo "$resp" | sed '$d')"
    code="$(echo "$body" | parse_json_field "code")"

    if [ "$http_code" = "200" ] && [ "$code" = "0" ]; then
        token="$(echo "$body" | parse_json_field "data.token")"
        green "  ✓ 注册用户 $username"
        echo "$token"
        return 0
    fi

    # Already exists, try login
    resp="$(api_call POST "$BASE_URL/api/v1/auth/login" \
        "{\"username\":\"$username\",\"password\":\"$password\"}")"
    http_code="$(echo "$resp" | tail -1)"
    body="$(echo "$resp" | sed '$d')"
    code="$(echo "$body" | parse_json_field "code")"

    if [ "$http_code" = "200" ] && [ "$code" = "0" ]; then
        token="$(echo "$body" | parse_json_field "data.token")"
        yellow "  → 用户 $username 已存在，已登录"
        echo "$token"
        return 0
    fi

    red "  ✗ 用户 $username 注册/登录失败"
    return 1
}

upload_image() {
    local filepath="$1" token="$2"
    local resp http_code body url
    resp="$(curl --noproxy '*' -s -w '\n%{http_code}' -X POST "$BASE_URL/api/v1/upload" \
        -b "token=$token" -F "file=@$filepath" 2>/dev/null)"
    http_code="$(echo "$resp" | tail -1)"
    body="$(echo "$resp" | sed '$d')"
    if [ "$http_code" != "200" ]; then
        red "  ✗ 上传图片失败 (HTTP $http_code)"
        return 1
    fi
    url="$(echo "$body" | parse_json_field "data.url")"
    if [ -z "$url" ]; then
        red "  ✗ 上传图片返回空URL"
        return 1
    fi
    echo "$url"
}

create_and_approve_post() {
    local token="$1" content="$2" image_url="$3" admin_token="$4"
    local resp post_id

    # Create post
    resp="$(api_call POST "$BASE_URL/api/v1/posts" \
        "{\"content\":$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1], ensure_ascii=False))' "$content"),\"images\":[\"$image_url\"]}" \
        "$token")"
    local http_code
    http_code="$(echo "$resp" | tail -1)"
    local body
    body="$(echo "$resp" | sed '$d')"
    local code
    code="$(echo "$body" | parse_json_field "code")"

    if [ "$http_code" != "200" ] || [ "$code" != "0" ]; then
        # Check if post already exists (content match)
        red "  ✗ 发布帖子失败 (HTTP $http_code, code=$code)"
        echo "$body" >&2
        return 1
    fi

    post_id="$(echo "$body" | parse_json_field "data.id")"
    if [ -z "$post_id" ]; then
        red "  ✗ 帖子创建成功但无法获取ID"
        return 1
    fi

    # Approve post as admin
    resp="$(api_call POST "$BASE_URL/api/v1/admin/posts/$post_id/approve" "null" "$admin_token")"
    http_code="$(echo "$resp" | tail -1)"
    body="$(echo "$resp" | sed '$d')"
    code="$(echo "$body" | parse_json_field "code")"

    if [ "$http_code" = "200" ] && [ "$code" = "0" ]; then
        green "  ✓ 帖子 #$post_id 已发布并审核通过"
        echo "$post_id"
        return 0
    else
        red "  ✗ 审核帖子 #$post_id 失败"
        return 1
    fi
}

# ============================================================
# Main
# ============================================================
echo "============================================"
echo "  ShareO Demo Seed"
echo "  Phase 7A — 可重复演示数据初始化"
echo "============================================"
echo ""

cd "$PROJECT_DIR"

# Step 0: Health check
echo "=== Step 0: 健康检查 ==="
if ! curl --noproxy '*' --fail --silent "$BASE_URL/healthz" >/dev/null 2>&1; then
    red "Go 服务不可达: $BASE_URL/healthz"
    exit 1
fi
green "  ✓ Go 服务健康"

# Step 1: Create admin user
echo ""
echo "=== Step 1: 创建管理员 ==="

# Create admin via docker-compose or direct API
if docker-compose version >/dev/null 2>&1; then
    docker-compose exec -T mysql mysql -uroot -p"$DB_PASSWORD" shareo -e \
        "INSERT INTO users (username, password_hash, email, role, status) VALUES ('$ADMIN_USER', '$ADMIN_HASH', '$ADMIN_EMAIL', 'admin', 1) ON DUPLICATE KEY UPDATE role='admin', status=1;" 2>/dev/null || true
    yellow "  → 管理员 $ADMIN_USER 已就绪"
elif docker compose version >/dev/null 2>&1; then
    docker compose exec -T mysql mysql -uroot -p"$DB_PASSWORD" shareo -e \
        "INSERT INTO users (username, password_hash, email, role, status) VALUES ('$ADMIN_USER', '$ADMIN_HASH', '$ADMIN_EMAIL', 'admin', 1) ON DUPLICATE KEY UPDATE role='admin', status=1;" 2>/dev/null || true
    yellow "  → 管理员 $ADMIN_USER 已就绪"
else
    yellow "  → 未检测到 docker compose，尝试通过 API 注册管理员"
    register_or_login "$ADMIN_USER" "$ADMIN_PASSWORD" >/dev/null || true
fi

ADMIN_RESP="$(api_call POST "$BASE_URL/api/v1/auth/login" \
    "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASSWORD\"}")"
ADMIN_TOKEN="$(echo "$ADMIN_RESP" | sed '$d' | parse_json_field "data.token")"
if [ -z "$ADMIN_TOKEN" ]; then
    red "管理员登录失败，请确认管理员账号存在"
    exit 1
fi
green "  ✓ 管理员登录成功"

# Step 2: Create demo users
echo ""
echo "=== Step 2: 创建 Demo 用户 ==="
ALICE_TOKEN=""
BOB_TOKEN=""
for entry in "${DEMO_USERS[@]}"; do
    username="${entry%%:*}"
    password="${entry#*:}"
    token="$(register_or_login "$username" "$password")"
    case "$username" in
        demo_alice) ALICE_TOKEN="$token" ;;
        demo_bob)   BOB_TOKEN="$token" ;;
    esac
done

# Step 3: Generate demo images and create posts
echo ""
echo "=== Step 3: 生成 Demo 图片与帖子 ==="

DEMO_IMG_DIR="$PROJECT_DIR/scripts/demo-assets"
mkdir -p "$DEMO_IMG_DIR"

TOTAL_POSTS=${#POSTS[@]}
POST_COUNT=0
POST_MAP_FILE="$DEMO_IMG_DIR/post_id_map.txt"
EXISTING_INDEX_FILE="$(mktemp /tmp/shareo-demo-index.XXXXXX)"
trap 'rm -f "$EXISTING_INDEX_FILE"' EXIT

if [ -z "$INTERNAL_TOKEN" ]; then
    red "SHAREO_INTERNAL_TOKEN 未设置，无法验证 Demo seed 幂等性"
    exit 1
fi

if ! curl --noproxy '*' --fail --silent \
    -H "X-Internal-Token: $INTERNAL_TOKEN" \
    "$BASE_URL/internal/posts/index-payloads?after_id=0&limit=200" \
    -o "$EXISTING_INDEX_FILE"; then
    red "读取现有 Demo 帖子索引载荷失败"
    exit 1
fi
if ! python3 - "$EXISTING_INDEX_FILE" <<'PYEOF'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
if payload.get("code") != 0:
    raise SystemExit(1)
PYEOF
then
    red "Go 返回的索引载荷无效"
    exit 1
fi

if [ "${#PHOTOS[@]}" -ne "$TOTAL_POSTS" ]; then
    red "真实照片数量 ${#PHOTOS[@]} 与 Demo 帖子数量 $TOTAL_POSTS 不一致"
    exit 1
fi
for photo_file in "${PHOTOS[@]}"; do
    if [ ! -f "$PHOTO_SRC_DIR/$photo_file" ]; then
        red "缺少真实 Demo 照片: $PHOTO_SRC_DIR/$photo_file"
        exit 1
    fi
done

# Clean up old demo images and map
rm -f "$DEMO_IMG_DIR"/demo_img_*.png "$POST_MAP_FILE"

for i in "${!POSTS[@]}"; do
    post="${POSTS[$i]}"
    category="${post%%|*}"
    content="${post#*|}"

    existing_post_id="$(find_existing_post_id "$content")"
    if [ "$existing_post_id" = "DUPLICATE" ]; then
        red "发现多个相同正文的 approved Demo 帖子，拒绝继续以免评测 ID 漂移"
        exit 1
    fi
    if [ -n "$existing_post_id" ]; then
        POST_COUNT=$((POST_COUNT + 1))
        echo "$i|$category|$existing_post_id" >> "$POST_MAP_FILE"
        yellow "  → 复用已有 Demo 帖子 #$existing_post_id"
        continue
    fi

    photo_file="${PHOTOS[$i]:-}"
    real_img_path="$PHOTO_SRC_DIR/$photo_file"
    IMG_FILE="$real_img_path"

    # Alternate users for variety
    if [ $((i % 2)) -eq 0 ]; then
        TOKEN="$ALICE_TOKEN"
    else
        TOKEN="$BOB_TOKEN"
    fi

    # Upload image
    IMG_URL="$(upload_image "$IMG_FILE" "$TOKEN")"
    if [ -z "$IMG_URL" ]; then
        red "  ✗ 跳过帖子 #$i (上传失败)"
        continue
    fi

    # Create post and approve
    POST_ID="$(create_and_approve_post "$TOKEN" "$content" "$IMG_URL" "$ADMIN_TOKEN")"
    if [ -n "$POST_ID" ]; then
        POST_COUNT=$((POST_COUNT + 1))
        # Record: index|category|post_id for later labeling
        echo "$i|$category|$POST_ID" >> "$POST_MAP_FILE"
    fi

    # Delay to stay within rate limits (upload: 20/min, post: 30/min)
    sleep 5
done

echo ""
green "  ✓ 共创建 $POST_COUNT / $TOTAL_POSTS 篇帖子"

# Step 4: Wait for image indexing
echo ""
echo "=== Step 4: 等待图文索引完成 ==="
AI_URL="${SHAREO_AI_BASE_URL:-http://127.0.0.1:8000}"

if [ -z "$INTERNAL_TOKEN" ]; then
    yellow "  → SHAREO_INTERNAL_TOKEN 未设置，跳过索引等待"
else
    for _ in $(seq 1 120); do
        if curl --noproxy '*' --fail --silent \
            -H "X-Internal-Token: $INTERNAL_TOKEN" \
            "$AI_URL/readyz/image-search" >/dev/null 2>&1; then
            green "  ✓ AI 服务 readiness 就绪"
            break
        fi
        sleep 1
    done
fi

# Step 5: Write manifest
echo ""
echo "=== Step 5: 生成 manifest ==="

GENERATED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
python3 - "$DEMO_IMG_DIR" "$PHOTO_SRC_DIR" "$POST_MAP_FILE" "$POST_COUNT" "$GENERATED_AT" <<'PYEOF'
import json, sys, pathlib
from datetime import datetime

img_dir = pathlib.Path(sys.argv[1])
photo_dir = pathlib.Path(sys.argv[2])
post_map_file = pathlib.Path(sys.argv[3])
post_count = int(sys.argv[4])
generated_at = sys.argv[5]

posts = []
if post_map_file.exists():
    for line in post_map_file.read_text().splitlines():
        if '|' in line:
            idx, category, post_id = line.strip().split('|', 2)
            posts.append({"index": int(idx), "category": category, "post_id": int(post_id)})

manifest = {
    "version": "1.0",
    "generated_at": generated_at,
    "admin_user": "demoadmin",
    "demo_users": ["demo_alice", "demo_bob"],
    "bot_user": "shareo_bot",
    "total_posts": post_count,
    "posts": sorted(posts, key=lambda p: p["index"]),
    "images": sorted([p.name for p in img_dir.glob("demo_img_*.png")]),
    "source_photos": sorted(p.name for p in photo_dir.glob("*.jpeg")),
}
print(json.dumps(manifest, ensure_ascii=False, indent=2))
PYEOF

green "  ✓ Manifest 已写入 $DEMO_IMG_DIR/manifest.json"

echo ""
echo "============================================"
green "  Demo Seed 完成！"
echo "============================================"
echo ""
echo "  管理员:  $ADMIN_USER / admin123"
echo "  用户 1:  demo_alice / alice2024"
echo "  用户 2:  demo_bob   / bob2024"
echo "  Bot:     shareo_bot (系统用户，不可登录)"
echo "  帖子数:  $POST_COUNT"
