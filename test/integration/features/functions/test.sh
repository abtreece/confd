#!/bin/bash
set -e

# Template Functions Test
# Tests various template functions: string, encoding, math, utility, data, sorting

export HOSTNAME="localhost"

# Set up environment variables for env backend
export KEY="foobar"
export DATABASE_HOST="127.0.0.1"
export DATABASE_PASSWORD="p@sSw0rd"
export DATABASE_PORT="3306"
export DATABASE_USERNAME="confd"
export UPSTREAM_APP1="10.0.1.10:8080"
export UPSTREAM_APP2="10.0.1.11:8080"

# Clean up before test
rm -f /tmp/confd-functions-test.conf

# Run confd with env backend (simplest to test functions)
confd env --onetime --log-level debug --confdir ./test/integration/features/functions/confdir

# Verify output
if [[ ! -f /tmp/confd-functions-test.conf ]]; then
    echo "ERROR: Output file not created"
    exit 1
fi

if ! diff -q /tmp/confd-functions-test.conf test/integration/features/functions/expect/functions.conf > /dev/null 2>&1; then
    echo "ERROR: Template functions output mismatch"
    echo "=== Expected ==="
    cat test/integration/features/functions/expect/functions.conf
    echo ""
    echo "=== Actual ==="
    cat /tmp/confd-functions-test.conf
    echo ""
    echo "=== Diff ==="
    diff test/integration/features/functions/expect/functions.conf /tmp/confd-functions-test.conf || true
    exit 1
fi

echo "Template functions test passed!"

# Clean up
rm -f /tmp/confd-functions-test.conf
