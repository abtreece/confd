#!/bin/bash
set -e

export HOSTNAME="localhost"

# Wait for LocalStack/SSM to be ready
wait_for_ssm() {
    local retries=30
    while ! aws ssm describe-parameters --endpoint-url "$SSM_ENDPOINT_URL" > /dev/null 2>&1; do
        retries=$((retries - 1))
        if [[ $retries -eq 0 ]]; then
            echo "ERROR: SSM not ready after 30 seconds" >&2
            exit 1
        fi
        sleep 1
    done
}

wait_for_ssm

# Clean up any existing parameters (ignore errors)
for param in /key /database/host /database/password /database/port /database/username \
             /upstream/app1 /upstream/app2 /nested/production/app1 /nested/production/app2 \
             /nested/staging/app1 /nested/staging/app2; do
    aws ssm delete-parameter --name "$param" --endpoint-url "$SSM_ENDPOINT_URL" 2>/dev/null || true
done

# Populate test data
aws ssm put-parameter --name "/key" --type "String" --value "foobar" --endpoint-url "$SSM_ENDPOINT_URL"
aws ssm put-parameter --name "/database/host" --type "String" --value "127.0.0.1" --endpoint-url "$SSM_ENDPOINT_URL"
aws ssm put-parameter --name "/database/password" --type "String" --value "p@sSw0rd" --endpoint-url "$SSM_ENDPOINT_URL"
aws ssm put-parameter --name "/database/port" --type "String" --value "3306" --endpoint-url "$SSM_ENDPOINT_URL"
aws ssm put-parameter --name "/database/username" --type "String" --value "confd" --endpoint-url "$SSM_ENDPOINT_URL"
aws ssm put-parameter --name "/upstream/app1" --type "String" --value "10.0.1.10:8080" --endpoint-url "$SSM_ENDPOINT_URL"
aws ssm put-parameter --name "/upstream/app2" --type "String" --value "10.0.1.11:8080" --endpoint-url "$SSM_ENDPOINT_URL"
aws ssm put-parameter --name "/nested/production/app1" --type "String" --value "10.0.1.10:8080" --endpoint-url "$SSM_ENDPOINT_URL"
aws ssm put-parameter --name "/nested/production/app2" --type "String" --value "10.0.1.11:8080" --endpoint-url "$SSM_ENDPOINT_URL"
aws ssm put-parameter --name "/nested/staging/app1" --type "String" --value "172.16.1.10:8080" --endpoint-url "$SSM_ENDPOINT_URL"
aws ssm put-parameter --name "/nested/staging/app2" --type "String" --value "172.16.1.11:8080" --endpoint-url "$SSM_ENDPOINT_URL"

# Run confd, expect it to work
confd ssm --onetime --log-level debug --confdir ./test/integration/confdir --interval 5

# Run confd with --watch, expecting it to fail (watch not supported)
if confd ssm --onetime --log-level debug --confdir ./test/integration/confdir --interval 5 --watch 2>/dev/null; then
    echo "ERROR: confd with --watch should have failed for SSM backend"
    exit 1
fi
echo "OK: --watch correctly rejected for SSM"

# Run confd without AWS credentials, expecting it to fail
unset AWS_ACCESS_KEY_ID
unset AWS_SECRET_ACCESS_KEY

if confd ssm --onetime --log-level debug --confdir ./test/integration/confdir --interval 5 2>/dev/null; then
    echo "ERROR: confd without credentials should have failed"
    exit 1
fi
echo "OK: missing credentials correctly rejected"

echo "SSM integration test passed"
