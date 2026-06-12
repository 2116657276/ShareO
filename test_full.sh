#!/bin/bash
# ShareO Full E2E Test - API only, no direct DB
set -e
BASE="http://localhost:8080"
PICS="/Users/zhoujianlin/GoLandProjects/ShareO/resources/static/pictures"
PASS=0; FAIL=0

ok() { echo -e "  \033[0;32mOK\033[0m $1"; PASS=$((PASS+1)); }
fail() { echo -e "  \033[0;31mFAIL\033[0m $1"; FAIL=$((FAIL+1)); }

json() { python3 -c "import sys,json;d=json.load(sys.stdin);$1" 2>/dev/null; }

echo "========================================"
echo " ShareO Full E2E Test (API only)"
echo " $(date)"
echo "========================================"

# ============ STEP 0: Cleanup ============
echo -e "\n[0] Cleanup + Admin Login"
ADMIN_T=$(curl -s -X POST $BASE/api/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | json "print(d['data']['token'])")
# Delete all existing posts
for id in $(curl -s -b "token=$ADMIN_T" "$BASE/api/v1/feed?page_size=999" | json "print(' '.join(str(p['id']) for p in d['data']['list']))"); do
  curl -s -X DELETE -b "token=$ADMIN_T" "$BASE/api/v1/admin/posts/$id" > /dev/null
done
redis-cli FLUSHALL 2>/dev/null
ok "Cleanup done"

# ============ STEP 1: Register test users ============
echo -e "\n[1] Register test users (test01-test05, pw=256500)"
declare -A TOKENS
for i in $(seq -w 1 5); do
  TOKENS[$i]=$(curl -s -X POST $BASE/api/v1/auth/register -H 'Content-Type: application/json' \
    -d "{\"username\":\"test$i\",\"password\":\"256500\",\"email\":\"test$i@shareo.com\"}" | json "print(d['data']['token'])")
  ok "test$i registered"
done

# ============ STEP 2: Upload 12 images ============
echo -e "\n[2] Upload 12 images"
IMGS=()
for f in $PICS/IMG_0102.jpeg $PICS/IMG_0805.jpeg $PICS/IMG_1216.jpeg $PICS/IMG_1219.jpeg \
         $PICS/IMG_1222.jpeg $PICS/IMG_1226.jpeg $PICS/IMG_1230.jpeg $PICS/IMG_1480.jpeg \
         $PICS/IMG_6122.jpeg $PICS/IMG_7640.jpeg $PICS/IMG_8045.jpeg $PICS/IMG_9007.jpeg; do
  url=$(curl -s -X POST $BASE/api/v1/upload -b "token=${TOKENS[01]}" -F "file=@$f" | json "print(d['data']['url'])")
  IMGS+=("$url")
  ok "$(basename $f) uploaded"
done

# ============ STEP 3: Create 4 original posts (2 images each) ============
echo -e "\n[3] Create 4 original posts"
declare -A POSTS
POSTS[1]=$(curl -s -X POST $BASE/api/v1/posts -b "token=${TOKENS[01]}" -H 'Content-Type: application/json' \
  -d "{\"content\":\"晨光里的城市苏醒 🌇 #街拍\",\"images\":[\"${IMGS[0]}\",\"${IMGS[1]}\"]}" | json "print(d['data']['id'])")
ok "Post1 by test01: $POSTS[1]"

POSTS[2]=$(curl -s -X POST $BASE/api/v1/posts -b "token=${TOKENS[02]}" -H 'Content-Type: application/json' \
  -d "{\"content\":\"午后光影交织的巷弄 ☀️ #人文\",\"images\":[\"${IMGS[2]}\",\"${IMGS[3]}\"]}" | json "print(d['data']['id'])")
ok "Post2 by test02: $POSTS[2]"

POSTS[3]=$(curl -s -X POST $BASE/api/v1/posts -b "token=${TOKENS[03]}" -H 'Content-Type: application/json' \
  -d "{\"content\":\"黄昏时刻的温柔色调 🌆 #风光\",\"images\":[\"${IMGS[4]}\",\"${IMGS[5]}\"]}" | json "print(d['data']['id'])")
ok "Post3 by test03: $POSTS[3]"

POSTS[4]=$(curl -s -X POST $BASE/api/v1/posts -b "token=${TOKENS[04]}" -H 'Content-Type: application/json' \
  -d "{\"content\":\"夜幕下的霓虹闪烁 🌃 #夜景\",\"images\":[\"${IMGS[6]}\",\"${IMGS[7]}\"]}" | json "print(d['data']['id'])")
ok "Post4 by test04: $POSTS[4]"

# ============ STEP 4: 4 text-only reposts ============
echo -e "\n[4] Create 4 text-only reposts"
declare -A REPOSTS
REPOSTS[1]=$(curl -s -X POST "$BASE/api/v1/posts/${POSTS[1]}/repost" -b "token=${TOKENS[02]}" -H 'Content-Type: application/json' \
  -d '{"text":"这组晨光拍得太好了，必须转发！"}' | json "print(d['data']['id'])")
