#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}Starting System Test for The Cloud...${NC}"

# 1. Start API in background
echo "Restarting API..."
fuser -k 8080/tcp || true
nohup ./bin/api > test_api.log 2>&1 &
API_PID=$!
sleep 2

# Cleanup on exit
trap "kill $API_PID || true" EXIT

# 2. Get API Key (bootstrap)
echo "Bootstrapping API Key..."
KEY_RESP=$(curl -s -X POST http://localhost:8080/auth/keys -H "Content-Type: application/json" -d '{"name": "test-user"}')
API_KEY=$(echo $KEY_RESP | grep -oP '"key":"\K[^"]+')
export THECLOUD_API_KEY=$API_KEY
echo "Using Key: $API_KEY"

TEST_NAME="sys-test-$RANDOM"
TEST_PORT=$((9000 + RANDOM % 1000))
echo "Using Test Name: $TEST_NAME, Port: $TEST_PORT"

# 3. Launch Instance
echo -e "${BLUE}Testing 'cloud compute launch'...${NC}"
./bin/cloud compute launch --name $TEST_NAME --image nginx:alpine --port $TEST_PORT:80
sleep 2

# 4. List Instances
echo -e "${BLUE}Testing 'cloud compute list'...${NC}"
./bin/cloud compute list | grep $TEST_NAME

# 5. Check Connectivity (Integration check)
echo -e "${BLUE}Testing Connectivity to Nginx...${NC}"
curl -s --retry 5 --retry-delay 1 localhost:$TEST_PORT | grep "Welcome to nginx"

# 6. Show Details
echo -e "${BLUE}Testing 'cloud compute show'...${NC}"
./bin/cloud compute show $TEST_NAME | grep "Status:.*RUNNING"

# 7. Get Logs
echo -e "${BLUE}Testing 'cloud compute logs'...${NC}"
./bin/cloud compute logs $TEST_NAME | head -n 20

# 8. Stop Instance
echo -e "${BLUE}Testing 'cloud compute stop'...${NC}"
./bin/cloud compute stop $TEST_NAME
sleep 1
./bin/cloud compute list | grep $TEST_NAME | grep "STOPPED"

# 9. Remove Instance
echo -e "${BLUE}Testing 'cloud compute rm'...${NC}"
./bin/cloud compute rm $TEST_NAME
sleep 1
! ./bin/cloud compute list | grep -q $TEST_NAME

echo -e "${GREEN}[SUCCESS] System Test Passed!${NC}"
