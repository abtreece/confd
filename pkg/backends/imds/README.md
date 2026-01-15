# AWS EC2 IMDS Backend

This backend retrieves configuration data from the AWS EC2 Instance Metadata Service version 2 (IMDSv2).

## Overview

The IMDS backend provides access to comprehensive EC2 instance metadata including:
- Instance information (ID, type, AMI ID, etc.)
- IAM role credentials
- Instance tags
- Network configuration
- Placement information
- User data
- Dynamic instance identity data

## Features

- **IMDSv2 Support**: Automatically handles session token management via AWS SDK
- **Caching**: In-memory cache with configurable TTL to reduce API calls
- **No Credentials Required**: Uses network-based authentication (169.254.169.254)
- **Recursive Directory Walking**: Automatically discovers nested metadata paths
- **Thread-Safe**: Safe for concurrent access

## Usage

### Basic Example

```bash
# Run with polling interval
confd imds --interval 300

# One-time execution
confd imds --onetime

# Custom cache TTL
confd imds --interval 300 --imds-cache-ttl 2m
```

### Template Resource Configuration

Create a template resource file at `/etc/confd/conf.d/instance.toml`:

```toml
[template]
src = "instance.tmpl"
dest = "/etc/app/instance.conf"
keys = [
    "/meta-data/instance-id",
    "/meta-data/instance-type",
    "/meta-data/ami-id",
    "/meta-data/tags/instance/Name",
    "/meta-data/tags/instance/Environment",
    "/meta-data/placement/availability-zone",
    "/meta-data/local-ipv4"
]
reload_cmd = "systemctl reload app"
```

### Template Example

Create a template at `/etc/confd/templates/instance.tmpl`:

```
# Instance Configuration
instance_id={{ getv "/meta-data/instance-id" }}
instance_type={{ getv "/meta-data/instance-type" }}
ami_id={{ getv "/meta-data/ami-id" }}
name={{ getv "/meta-data/tags/instance/Name" }}
environment={{ getv "/meta-data/tags/instance/Environment" }}
availability_zone={{ getv "/meta-data/placement/availability-zone" }}
local_ip={{ getv "/meta-data/local-ipv4" }}
```

## Available Metadata Paths

### Instance Information

- `/meta-data/instance-id` - Instance ID (e.g., i-1234567890abcdef0)
- `/meta-data/instance-type` - Instance type (e.g., t3.micro)
- `/meta-data/ami-id` - AMI ID used to launch the instance
- `/meta-data/ami-launch-index` - Launch index
- `/meta-data/hostname` - Private hostname
- `/meta-data/public-hostname` - Public hostname

### Network Information

- `/meta-data/local-ipv4` - Private IPv4 address
- `/meta-data/public-ipv4` - Public IPv4 address (if assigned)
- `/meta-data/mac` - MAC address of primary network interface
- `/meta-data/network/interfaces/macs/{mac}/` - Network interface details
  - `/meta-data/network/interfaces/macs/{mac}/local-ipv4` - Interface private IP
  - `/meta-data/network/interfaces/macs/{mac}/subnet-id` - Subnet ID
  - `/meta-data/network/interfaces/macs/{mac}/vpc-id` - VPC ID
  - `/meta-data/network/interfaces/macs/{mac}/security-group-ids` - Security groups

### Placement Information

- `/meta-data/placement/availability-zone` - Availability zone
- `/meta-data/placement/region` - AWS region

### Instance Tags

- `/meta-data/tags/instance/` - All instance tags (directory)
- `/meta-data/tags/instance/{tag-name}` - Specific tag value

### IAM Credentials

- `/meta-data/iam/security-credentials/` - List available IAM roles
- `/meta-data/iam/security-credentials/{role-name}` - Temporary credentials (JSON)

### Dynamic Data

- `/dynamic/instance-identity/document` - Instance identity document (JSON)
- `/dynamic/instance-identity/signature` - Instance identity signature

### User Data

- `/user-data` - User data script provided at instance launch

## Configuration Options

### Command-Line Flags

- `--imds-cache-ttl` - Cache TTL for metadata (default: 60s)
  - Examples: `30s`, `2m`, `1h`

### Environment Variables

- `IMDS_ENDPOINT` - Custom IMDS endpoint (for testing only)
  - Default: AWS SDK handles endpoint automatically

## Caching Behavior

The IMDS backend implements intelligent caching to minimize API calls:

1. **Cache TTL**: Metadata is cached for the configured duration (default 60 seconds)
2. **Per-Value Caching**: Each metadata path is cached independently
3. **Directory Walking**: When fetching a directory path (e.g., `/meta-data/tags/instance/`), all discovered values are cached
4. **Thread-Safe**: Cache uses read-write locks for safe concurrent access
5. **Expiration**: Cache entries are validated on each read; expired entries trigger refetch

