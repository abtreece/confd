# etcd Backend

The etcd backend enables confd to retrieve configuration data from [etcd](https://etcd.io/), a distributed key-value store. This backend uses the etcd v3 API.

## Configuration

### Basic Connection

Connect to etcd without authentication:

```bash
confd etcd --node http://127.0.0.1:2379 --onetime
```

Multiple nodes for high availability:

```bash
confd etcd \
  --node http://etcd1.example.com:2379 \
  --node http://etcd2.example.com:2379 \
  --node http://etcd3.example.com:2379 --onetime
```

### Authentication

#### Username/Password

```bash
confd etcd --node http://127.0.0.1:2379 \
  --basic-auth --username admin --password secret --onetime
```

#### TLS Client Certificates

```bash
confd etcd --node https://127.0.0.1:2379 \
  --client-cert /path/to/client.crt \
  --client-key /path/to/client.key \
  --client-ca-keys /path/to/ca.crt --onetime
```

#### TLS with Authentication

```bash
confd etcd --node https://127.0.0.1:2379 \
  --client-cert /path/to/client.crt \
  --client-key /path/to/client.key \
  --client-ca-keys /path/to/ca.crt \
  --basic-auth --username admin --password secret --onetime
```

## Options

| Flag | Description | Default |
|------|-------------|---------|
| `-n, --node` | etcd node address (can be specified multiple times) | - |
| `--basic-auth` | Enable basic authentication | `false` |
| `--username` | Username for basic auth | - |
| `--password` | Password for basic auth | - |
| `--client-cert` | Path to client certificate | - |
| `--client-key` | Path to client private key | - |
| `--client-ca-keys` | Path to CA certificate | - |
| `--scheme` | URI scheme (http or https) | `http` |
| `--client-insecure` | Skip TLS certificate verification | `false` |

## Basic Example

Add keys to etcd:

```bash
etcdctl put /myapp/database/url "db.example.com"
etcdctl put /myapp/database/user "admin"
etcdctl put /myapp/database/password "secret123"
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
confd etcd --node http://127.0.0.1:2379 --onetime
```

## Advanced Example

### Using DNS SRV Records

Discover etcd nodes via DNS SRV records:

```bash
confd etcd \
  --srv-record _etcd-client._tcp.example.com \
  --scheme https --onetime
```

### Watch Mode with TLS

```bash
confd etcd \
  --node https://etcd.example.com:2379 \
  --client-ca-keys /etc/ssl/certs/etcd-ca.crt \
  --watch
```

### Kubernetes Deployment

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: myapp
spec:
  containers:
  - name: myapp
    env:
    - name: ETCD_USERNAME
      valueFrom:
        secretKeyRef:
          name: etcd-credentials
          key: username
    - name: ETCD_PASSWORD
      valueFrom:
        secretKeyRef:
          name: etcd-credentials
          key: password
    command:
    - confd
    - etcd
    - --node=http://etcd.default.svc:2379
    - --basic-auth
    - --username=$(ETCD_USERNAME)
    - --password=$(ETCD_PASSWORD)
    - --watch
```

## Watch Mode Support

Watch mode **is supported** for the etcd backend. confd uses etcd's native watch API for efficient real-time updates.

```bash
confd etcd --node http://127.0.0.1:2379 --watch
```

When keys change in etcd, confd immediately detects the change and re-renders affected templates.

## Connection Behavior

- **Dial timeout**: 5 seconds
- **Keep-alive**: 10 seconds interval, 3 seconds timeout
- **Transaction limit**: 128 operations per transaction (etcd v3 default)
- **Automatic reconnection**: Watch connections automatically reconnect after disconnection
