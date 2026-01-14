#!/bin/bash
set -e

# Negative Tests
# Tests error handling for various invalid configurations

export HOSTNAME="localhost"
export KEY="foobar"

echo "=== Negative/Error Condition Tests ==="

# Create temporary directories for invalid configs
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

# Test 1: Invalid backend type
echo ""
echo "Test 1: Invalid backend type"
if confd invalidbackend --onetime --log-level error 2>&1; then
    echo "ERROR: Should have failed with invalid backend"
    exit 1
fi
echo "OK: Invalid backend type rejected"

# Test 2: Non-existent confdir (logs warning but succeeds - no templates to process)
echo ""
echo "Test 2: Non-existent confdir"
# confd treats missing confdir as a warning, not an error (returns success with no templates)
output=$(confd env --onetime --log-level warn --confdir /nonexistent/path 2>&1)
if ! echo "$output" | grep -q "does not exist"; then
    echo "ERROR: Should have warned about non-existent confdir"
    echo "Output: $output"
    exit 1
fi
echo "OK: Non-existent confdir handled with warning"

# Test 3: Malformed TOML configuration
echo ""
echo "Test 3: Malformed TOML configuration"
mkdir -p "$TMPDIR/malformed/conf.d" "$TMPDIR/malformed/templates"
cat > "$TMPDIR/malformed/conf.d/bad.toml" << 'EOF'
[template
# Missing closing bracket - invalid TOML
mode = "0644"
src = "test.tmpl"
dest = "/tmp/test.conf"
EOF
cat > "$TMPDIR/malformed/templates/test.tmpl" << 'EOF'
test
EOF

if confd env --onetime --log-level error --confdir "$TMPDIR/malformed" 2>&1; then
    echo "ERROR: Should have failed with malformed TOML"
    exit 1
fi
echo "OK: Malformed TOML rejected"

# Test 4: Template with syntax error
echo ""
echo "Test 4: Template with syntax error"
mkdir -p "$TMPDIR/badsyntax/conf.d" "$TMPDIR/badsyntax/templates"
cat > "$TMPDIR/badsyntax/conf.d/test.toml" << 'EOF'
[template]
mode = "0644"
src = "bad.tmpl"
dest = "/tmp/test.conf"
keys = ["/key"]
EOF
cat > "$TMPDIR/badsyntax/templates/bad.tmpl" << 'EOF'
{{ if .Key }
Missing closing braces
{{ end }}
EOF

if confd env --onetime --log-level error --confdir "$TMPDIR/badsyntax" 2>&1; then
    echo "ERROR: Should have failed with template syntax error"
    exit 1
fi
echo "OK: Template syntax error rejected"

# Test 5: Missing template file
echo ""
echo "Test 5: Missing template file"
mkdir -p "$TMPDIR/missingtmpl/conf.d" "$TMPDIR/missingtmpl/templates"
cat > "$TMPDIR/missingtmpl/conf.d/test.toml" << 'EOF'
[template]
mode = "0644"
src = "nonexistent.tmpl"
dest = "/tmp/test.conf"
keys = ["/key"]
EOF

if confd env --onetime --log-level error --confdir "$TMPDIR/missingtmpl" 2>&1; then
    echo "ERROR: Should have failed with missing template"
    exit 1
fi
echo "OK: Missing template file rejected"

# Test 6: Template referencing non-existent key
echo ""
echo "Test 6: Template referencing non-existent key"
mkdir -p "$TMPDIR/badkey/conf.d" "$TMPDIR/badkey/templates"
cat > "$TMPDIR/badkey/conf.d/test.toml" << 'EOF'
[template]
mode = "0644"
src = "test.tmpl"
dest = "/tmp/test.conf"
keys = ["/nonexistent/key"]
EOF
cat > "$TMPDIR/badkey/templates/test.tmpl" << 'EOF'
value: {{ getv "/nonexistent/key" }}
EOF

# This should fail because the key doesn't exist
if confd env --onetime --log-level error --confdir "$TMPDIR/badkey" 2>&1; then
    echo "ERROR: Should have failed with non-existent key"
    exit 1
fi
echo "OK: Non-existent key rejected"

# Test 7: Invalid mode format
echo ""
echo "Test 7: Invalid mode format"
mkdir -p "$TMPDIR/badmode/conf.d" "$TMPDIR/badmode/templates"
cat > "$TMPDIR/badmode/conf.d/test.toml" << 'EOF'
[template]
mode = "invalid"
src = "test.tmpl"
dest = "/tmp/test.conf"
keys = ["/key"]
EOF
cat > "$TMPDIR/badmode/templates/test.tmpl" << 'EOF'
key: {{ getv "/key" }}
EOF

if confd env --onetime --log-level error --confdir "$TMPDIR/badmode" 2>&1; then
    echo "ERROR: Should have failed with invalid mode"
    exit 1
fi
echo "OK: Invalid mode format rejected"

# Test 8: Empty confdir (no template resources)
echo ""
echo "Test 8: Empty confdir (no template resources)"
mkdir -p "$TMPDIR/empty/conf.d" "$TMPDIR/empty/templates"

# Should succeed but do nothing
if ! confd env --onetime --log-level error --confdir "$TMPDIR/empty" 2>&1; then
    echo "ERROR: Empty confdir should succeed (no-op)"
    exit 1
fi
echo "OK: Empty confdir handled gracefully"

# Test 9: Verify that valid config still works after all the error tests
echo ""
echo "Test 9: Verify valid configuration still works"
rm -f /tmp/confd-negative-valid.conf
confd env --onetime --log-level debug --confdir ./test/integration/negative/confdir

if [[ ! -f /tmp/confd-negative-valid.conf ]]; then
    echo "ERROR: Valid configuration should have created output file"
    exit 1
fi
echo "OK: Valid configuration works"

# Clean up
rm -f /tmp/confd-negative-valid.conf

echo ""
echo "=== All negative tests passed! ==="
