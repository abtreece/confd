# SSM Backend

The SSM backend enables confd to retrieve configuration data from [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html). It automatically decrypts SecureString parameters and supports recursive key retrieval.

## Configuration

The SSM backend uses the [AWS SDK for Go v2](https://aws.github.io/aws-sdk-go-v2/docs/) credential chain, which checks credentials in the following order:

1. Environment variables
2. Shared credentials file (`~/.aws/credentials`)
3. IAM role for EC2/ECS/EKS

### Environment Variables

```bash
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
export AWS_REGION=us-east-1
```

### Config and Credentials Files

AWS credentials can be stored in the standard AWS CLI configuration files.

`~/.aws/credentials`:

```ini
[default]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

`~/.aws/config`:

```ini
[default]
region = us-east-1
```

Use a named profile with `AWS_PROFILE`:

```bash
export AWS_PROFILE=production
confd ssm --onetime
```

### IAM Role for EC2/ECS/EKS

When running on AWS compute (EC2, ECS, EKS), confd can use the instance/task role automatically. No credential configuration is needed.

For EKS with IAM Roles for Service Accounts (IRSA):

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: confd
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/confd-ssm-role
```

The region is automatically detected from EC2 instance metadata if `AWS_REGION` is not set.

### Required IAM Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ssm:GetParameter",
        "ssm:GetParametersByPath"
      ],
      "Resource": "arn:aws:ssm:*:*:parameter/myapp/*"
    },
    {
      "Effect": "Allow",
      "Action": "kms:Decrypt",
      "Resource": "arn:aws:kms:*:*:key/*",
      "Condition": {
        "StringEquals": {
          "kms:ViaService": "ssm.*.amazonaws.com"
        }
      }
    }
  ]
}
```

## Options

The SSM backend has no backend-specific flags. It uses standard AWS environment variables for configuration:

| Environment Variable | Description |
|---------------------|-------------|
| `AWS_ACCESS_KEY_ID` | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key |
| `AWS_REGION` | AWS region |
| `AWS_PROFILE` | Named profile from credentials file |
| `SSM_LOCAL` | Enable local endpoint (for testing) |
| `SSM_ENDPOINT_URL` | Custom SSM endpoint URL |

## Basic Example

Create parameters in SSM:

```bash
aws ssm put-parameter --name "/myapp/database/url" --type "String" --value "db.example.com"
aws ssm put-parameter --name "/myapp/database/user" --type "String" --value "admin"
aws ssm put-parameter --name "/myapp/database/password" --type "SecureString" --value "secret123"
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
confd ssm --onetime
```

## Advanced Example

Using hierarchical parameters with multiple applications:

```bash
# Create parameters for multiple apps
aws ssm put-parameter --name "/production/app1/db_host" --type "String" --value "db1.example.com"
aws ssm put-parameter --name "/production/app1/db_pass" --type "SecureString" --value "secret1"
aws ssm put-parameter --name "/production/app2/api_key" --type "SecureString" --value "key123"
```

Template resource with prefix (`/etc/confd/conf.d/app1.toml`):

```toml
[template]
prefix = "/production/app1"
src = "app1.conf.tmpl"
dest = "/etc/app1/config.conf"
keys = [
  "/db_host",
  "/db_pass",
]
```

Running as a daemon with interval polling:

```bash
confd ssm --interval 300
```

### Local Development with LocalStack

For local testing with LocalStack:

```bash
export SSM_LOCAL=true
export SSM_ENDPOINT_URL=http://localhost:4566
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_REGION=us-east-1

confd ssm --onetime
```

## Watch Mode Support

Watch mode is **not supported** for the SSM backend. Use interval mode (`--interval`) for periodic polling.

## Parameter Types

SSM Parameter Store supports three parameter types:

- **String** - Plain text values
- **StringList** - Comma-separated values (returned as-is)
- **SecureString** - Encrypted with KMS (automatically decrypted by confd)

All parameter types are retrieved and decrypted automatically.

## Per-Resource Backend Configuration

Instead of using the global backend, individual template resources can specify their own SSM backend configuration. This allows mixing backends within a single confd instance.

Add a `[backend]` section to your template resource file:

```toml
[template]
src = "myapp.conf.tmpl"
dest = "/etc/myapp/config.conf"
keys = [
  "/myapp/database",
]

[backend]
backend = "ssm"
```

Available backend options:
- `backend` - Must be `"ssm"`

Note: AWS credentials are still read from the environment or IAM role.
