#!/bin/bash
set -e

export HOSTNAME="localhost"

# Wait for LocalStack/Secrets Manager to be ready
wait_for_secretsmanager() {
    local retries=30
    while ! aws secretsmanager list-secrets --endpoint-url "$SECRETSMANAGER_ENDPOINT_URL" > /dev/null 2>&1; do
        retries=$((retries - 1))
        if [[ $retries -eq 0 ]]; then
            echo "ERROR: Secrets Manager not ready after 30 seconds" >&2
            exit 1
        fi
        sleep 1
    done
}

wait_for_secretsmanager

# Clean up any existing secrets (ignore errors)
for secret in database key upstream/app1 upstream/app2 nested/production/app1 \
              nested/production/app2 nested/staging/app1 nested/staging/app2; do
    aws secretsmanager delete-secret --secret-id "$secret" --force-delete-without-recovery \
        --endpoint-url "$SECRETSMANAGER_ENDPOINT_URL" 2>/dev/null || true
done

# Create secrets in Secrets Manager
# JSON secret (will be flattened)
aws secretsmanager create-secret --name "database" \
    --secret-string '{"host":"127.0.0.1","port":"3306","username":"confd","password":"p@sSw0rd"}' \
    --endpoint-url "$SECRETSMANAGER_ENDPOINT_URL"

# Plain string secret
aws secretsmanager create-secret --name "key" \
    --secret-string "foobar" \
    --endpoint-url "$SECRETSMANAGER_ENDPOINT_URL"

# Upstream servers (individual secrets)
aws secretsmanager create-secret --name "upstream/app1" \
    --secret-string "10.0.1.10:8080" \
    --endpoint-url "$SECRETSMANAGER_ENDPOINT_URL"

aws secretsmanager create-secret --name "upstream/app2" \
    --secret-string "10.0.1.11:8080" \
    --endpoint-url "$SECRETSMANAGER_ENDPOINT_URL"

# Nested secrets
aws secretsmanager create-secret --name "nested/production/app1" \
    --secret-string "10.0.1.10:8080" \
    --endpoint-url "$SECRETSMANAGER_ENDPOINT_URL"

aws secretsmanager create-secret --name "nested/production/app2" \
    --secret-string "10.0.1.11:8080" \
    --endpoint-url "$SECRETSMANAGER_ENDPOINT_URL"

aws secretsmanager create-secret --name "nested/staging/app1" \
    --secret-string "172.16.1.10:8080" \
    --endpoint-url "$SECRETSMANAGER_ENDPOINT_URL"

aws secretsmanager create-secret --name "nested/staging/app2" \
    --secret-string "172.16.1.11:8080" \
    --endpoint-url "$SECRETSMANAGER_ENDPOINT_URL"

# Run confd, expect it to work
confd secretsmanager --onetime --log-level debug --confdir ./test/integration/confdir --interval 5

# Run confd with --watch, expecting it to fail (watch not supported)
if confd secretsmanager --onetime --log-level debug --confdir ./test/integration/confdir --interval 5 --watch 2>/dev/null; then
    echo "ERROR: confd with --watch should have failed for Secrets Manager backend"
    exit 1
fi
echo "OK: --watch correctly rejected for Secrets Manager"

# Run confd without AWS credentials, expecting it to fail
unset AWS_ACCESS_KEY_ID
unset AWS_SECRET_ACCESS_KEY

if confd secretsmanager --onetime --log-level debug --confdir ./test/integration/confdir --interval 5 2>/dev/null; then
    echo "ERROR: confd without credentials should have failed"
    exit 1
fi
echo "OK: missing credentials correctly rejected"

echo "Secrets Manager integration test passed"