### Cache Performance

- **First Request**: Fetches from IMDS and populates cache
- **Subsequent Requests**: Served from cache until TTL expires
- **Directory Requests**: Single IMDS request can populate multiple cache entries

## Limitations

### Watch Mode Not Supported

The IMDS backend only supports polling mode, not watch mode:

```bash
# This works
confd imds --interval 300

# This will error
confd imds --watch
```

**Rationale**: Continuous watching of IMDS would be inefficient and expensive. Use `--interval` with an appropriate duration instead.

### EC2 Instance Required

This backend only works on EC2 instances where IMDS is available. It will fail during initialization if IMDS is not accessible.

### IMDSv2 Required

The backend uses IMDSv2, which requires instance metadata options:
- `HttpTokens: required` (recommended for security)
- Instance metadata service enabled

## Security Considerations

### IAM Role Credentials

The `/meta-data/iam/security-credentials/{role-name}` path returns temporary AWS credentials. When using these in templates:

```toml
[template]
src = "aws-credentials.tmpl"
dest = "/etc/app/aws-credentials"
keys = ["/meta-data/iam/security-credentials/my-app-role"]
mode = "0600"  # Restrict permissions
reload_cmd = "systemctl reload app"
```

### Network-Based Authentication

- IMDS is only accessible from the instance itself (169.254.169.254)
- No AWS credentials required for IMDS access
- IMDSv2 uses session tokens for additional security

### Sensitive Data

Instance tags and user data may contain sensitive information. Use appropriate file permissions:

```toml
[template]
mode = "0600"
uid = 0
gid = 0
```

## Monitoring

### Metrics

When metrics are enabled (`--metrics-addr`), the IMDS backend exposes:

- `confd_backend_request_duration_seconds` - IMDS request duration
- `confd_backend_request_total` - Total IMDS requests
- `confd_backend_errors_total` - IMDS errors
- `confd_backend_healthy` - Health status

### Health Checks

- `/health` - Basic IMDS availability check
- `/ready/detailed` - Detailed status including cache statistics

Example:
```bash
curl http://localhost:9100/health
curl http://localhost:9100/ready/detailed
```

## Examples

### Dynamic Service Discovery

Use instance tags for service configuration:

```toml
[template]
src = "service.tmpl"
dest = "/etc/app/service.conf"
keys = [
    "/meta-data/tags/instance/ServiceName",
    "/meta-data/tags/instance/Environment",
    "/meta-data/placement/availability-zone"
]
```

### Regional Configuration

Configure based on instance region:

```toml
[template]
src = "region.tmpl"
dest = "/etc/app/region.conf"
keys = [
    "/meta-data/placement/region",
    "/meta-data/placement/availability-zone"
]
```

Template:
```
region={{ getv "/meta-data/placement/region" }}
az={{ getv "/meta-data/placement/availability-zone" }}
endpoint=https://api.{{ getv "/meta-data/placement/region" }}.amazonaws.com
```

### Network Configuration

```toml
[template]
src = "network.tmpl"
dest = "/etc/app/network.conf"
keys = [
    "/meta-data/local-ipv4",
    "/meta-data/public-ipv4",
    "/meta-data/mac"
]
```

## Troubleshooting

### IMDS Not Available

Error: `IMDS not available: RequestError`

**Solutions**:
1. Verify you're running on an EC2 instance
2. Check instance metadata service is enabled
3. Verify IMDSv2 is configured (HttpTokens)
4. Check security groups allow outbound to 169.254.169.254

### Cache Not Working

If seeing too many IMDS requests:

1. Increase cache TTL: `--imds-cache-ttl 5m`
2. Check logs for cache hit/miss: `--log-level debug`
3. Verify concurrent requests aren't bypassing cache

### 404 Errors

Error: `path not found`

**Solutions**:
1. Verify the metadata path exists on your instance type
2. Check instance has tags if accessing `/meta-data/tags/instance/`
3. Verify IAM role is attached if accessing credentials
4. Some paths only exist on specific instance types

## Testing

### Unit Tests

```bash
go test ./pkg/backends/imds/
```

### Integration Tests

```bash
./test/integration/imds/test.sh
```

### Manual Testing (on EC2)

```bash
# Enable debug logging
confd imds --onetime --log-level debug

# Check generated files
cat /etc/app/instance.conf

# Verify metrics
confd imds --interval 300 --metrics-addr :9100 &
curl http://localhost:9100/metrics | grep confd_backend
```

## References

- [AWS EC2 Instance Metadata Service](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html)
- [IMDSv2 Documentation](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-service.html)
- [AWS SDK for Go v2 - IMDS](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/ec2/imds)
