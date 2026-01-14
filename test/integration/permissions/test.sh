#!/bin/bash
set -e

# File Permissions Test
# Tests that the mode setting in template config is properly applied

export HOSTNAME="localhost"

# Set up environment variables for env backend
export KEY="foobar"

# Clean up before test
rm -f /tmp/confd-perm-*.conf /tmp/confd-perm-*.sh

# Run confd
confd env --onetime --log-level debug --confdir ./test/integration/permissions/confdir

# Check file permissions
check_permission() {
    local file="$1"
    local expected="$2"
    local name="$3"

    if [[ ! -f "$file" ]]; then
        echo "ERROR: $name - file not created: $file"
        exit 1
    fi

    # Get actual permissions (numeric)
    actual=$(stat -c '%a' "$file" 2>/dev/null || stat -f '%Lp' "$file" 2>/dev/null)

    if [[ "$actual" != "$expected" ]]; then
        echo "ERROR: $name - expected mode $expected, got $actual"
        exit 1
    fi
    echo "OK: $name (mode $actual)"
}

check_permission /tmp/confd-perm-644.conf "644" "mode 0644"
check_permission /tmp/confd-perm-600.conf "600" "mode 0600"
check_permission /tmp/confd-perm-755.sh "755" "mode 0755 (executable)"

# Test that 0755 file is actually executable
if [[ -x /tmp/confd-perm-755.sh ]]; then
    echo "OK: mode 0755 file is executable"
else
    echo "ERROR: mode 0755 file is not executable"
    exit 1
fi

echo ""
echo "File permissions test passed!"

# Clean up
rm -f /tmp/confd-perm-*.conf /tmp/confd-perm-*.sh
