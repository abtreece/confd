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

# Clean up any existing secrets engine and auth method (ignore errors if doesn't exist)
vault secrets disable kv-v1 2>/dev/null || true
vault auth disable test 2>/dev/null || true

vault secrets enable -version 1 -path kv-v1 kv

# Populate test data
vault write kv-v1/database host=127.0.0.1 port=3306 username=confd password=p@sSw0rd
vault write kv-v1/upstream app1=10.0.1.10:8080 app2=10.0.1.11:8080
vault write kv-v1/nested/production app1=10.0.1.10:8080 app2=10.0.1.11:8080
vault write kv-v1/nested/staging app1=172.16.1.10:8080 app2=172.16.1.11:8080

# Set up AppRole authentication
vault auth enable -path=test approle

cat > /tmp/my-policy.hcl <<EOF
path "*" {
  capabilities = ["read"]
}
EOF

vault write sys/policy/my-policy policy=@/tmp/my-policy.hcl

vault write auth/test/role/my-role \
    secret_id_ttl=120m \
    token_num_uses=1000 \
    token_ttl=60m \
    token_max_ttl=120m \
    secret_id_num_uses=10000 \
    policies=my-policy

ROLE_ID=$(vault read -field=role_id auth/test/role/my-role/role-id)
SECRET_ID=$(vault write -f -field=secret_id auth/test/role/my-role/secret-id)

# Run confd
confd vault --onetime --log-level debug \
      --confdir ./test/integration/shared/confdir \
      --auth-type app-role \
      --role-id "$ROLE_ID" \
      --secret-id "$SECRET_ID" \
      --path=test \
      --prefix "kv-v1" \
      --node "$VAULT_ADDR"

# Clean up
rm -f /tmp/my-policy.hcl
vault auth disable test
vault secrets disable kv-v1
