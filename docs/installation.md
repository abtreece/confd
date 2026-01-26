# Installation

## Linux Packages (Recommended)

confd provides native packages for Debian/Ubuntu (.deb) and RHEL/Fedora/CentOS (.rpm). These packages include systemd integration with security hardening.

### Debian / Ubuntu

```bash
# Download the latest release (replace VERSION and ARCH as needed)
VERSION=0.34.0
ARCH=amd64  # or arm64

curl -LO "https://github.com/abtreece/confd/releases/download/v${VERSION}/confd_${VERSION}_linux_${ARCH}.deb"
sudo dpkg -i "confd_${VERSION}_linux_${ARCH}.deb"
```

### RHEL / Fedora / CentOS

```bash
# Download the latest release (replace VERSION and ARCH as needed)
VERSION=0.34.0
ARCH=x86_64  # or aarch64

curl -LO "https://github.com/abtreece/confd/releases/download/v${VERSION}/confd-${VERSION}-1.${ARCH}.rpm"
sudo rpm -i "confd-${VERSION}-1.${ARCH}.rpm"
```

### Package Contents

The packages install:

| Path | Description |
|------|-------------|
| `/usr/bin/confd` | Binary |
| `/usr/lib/systemd/system/confd.service` | Systemd service with security hardening |
| `/etc/confd/confd.toml` | Default configuration file |
| `/etc/confd/conf.d/` | Template resource directory |
| `/etc/confd/templates/` | Template directory |
| `/etc/default/confd` | Environment file (Debian) |
| `/etc/sysconfig/confd` | Environment file (RHEL) |
| `/var/lib/confd/` | State directory |

### Post-Installation Setup

1. Configure the backend and options in the environment file:

   ```bash
   # Debian/Ubuntu
   sudo vi /etc/default/confd

   # RHEL/Fedora
   sudo vi /etc/sysconfig/confd
   ```

   Example configuration:
   ```bash
   CONFD_BACKEND="etcd"
   CONFD_OPTS="--watch --systemd-notify --watchdog-interval 30s --log-level info"
   ```

2. Create template resources and templates in `/etc/confd/conf.d/` and `/etc/confd/templates/`

3. Enable and start the service:
   ```bash
   sudo systemctl enable confd
   sudo systemctl start confd
   ```

See [Service Deployment Guide](service-deployment.md) for advanced systemd configuration.

---

## Binary Download

confd ships binaries for OS X, Linux, and Windows for both amd64 and arm64 architectures. You can download the latest release from [GitHub](https://github.com/abtreece/confd/releases).

#### OS X

```bash
# For Intel Macs (amd64)
curl -SL https://github.com/abtreece/confd/releases/download/v0.33.0/confd-v0.33.0-darwin-amd64.tar.gz | tar -xz -C /usr/local/bin/

# For Apple Silicon (arm64)
curl -SL https://github.com/abtreece/confd/releases/download/v0.33.0/confd-v0.33.0-darwin-arm64.tar.gz | tar -xz -C /usr/local/bin/
```

#### Linux

Download and extract the binary:
```bash
# For amd64
curl -SL https://github.com/abtreece/confd/releases/download/v0.33.0/confd-v0.33.0-linux-amd64.tar.gz | tar -xz -C /usr/local/bin/

# For arm64
curl -SL https://github.com/abtreece/confd/releases/download/v0.33.0/confd-v0.33.0-linux-arm64.tar.gz | tar -xz -C /usr/local/bin/
```

Or manually:
```bash
wget https://github.com/abtreece/confd/releases/download/v0.33.0/confd-v0.33.0-linux-amd64.tar.gz
tar -xzf confd-v0.33.0-linux-amd64.tar.gz
mv confd /usr/local/bin/
```

#### Windows

Download the appropriate `.zip` file from the [releases page](https://github.com/abtreece/confd/releases) and extract `confd.exe` to a directory in your PATH.

#### Docker

Official Docker images are available from Docker Hub and GitHub Container Registry:

```bash
# Pull from Docker Hub
docker pull abtreece/confd:latest

# Or from GitHub Container Registry
docker pull ghcr.io/abtreece/confd:latest

# Run with env backend
docker run --rm \
  -e MY_VAR=value \
  -v $(pwd)/conf.d:/etc/confd/conf.d:ro \
  -v $(pwd)/templates:/etc/confd/templates:ro \
  -v $(pwd)/output:/output \
  abtreece/confd:latest env --onetime
```

Available image tags:
- `latest` - Latest stable release
- `v0.33.0` - Specific version
- `v0.33.0-amd64`, `v0.33.0-arm64` - Architecture-specific images

See [Docker documentation](docker.md) for complete usage examples including Docker Compose and Kubernetes.

#### Installing in Dockerfile

To install confd in your own Docker image:

```dockerfile
ARG CONFD_VERSION=0.33.0
RUN CONFD_ARCH=$(arch | sed s/aarch64/arm64/ | sed s/x86_64/amd64/) \
    && curl -SL "https://github.com/abtreece/confd/releases/download/v${CONFD_VERSION}/confd-v${CONFD_VERSION}-linux-${CONFD_ARCH}.tar.gz" | tar -xz -C /usr/local/bin/ \
    && confd --version
```

#### Building from Source

```bash
make build
make install
```

#### Building from Source with Docker

Build confd using Docker for a reproducible build environment:

```bash
docker build -t confd:local -f docker/Dockerfile.build .
```

#### Multi-Stage Build for Custom Images

Include confd in your own Docker image using a multi-stage build:

```dockerfile
FROM golang:1.25-alpine AS confd-builder

RUN apk add --no-cache git
WORKDIR /src
RUN git clone https://github.com/abtreece/confd.git .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /confd ./cmd/confd

FROM your-base-image:latest

COPY --from=confd-builder /confd /usr/local/bin/confd

# Your application setup...
```

Or use the official image directly:

```dockerfile
FROM abtreece/confd:latest AS confd

FROM your-base-image:latest
COPY --from=confd /usr/local/bin/confd /usr/local/bin/confd
```

### Next Steps

Get up and running with the [Quick Start Guide](quick-start-guide.md).
