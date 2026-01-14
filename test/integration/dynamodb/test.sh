#!/bin/bash
set -e

export HOSTNAME="localhost"

# Wait for LocalStack/DynamoDB to be ready
wait_for_dynamodb() {
    local retries=30
    while ! aws dynamodb list-tables --endpoint-url "$DYNAMODB_ENDPOINT_URL" > /dev/null 2>&1; do
        retries=$((retries - 1))
        if [[ $retries -eq 0 ]]; then
            echo "ERROR: DynamoDB not ready after 30 seconds" >&2
            exit 1
        fi
        sleep 1
    done
}

wait_for_dynamodb

# Clean up any existing table (ignore errors if doesn't exist)
aws dynamodb delete-table --table-name confd --region "$AWS_REGION" \
    --endpoint-url "$DYNAMODB_ENDPOINT_URL" 2>/dev/null || true

# Create table
aws dynamodb create-table \
    --region "$AWS_REGION" --table-name confd \
    --attribute-definitions AttributeName=key,AttributeType=S \
    --key-schema AttributeName=key,KeyType=HASH \
    --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1 \
    --endpoint-url "$DYNAMODB_ENDPOINT_URL"

# Wait for table to be active
aws dynamodb wait table-exists --table-name confd --region "$AWS_REGION" \
    --endpoint-url "$DYNAMODB_ENDPOINT_URL"

# Populate test data
aws dynamodb put-item --table-name confd --region "$AWS_REGION" \
    --item '{ "key": { "S": "/key" }, "value": {"S": "foobar"}}' \
    --endpoint-url "$DYNAMODB_ENDPOINT_URL"
aws dynamodb put-item --table-name confd --region "$AWS_REGION" \
    --item '{ "key": { "S": "/database/host" }, "value": {"S": "127.0.0.1"}}' \
    --endpoint-url "$DYNAMODB_ENDPOINT_URL"
aws dynamodb put-item --table-name confd --region "$AWS_REGION" \
    --item '{ "key": { "S": "/database/password" }, "value": {"S": "p@sSw0rd"}}' \
    --endpoint-url "$DYNAMODB_ENDPOINT_URL"
aws dynamodb put-item --table-name confd --region "$AWS_REGION" \
    --item '{ "key": { "S": "/database/port" }, "value": {"S": "3306"}}' \
    --endpoint-url "$DYNAMODB_ENDPOINT_URL"
aws dynamodb put-item --table-name confd --region "$AWS_REGION" \
    --item '{ "key": { "S": "/database/username" }, "value": {"S": "confd"}}' \
    --endpoint-url "$DYNAMODB_ENDPOINT_URL"
aws dynamodb put-item --table-name confd --region "$AWS_REGION" \
    --item '{ "key": { "S": "/upstream/app1" }, "value": {"S": "10.0.1.10:8080"}}' \
    --endpoint-url "$DYNAMODB_ENDPOINT_URL"
aws dynamodb put-item --table-name confd --region "$AWS_REGION" \
    --item '{ "key": { "S": "/upstream/app2" }, "value": {"S": "10.0.1.11:8080"}}' \
    --endpoint-url "$DYNAMODB_ENDPOINT_URL"
# Add a broken value, to see if it is handled
aws dynamodb put-item --table-name confd --region "$AWS_REGION" \
    --item '{ "key": { "S": "/upstream/broken" }, "value": {"N": "4711"}}' \
    --endpoint-url "$DYNAMODB_ENDPOINT_URL"
aws dynamodb put-item --table-name confd --region "$AWS_REGION" \
    --item '{ "key": { "S": "/nested/production/app1" }, "value": {"S": "10.0.1.10:8080"}}' \
    --endpoint-url "$DYNAMODB_ENDPOINT_URL"
aws dynamodb put-item --table-name confd --region "$AWS_REGION" \
    --item '{ "key": { "S": "/nested/production/app2" }, "value": {"S": "10.0.1.11:8080"}}' \
    --endpoint-url "$DYNAMODB_ENDPOINT_URL"
aws dynamodb put-item --table-name confd --region "$AWS_REGION" \
    --item '{ "key": { "S": "/nested/staging/app1" }, "value": {"S": "172.16.1.10:8080"}}' \
    --endpoint-url "$DYNAMODB_ENDPOINT_URL"
aws dynamodb put-item --table-name confd --region "$AWS_REGION" \
    --item '{ "key": { "S": "/nested/staging/app2" }, "value": {"S": "172.16.1.11:8080"}}' \
    --endpoint-url "$DYNAMODB_ENDPOINT_URL"

# Run confd, expect it to work
confd dynamodb --onetime --log-level debug --confdir ./test/integration/confdir --interval 5 --table confd

# Run confd with --watch, expecting it to fail (watch not supported)
if confd dynamodb --onetime --log-level debug --confdir ./test/integration/confdir --interval 5 --table confd --watch 2>/dev/null; then
    echo "ERROR: confd with --watch should have failed for DynamoDB backend"
    exit 1
fi
echo "OK: --watch correctly rejected for DynamoDB"

# Run confd without AWS credentials, expecting it to fail
unset AWS_ACCESS_KEY_ID
unset AWS_SECRET_ACCESS_KEY

if confd dynamodb --onetime --log-level debug --confdir ./test/integration/confdir --interval 5 --table confd 2>/dev/null; then
    echo "ERROR: confd without credentials should have failed"
    exit 1
fi
echo "OK: missing credentials correctly rejected"

echo "DynamoDB integration test passed"
