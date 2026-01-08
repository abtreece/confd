# Consul Backend

The Consul backend enables confd to retrieve configuration data from [HashiCorp Consul](https://www.consul.io/)'s key-value store.

## Configuration

### Basic Connection

Connect to Consul without authentication:

```bash
confd consul --node 127.0.0.1:8500 --onetime
```

### Authentication

#### HTTP Basic Auth

```bash
confd consul --node 127.0.0.1:8500 \
  --basic-auth --username admin --password secret --onetime
```

#### ACL Token

Set the `CONSUL_HTTP_TOKEN` environment variable:

```bash
export CONSUL_HTTP_TOKEN=your-acl-token
confd consul --node 127.0.0.1:8500 --onetime
```

#### TLS Client Certificates

```bash
confd consul --node 127.0.0.1:8501 \
  --scheme https \
  --client-cert /path/to/client.crt \
  --client-key /path/to/client.key \
  --client-ca-keys /path/to/ca.crt --onetime
```

## Options

| Flag | Description | Default |
|------|-------------|---------|
| `-n, --node` | Consul agent address | `127.0.0.1:8500` |
| `--scheme` | HTTP scheme (`http` or `https`) | `http` |
| `--basic-auth` | Enable HTTP basic authentication | `false` |
| `--username` | Username for basic auth | - |
| `--password` | Password for basic auth | - |
| `--client-cert` | Path to client certificate | - |
| `--client-key` | Path to client private key | - |
| `--client-ca-keys` | Path to CA certificate | - |

### Environment Variables

Consul's standard environment variables are also supported:

| Variable | Description |
|----------|-------------|
| `CONSUL_HTTP_ADDR` | Consul agent address |
| `CONSUL_HTTP_TOKEN` | ACL token |
| `CONSUL_HTTP_SSL` | Enable HTTPS |
| `CONSUL_CACERT` | CA certificate path |
| `CONSUL_CLIENT_CERT` | Client certificate path |
| `CONSUL_CLIENT_KEY` | Client key path |

## Basic Example

Add keys to Consul:

```bash
consul kv put myapp/database/url "db.example.com"
consul kv put myapp/database/user "admin"
consul kv put myapp/database/password "secret123"
```

Or using the HTTP API:

```bash
curl -X PUT -d 'db.example.com' http://localhost:8500/v1/kv/myapp/database/url
curl -X PUT -d 'admin' http://localhost:8500/v1/kv/myapp/database/user
curl -X PUT -d 'secret123' http://localhost:8500/v1/kv/myapp/database/password
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
confd consul --node 127.0.0.1:8500 --onetime
```

## Advanced Example

### Using ACL Tokens

Create a policy for confd:

```hcl
# confd-policy.hcl
key_prefix "myapp/" {
  policy = "read"
}
```

```bash
# Create the policy
consul acl policy create -name confd -rules @confd-policy.hcl

# Create a token
consul acl token create -policy-name confd -description "confd token"
```

Use the token:

```bash
export CONSUL_HTTP_TOKEN=<token-secret-id>
confd consul --node 127.0.0.1:8500 --watch
```

### TLS Configuration

```bash
confd consul --node consul.example.com:8501 \
  --scheme https \
  --client-ca-keys /etc/consul.d/ca.pem \
  --client-cert /etc/consul.d/client.pem \
  --client-key /etc/consul.d/client-key.pem \
  --watch
```

### Kubernetes with Consul Connect

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: myapp
  annotations:
    consul.hashicorp.com/connect-inject: "true"
spec:
  containers:
  - name: myapp
    env:
    - name: CONSUL_HTTP_TOKEN
      valueFrom:
        secretKeyRef:
          name: consul-token
          key: token
    command:
    - confd
    - consul
    - --node=127.0.0.1:8500
    - --watch
```

## Watch Mode Support

Watch mode **is supported** for the Consul backend. confd uses Consul's blocking queries for efficient change detection.

```bash
confd consul --node 127.0.0.1:8500 --watch
```

Consul blocking queries long-poll the server, returning immediately when data changes. This provides near-real-time updates without constant polling.

## Per-Resource Backend Configuration

Instead of using the global backend, individual template resources can specify their own Consul backend configuration. This allows mixing backends within a single confd instance.

Add a `[backend]` section to your template resource file:

```toml
[template]
src = "myapp.conf.tmpl"
dest = "/etc/myapp/config.conf"
keys = [
  "/myapp/database",
]

[backend]
backend = "consul"
nodes = ["consul.example.com:8500"]
scheme = "https"
basic_auth = true
username = "admin"
password = "secret"
```

Available backend options:
- `backend` - Must be `"consul"`
- `nodes` - Array of Consul agent addresses
- `scheme` - `"http"` or `"https"`
- `basic_auth` - Enable HTTP basic authentication
- `username` - Username for basic auth
- `password` - Password for basic auth
- `client_cert` - Path to client certificate
- `client_key` - Path to client private key
- `client_cakeys` - Path to CA certificate

## Connection Notes

- Only the first `--node` is used; Consul client does not support multiple nodes
- Use a local Consul agent or load balancer for high availability
- Consul's default HTTP timeout applies to blocking queries
