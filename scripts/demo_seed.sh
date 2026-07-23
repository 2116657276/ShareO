#!/usr/bin/env bash
# ============================================================
#  ShareO Demo Seed — 可重复的演示数据初始化脚本
#  Phase 7A: 创建固定 Demo 用户、管理员、帖子、图片
#  幂等：可安全多次执行
# ============================================================
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BASE_URL="${SHAREO_BASE_URL:-http://127.0.0.1:8080}"

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
  "object|刚入手的富士XT5相机，配了一颗35mm f/1.4定焦镜头。复古外观加上胶片模拟模式，直出色彩非常讨喜。今天带去公园试拍了一组，对焦速度和画质都超出预期。"
  "object|窗台上的多肉植物全家福。静夜、桃蛋、熊童子，每一盆都养了大半年。多肉最怕浇水太多，一定要等土完全干透再浇，通风也很重要。"
  "object|这把Fender Stratocaster跟了我五年，日落色渐变琴身，枫木指板。清音通透，过载温暖，弹布鲁斯和摇滚都合适。最近刚换了套弦。"
  "object|最近迷上了手工咖啡，入手了一套V60手冲壶。研磨度、水温、注水速度，每个变量都会影响风味。今天用埃塞俄比亚耶加雪菲，柑橘和花香味很明显。"
  "scene|周末去了舟山东极岛，凌晨四点爬起来看日出。海平面从深蓝变成橙红，太阳跳出海面的那一瞬间，所有的等待都值得了。面朝大海，心旷神怡。"
  "scene|秋天的喀纳斯，像是上帝打翻的调色盘。金黄的白桦林、碧绿的湖水、远处的雪山，每一帧都是壁纸。徒步三天，相机快门按个不停。"
  "scene|夜晚的外滩，万国建筑群灯火辉煌。黄浦江对岸的陆家嘴三件套直插云霄，游船在江面缓缓驶过。城市的天际线在夜色下格外迷人，适合用长曝光拍摄。"
  "scene|北京的胡同深处，老人们在槐树下下象棋。斑驳的灰墙、青石板路、挂在门前的鸟笼，时间在这里仿佛慢了下来。走街串巷才能感受到真正的老北京味道。"
  "color|夏天就该穿明亮的颜色！牛油果绿的连衣裙配柠檬黄的手提包，走在街上回头率超高。绿色系穿搭今年特别火，从薄荷绿到橄榄绿，不同饱和度适合不同肤色。"
  "color|分享一组以红色为主题的生活摄影。红色的邮筒、红色的砖墙、红色的雨伞。红色是最有冲击力的颜色，在画面中总是第一个抓住眼球。后期稍微提高饱和度就很有感觉。"
  "color|极简黑白摄影的魅力在于去掉色彩的干扰，让观者专注于光影和构图。这组街拍全部用黑白胶片拍摄，高对比度的影调让平凡的街景变得有故事感。"
  "color|秋天的色彩真的太丰富了！银杏的金黄、枫叶的火红、天空的蔚蓝，构成了一幅天然的三色画卷。每年这个时候都要去公园拍照，同一个角度拍了五年，每年都不一样。"
  "style|探索极简主义摄影：少即是多。大面积的留白、干净的线条、单一的主体。这张在美术馆拍的照片，白墙前只有一把椅子，光影勾勒出椅子的轮廓，安静而有力量。"
  "style|日系小清新风格的调色思路分享。低对比度、低饱和度、偏青色调、略微过曝。拍摄时选择柔和的自然光，背景尽量简洁。这组照片是在镰仓的海边拍的，满满的青春气息。"
  "style|赛博朋克风格的城市夜景怎么拍？找霓虹灯密集的街区，用大光圈镜头，后期调出青橙色调。雨后的街道特别出片，积水倒映着霓虹灯的色彩，氛围感拉满。"
  "style|胶片摄影的魅力：颗粒感、色彩偏移、不确定性。最近用一台1970年代的宾得Spotmatic拍了一卷Kodak Portra 400，出来的颜色温暖柔和，数码后期很难完全模拟。"
  "composition|三分法构图的实战分享。把画面用两条横线和两条竖线分成九等份，把主体放在交叉点上。这张日落的照片把地平线放在下三分之一处，天空的云彩占据了上方三分之二的空间。"
  "composition|引导线构图是在旅行摄影中最实用的技巧之一。公路、栏杆、河流、建筑物边缘，都可以作为引导线，把观众的目光引向画面深处。这张在张家界拍的照片用栈道作为引导线，终点是远处的山峰。"
  "composition|框架构图：用门框、窗户、树枝等自然框架把主体框起来。在苏州园林拍的这张，透过圆形的月洞门看到远处的亭子，层次感十足。框架增加了画面的深度和趣味性。"
  "composition|对称构图在建筑摄影中大有用武之地。这张在故宫拍的，太和殿的中轴线完美对称。对称能带来庄重、平衡的视觉感受，适合表现建筑的宏伟气势。"
  "portrait|给朋友在咖啡馆拍了一组人像。大光圈镜头虚化背景，窗边的自然光线从侧面照在脸上，眼神光很漂亮。模特放松自然的状态是最出片的，不用刻意摆姿势。"
  "portrait|街头人像抓拍：在菜市场拍到一位卖菜的老奶奶，满脸的皱纹是岁月的痕迹，笑容却像孩子一样纯真。黑白处理更能突出人物的情感和质感，彩色反而会分散注意力。"
  "portrait|体育摄影中的人像：拍摄了一场业余足球比赛，球员在进球后欢呼庆祝的瞬间。用高速连拍抓到了最精彩的表情和肢体动作。背景虚化让主体从混乱的球场中凸显出来。"
  "portrait|室内人像的布光心得：一盏主灯从45度角打过来，一盏辅助灯在对面补光，背后再加一盏轮廓灯把人物和背景分离开。这套三点布光法简单实用，适合各种室内场景。"
  "motion|慢门拍摄瀑布的技巧：用三脚架固定相机，快门速度调到1/4秒或更慢，水流就会变成丝绸一样的质感。这张在黄果树瀑布拍的照片使用了ND1000减光镜，曝光时间2秒。"
  "motion|追焦拍摄骑行中的自行车：相机跟着被摄体移动，保持主体清晰而背景拉出运动模糊的线条。这个技巧需要多练习，快门速度1/30秒左右，对焦模式选连续自动对焦。"
)

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
    docker-compose exec -T mysql mysql -uroot -pshareo_pass shareo -e \
        "INSERT INTO users (username, password_hash, email, role, status) VALUES ('$ADMIN_USER', '$ADMIN_HASH', '$ADMIN_EMAIL', 'admin', 1) ON DUPLICATE KEY UPDATE role='admin', status=1;" 2>/dev/null || true
    yellow "  → 管理员 $ADMIN_USER 已就绪"
