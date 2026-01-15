#!/bin/bash
set -e

# Cleanup function
cleanup() {
  if [[ -n "$CONFD_PID" ]]; then
    kill "$CONFD_PID" 2>/dev/null || true
    wait "$CONFD_PID" 2>/dev/null || true
  fi
  rm -f /tmp/confd-metrics-test.conf
}
trap cleanup EXIT

# Export test data
export SERVICE_NAME="metrics-test"

# Build confd
echo "Building confd..."
make build >/dev/null 2>&1

# Start confd with metrics endpoint
echo "Starting confd with metrics endpoint..."
./bin/confd env --watch --confdir ./test/integration/metrics/confdir --metrics-addr ":9101" --log-level error &
CONFD_PID=$!

# Wait for confd to start
echo "Waiting for confd to start..."
sleep 3

# Test 1: /metrics returns valid Prometheus format
echo "Test 1: /metrics returns valid Prometheus format"
METRICS_RESPONSE=$(curl -s http://localhost:9101/metrics)

# Check for basic Prometheus format (lines starting with # or metric names)
if ! echo "$METRICS_RESPONSE" | grep -q "^# HELP"; then
  echo "FAIL: Response doesn't look like Prometheus format"
  echo "Response preview: $(echo "$METRICS_RESPONSE" | head -n5)"
  exit 1
fi
echo "PASS: /metrics returned Prometheus format"

# Test 2: Trigger backend health check to populate backend metrics
echo "Test 2: Triggering backend health check to populate backend metrics"
curl -s http://localhost:9101/ready >/dev/null
sleep 1

# Fetch metrics again after health check
METRICS_RESPONSE=$(curl -s http://localhost:9101/metrics)

# Test 3: Key confd metrics are present
echo "Test 3: Key confd metrics are present"

# Check for confd_templates_loaded (always present, it's a Gauge set at startup)
if ! echo "$METRICS_RESPONSE" | grep -q "confd_templates_loaded"; then
  echo "FAIL: Missing metric 'confd_templates_loaded'"
  exit 1
fi
echo "PASS: Found confd_templates_loaded"

# Check for confd_backend_healthy (appears after health check)
if ! echo "$METRICS_RESPONSE" | grep -q "confd_backend_healthy"; then
  echo "FAIL: Missing metric 'confd_backend_healthy'"
  exit 1
fi
echo "PASS: Found confd_backend_healthy"

# Check for template cache metrics (counters initialized at startup)
if ! echo "$METRICS_RESPONSE" | grep -q "confd_template_cache_hits_total"; then
  echo "FAIL: Missing metric 'confd_template_cache_hits_total'"
  exit 1
fi
echo "PASS: Found confd_template_cache_hits_total"

# Test 4: Go runtime metrics are present
echo "Test 4: Go runtime metrics are present"
if ! echo "$METRICS_RESPONSE" | grep -q "go_goroutines"; then
  echo "FAIL: Missing Go runtime metric 'go_goroutines'"
  exit 1
fi
echo "PASS: Found go_goroutines metric"

# Test 5: Verify backend health metric value
echo "Test 5: Verify backend health metric value is 1 (healthy)"
BACKEND_HEALTH=$(echo "$METRICS_RESPONSE" | grep "^confd_backend_healthy" | awk '{print $2}')
if [[ "$BACKEND_HEALTH" != "1" ]]; then
  echo "FAIL: Expected backend health=1, got $BACKEND_HEALTH"
  exit 1
fi
echo "PASS: Backend health metric is 1 (healthy)"

echo ""
echo "All metrics endpoint tests passed!"
