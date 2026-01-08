# DynamoDB Backend

The DynamoDB backend enables confd to retrieve configuration data from [Amazon DynamoDB](https://aws.amazon.com/dynamodb/).

## Configuration

The DynamoDB backend uses the [AWS SDK for Go v2](https://aws.github.io/aws-sdk-go-v2/docs/) credential chain, which checks credentials in the following order:

1. Environment variables
2. Shared credentials file (`~/.aws/credentials`)
3. IAM role for EC2/ECS/EKS

### Environment Variables

```bash
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
export AWS_REGION=us-east-1
```

### IAM Role for EC2/ECS/EKS

When running on AWS compute (EC2, ECS, EKS), confd can use the instance/task role automatically. No credential configuration is needed.

### Required IAM Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:GetItem",
        "dynamodb:Scan",
        "dynamodb:DescribeTable"
      ],
      "Resource": "arn:aws:dynamodb:*:*:table/confd-config"
    }
  ]
}
```

## Table Schema

The DynamoDB table must have the following schema:

| Attribute | Type | Key |
|-----------|------|-----|
| `key` | String (S) | Partition Key (HASH) |
| `value` | String (S) | - |

Create the table:

```bash
aws dynamodb create-table \
  --region us-east-1 \
  --table-name confd-config \
  --attribute-definitions AttributeName=key,AttributeType=S \
  --key-schema AttributeName=key,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST
```

## Options

| Flag | Description | Default |
|------|-------------|---------|
| `--table` | DynamoDB table name | Required |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `AWS_ACCESS_KEY_ID` | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key |
| `AWS_REGION` | AWS region |
| `AWS_PROFILE` | Named profile from credentials file |
| `DYNAMODB_LOCAL` | Enable local endpoint (for testing) |
| `DYNAMODB_ENDPOINT_URL` | Custom DynamoDB endpoint URL |

## Basic Example

Add items to DynamoDB:

```bash
aws dynamodb put-item --table-name confd-config \
  --item '{"key": {"S": "/myapp/database/url"}, "value": {"S": "db.example.com"}}'

aws dynamodb put-item --table-name confd-config \
  --item '{"key": {"S": "/myapp/database/user"}, "value": {"S": "admin"}}'

aws dynamodb put-item --table-name confd-config \
  --item '{"key": {"S": "/myapp/database/password"}, "value": {"S": "secret123"}}'
```

Create template resource (`/etc/confd/conf.d/myapp.toml`):

```toml
[template]
src = "myapp.conf.tmpl"
dest = "/etc/myapp/config.conf"
keys = [
  "/myapp/database",
]
```

Create template (`/etc/confd/templates/myapp.conf.tmpl`):

```
[database]
url = {{getv "/myapp/database/url"}}
user = {{getv "/myapp/database/user"}}
password = {{getv "/myapp/database/password"}}
```

Run confd:

```bash
confd dynamodb --table confd-config --onetime
```

## Advanced Example

### Using Prefix-Based Retrieval

confd can retrieve all keys with a common prefix:

```bash
# Store hierarchical config
aws dynamodb put-item --table-name confd-config \
  --item '{"key": {"S": "/production/api/host"}, "value": {"S": "api.example.com"}}'
aws dynamodb put-item --table-name confd-config \
  --item '{"key": {"S": "/production/api/port"}, "value": {"S": "443"}}'
aws dynamodb put-item --table-name confd-config \
  --item '{"key": {"S": "/production/db/host"}, "value": {"S": "db.example.com"}}'
```

Template resource:

```toml
[template]
src = "app.conf.tmpl"
dest = "/etc/app/config.conf"
keys = [
  "/production",
]
```

### Local Development with DynamoDB Local

```bash
# Start DynamoDB Local
docker run -p 8000:8000 amazon/dynamodb-local

# Create table
aws dynamodb create-table \
  --endpoint-url http://localhost:8000 \
  --table-name confd-config \
  --attribute-definitions AttributeName=key,AttributeType=S \
  --key-schema AttributeName=key,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST

# Run confd with local endpoint
export DYNAMODB_LOCAL=true
export DYNAMODB_ENDPOINT_URL=http://localhost:8000
export AWS_ACCESS_KEY_ID=dummy
export AWS_SECRET_ACCESS_KEY=dummy
export AWS_REGION=us-east-1

confd dynamodb --table confd-config --onetime
```

### Kubernetes Deployment

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: myapp
spec:
  serviceAccountName: confd-sa  # With IRSA configured
  containers:
  - name: myapp
    command:
    - confd
    - dynamodb
    - --table=confd-config
    - --interval=300
```

## Watch Mode Support

Watch mode is **not supported** for the DynamoDB backend. Use interval mode (`--interval`) for periodic polling.

```bash
confd dynamodb --table confd-config --interval 60
```

## Per-Resource Backend Configuration

Instead of using the global backend, individual template resources can specify their own DynamoDB backend configuration. This allows mixing backends within a single confd instance.

Add a `[backend]` section to your template resource file:

```toml
[template]
src = "myapp.conf.tmpl"
dest = "/etc/myapp/config.conf"
keys = [
  "/myapp/database",
]

[backend]
backend = "dynamodb"
table = "my-config-table"
```

Available backend options:
- `backend` - Must be `"dynamodb"`
- `table` - DynamoDB table name

Note: AWS credentials are still read from the environment or IAM role.

## Data Retrieval Behavior

1. **Exact key lookup**: First attempts to get the item by exact key match
2. **Prefix scan**: If not found, scans for items with keys starting with the specified prefix
3. **Value type**: Only string (`S`) type values are supported; other types are skipped with a warning
