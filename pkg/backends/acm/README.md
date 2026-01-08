# ACM Backend

The ACM backend enables confd to retrieve SSL/TLS certificates from [AWS Certificate Manager](https://aws.amazon.com/certificate-manager/). It can retrieve certificate bodies, certificate chains, and optionally export private keys for certificates that support it.

## Configuration

The ACM backend uses the [AWS SDK for Go v2](https://aws.github.io/aws-sdk-go-v2/docs/) credential chain, which checks credentials in the following order:

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

For retrieving certificates (without private key):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "acm:GetCertificate",
      "Resource": "arn:aws:acm:*:*:certificate/*"
    }
  ]
}
```

For exporting certificates with private keys:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "acm:ExportCertificate",
      "Resource": "arn:aws:acm:*:*:certificate/*"
    }
  ]
}
```

## Options

| Flag | Description | Default |
|------|-------------|---------|
| `--export-private-key` | Enable private key export | `false` |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `AWS_ACCESS_KEY_ID` | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key |
| `AWS_REGION` | AWS region (required) |
| `AWS_PROFILE` | Named profile from credentials file |
| `ACM_PASSPHRASE` | Passphrase for private key encryption (required with `--export-private-key`) |
| `ACM_LOCAL` | Enable local endpoint (for testing) |
| `ACM_ENDPOINT_URL` | Custom ACM endpoint URL |

## Certificate Identification

Certificates are identified by their ARN. Use the full ARN as the key:

```
arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012
```

## Retrieved Keys

For each certificate ARN, the following keys are available:

| Key Suffix | Description | Always Available |
|------------|-------------|------------------|
| (none) | Certificate body (PEM) | Yes |
| `_chain` | Certificate chain (PEM) | Yes |
| `_private_key` | Encrypted private key (PKCS#8 PEM) | Only with export enabled |

## Basic Example

Create template resource (`/etc/confd/conf.d/certificate.toml`):

```toml
[template]
src = "certificate.tmpl"
dest = "/etc/ssl/certs/app-cert.pem"
mode = "0644"
keys = [
  "arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012",
]
```

Create template (`/etc/confd/templates/certificate.tmpl`):

```
{{getv "/arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012"}}
```

Run confd:

```bash
confd acm --onetime
```

## Advanced Example

### Retrieving Certificate with Chain

Template for full certificate bundle:

```toml
[template]
src = "fullchain.tmpl"
dest = "/etc/ssl/certs/fullchain.pem"
mode = "0644"
keys = [
  "arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012",
]
```

Template (`/etc/confd/templates/fullchain.tmpl`):

```
{{getv "/arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012"}}
{{getv "/arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012_chain"}}
```

### Exporting Private Keys

Private key export is supported for:
- AWS Private CA issued certificates
- Imported certificates
- Public certificates issued after June 17, 2025 with export option enabled

```bash
export ACM_PASSPHRASE="your-secure-passphrase"
confd acm --export-private-key --onetime
```

Template for private key (`/etc/confd/templates/private-key.tmpl`):

```
{{getv "/arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012_private_key"}}
```

**Note**: The private key is returned encrypted with the passphrase. To decrypt:

```bash
openssl rsa -in encrypted_key.pem -out decrypted_key.pem
```

### Multiple Certificates

Template resource for multiple certificates:

```toml
[template]
src = "nginx-ssl.conf.tmpl"
dest = "/etc/nginx/ssl.conf"
keys = [
  "arn:aws:acm:us-east-1:123456789012:certificate/cert-1",
  "arn:aws:acm:us-east-1:123456789012:certificate/cert-2",
]
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
    env:
    - name: ACM_PASSPHRASE
      valueFrom:
        secretKeyRef:
          name: acm-secrets
          key: passphrase
    command:
    - confd
    - acm
    - --export-private-key
    - --interval=3600
    volumeMounts:
    - name: certs
      mountPath: /etc/ssl/certs
  volumes:
  - name: certs
    emptyDir: {}
```

## Watch Mode Support

Watch mode is **not supported** for the ACM backend. Use interval mode (`--interval`) for periodic polling.

```bash
confd acm --interval 3600
```

Certificates typically don't change frequently, so longer intervals (e.g., hourly) are usually sufficient.

## Per-Resource Backend Configuration

Instead of using the global backend, individual template resources can specify their own ACM backend configuration. This allows mixing backends within a single confd instance.

Add a `[backend]` section to your template resource file:

```toml
[template]
src = "certificate.tmpl"
dest = "/etc/ssl/certs/app-cert.pem"
mode = "0644"
keys = [
  "arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012",
]

[backend]
backend = "acm"
acm_export_private_key = true
```

Available backend options:
- `backend` - Must be `"acm"`
- `acm_export_private_key` - Enable private key export (default: `false`)

Note: AWS credentials are still read from the environment or IAM role. The `ACM_PASSPHRASE` environment variable is required when exporting private keys.

## Security Considerations

1. **Private Key Protection**: When exporting private keys, ensure the passphrase is stored securely (e.g., in Kubernetes Secrets or AWS Secrets Manager)
2. **IAM Permissions**: Use least-privilege IAM policies, restricting access to specific certificate ARNs
3. **File Permissions**: Set restrictive file modes for certificate files, especially private keys (`mode = "0600"`)
4. **Encrypted Private Keys**: The exported private key is encrypted; decrypt only when necessary
