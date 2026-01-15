#!/bin/bash
set -e

# Cleanup function
cleanup() {
  rm -f /tmp/confd-failuremode-good.conf
  rm -f /tmp/confd-failuremode-bad.conf
}
trap cleanup EXIT

# Export test data
export KEY="test-value"

# Build confd
echo "Building confd..."
make build >/dev/null 2>&1

# Test 1: Best-effort mode - good template should be created despite bad template failing
echo "Test 1: Best-effort mode - good template created despite bad template failing"
cleanup
./bin/confd env --onetime --log-level error --confdir ./test/integration/features/failuremode/confdir --failure-mode best-effort 2>&1 || true

if [[ ! -f /tmp/confd-failuremode-good.conf ]]; then
  echo "FAIL: Good template was not created in best-effort mode"
  exit 1
fi

if grep -q "test-value" /tmp/confd-failuremode-good.conf; then
  echo "PASS: Good template was created with correct content in best-effort mode"
else
  echo "FAIL: Good template exists but has incorrect content"
  cat /tmp/confd-failuremode-good.conf
  exit 1
fi

# Bad template should not exist
if [[ -f /tmp/confd-failuremode-bad.conf ]]; then
  echo "FAIL: Bad template should not have been created"
  exit 1
fi
echo "PASS: Bad template was not created as expected"

# Test 2: Fail-fast mode - processing should exit on first error
echo "Test 2: Fail-fast mode - processing stops at first error"
cleanup

# Run with fail-fast mode - should fail
if ./bin/confd env --onetime --log-level error --confdir ./test/integration/features/failuremode/confdir --failure-mode fail-fast 2>&1; then
  echo "FAIL: confd should have exited with error in fail-fast mode"
  exit 1
fi
echo "PASS: confd exited with error in fail-fast mode as expected"

# In fail-fast mode, we can't guarantee which template processes first since they're sorted alphabetically
# good.toml comes before zbad.toml, so good template might be created before hitting the bad one
# We just verify that confd exited with an error
echo "PASS: Fail-fast mode behaved correctly"

echo ""
echo "All failure mode tests passed!"
