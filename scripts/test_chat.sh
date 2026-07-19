#!/bin/bash
# ShareO IM Smoke Test Script
# Requires: running server at localhost:8080, curl, jq
# Usage: bash scripts/test_chat.sh
set -e
BASE="http://localhost:8080/api/v1"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; exit 1; }

echo "=== ShareO IM Smoke Test ==="

# 1. Register two test users
echo "--- Register test users ---"
USER1=$(curl -sf -X POST "$BASE/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"chattest1","password":"test123456"}' | jq -r '.data.token // empty')
if [ -z "$USER1" ]; then
  # User might already exist, try login
  USER1=$(curl -sf -X POST "$BASE/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"chattest1","password":"test123456"}' | jq -r '.data.token')
fi

USER2=$(curl -sf -X POST "$BASE/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"chattest2","password":"test123456"}' | jq -r '.data.token // empty')
if [ -z "$USER2" ]; then
  USER2=$(curl -sf -X POST "$BASE/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"chattest2","password":"test123456"}' | jq -r '.data.token')
fi
echo "User1 token: ${USER1:0:20}..."
echo "User2 token: ${USER2:0:20}..."

# Get actual user IDs
USER1_ID=$(curl -sf "$BASE/auth/me" -H "Authorization: Bearer $USER1" | jq -r '.data.id')
USER2_ID=$(curl -sf "$BASE/auth/me" -H "Authorization: Bearer $USER2" | jq -r '.data.id')
echo "User1 ID: $USER1_ID, User2 ID: $USER2_ID"

# 2. Create DM
echo "--- Create DM ---"
DM=$(curl -sf -X POST "$BASE/conversations" \
  -H "Authorization: Bearer $USER1" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":$USER2_ID}" | jq '.data')
CONV_ID=$(echo "$DM" | jq -r '.id')
echo "DM conversation: $CONV_ID"
[ "$CONV_ID" != "null" ] && [ "$CONV_ID" != "" ] || fail "Failed to create DM"

# 3. Send messages
echo "--- Send messages ---"
MSG1=$(curl -sf -X POST "$BASE/conversations/$CONV_ID/messages" \
  -H "Authorization: Bearer $USER1" \
  -H "Content-Type: application/json" \
  -d '{"content":"Hello from user1!"}' | jq '.data')
[ "$(echo "$MSG1" | jq -r '.id')" != "null" ] || fail "Failed to send message"
pass "User1 sent message"

MSG2=$(curl -sf -X POST "$BASE/conversations/$CONV_ID/messages" \
  -H "Authorization: Bearer $USER2" \
  -H "Content-Type: application/json" \
  -d '{"content":"Hello from user2!"}' | jq '.data')
[ "$(echo "$MSG2" | jq -r '.id')" != "null" ] || fail "Failed to send message"
pass "User2 sent message"

# 4. Get message history
echo "--- Get message history ---"
MSGS=$(curl -sf "$BASE/conversations/$CONV_ID/messages?limit=10" \
  -H "Authorization: Bearer $USER1" | jq '.data.messages')
COUNT=$(echo "$MSGS" | jq 'length')
[ "$COUNT" -ge 2 ] || fail "Expected >=2 messages, got $COUNT"
pass "Message history: $COUNT messages"

# 5. Mark read
echo "--- Mark read ---"
LAST_ID=$(echo "$MSGS" | jq '.[-1].id')
curl -sf -X PUT "$BASE/conversations/$CONV_ID/read" \
  -H "Authorization: Bearer $USER1" \
  -H "Content-Type: application/json" \
  -d "{\"message_id\":$LAST_ID}" > /dev/null
pass "Marked as read"

# 6. Conversation list
echo "--- Conversation list ---"
CONVS=$(curl -sf "$BASE/conversations" \
  -H "Authorization: Bearer $USER1" | jq '.data')
[ "$(echo "$CONVS" | jq 'length')" -ge 1 ] || fail "Expected >=1 conversation"
pass "Conversation list: $(echo "$CONVS" | jq 'length') conversations"

# 7. Create group
echo "--- Create group ---"
GROUP=$(curl -sf -X POST "$BASE/conversations" \
  -H "Authorization: Bearer $USER1" \
  -H "Content-Type: application/json" \
  -d "{\"title\":\"Test Group\",\"member_ids\":[$USER2_ID]}" | jq '.data')
GROUP_ID=$(echo "$GROUP" | jq -r '.id')
[ "$GROUP_ID" != "null" ] && [ "$GROUP_ID" != "" ] || fail "Failed to create group"
pass "Group created: $GROUP_ID"

# 8. Group message
echo "--- Group message ---"
curl -sf -X POST "$BASE/conversations/$GROUP_ID/messages" \
  -H "Authorization: Bearer $USER1" \
  -H "Content-Type: application/json" \
  -d '{"content":"Hello group!"}' > /dev/null
pass "Group message sent"

echo ""
echo -e "${GREEN}=== All smoke tests passed! ===${NC}"
