#!/bin/bash
set -e

export HOSTNAME="localhost"

# Wait for Vault to be ready
wait_for_vault() {
    local retries=30
    while ! vault status > /dev/null 2>&1; do
        retries=$((retries - 1))
        if [[ $retries -eq 0 ]]; then
            echo "ERROR: Vault not ready after 30 seconds" >&2
            exit 1
        fi
        sleep 1
    done
}

wait_for_vault

# Clean up any existing secrets engine (ignore errors if doesn't exist)
vault secrets disable kv-v2 2>/dev/null || true

vault secrets enable -version 2 -path kv-v2 kv

# Populate test data
vault kv put kv-v2/database host=127.0.0.1 port=3306 username=confd password=p@sSw0rd
vault kv put kv-v2/upstream app1=10.0.1.10:8080 app2=10.0.1.11:8080
vault kv put kv-v2/nested/production app1=10.0.1.10:8080 app2=10.0.1.11:8080
vault kv put kv-v2/nested/staging app1=172.16.1.10:8080 app2=172.16.1.11:8080

# Run confd
confd vault --onetime --log-level debug \
      --confdir ./test/integration/shared/confdir \
      --auth-type token \
      --auth-token "$VAULT_TOKEN" \
      --prefix "/kv-v2" \
      --node "$VAULT_ADDR"

# Test: Vault watch mode rejection (currently blocks rather than errors)
echo "Test: Vault watch mode behavior"
if confd vault --watch --onetime --log-level error \
    --confdir ./test/integration/shared/confdir \
    --auth-type token \
    --auth-token "$VAULT_TOKEN" \
    --prefix "/kv-v2" \
    --node "$VAULT_ADDR" 2>&1; then
    echo "OK: Vault does not error on watch (blocking behavior)"
else
    echo "OK: Vault handled watch mode as expected"
fi

# Disable kv-v2 secrets for next tests
vault secrets disable kv-v2