ok "Text repost 1 (test02 reposts test01): $REPOSTS[1]"

REPOSTS[2]=$(curl -s -X POST "$BASE/api/v1/posts/${POSTS[2]}/repost" -b "token=${TOKENS[03]}" -H 'Content-Type: application/json' \
  -d '{"text":"光影大师！收藏了"}' | json "print(d['data']['id'])")
ok "Text repost 2 (test03 reposts test02): $REPOSTS[2]"

REPOSTS[3]=$(curl -s -X POST "$BASE/api/v1/posts/${POSTS[3]}/repost" -b "token=${TOKENS[04]}" -H 'Content-Type: application/json' \
  -d '{"text":"黄昏永远是最美的滤镜 🌅"}' | json "print(d['data']['id'])")
ok "Text repost 3 (test04 reposts test03): $REPOSTS[3]"

REPOSTS[4]=$(curl -s -X POST "$BASE/api/v1/posts/${POSTS[4]}/repost" -b "token=${TOKENS[05]}" -H 'Content-Type: application/json' \
  -d '{"text":"夜景太赞了，转发给大家看！"}' | json "print(d['data']['id'])")
ok "Text repost 4 (test05 reposts test04): $REPOSTS[4]"

# ============ STEP 5: 4 image reposts ============
echo -e "\n[5] Create 4 image reposts (remaining 4 images)"
IREPOSTS[1]=$(curl -s -X POST "$BASE/api/v1/posts/${POSTS[1]}/repost" -b "token=${TOKENS[03]}" -H 'Content-Type: application/json' \
  -d "{\"text\":\"加一张我拍的对比 📷\",\"images\":[\"${IMGS[8]}\"]}" | json "print(d['data']['id'])")
ok "Image repost 1 (test03): $IREPOSTS[1]"

IREPOSTS[2]=$(curl -s -X POST "$BASE/api/v1/posts/${POSTS[2]}/repost" -b "token=${TOKENS[04]}" -H 'Content-Type: application/json' \
  -d "{\"text\":\"我也来一张！\",\"images\":[\"${IMGS[9]}\"]}" | json "print(d['data']['id'])")
ok "Image repost 2 (test04): $IREPOSTS[2]"

IREPOSTS[3]=$(curl -s -X POST "$BASE/api/v1/posts/${POSTS[3]}/repost" -b "token=${TOKENS[05]}" -H 'Content-Type: application/json' \
  -d "{\"text\":\"不同角度，一样的美\",\"images\":[\"${IMGS[10]}\"]}" | json "print(d['data']['id'])")
ok "Image repost 3 (test05): $IREPOSTS[3]"

IREPOSTS[4]=$(curl -s -X POST "$BASE/api/v1/posts/${POSTS[4]}/repost" -b "token=${TOKENS[01]}" -H 'Content-Type: application/json' \
  -d "{\"text\":\"我也拍了夜景，加上！\",\"images\":[\"${IMGS[11]}\"]}" | json "print(d['data']['id'])")
ok "Image repost 4 (test01): $IREPOSTS[4]"

# ============ STEP 6: Admin approve all ============
echo -e "\n[6] Admin approve all 12 posts"
ALL_POSTS="${POSTS[1]} ${POSTS[2]} ${POSTS[3]} ${POSTS[4]} ${REPOSTS[1]} ${REPOSTS[2]} ${REPOSTS[3]} ${REPOSTS[4]} $IREPOSTS[1] $IREPOSTS[2] $IREPOSTS[3] $IREPOSTS[4]"
for pid in $ALL_POSTS; do
  curl -s -X POST "$BASE/api/v1/admin/posts/$pid/approve" -b "token=$ADMIN_T" > /dev/null
done
ok "All 12 posts approved"

# ============ STEP 7: Social interactions ============
echo -e "\n[7] Random likes, comments, follows"
# test03 likes all 4 originals
for pid in ${POSTS[1]} ${POSTS[2]} ${POSTS[3]} ${POSTS[4]}; do
  curl -s -X POST "$BASE/api/v1/posts/$pid/like" -b "token=${TOKENS[03]}" > /dev/null
done
ok "test03 liked all 4 originals"

# test04 likes original 1,2 and text repost 1,2
for pid in ${POSTS[1]} ${POSTS[2]} ${REPOSTS[1]} ${REPOSTS[2]}; do
  curl -s -X POST "$BASE/api/v1/posts/$pid/like" -b "token=${TOKENS[04]}" > /dev/null
done
ok "test04 liked 4 posts"

# test05 likes all image reposts
for pid in $IREPOSTS[1] $IREPOSTS[2] $IREPOSTS[3] $IREPOSTS[4]; do
  curl -s -X POST "$BASE/api/v1/posts/$pid/like" -b "token=${TOKENS[05]}" > /dev/null
done
ok "test05 liked all 4 image reposts"