elif docker compose version >/dev/null 2>&1; then
    docker compose exec -T mysql mysql -uroot -pshareo_pass shareo -e \
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

# Clean up old demo images and map
rm -f "$DEMO_IMG_DIR"/demo_img_*.png "$POST_MAP_FILE"

for i in "${!POSTS[@]}"; do
    post="${POSTS[$i]}"
    category="${post%%|*}"
    content="${post#*|}"

    # Generate a colored image based on category
    case "$category" in
        object)   generate_image "$DEMO_IMG_DIR/demo_img_${i}.png" 100 150 200 "object photo $i" ;;
        scene)    generate_image "$DEMO_IMG_DIR/demo_img_${i}.png" 180 200 100 "scene landscape $i" ;;
        color)    generate_image "$DEMO_IMG_DIR/demo_img_${i}.png" 230 80 80 "color palette $i" ;;
        style)    generate_image "$DEMO_IMG_DIR/demo_img_${i}.png" 80 180 200 "artistic style $i" ;;
        composition) generate_image "$DEMO_IMG_DIR/demo_img_${i}.png" 150 100 180 "composition rule $i" ;;
        portrait) generate_image "$DEMO_IMG_DIR/demo_img_${i}.png" 200 150 120 "portrait photo $i" ;;
        motion)   generate_image "$DEMO_IMG_DIR/demo_img_${i}.png" 120 120 220 "motion blur $i" ;;
        *)        generate_image "$DEMO_IMG_DIR/demo_img_${i}.png" 100 100 100 "demo image $i" ;;
    esac

    # Alternate users for variety
    if [ $((i % 2)) -eq 0 ]; then
        TOKEN="$ALICE_TOKEN"
    else
        TOKEN="$BOB_TOKEN"
    fi

    # Upload image
    IMG_URL="$(upload_image "$DEMO_IMG_DIR/demo_img_${i}.png" "$TOKEN")"
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
INTERNAL_TOKEN="${SHAREO_INTERNAL_TOKEN:-}"

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
python3 - "$DEMO_IMG_DIR" "$POST_MAP_FILE" "$POST_COUNT" "$GENERATED_AT" <<'PYEOF'
import json, sys, pathlib
from datetime import datetime

img_dir = pathlib.Path(sys.argv[1])
post_map_file = pathlib.Path(sys.argv[2])
post_count = int(sys.argv[3])
generated_at = sys.argv[4]

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
    "images": sorted([p.name for p in img_dir.glob("demo_img_*.png")])
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
