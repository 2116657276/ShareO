#!/bin/bash
# ShareO IM API smoke test. Requires a running server, curl, and jq.
set -euo pipefail

BASE="${SHAREO_BASE_URL:-http://localhost:8080/api/v1}"
RUN_ID="$(date +%s)"
PASSWORD="test123456"

pass() { printf '\033[32m[PASS]\033[0m %s\n' "$1"; }
fail() { printf '\033[31m[FAIL]\033[0m %s\n' "$1"; exit 1; }

api() {
    local method="$1" url="$2" token="${3:-}" body="${4:-}"
    local args=(-sS -X "$method" "$BASE$url" -H 'Content-Type: application/json')
    if [ -n "$token" ]; then args+=(-H "Authorization: Bearer $token"); fi
    if [ -n "$body" ]; then args+=(-d "$body"); fi
    curl "${args[@]}"
}

register() {
    local username="$1"
    api POST /auth/register "" "{\"username\":\"$username\",\"password\":\"$PASSWORD\"}"
}

U1_JSON=$(register "ct1$RUN_ID")
U2_JSON=$(register "ct2$RUN_ID")
U3_JSON=$(register "ct3$RUN_ID")
U1=$(jq -r '.data.token // empty' <<<"$U1_JSON")
U2=$(jq -r '.data.token // empty' <<<"$U2_JSON")
U3=$(jq -r '.data.token // empty' <<<"$U3_JSON")
U1_ID=$(jq -r '.data.user.id // empty' <<<"$U1_JSON")
U2_ID=$(jq -r '.data.user.id // empty' <<<"$U2_JSON")
U3_ID=$(jq -r '.data.user.id // empty' <<<"$U3_JSON")
[ -n "$U1" ] && [ -n "$U2" ] && [ -n "$U3" ] || fail "register three users"
pass "registered three isolated users"

DM1=$(api POST /conversations "$U1" "{\"user_id\":$U2_ID}")
DM2=$(api POST /conversations "$U2" "{\"user_id\":$U1_ID}")
CONV_ID=$(jq -r '.data.id // empty' <<<"$DM1")
[ "$CONV_ID" = "$(jq -r '.data.id // empty' <<<"$DM2")" ] || fail "EnsureDM is idempotent"
pass "EnsureDM returns one conversation"

MSG1=$(api POST "/conversations/$CONV_ID/messages" "$U1" '{"content":" first message "}')
MSG1_ID=$(jq -r '.data.id // empty' <<<"$MSG1")
[ "$(jq -r '.data.content' <<<"$MSG1")" = "first message" ] || fail "message trim"

U2_CONVS=$(api GET /conversations "$U2")
[ "$(jq --argjson id "$CONV_ID" '[.data[] | select(.id == $id)][0].unread_count' <<<"$U2_CONVS")" = "1" ] || fail "exact unread after one foreign message"
pass "exact unread count increments"

MSG2=$(api POST "/conversations/$CONV_ID/messages" "$U2" '{"content":"second message"}')
MSG2_ID=$(jq -r '.data.id // empty' <<<"$MSG2")
api PUT "/conversations/$CONV_ID/read" "$U2" "{\"message_id\":$MSG2_ID}" >/dev/null
[ "$(jq --argjson id "$CONV_ID" '[.data[] | select(.id == $id)][0].unread_count' <<<"$(api GET /conversations "$U2")")" = "0" ] || fail "mark read"
pass "read marker clears unread count"

AFTER=$(api GET "/conversations/$CONV_ID/messages?after_id=$MSG1_ID&limit=10" "$U1")
[ "$(jq -r '.data.messages[0].id' <<<"$AFTER")" = "$MSG2_ID" ] || fail "after_id ascending recovery"
pass "after_id returns ascending recovery messages"

CONFLICT_CODE=$(curl -sS -o /dev/null -w '%{http_code}' \
    "$BASE/conversations/$CONV_ID/messages?before_id=$MSG2_ID&after_id=$MSG1_ID" \
    -H "Authorization: Bearer $U1")
[ "$CONFLICT_CODE" = "400" ] || fail "before_id plus after_id must be 400"

FOREIGN_READ_CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X PUT \
    "$BASE/conversations/$CONV_ID/read" -H "Authorization: Bearer $U1" \
    -H 'Content-Type: application/json' -d '{"message_id":999999999}')
[ "$FOREIGN_READ_CODE" = "404" ] || fail "foreign message read marker must be 404"
pass "pagination conflict and read validation enforced"

GROUP=$(api POST /conversations "$U1" "{\"title\":\" Test Group \",\"member_ids\":[$U2_ID]}")
GROUP_ID=$(jq -r '.data.id // empty' <<<"$GROUP")
[ "$(jq -r '.data.title' <<<"$GROUP")" = "Test Group" ] || fail "group title trim"

OLD_JOIN_CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X POST \
    "$BASE/conversations/$GROUP_ID/join" -H "Authorization: Bearer $U3")
[ "$OLD_JOIN_CODE" = "404" ] || fail "public join route must be removed"

api POST "/conversations/$GROUP_ID/members" "$U1" "{\"user_ids\":[$U3_ID,$U3_ID]}" >/dev/null
api POST "/conversations/$GROUP_ID/members" "$U1" "{\"user_ids\":[$U3_ID]}" >/dev/null
pass "owner invitation is idempotent and public join is absent"

api DELETE "/conversations/$GROUP_ID/members/me" "$U2" >/dev/null
OWNER_LEAVE_CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X DELETE \
    "$BASE/conversations/$GROUP_ID/members/me" -H "Authorization: Bearer $U1")
[ "$OWNER_LEAVE_CODE" = "409" ] || fail "owner leave must be conflict"
pass "ordinary member can leave; owner cannot"

api DELETE "/conversations/$GROUP_ID" "$U1" >/dev/null
GONE_CODE=$(curl -sS -o /dev/null -w '%{http_code}' \
    "$BASE/conversations/$GROUP_ID/messages" -H "Authorization: Bearer $U3")
[ "$GONE_CODE" = "404" ] || fail "dissolved group must be inaccessible"
pass "owner dissolves group transactionally"

TOO_LONG=$(printf 'x%.0s' $(seq 1 2001))
LIMIT_CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X POST \
    "$BASE/conversations/$CONV_ID/messages" -H "Authorization: Bearer $U1" \
    -H 'Content-Type: application/json' -d "{\"content\":\"$TOO_LONG\"}")
[ "$LIMIT_CODE" = "400" ] || fail "2000-character message limit"
pass "message length limit enforced"

printf '\n\033[32m=== ShareO IM smoke tests passed ===\033[0m\n'
