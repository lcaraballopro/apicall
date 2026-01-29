#!/bin/bash

# Test script for the redesigned reports functionality

echo "=== Testing Reports API Endpoints ==="

# Start the server in background
echo "Starting server..."
./bin/apicall start &
SERVER_PID=$!

# Wait for server to start
sleep 3

echo "Server started with PID: $SERVER_PID"

# Test health endpoint
echo "Testing health endpoint..."
curl -s http://localhost:8080/health

echo -e "\n\nTesting login endpoint..."
TOKEN_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin123"}')

echo "Login response: $TOKEN_RESPONSE"

# Extract token
TOKEN=$(echo $TOKEN_RESPONSE | python3 -c "import sys, json; print(json.load(sys.stdin).get('token', ''))")
if [ -z "$TOKEN" ]; then
    echo "Failed to get token, attempting to create admin user first..."
    # This would need database setup first
    exit 1
fi

echo "Token obtained: ${TOKEN:0:20}..."

echo -e "\n\nTesting logs endpoint..."
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/logs?limit=5" | python3 -m json.tool

echo -e "\n\nTesting logs endpoint with date filters..."
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/logs?from_date=2026-01-01&to_date=2026-01-31&limit=5" | python3 -m json.tool

echo -e "\n\nTesting proyectos endpoint..."
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/proyectos" | python3 -m json.tool

echo -e "\n=== Tests completed ==="

# Cleanup
kill $SERVER_PID
wait $SERVER_PID 2>/dev/null
echo "Server stopped"