# Docker

confd provides official Docker images for easy containerized deployment.

## Images

Official images are available from:

- **Docker Hub**: `abtreece/confd`
- **GitHub Container Registry**: `ghcr.io/abtreece/confd`

## Quick Start

```bash
# Pull the latest stable release
docker pull abtreece/confd:latest

# Or from GitHub Container Registry
docker pull ghcr.io/abtreece/confd:latest
```

## Image Tags

| Tag | Description |
|-----|-------------|
| `latest` | Latest stable release (not updated for RCs) |
| `v0.40.0` | Specific version |
| `v0.40.0-rc.1` | Release candidate |
| `v0.40.0-amd64` | Architecture-specific (amd64) |
| `v0.40.0-arm64` | Architecture-specific (arm64) |

## Image Details

- **Base image**: Alpine 3.21 (~5MB)
- **User**: `confd` (UID 1000, GID 1000)
- **Architectures**: `linux/amd64`, `linux/arm64`
- **Working directory**: `/etc/confd`

Included packages:
- `ca-certificates` - Required for TLS backends (Vault, Consul, etcd)
- `tzdata` - Timezone support for the `datetime` template function

## Usage Examples

### Environment Variables Backend

```bash
docker run --rm \
  -e DATABASE_HOST=db.example.com \
  -e DATABASE_PORT=5432 \
  -v $(pwd)/conf.d:/etc/confd/conf.d:ro \
  -v $(pwd)/templates:/etc/confd/templates:ro \
  -v $(pwd)/output:/output \
  abtreece/confd:latest env --onetime
```

### etcd Backend

```bash
docker run --rm \
  -v $(pwd)/conf.d:/etc/confd/conf.d:ro \
  -v $(pwd)/templates:/etc/confd/templates:ro \
  -v $(pwd)/output:/output \
  abtreece/confd:latest etcd \
    --node http://etcd:2379 \
    --watch
```

### Consul Backend with Metrics

```bash
docker run --rm \
  -p 9100:9100 \
  -v $(pwd)/conf.d:/etc/confd/conf.d:ro \
  -v $(pwd)/templates:/etc/confd/templates:ro \
  -v $(pwd)/output:/output \
  abtreece/confd:latest consul \
    --node http://consul:8500 \
    --watch \
    --metrics-addr :9100
```

### Vault Backend

```bash
docker run --rm \
  -e VAULT_TOKEN=s.xxxxx \
  -v $(pwd)/conf.d:/etc/confd/conf.d:ro \
  -v $(pwd)/templates:/etc/confd/templates:ro \
  -v $(pwd)/output:/output \
  abtreece/confd:latest vault \
    --node http://vault:8200 \
    --auth-type token \
    --interval 60
```

## Docker Compose

```yaml
services:
  confd:
    image: abtreece/confd:latest
    volumes:
      - ./conf.d:/etc/confd/conf.d:ro
      - ./templates:/etc/confd/templates:ro
      - ./output:/output
    environment:
      - MY_APP_CONFIG=value
    command: ["env", "--watch"]
    restart: unless-stopped

  # Example with etcd
  confd-etcd:
    image: abtreece/confd:latest
    volumes:
      - ./conf.d:/etc/confd/conf.d:ro
      - ./templates:/etc/confd/templates:ro
      - ./output:/output
    command: ["etcd", "--node", "http://etcd:2379", "--watch"]
    depends_on:
      - etcd
    restart: unless-stopped

  etcd:
    image: quay.io/coreos/etcd:v3.5.12
    environment:
      - ETCD_ADVERTISE_CLIENT_URLS=http://etcd:2379
      - ETCD_LISTEN_CLIENT_URLS=http://0.0.0.0:2379
```

## Kubernetes

### Basic Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: confd
spec:
  replicas: 1
  selector:
    matchLabels:
      app: confd
  template:
    metadata:
      labels:
        app: confd
    spec:
      containers:
        - name: confd
          image: abtreece/confd:latest
          args:
            - consul
            - --node
            - http://consul:8500
            - --watch
            - --metrics-addr
            - ":9100"
          ports:
            - containerPort: 9100
              name: metrics
          livenessProbe:
            httpGet:
              path: /health
              port: metrics
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /ready
              port: metrics
            initialDelaySeconds: 5
            periodSeconds: 5
          volumeMounts:
            - name: conf-d
              mountPath: /etc/confd/conf.d
              readOnly: true
            - name: templates
              mountPath: /etc/confd/templates
              readOnly: true
            - name: output
              mountPath: /output
      volumes:
        - name: conf-d
          configMap:
            name: confd-resources
        - name: templates
          configMap:
            name: confd-templates
        - name: output
          emptyDir: {}
