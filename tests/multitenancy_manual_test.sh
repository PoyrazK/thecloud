#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

BASE_URL="http://localhost:8080"

echo -e "${GREEN}Starting Multi-Tenancy Manual Test${NC}"

# Function to register and login
get_token() {
    EMAIL=$1
    NAME=$2
    PASS="password123"

    # Register (ignore error if exists)
    curl -s -X POST $BASE_URL/auth/register -H "Content-Type: application/json" -d "{\"email\": \"$EMAIL\", \"password\": \"$PASS\", \"name\": \"$NAME\"}" > /dev/null

    # Login
    RESP=$(curl -s -X POST $BASE_URL/auth/login -H "Content-Type: application/json" -d "{\"email\": \"$EMAIL\", \"password\": \"$PASS\"}")
    
    # Extract API Key (not token)
    TOKEN=$(echo $RESP | jq -r '.data.api_key')
    echo $TOKEN
}

# 1. Get Tokens
echo "Getting tokens..."
TOKEN_A=$(get_token "usera_manual@test.com" "User A Manual")
TOKEN_B=$(get_token "userb_manual@test.com" "User B Manual")

if [ "$TOKEN_A" == "null" ] || [ -z "$TOKEN_A" ] || [ "$TOKEN_B" == "null" ] || [ -z "$TOKEN_B" ]; then
    echo -e "${RED}Failed to get tokens${NC}"
    exit 1
fi
echo "User A Token: ${TOKEN_A:0:10}..."
echo "User B Token: ${TOKEN_B:0:10}..."

# 2. User A Creates Instance
echo "User A creating instance..."
INST_RESP=$(curl -s -X POST $BASE_URL/instances -H "X-API-Key: $TOKEN_A" -H "Content-Type: application/json" -d '{"name": "manual-inst-a", "image": "alpine"}')
INST_ID=$(echo $INST_RESP | jq -r '.data.id')

if [ "$INST_ID" == "null" ] || [ -z "$INST_ID" ]; then
    echo -e "${RED}Failed to create instance for User A${NC}"
    echo $INST_RESP
    exit 1
fi
echo "Created Instance A: $INST_ID"

# 3. User B Lists Instances (Should NOT see A)
echo "User B listing instances..."
LIST_B=$(curl -s -X GET $BASE_URL/instances -H "X-API-Key: $TOKEN_B")
# Check if list contains the ID
if [[ "$LIST_B" == *"$INST_ID"* ]]; then
    echo -e "${RED}FAIL: User B can see User A's instance!${NC}"
    exit 1
else
    echo -e "${GREEN}PASS: User B cannot see User A's instance in list${NC}"
fi

# 4. User B Gets Instance A (Should Fail)
echo "User B trying to GET instance A..."
GET_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X GET $BASE_URL/instances/$INST_ID -H "X-API-Key: $TOKEN_B")

if [ "$GET_CODE" == "404" ] || [ "$GET_CODE" == "403" ]; then
    echo -e "${GREEN}PASS: User B got $GET_CODE accessing User A's instance${NC}"
else
    echo -e "${RED}FAIL: User B got $GET_CODE accessing User A's instance (expected 404/403)${NC}"
    exit 1
fi

# Cleanup
echo "Cleaning up..."
curl -s -X DELETE $BASE_URL/instances/$INST_ID -H "X-API-Key: $TOKEN_A" > /dev/null

echo -e "${GREEN}Multi-Tenancy Manual Test Completed Successfully!${NC}"
