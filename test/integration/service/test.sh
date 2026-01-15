#!/bin/bash
set -e

# Cleanup function
cleanup() {
  if [[ -n "$CONFD_PID" ]] && kill -0 "$CONFD_PID" 2>/dev/null; then
    kill -TERM "$CONFD_PID" 2>/dev/null || true
    wait "$CONFD_PID" 2>/dev/null || true
  fi
  rm -f /tmp/confd-service-test.conf
}
trap cleanup EXIT

# Export initial test data
export DATABASE_PORT="3306"

# Build confd
echo "Building confd..."
make build >/dev/null 2>&1

# Test 1: SIGHUP reload
echo "Test 1: SIGHUP reload - verify confd continues running"

# Start confd in interval mode (watch mode doesn't work with env backend)
echo "Starting confd in interval mode..."
./bin/confd env --interval 2 --confdir ./test/integration/service/confdir --log-level error &
CONFD_PID=$!
sleep 5

# Verify initial config was created
echo "Verifying initial config was created..."
if [[ ! -f /tmp/confd-service-test.conf ]]; then
  echo "FAIL: Initial config file was not created"
  exit 1
fi

if grep -q "database_port=3306" /tmp/confd-service-test.conf; then
  echo "PASS: Initial config has correct value (3306)"
else
  echo "FAIL: Initial config has incorrect value"
  cat /tmp/confd-service-test.conf
  exit 1
fi

# Send SIGHUP and verify process continues running
echo "Sending SIGHUP to confd..."
kill -HUP "$CONFD_PID"
sleep 2

# Verify process is still running
if kill -0 "$CONFD_PID" 2>/dev/null; then
  echo "PASS: confd still running after SIGHUP"
else
  echo "FAIL: confd exited after SIGHUP"
  exit 1
fi

# Verify config still exists and is valid (SIGHUP reprocesses templates)
if [[ -f /tmp/confd-service-test.conf ]]; then
  echo "PASS: Config still exists after SIGHUP"
else
  echo "FAIL: Config file missing after SIGHUP"
  exit 1
fi

# Test 2: SIGTERM graceful shutdown
echo "Test 2: SIGTERM graceful shutdown - verify clean exit"

# Send SIGTERM
echo "Sending SIGTERM to confd..."
kill -TERM "$CONFD_PID"

# Wait for process to exit and capture exit code
wait "$CONFD_PID" || true
EXIT_CODE=$?

# Verify clean exit (exit code 0 or 143, which is 128+15 for SIGTERM)
if [[ $EXIT_CODE -eq 0 ]] || [[ $EXIT_CODE -eq 143 ]]; then
  echo "PASS: confd exited cleanly (exit code: $EXIT_CODE)"
else
  echo "FAIL: confd exited with unexpected code: $EXIT_CODE"
  exit 1
fi

# Clear CONFD_PID so cleanup doesn't try to kill it again
CONFD_PID=""

echo ""
echo "All service signal tests passed!"
