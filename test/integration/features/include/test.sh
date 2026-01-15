#!/bin/bash
set -e

# Cleanup function
cleanup() {
  rm -f /tmp/confd-include-test.conf
}
trap cleanup EXIT

# Export test data
export TITLE="Test Include"
export FOOTER="End of document"

# Build confd
echo "Building confd..."
make build >/dev/null 2>&1

# Run confd with include templates
echo "Running confd with template includes..."
./bin/confd env --onetime --log-level error --confdir ./test/integration/features/include/confdir

# Test 1: Output file was created
echo "Test 1: Checking if output file was created"
if [[ ! -f /tmp/confd-include-test.conf ]]; then
  echo "FAIL: Output file was not created"
  exit 1
fi
echo "PASS: Output file was created"

# Test 2: Content matches expected output
echo "Test 2: Checking if content matches expected output"
if diff -u ./test/integration/features/include/expect/include.conf /tmp/confd-include-test.conf; then
  echo "PASS: Content matches expected output"
else
  echo "FAIL: Content does not match expected output"
  echo "Expected:"
  cat ./test/integration/features/include/expect/include.conf
  echo ""
  echo "Actual:"
  cat /tmp/confd-include-test.conf
  exit 1
fi

# Test 3: Verify basic include worked (header.tmpl)
echo "Test 3: Verifying basic include worked"
if grep -q "Title: Test Include" /tmp/confd-include-test.conf; then
  echo "PASS: Basic include (header.tmpl) worked"
else
  echo "FAIL: Basic include did not work"
  cat /tmp/confd-include-test.conf
  exit 1
fi

# Test 4: Verify subdirectory include worked (partials/footer.tmpl)
echo "Test 4: Verifying subdirectory include worked"
if grep -q "Footer: End of document" /tmp/confd-include-test.conf; then
  echo "PASS: Subdirectory include (partials/footer.tmpl) worked"
else
  echo "FAIL: Subdirectory include did not work"
  cat /tmp/confd-include-test.conf
  exit 1
fi

echo ""
echo "All template include tests passed!"
