#!/bin/bash
set -e

# check_cmd and reload_cmd Test
# Tests that check_cmd and reload_cmd are executed correctly

export HOSTNAME="localhost"

# Set up environment variables for env backend
export KEY="foobar"

# Clean up before test
rm -f /tmp/confd-check-cmd-test.conf
rm -f /tmp/confd-reload-cmd-test.conf
rm -f /tmp/confd-reload-marker
rm -f /tmp/confd-check-fail-test.conf

# Test 1: check_cmd success
echo "Test 1: check_cmd with successful command"
confd env --onetime --log-level debug --confdir ./test/integration/commands/confdir --config-file /dev/null 2>&1 | grep -v "check_fail" || true

# For the first run, we need to test each config individually since check_fail will cause issues

# Create a temporary confdir with just the check_cmd test
mkdir -p /tmp/confd-cmd-test/conf.d /tmp/confd-cmd-test/templates
cp test/integration/commands/confdir/conf.d/check_cmd.toml /tmp/confd-cmd-test/conf.d/
cp test/integration/commands/confdir/templates/check.conf.tmpl /tmp/confd-cmd-test/templates/

confd env --onetime --log-level debug --confdir /tmp/confd-cmd-test

if [[ ! -f /tmp/confd-check-cmd-test.conf ]]; then
    echo "ERROR: check_cmd test - output file not created"
    exit 1
fi

if ! diff -q /tmp/confd-check-cmd-test.conf test/integration/commands/expect/check.conf > /dev/null 2>&1; then
    echo "ERROR: check_cmd test - output mismatch"
    diff test/integration/commands/expect/check.conf /tmp/confd-check-cmd-test.conf || true
    exit 1
fi
echo "OK: check_cmd with successful command"

# Test 2: reload_cmd execution
echo "Test 2: reload_cmd execution"
rm -rf /tmp/confd-cmd-test/conf.d/*
cp test/integration/commands/confdir/conf.d/reload_cmd.toml /tmp/confd-cmd-test/conf.d/
cp test/integration/commands/confdir/templates/reload.conf.tmpl /tmp/confd-cmd-test/templates/

confd env --onetime --log-level debug --confdir /tmp/confd-cmd-test

if [[ ! -f /tmp/confd-reload-cmd-test.conf ]]; then
    echo "ERROR: reload_cmd test - output file not created"
    exit 1
fi

if [[ ! -f /tmp/confd-reload-marker ]]; then
    echo "ERROR: reload_cmd test - reload marker not created (reload_cmd was not executed)"
    exit 1
fi

if [[ "$(cat /tmp/confd-reload-marker)" != "reloaded" ]]; then
    echo "ERROR: reload_cmd test - reload marker has wrong content"
    cat /tmp/confd-reload-marker
    exit 1
fi

if ! diff -q /tmp/confd-reload-cmd-test.conf test/integration/commands/expect/reload.conf > /dev/null 2>&1; then
    echo "ERROR: reload_cmd test - output mismatch"
    diff test/integration/commands/expect/reload.conf /tmp/confd-reload-cmd-test.conf || true
    exit 1
fi
echo "OK: reload_cmd execution"

# Test 3: check_cmd failure prevents file write
# Note: confd returns exit 0 even when check_cmd fails, but the file should not be updated
# if check fails on a NEW file (staging file gets removed)
echo "Test 3: check_cmd failure handling"
rm -rf /tmp/confd-cmd-test/conf.d/*
rm -f /tmp/confd-check-fail-test.conf
cp test/integration/commands/confdir/conf.d/check_fail.toml /tmp/confd-cmd-test/conf.d/
cp test/integration/commands/confdir/templates/check_fail.conf.tmpl /tmp/confd-cmd-test/templates/

# This should complete but the file should not exist (check_cmd failed)
confd env --onetime --log-level debug --confdir /tmp/confd-cmd-test 2>&1 || true

# The file should NOT exist because check_cmd failed
if [[ -f /tmp/confd-check-fail-test.conf ]]; then
    echo "ERROR: check_cmd failure test - file was created despite check_cmd failing"
    cat /tmp/confd-check-fail-test.conf
    exit 1
fi
echo "OK: check_cmd failure prevents file write"

echo ""
echo "check_cmd and reload_cmd tests passed!"

# Clean up
rm -f /tmp/confd-check-cmd-test.conf
rm -f /tmp/confd-reload-cmd-test.conf
rm -f /tmp/confd-reload-marker
rm -f /tmp/confd-check-fail-test.conf
rm -rf /tmp/confd-cmd-test
