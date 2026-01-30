#!/bin/bash

# Test script for campaign filtering in reports using Python for JSON parsing

SERVER_URL="http://localhost:8080"
ADMIN_USER="admin"
ADMIN_PASS="admin123"

parse_json() {
    python3 -c "import sys, json; print(json.load(sys.stdin)$1)"
}

# 0. Build and Start Server
echo "Killing old server..."
pkill apicall || true
sleep 2

echo "Building server..."
go build -o bin/apicall cmd/apicall/main.go
if [ $? -ne 0 ]; then
    echo "Build failed"
    exit 1
fi

echo "Starting server..."
./bin/apicall start > /dev/null 2>&1 &
SERVER_PID=$!
echo "Server started with PID: $SERVER_PID"
sleep 5

# Ensure cleanup on exit
trap "kill $SERVER_PID" EXIT

# 1. Login
echo "Logging in..."
TOKEN_RESPONSE=$(curl -s -X POST $SERVER_URL/api/v1/login \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"$ADMIN_USER\", \"password\": \"$ADMIN_PASS\"}")

TOKEN=$(echo $TOKEN_RESPONSE | parse_json ".get('token', '')")

if [ -z "$TOKEN" ] || [ "$TOKEN" == "null" ]; then
    echo "Login failed. Response: $TOKEN_RESPONSE"
    exit 1
fi

echo "Login successful. Token: ${TOKEN:0:10}..."

# 2. Get Projects
echo "Fetching projects..."
PROYECTO_ID=$(curl -s -H "Authorization: Bearer $TOKEN" $SERVER_URL/api/v1/proyectos | parse_json "[0]['id']")
echo "Using Project ID: $PROYECTO_ID"

if [ -z "$PROYECTO_ID" ] || [ "$PROYECTO_ID" == "null" ]; then
    echo "No projects found. Create one first."
    exit 1
fi

# 3. Create a Dummy Campaign
echo "Creating dummy campaign..."
CAMPAIGN_ID=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d "{\"nombre\": \"Test Campaign Filter\", \"proyecto_id\": $PROYECTO_ID, \"estado\": \"active\", \"total_contactos\": 100}" \
  $SERVER_URL/api/v1/campaigns | parse_json "['id']")

echo "Created Campaign ID: $CAMPAIGN_ID"

# 4. Insert test log manually
echo "Inserting test log manually..."
mysql -u root -p'admin123' apicall_db -e "INSERT INTO apicall_call_log (proyecto_id, campaign_id, telefono, status, interacciono, created_at) VALUES ($PROYECTO_ID, $CAMPAIGN_ID, '9999999999', 'ANSWERED', 1, NOW());"

# 5. Fetch logs filtered by Campaign
echo "Fetching logs for Campaign $CAMPAIGN_ID..."
LOGS_COUNT=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/logs?proyecto_id=$PROYECTO_ID&campaign_id=$CAMPAIGN_ID" | parse_json ".__len__()")

echo "Logs found: $LOGS_COUNT"

if [ "$LOGS_COUNT" -ge 1 ]; then
    echo "TEST PASSED: Logs filtered by campaign successfully."
else
    echo "TEST FAILED: No logs found with campaign filter."
    exit 1
fi

# 6. Fetch logs for a non-existent campaign
echo "Fetching logs for non-existent campaign..."
LOGS_COUNT_EMPTY=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/logs?proyecto_id=$PROYECTO_ID&campaign_id=999999" | parse_json ".__len__()")

if [ "$LOGS_COUNT_EMPTY" -eq 0 ]; then
    echo "TEST PASSED: Correctly returned 0 logs for invalid campaign."
else
    echo "TEST FAILED: Returned logs for invalid campaign."
    exit 1
fi

echo "Done."