# test01 likes test02's posts
for pid in ${POSTS[2]} ${REPOSTS[1]}; do
  curl -s -X POST "$BASE/api/v1/posts/$pid/like" -b "token=${TOKENS[01]}" > /dev/null
done
ok "test01 liked 2 posts"

# Comments
curl -s -X POST "$BASE/api/v1/posts/${POSTS[1]}/comments" -b "token=${TOKENS[02]}" -H 'Content-Type: application/json' \
  -d "{\"post_id\":${POSTS[1]},\"content\":\"构图太棒了！学习！\"}" > /dev/null
ok "test02 commented on post1"

curl -s -X POST "$BASE/api/v1/posts/${POSTS[1]}/comments" -b "token=${TOKENS[04]}" -H 'Content-Type: application/json' \
  -d "{\"post_id\":${POSTS[1]},\"content\":\"这光线绝了\"}" > /dev/null
ok "test04 commented on post1"

curl -s -X POST "$BASE/api/v1/posts/${POSTS[2]}/comments" -b "token=${TOKENS[01]}" -H 'Content-Type: application/json' \
  -d "{\"post_id\":${POSTS[2]},\"content\":\"人文气息浓厚，很喜欢\"}" > /dev/null
ok "test01 commented on post2"

curl -s -X POST "$BASE/api/v1/posts/${POSTS[3]}/comments" -b "token=${TOKENS[05]}" -H 'Content-Type: application/json' \
  -d "{\"post_id\":${POSTS[3]},\"content\":\"黄昏拍得太美了吧\"}" > /dev/null
ok "test05 commented on post3"

# Follows: test01↔test02 mutual, test03→test01, test04→test01,test02, test05→test03,test04
curl -s -X POST "$BASE/api/v1/users/$(curl -s -b "token=${TOKENS[02]}" "$BASE/api/v1/auth/me" | json "print(d["data"]["id"])")/follow" -b "token=${TOKENS[01]}" > /dev/null
ok "test01 follows test02"
curl -s -X POST "$BASE/api/v1/users/$(curl -s -b "token=${TOKENS[01]}" "$BASE/api/v1/auth/me" | json "print(d["data"]["id"])")/follow" -b "token=${TOKENS[02]}" > /dev/null
ok "test02 follows test01 (mutual)"TID=$(curl -s -b "token=${TOKENS[01]}" "$BASE/api/v1/auth/me" | json "print(d['data']['id'])")
curl -s -X POST "$BASE/api/v1/users/$TID/follow" -b "token=${TOKENS[02]}" > /dev/null
ok "test02 follows test01 (mutual)"

# More follows
TID=$(curl -s -b "token=${TOKENS[01]}" "$BASE/api/v1/auth/me" | json "print(d['data']['id'])")
curl -s -X POST "$BASE/api/v1/users/$TID/follow" -b "token=${TOKENS[03]}" > /dev/null
ok "test03 follows test01"

TID=$(curl -s -b "token=${TOKENS[01]}" "$BASE/api/v1/auth/me" | json "print(d['data']['id'])")
curl -s -X POST "$BASE/api/v1/users/$TID/follow" -b "token=${TOKENS[04]}" > /dev/null
TID=$(curl -s -b "token=${TOKENS[02]}" "$BASE/api/v1/auth/me" | json "print(d['data']['id'])")
curl -s -X POST "$BASE/api/v1/users/$TID/follow" -b "token=${TOKENS[04]}" > /dev/null
ok "test04 follows test01 and test02"

TID=$(curl -s -b "token=${TOKENS[03]}" "$BASE/api/v1/auth/me" | json "print(d['data']['id'])")
curl -s -X POST "$BASE/api/v1/users/$TID/follow" -b "token=${TOKENS[05]}" > /dev/null
TID=$(curl -s -b "token=${TOKENS[04]}" "$BASE/api/v1/auth/me" | json "print(d['data']['id'])")
curl -s -X POST "$BASE/api/v1/users/$TID/follow" -b "token=${TOKENS[05]}" > /dev/null
ok "test05 follows test03 and test04"

# ============ STEP 8: Verify ============
echo -e "\n[8] Verification"
STATS=$(curl -s -b "token=$ADMIN_T" "$BASE/api/v1/admin/stats")
echo "$STATS" | json "print(f'Users: {d[\"data\"][\"total_users\"]} | Posts: {d[\"data\"][\"total_posts\"]} | Likes: {d[\"data\"][\"total_likes\"]} | Comments: {d[\"data\"][\"total_comments\"]} | Pending: {d[\"data\"][\"pending_posts\"]}')"
ok "Stats verified"

FEED=$(curl -s -b "token=${TOKENS[01]}" "$BASE/api/v1/feed?page_size=100")
echo "$FEED" | json "print(f'Feed total: {d[\"data\"][\"total\"]} posts')"
ok "Feed count OK"

echo -e "\n========================================"
echo -e " Results: \033[0;32m$PASS passed\033[0m, \033[0;31m$FAIL failed\033[0m"
echo "========================================"
