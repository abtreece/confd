#!/bin/bash
set -e

check_output() {
    local actual="$1"
    local expected="$2"
    local name="$3"

    if [[ ! -f "$actual" ]]; then
        echo "ERROR: Output file not found: $actual"
        exit 1
    fi

    if ! diff -q "$actual" "$expected" > /dev/null 2>&1; then
        echo "ERROR: $name output mismatch"
        echo "=== Expected ($expected) ==="
        cat "$expected"
        echo ""
        echo "=== Actual ($actual) ==="
        cat "$actual"
        echo ""
        echo "=== Diff ==="
        diff "$actual" "$expected" || true
        exit 1
    fi
    echo "OK: $name"
}

check_output /tmp/confd-basic-test.conf test/integration/shared/expect/basic.conf "basic.conf"

# exists-test uses a function that doesn't work with Vault backend
if [[ ! -v VAULT_ADDR ]]; then
    check_output /tmp/confd-exists-test.conf test/integration/shared/expect/exists-test.conf "exists-test.conf"
fi

check_output /tmp/confd-iteration-test.conf test/integration/shared/expect/iteration.conf "iteration.conf"
check_output /tmp/confd-manykeys-test.conf test/integration/shared/expect/basic.conf "manykeys.conf"
check_output /tmp/confd-nested-test.conf test/integration/shared/expect/nested.conf "nested.conf"

# Clean up output files
rm -f /tmp/confd-*.conf

echo "All checks passed!"
