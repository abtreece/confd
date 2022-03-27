#!/bin/bash

export HOSTNAME="localhost"
export AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY_ID"
export AWS_SECRET_ACCESS_KEY="$AWS_SECRET_ACCESS_KEY"
export AWS_DEFAULT_REGION="$AWS_DEFAULT_REGION"
export AWS_REGION="$AWS_REGION"

aws dynamodb create-table \
    --region $AWS_REGION --table-name confd \
    --attribute-definitions AttributeName=key,AttributeType=S \
    --key-schema AttributeName=key,KeyType=HASH \
    --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1 \
    --endpoint-url $DYNAMODB_ENDPOINT_URL

aws dynamodb put-item --table-name confd --region $AWS_REGION \
    --item '{ "key": { "S": "/key" }, "value": {"S": "foobar"}}' \
    --endpoint-url $DYNAMODB_ENDPOINT_URL
aws dynamodb put-item --table-name confd --region $AWS_REGION \
    --item '{ "key": { "S": "/database/host" }, "value": {"S": "127.0.0.1"}}' \
    --endpoint-url $DYNAMODB_ENDPOINT_URL
aws dynamodb put-item --table-name confd --region $AWS_REGION \
    --item '{ "key": { "S": "/database/password" }, "value": {"S": "p@sSw0rd"}}' \
    --endpoint-url $DYNAMODB_ENDPOINT_URL
aws dynamodb put-item --table-name confd --region $AWS_REGION \
    --item '{ "key": { "S": "/database/port" }, "value": {"S": "3306"}}' \
    --endpoint-url $DYNAMODB_ENDPOINT_URL
aws dynamodb put-item --table-name confd --region $AWS_REGION \
    --item '{ "key": { "S": "/database/username" }, "value": {"S": "confd"}}' \
    --endpoint-url $DYNAMODB_ENDPOINT_URL
aws dynamodb put-item --table-name confd --region $AWS_REGION \
    --item '{ "key": { "S": "/upstream/app1" }, "value": {"S": "10.0.1.10:8080"}}' \
    --endpoint-url $DYNAMODB_ENDPOINT_URL
aws dynamodb put-item --table-name confd --region $AWS_REGION \
    --item '{ "key": { "S": "/upstream/app2" }, "value": {"S": "10.0.1.11:8080"}}' \
    --endpoint-url $DYNAMODB_ENDPOINT_URL
# Add a broken value, to see if it is handled
aws dynamodb put-item --table-name confd --region $AWS_REGION \
    --item '{ "key": { "S": "/upstream/broken" }, "value": {"N": "4711"}}' \
    --endpoint-url $DYNAMODB_ENDPOINT_URL
aws dynamodb put-item --table-name confd --region $AWS_REGION \
    --item '{ "key": { "S": "/prefix/database/host" }, "value": {"S": "127.0.0.1"}}' \
    --endpoint-url $DYNAMODB_ENDPOINT_URL
aws dynamodb put-item --table-name confd --region $AWS_REGION \
    --item '{ "key": { "S": "/prefix/database/password" }, "value": {"S": "p@sSw0rd"}}' \
    --endpoint-url $DYNAMODB_ENDPOINT_URL
aws dynamodb put-item --table-name confd --region $AWS_REGION \
    --item '{ "key": { "S": "/prefix/database/port" }, "value": {"S": "3306"}}' \
    --endpoint-url $DYNAMODB_ENDPOINT_URL
aws dynamodb put-item --table-name confd --region $AWS_REGION \
    --item '{ "key": { "S": "/prefix/database/username" }, "value": {"S": "confd"}}' \
    --endpoint-url $DYNAMODB_ENDPOINT_URL
aws dynamodb put-item --table-name confd --region $AWS_REGION \
    --item '{ "key": { "S": "/prefix/upstream/app1" }, "value": {"S": "10.0.1.10:8080"}}' \
    --endpoint-url $DYNAMODB_ENDPOINT_URL
aws dynamodb put-item --table-name confd --region $AWS_REGION \
    --item '{ "key": { "S": "/prefix/upstream/app2" }, "value": {"S": "10.0.1.11:8080"}}' \
    --endpoint-url $DYNAMODB_ENDPOINT_URL

# Run confd, expect it to work
confd --onetime --log-level debug --confdir ./test/integration/confdir --interval 5 --backend dynamodb --table confd
if [ $? -ne 0 ]
then
        exit 1
fi

# Run confd with --watch, expecting it to fail
confd --onetime --log-level debug --confdir ./test/integration/confdir --interval 5 --backend dynamodb --table confd --watch
if [ $? -eq 0 ]
then
        exit 1
fi

# Run confd without AWS credentials, expecting it to fail
unset AWS_ACCESS_KEY_ID
unset AWS_SECRET_ACCESS_KEY

confd --onetime --log-level debug --confdir ./test/integration/confdir --interval 5 --backend dynamodb --table confd
if [ $? -eq 0 ]
then
        exit 1
fi