# Service Deployment Guide

This guide covers deploying confd as a production service with systemd, Docker, and Kubernetes.

## Table of Contents

- [Systemd Deployment](#systemd-deployment)
- [SIGHUP Reload](#sighup-reload)
- [Graceful Shutdown](#graceful-shutdown)
- [Docker Deployment](#docker-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Monitoring and Health Checks](#monitoring-and-health-checks)

## Systemd Deployment

confd supports systemd's `sd_notify` protocol for improved service management.

### Type=notify (Recommended)

The `Type=notify` service provides better reliability and monitoring:

```ini
[Unit]
Description=confd configuration management
After=network-online.target
Wants=network-online.target

[Service]
Type=notify
User=confd
Group=confd

ExecStart=/usr/local/bin/confd etcd \
  --systemd-notify \
  --watchdog-interval=30s \
  --watch \
  --log-level=info

ExecReload=/bin/kill -HUP $MAINPID

WatchdogSec=60s
TimeoutStopSec=30

Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

**Key flags:**
- `--systemd-notify` - Enable systemd integration
- `--watchdog-interval=30s` - Send watchdog pings every 30 seconds

**Watchdog configuration:**
- `WatchdogSec` must be greater than `--watchdog-interval`
- Recommended: `WatchdogSec = 2 * watchdog-interval`
- If confd stops responding, systemd will restart it

### Type=simple (Fallback)

If systemd integration is not needed, use `Type=simple`:

```ini
[Service]
Type=simple
ExecStart=/usr/local/bin/confd etcd --watch
ExecReload=/bin/kill -HUP $MAINPID
```

See `examples/systemd/` for complete service files.

### Installation

```bash
# Copy service file
sudo cp examples/systemd/confd.service /etc/systemd/system/

# Reload systemd
sudo systemctl daemon-reload

# Enable and start
sudo systemctl enable confd
sudo systemctl start confd

# Check status
sudo systemctl status confd
```

## SIGHUP Reload

confd supports configuration reload via SIGHUP without restarting:

```bash
# Reload via systemctl
sudo systemctl reload confd

# Or send signal directly
sudo kill -HUP $(pidof confd)
```

**What happens on SIGHUP:**
1. Template cache is cleared
2. Template resources are reloaded from `conf.d/`
3. Watches are restarted with new configuration
4. Service continues running without downtime

**Use cases:**
- Add new template resources
- Modify existing templates
- Change template configuration
- Update check/reload commands

**Note:** Backend configuration changes require a full restart.

## Graceful Shutdown

confd implements graceful shutdown with configurable timeout:

```bash
# Default 30s timeout
confd --shutdown-timeout=30s etcd --watch

# Custom timeout
confd --shutdown-timeout=60s etcd --watch
```

**Shutdown phases:**
1. **Wait for in-flight commands** - Check and reload commands complete
2. **Shutdown metrics server** - HTTP server stops gracefully
3. **Close backend connections** - Clean connection teardown

**Behavior:**
- `SIGTERM` or `SIGINT` triggers shutdown
- In-flight commands are given `shutdown-timeout` to complete
- Metrics server gets min(5s, remaining timeout)
- Backend connections closed cleanly

## Docker Deployment

### Signal Forwarding

Docker requires a signal-forwarding entrypoint for graceful shutdown:

```dockerfile
FROM alpine:latest

# Install confd
COPY confd /usr/local/bin/confd
COPY examples/docker/entrypoint.sh /entrypoint.sh

RUN chmod +x /usr/local/bin/confd /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["etcd", "--watch"]
```

### Docker Compose

```yaml
version: '3.8'
services:
  confd:
    image: confd:latest
    command: ["etcd", "--watch", "--shutdown-timeout=30s"]
    volumes:
      - ./conf.d:/etc/confd/conf.d
      - ./templates:/etc/confd/templates
    environment:
      - CONFD_LOG_LEVEL=info
    restart: unless-stopped
```

### Reload in Docker

```bash
# Reload configuration
docker kill --signal=HUP confd

# Graceful shutdown
docker stop confd
```

## Kubernetes Deployment

### Deployment with Probes

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
        image: confd:latest
        args:
          - "etcd"
          - "--watch"
          - "--metrics-addr=:9100"
          - "--shutdown-timeout=30s"
        ports:
        - containerPort: 9100
          name: metrics
        livenessProbe:
          httpGet:
            path: /health
            port: 9100
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 9100
          initialDelaySeconds: 5
          periodSeconds: 5
        lifecycle:
          preStop:
            exec:
              command: ["/bin/sh", "-c", "sleep 5"]
        terminationGracePeriodSeconds: 35
```

**Key settings:**
- `livenessProbe` - Restart if unhealthy
- `readinessProbe` - Remove from service if not ready
- `preStop` - Delay before SIGTERM (allows load balancer updates)
- `terminationGracePeriodSeconds` - Must be > `shutdown-timeout`

### ConfigMap Reload

```bash
# Reload after ConfigMap update
kubectl exec deployment/confd -- kill -HUP 1
```

## Monitoring and Health Checks

### Endpoints

Enable metrics server:
```bash
confd --metrics-addr=:9100 etcd --watch
```

**Available endpoints:**
- `/metrics` - Prometheus metrics
- `/health` - Basic health check
- `/ready` - Readiness check
- `/ready/detailed` - Detailed diagnostics

### Health Check Response

```bash
$ curl http://localhost:9100/health
OK
```

### Ready Check Response

```bash
$ curl http://localhost:9100/ready/detailed | jq
{
  "healthy": true,
  "message": "etcd backend is healthy",
  "duration_ms": 5,
  "checked_at": "2025-01-14T10:30:00Z",
  "details": {
    "endpoint": "localhost:2379",
    "version": "3.5.0",
    "db_size": "1234567"
  }
}
```

### Prometheus Metrics

Key metrics for monitoring:
- `confd_backend_healthy` - Backend connection status (1=healthy)
- `confd_backend_request_duration_seconds` - Backend operation latency
- `confd_template_process_total` - Template processing counter
- `confd_command_total` - Check/reload command execution
- `confd_file_changed_total` - Configuration file changes

See [observability documentation](../CLAUDE.md#metrics-and-observability) for complete metrics list.

## Best Practices

### Production Configuration

```bash
confd \
  --watch \
  --metrics-addr=:9100 \
  --shutdown-timeout=30s \
  --backend-timeout=30s \
  --check-cmd-timeout=30s \
  --reload-cmd-timeout=60s \
  --log-level=info \
  --log-format=json \
  etcd
```

### Security Hardening

**Systemd:**
```ini
[Service]
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/confd /etc/confd
```

**Docker:**
```dockerfile
RUN addgroup -g 1000 confd && \
    adduser -u 1000 -G confd -s /bin/sh -D confd
USER 1000:1000
```

### Resource Limits

**Systemd:**
```ini
[Service]
LimitNOFILE=65536
MemoryMax=512M
CPUQuota=100%
```

**Kubernetes:**
```yaml
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

## Troubleshooting

### Service Won't Start

```bash
# Check logs
journalctl -u confd -n 50 --no-pager

# Check configuration
confd --check-config

# Test connectivity
confd --preflight etcd
```

### Reload Not Working

```bash
# Verify SIGHUP is received
journalctl -u confd | grep SIGHUP

# Check template cache clearing
journalctl -u confd | grep "Template cache cleared"

# Ensure process is running
systemctl status confd
```

### Graceful Shutdown Timeout

If shutdown times out:
1. Increase `--shutdown-timeout`
2. Check for long-running check/reload commands
3. Review backend connectivity

```bash
# Monitor shutdown
journalctl -u confd -f
sudo systemctl stop confd
```

### Watchdog Failures

If systemd restarts due to watchdog:
1. Increase `WatchdogSec` in service file
2. Increase `--watchdog-interval` flag
3. Check for backend responsiveness issues
4. Review system resource constraints

```bash
# Check for watchdog restarts
journalctl -u confd | grep -i watchdog
```
