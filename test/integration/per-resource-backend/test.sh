#!/bin/bash
set -e

# This test demonstrates per-resource backend configuration
# It uses env backend as the global backend, but one template
# resource overrides to use the file backend

export HOSTNAME="localhost"

# Set up environment variables for env backend
# Env backend maps /app/config/name -> APP_CONFIG_NAME
export APP_CONFIG_NAME="myapp"
export APP_CONFIG_VERSION="1.0.0"

# Ensure we use the built binary
export PATH="./bin:$PATH"

# Clean up before test
rm -rf backends/per-resource
rm -f /tmp/confd-per-resource-*.conf

# Create file backend data
mkdir -p backends/per-resource
cat <<EOT > backends/per-resource/secrets.yaml
secrets:
  database:
    password: super-secret-password
  api:
    key: api-key-12345
EOT

# Run confd with env as global backend
# The secrets.toml resource will use file backend via per-resource config
confd env --onetime --log-level debug --confdir ./test/integration/per-resource-backend/confdir

# Check results
if ! diff -q /tmp/confd-per-resource-app.conf test/integration/per-resource-backend/expect/app.conf > /dev/null 2>&1; then
    echo "ERROR: app.conf output mismatch"
    echo "=== Expected ==="
    cat test/integration/per-resource-backend/expect/app.conf
    echo "=== Actual ==="
    cat /tmp/confd-per-resource-app.conf
    exit 1
fi
echo "OK: app.conf"

if ! diff -q /tmp/confd-per-resource-secrets.conf test/integration/per-resource-backend/expect/secrets.conf > /dev/null 2>&1; then
    echo "ERROR: secrets.conf output mismatch"
    echo "=== Expected ==="
    cat test/integration/per-resource-backend/expect/secrets.conf
    echo "=== Actual ==="
    cat /tmp/confd-per-resource-secrets.conf
    exit 1
fi
echo "OK: secrets.conf"

echo "Per-resource backend test passed!"

# Clean up
rm -rf backends/per-resource
rm -f /tmp/confd-per-resource-*.conf
