# Secrets Manager Backend

The Secrets Manager backend enables confd to retrieve configuration data from [AWS Secrets Manager](https://aws.amazon.com/secrets-manager/). It supports both plain string secrets and JSON secrets with automatic flattening.

## Configuration

The Secrets Manager backend uses the [AWS SDK for Go v2](https://aws.github.io/aws-sdk-go-v2/docs/) credential chain, which checks credentials in the following order:

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

The region is automatically detected from EC2 instance metadata if `AWS_REGION` is not set.

### Required IAM Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "secretsmanager:GetSecretValue",
      "Resource": "arn:aws:secretsmanager:*:*:secret:myapp/*"
    }
  ]
}
```

## Options

| Flag | Description | Default |
|------|-------------|---------|
| `--no-flatten` | Disable JSON flattening, return raw secret string | `false` |
| `--version-stage` | Version stage (AWSCURRENT, AWSPREVIOUS, or custom label) | `AWSCURRENT` |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `AWS_ACCESS_KEY_ID` | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key |
| `AWS_REGION` | AWS region (required) |
| `AWS_PROFILE` | Named profile from credentials file |
| `SECRETSMANAGER_LOCAL` | Enable local endpoint (for testing) |
| `SECRETSMANAGER_ENDPOINT_URL` | Custom Secrets Manager endpoint URL |

## Secret Types

### Plain String Secrets

```bash
aws secretsmanager create-secret \
  --name "/myapp/api-key" \
  --secret-string "sk-1234567890abcdef"
```

Access as `/myapp/api-key`.

### JSON Secrets (Automatic Flattening)

JSON secrets are automatically flattened to individual key-value pairs:

```bash
aws secretsmanager create-secret \
  --name "/myapp/database" \
  --secret-string '{"url":"db.example.com","user":"admin","password":"secret123"}'
```

Access individual fields:
- `/myapp/database/url` = `db.example.com`
- `/myapp/database/user` = `admin`
- `/myapp/database/password` = `secret123`

### Binary Secrets

Binary secrets are returned as base64-encoded strings.

## Basic Example

Create secrets in Secrets Manager:

```bash
# Plain string secret
aws secretsmanager create-secret \
  --name "/myapp/api-key" \
  --secret-string "sk-1234567890"

# JSON secret
aws secretsmanager create-secret \
  --name "/myapp/database" \
  --secret-string '{"url":"db.example.com","user":"admin","password":"secret123"}'
```

Create template resource (`/etc/confd/conf.d/myapp.toml`):

```toml
[template]
src = "myapp.conf.tmpl"
dest = "/etc/myapp/config.conf"
keys = [
  "/myapp/database",
  "/myapp/api-key",
]
```

Create template (`/etc/confd/templates/myapp.conf.tmpl`):

```
[database]
url = {{getv "/myapp/database/url"}}
user = {{getv "/myapp/database/user"}}
password = {{getv "/myapp/database/password"}}

[api]
key = {{getv "/myapp/api-key"}}
```

Run confd:

```bash
confd secretsmanager --onetime
```

## Advanced Example

### Disable JSON Flattening

To get the raw JSON string instead of flattened keys:

```bash
confd secretsmanager --no-flatten --onetime
```

Template with raw JSON:

```
{{$db := getv "/myapp/database" | json}}
[database]
url = {{$db.url}}
user = {{$db.user}}
```

### Version Staging

Retrieve a specific version of a secret:

```bash
# Get the previous version
confd secretsmanager --version-stage AWSPREVIOUS --onetime

# Get a custom-labeled version
confd secretsmanager --version-stage MyCustomLabel --onetime
```

### Local Development with LocalStack

```bash
# Start LocalStack
docker run -p 4566:4566 localstack/localstack

# Create secret
aws --endpoint-url=http://localhost:4566 secretsmanager create-secret \
  --name "/myapp/config" \
  --secret-string '{"key":"value"}'

# Run confd
export SECRETSMANAGER_LOCAL=true
export SECRETSMANAGER_ENDPOINT_URL=http://localhost:4566
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_REGION=us-east-1

confd secretsmanager --onetime
```

### Kubernetes with IRSA

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: confd
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/confd-secrets-role
---
apiVersion: v1
kind: Pod
metadata:
  name: myapp
spec:
  serviceAccountName: confd
  containers:
  - name: myapp
    command:
    - confd
    - secretsmanager
    - --interval=300
```

## Watch Mode Support

Watch mode is **not supported** for the Secrets Manager backend. Use interval mode (`--interval`) for periodic polling.

```bash
confd secretsmanager --interval 300
```

## JSON Flattening Behavior

When a JSON secret is retrieved:

1. confd attempts to parse the secret string as JSON
2. If successful and flattening is enabled, each top-level key becomes a nested path
3. The original secret path is not available when flattening occurs
4. Nested JSON objects are converted to string representation

Example:

```json
{"host": "db.example.com", "port": 5432, "ssl": true}
```

Becomes:
- `/myapp/database/host` = `db.example.com`
- `/myapp/database/port` = `5432`
- `/myapp/database/ssl` = `true`

## Per-Resource Backend Configuration

Instead of using the global backend, individual template resources can specify their own Secrets Manager backend configuration. This is especially useful for fetching secrets while using a different backend for application config.

Add a `[backend]` section to your template resource file:

```toml
[template]
src = "secrets.conf.tmpl"
dest = "/etc/myapp/secrets.conf"
mode = "0600"
keys = [
  "/myapp/database",
]

[backend]
backend = "secretsmanager"
secretsmanager_version_stage = "AWSCURRENT"
secretsmanager_no_flatten = false
```

Available backend options:
- `backend` - Must be `"secretsmanager"`
- `secretsmanager_version_stage` - Version stage (default: `AWSCURRENT`)
- `secretsmanager_no_flatten` - Disable JSON flattening (default: `false`)

Note: AWS credentials are still read from the environment or IAM role.
