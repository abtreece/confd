#!/bin/bash
set -e

# Cleanup function
cleanup() {
  if [[ -n "$CONFD_PID" ]]; then
    kill "$CONFD_PID" 2>/dev/null || true
    wait "$CONFD_PID" 2>/dev/null || true
  fi
  rm -f /tmp/confd-health-test.conf
}
trap cleanup EXIT

# Export test data
export APP_NAME="healthcheck-test"

# Build confd
echo "Building confd..."
make build >/dev/null 2>&1

# Start confd with metrics endpoint
echo "Starting confd with metrics endpoint..."
./bin/confd env --watch --confdir ./test/integration/operations/healthcheck/confdir --metrics-addr ":9100" --log-level error &
CONFD_PID=$!

# Wait for confd to start
echo "Waiting for confd to start..."
sleep 3

# Test 1: /health endpoint
echo "Test 1: /health endpoint returns HTTP 200 with body 'ok'"
HEALTH_RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:9100/health)
HEALTH_BODY=$(echo "$HEALTH_RESPONSE" | head -n1)
HEALTH_CODE=$(echo "$HEALTH_RESPONSE" | tail -n1)

if [[ "$HEALTH_CODE" != "200" ]]; then
  echo "FAIL: Expected HTTP 200, got $HEALTH_CODE"
  exit 1
fi

if [[ "$HEALTH_BODY" != "ok" ]]; then
  echo "FAIL: Expected body 'ok', got '$HEALTH_BODY'"
  exit 1
fi
echo "PASS: /health endpoint returned 200 with body 'ok'"

# Test 2: /ready endpoint
echo "Test 2: /ready endpoint returns HTTP 200 when backend healthy"
READY_RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:9100/ready)
READY_CODE=$(echo "$READY_RESPONSE" | tail -n1)

if [[ "$READY_CODE" != "200" ]]; then
  echo "FAIL: Expected HTTP 200, got $READY_CODE"
  exit 1
fi
echo "PASS: /ready endpoint returned 200"

# Test 3: /ready/detailed endpoint
echo "Test 3: /ready/detailed returns valid JSON with required fields"
DETAILED_RESPONSE=$(curl -s http://localhost:9100/ready/detailed)

# Check if jq is available
if ! command -v jq &> /dev/null; then
  echo "WARNING: jq not found, skipping JSON validation"
else
  # Validate JSON structure
  echo "$DETAILED_RESPONSE" | jq -e '.healthy' >/dev/null 2>&1 || {
    echo "FAIL: Missing 'healthy' field in response"
    echo "Response: $DETAILED_RESPONSE"
    exit 1
  }

  echo "$DETAILED_RESPONSE" | jq -e '.message' >/dev/null 2>&1 || {
    echo "FAIL: Missing 'message' field in response"
    echo "Response: $DETAILED_RESPONSE"
    exit 1
  }

  echo "$DETAILED_RESPONSE" | jq -e '.duration_ms' >/dev/null 2>&1 || {
    echo "FAIL: Missing 'duration_ms' field in response"
    echo "Response: $DETAILED_RESPONSE"
    exit 1
  }

  echo "$DETAILED_RESPONSE" | jq -e '.details' >/dev/null 2>&1 || {
    echo "FAIL: Missing 'details' field in response"
    echo "Response: $DETAILED_RESPONSE"
    exit 1
  }

  # Check that healthy is true
  HEALTHY=$(echo "$DETAILED_RESPONSE" | jq -r '.healthy')
  if [[ "$HEALTHY" != "true" ]]; then
    echo "FAIL: Expected healthy=true, got $HEALTHY"
    echo "Response: $DETAILED_RESPONSE"
    exit 1
  fi

  echo "PASS: /ready/detailed returned valid JSON with all required fields"
fi

echo ""
echo "All health check endpoint tests passed!"