```

### Sidecar Pattern

Use confd as a sidecar to manage configuration for another container:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  replicas: 1
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
        - name: myapp
          image: myapp:latest
          volumeMounts:
            - name: config
              mountPath: /etc/myapp
              readOnly: true

        - name: confd
          image: abtreece/confd:latest
          args:
            - consul
            - --node
            - http://consul:8500
            - --watch
          volumeMounts:
            - name: conf-d
              mountPath: /etc/confd/conf.d
              readOnly: true
            - name: templates
              mountPath: /etc/confd/templates
              readOnly: true
            - name: config
              mountPath: /output

      volumes:
        - name: conf-d
          configMap:
            name: confd-resources
        - name: templates
          configMap:
            name: confd-templates
        - name: config
          emptyDir: {}
```

## Volume Mounts

| Path | Purpose | Mount Type |
|------|---------|------------|
| `/etc/confd/conf.d` | Template resource definitions (`.toml` files) | Read-only |
| `/etc/confd/templates` | Template files (`.tmpl` files) | Read-only |
| `/output` (or custom) | Generated configuration files | Read-write |

## Configuration via Environment

confd supports configuration via environment variables with the `CONFD_` prefix:

```bash
docker run --rm \
  -e CONFD_LOG_LEVEL=debug \
  -e CONFD_LOG_FORMAT=json \
  -e CONFD_INTERVAL=60 \
  -v $(pwd)/conf.d:/etc/confd/conf.d:ro \
  -v $(pwd)/templates:/etc/confd/templates:ro \
  abtreece/confd:latest env --onetime
```

## Health Checks

When running with `--metrics-addr`, the container exposes health endpoints:

```bash
docker run --rm -p 9100:9100 \
  -v $(pwd)/conf.d:/etc/confd/conf.d:ro \
  -v $(pwd)/templates:/etc/confd/templates:ro \
  abtreece/confd:latest consul \
    --node http://consul:8500 \
    --watch \
    --metrics-addr :9100
```

Endpoints:
- `GET /health` - Basic health check
- `GET /ready` - Readiness check (backend connectivity)
- `GET /ready/detailed` - Detailed readiness with diagnostics
- `GET /metrics` - Prometheus metrics

## Signal Handling

The container handles signals gracefully:

- `SIGTERM` - Graceful shutdown (wait for in-flight operations)
- `SIGHUP` - Reload templates and configuration

```bash
# Graceful stop
docker stop confd

# Reload configuration
docker kill --signal=HUP confd
```

## Building Custom Images

### Using Official Image as Base

```dockerfile
FROM abtreece/confd:latest

# Add your configuration
COPY conf.d/ /etc/confd/conf.d/
COPY templates/ /etc/confd/templates/

# Set default backend
CMD ["consul", "--node", "http://consul:8500", "--watch"]
```

### Multi-stage Build from Source

```dockerfile
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git make
WORKDIR /src

# Clone and build
RUN git clone https://github.com/abtreece/confd.git .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /confd ./cmd/confd

FROM alpine:3.21

RUN addgroup -g 1000 confd && \
    adduser -u 1000 -G confd -s /bin/sh -D confd
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /confd /usr/local/bin/confd

RUN mkdir -p /etc/confd/conf.d /etc/confd/templates && \
    chown -R confd:confd /etc/confd

USER confd:confd
WORKDIR /etc/confd

ENTRYPOINT ["/usr/local/bin/confd"]
```

## Troubleshooting

### Permission Denied

The container runs as non-root user `confd` (UID 1000). Ensure output directories are writable:

```bash
# Create output directory with correct permissions
mkdir -p output
chmod 777 output

# Or run with specific user
docker run --user $(id -u):$(id -g) ...
```

### Template Not Found

Verify template paths in your resource files match the container paths:

```toml
# conf.d/myapp.toml
[template]
src = "myapp.tmpl"        # Relative to /etc/confd/templates/
dest = "/output/myapp.conf"
```

### Backend Connection Issues

For backends running on the host:

```bash
# Linux
docker run --network host ...

# macOS/Windows
docker run ... --node http://host.docker.internal:2379
```

### Debug Mode

Enable debug logging for troubleshooting:

```bash
docker run --rm \
  -e CONFD_LOG_LEVEL=debug \
  ...
  abtreece/confd:latest env --onetime
```
