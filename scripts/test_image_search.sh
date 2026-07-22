#!/usr/bin/env bash
# Terminal-only semantic image search smoke test.
# Requires the single Docker Compose stack to be running.
set -euo pipefail

BASE="${SHAREO_BASE_URL:-http://127.0.0.1:8080}"
export NO_PROXY="${NO_PROXY:+$NO_PROXY,}127.0.0.1,localhost"
export no_proxy="$NO_PROXY"
ADMIN_USER="${SHAREO_TEST_ADMIN_USER:-admin}"
ADMIN_PASSWORD="${SHAREO_TEST_ADMIN_PASSWORD:-admin123}"
SUFFIX="$(date +%s)"
USERNAME="image-search-${SUFFIX}"
PASSWORD="shareo-test-pass"
TMPIMG="$(mktemp /tmp/shareo-image-search.XXXXXX.png)"
trap 'rm -f "$TMPIMG"' EXIT

python3 - "$TMPIMG" <<'PY'
import base64
import pathlib
import sys

png = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
pathlib.Path(sys.argv[1]).write_bytes(base64.b64decode(png))
PY

curl --fail --silent "$BASE/healthz" >/dev/null
REGISTER="$(curl --fail --silent -X POST "$BASE/api/v1/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")"
TOKEN="$(printf '%s' "$REGISTER" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["token"])' 2>/dev/null || true)"
if [ -z "$TOKEN" ]; then
  LOGIN="$(curl --fail --silent -X POST "$BASE/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")"
  TOKEN="$(printf '%s' "$LOGIN" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["token"])')"
fi

UPLOAD="$(curl --fail --silent -X POST "$BASE/api/v1/upload" -b "token=$TOKEN" -F "file=@$TMPIMG")"
IMGURL="$(printf '%s' "$UPLOAD" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["url"])')"
CREATE="$(curl --fail --silent -X POST "$BASE/api/v1/posts" -b "token=$TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"content\":\"semantic image smoke test\",\"images\":[\"$IMGURL\"]}")"
POSTID="$(printf '%s' "$CREATE" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')"
if [ -n "${SHAREO_TEST_POST_ID_FILE:-}" ]; then
  printf '%s\n' "$POSTID" > "$SHAREO_TEST_POST_ID_FILE"
fi

ADMIN_LOGIN="$(curl --fail --silent -X POST "$BASE/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASSWORD\"}")"
ADMIN_TOKEN="$(printf '%s' "$ADMIN_LOGIN" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["token"])')"
curl --fail --silent -X POST "$BASE/api/v1/admin/posts/$POSTID/approve" \
  -b "token=$ADMIN_TOKEN" >/dev/null

FOUND=""
for _ in $(seq 1 180); do
  if ! FOUND="$(curl --fail --silent --get "$BASE/api/v1/search/images" \
    --data-urlencode 'q=red square' --data-urlencode 'limit=20')"; then
    FOUND=""
  fi
  if printf '%s' "$FOUND" | grep -q "\"post_id\":$POSTID"; then
    echo "PASS semantic image search hit post=$POSTID"
    break
  fi
  sleep 1
done
if ! printf '%s' "$FOUND" | grep -q "\"post_id\":$POSTID"; then
  echo "semantic image search did not return post=$POSTID" >&2
  exit 1
fi

if [ "${SHAREO_TEST_SKIP_DELETE:-0}" = "1" ]; then
  echo "PASS approved post retained for isolated recovery checks post=$POSTID"
  exit 0
fi

curl --fail --silent -X DELETE "$BASE/api/v1/admin/posts/$POSTID" -b "token=$ADMIN_TOKEN" >/dev/null
for _ in $(seq 1 10); do
  if ! FOUND="$(curl --fail --silent --get "$BASE/api/v1/search/images" \
    --data-urlencode 'q=red square' --data-urlencode 'limit=20')"; then
    FOUND=""
  fi
  if ! printf '%s' "$FOUND" | grep -q "\"post_id\":$POSTID"; then
    echo "PASS deleted post removed from semantic index post=$POSTID"
    exit 0
  fi
  sleep 1
done
echo "deleted post still appears in semantic search after 10 seconds post=$POSTID" >&2
exit 1
