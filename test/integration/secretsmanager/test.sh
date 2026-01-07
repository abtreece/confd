#!/bin/bash -x

export HOSTNAME="localhost"
export AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY_ID"
export AWS_SECRET_ACCESS_KEY="$AWS_SECRET_ACCESS_KEY"
export AWS_DEFAULT_REGION="$AWS_DEFAULT_REGION"
export AWS_REGION="$AWS_REGION"
export SECRETSMANAGER_LOCAL="true"
export SECRETSMANAGER_ENDPOINT_URL="$SECRETSMANAGER_ENDPOINT_URL"

# Create secrets in Secrets Manager
# JSON secret (will be flattened)
aws secretsmanager create-secret --name "database" \
    --secret-string '{"host":"127.0.0.1","port":"3306","username":"confd","password":"p@sSw0rd"}' \
    --endpoint-url $SECRETSMANAGER_ENDPOINT_URL

# Plain string secret
aws secretsmanager create-secret --name "key" \
    --secret-string "foobar" \
    --endpoint-url $SECRETSMANAGER_ENDPOINT_URL

# Upstream servers (individual secrets)
aws secretsmanager create-secret --name "upstream/app1" \
    --secret-string "10.0.1.10:8080" \
    --endpoint-url $SECRETSMANAGER_ENDPOINT_URL

aws secretsmanager create-secret --name "upstream/app2" \
    --secret-string "10.0.1.11:8080" \
    --endpoint-url $SECRETSMANAGER_ENDPOINT_URL

# Nested secrets
aws secretsmanager create-secret --name "nested/production/app1" \
    --secret-string "10.0.1.10:8080" \
    --endpoint-url $SECRETSMANAGER_ENDPOINT_URL

aws secretsmanager create-secret --name "nested/production/app2" \
    --secret-string "10.0.1.11:8080" \
    --endpoint-url $SECRETSMANAGER_ENDPOINT_URL

aws secretsmanager create-secret --name "nested/staging/app1" \
    --secret-string "172.16.1.10:8080" \
    --endpoint-url $SECRETSMANAGER_ENDPOINT_URL

aws secretsmanager create-secret --name "nested/staging/app2" \
    --secret-string "172.16.1.11:8080" \
    --endpoint-url $SECRETSMANAGER_ENDPOINT_URL

# Run confd, expect it to work
confd secretsmanager --onetime --log-level debug --confdir ./test/integration/confdir --interval 5
if [ $? -ne 0 ]
then
    exit 1
fi

# Run confd with --watch, expecting it to fail (watch not supported)
confd secretsmanager --onetime --log-level debug --confdir ./test/integration/confdir --interval 5 --watch
if [ $? -eq 0 ]
then
    exit 1
fi

# Run confd without AWS credentials, expecting it to fail
unset AWS_ACCESS_KEY_ID
unset AWS_SECRET_ACCESS_KEY

confd secretsmanager --onetime --log-level debug --confdir ./test/integration/confdir --interval 5
if [ $? -eq 0 ]
then
    exit 1
fi

echo "Secrets Manager integration test passed"
