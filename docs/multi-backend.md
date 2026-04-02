# Multi-Backend Architectures

confd supports fetching configuration from multiple backends simultaneously. A common pattern is storing application config in Consul or etcd while fetching secrets from Vault — all within a single confd instance.

## Per-Resource Backend Override

Each template resource can specify its own backend using a `[backend]` section that overrides the global backend. This is the primary mechanism for multi-backend architectures.

### Example: Secrets from Vault, Config from Consul

Run confd with Consul as the global backend:

```bash
confd consul --node 127.0.0.1:8500 --watch
```

Application config uses the global Consul backend:

`/etc/confd/conf.d/app-config.toml`:

```toml
[template]
src = "app.conf.tmpl"
dest = "/etc/myapp/app.conf"
mode = "0644"
keys = [
  "/myapp/config",
]
reload_cmd = "/usr/bin/systemctl reload myapp"
```

Secrets override the backend to use Vault:

`/etc/confd/conf.d/app-secrets.toml`:

```toml
[template]
src = "secrets.conf.tmpl"
dest = "/etc/myapp/secrets.conf"
mode = "0600"
keys = [
  "/secret/data/myapp",
]
reload_cmd = "/usr/bin/systemctl reload myapp"

[backend]
backend = "vault"
nodes = ["https://vault.example.com:8200"]
auth_type = "approle"
role_id = "my-role-id"
secret_id = "my-secret-id"
```

### Example: SSM Parameters with IMDS Metadata

Use SSM as the global backend for application parameters, while fetching EC2 instance metadata from IMDS for a host-info template:

```bash
confd ssm --interval 300
```

`/etc/confd/conf.d/host-info.toml`:

```toml
[template]
src = "host-info.conf.tmpl"
dest = "/etc/myapp/host-info.conf"
mode = "0644"
keys = [
  "/meta-data/instance-id",
  "/meta-data/placement/availability-zone",
  "/meta-data/local-ipv4",
]

[backend]
backend = "imds"
```

### Example: File Backend for Local Overrides

Use etcd as the global backend but allow local file-based overrides for development or testing:

`/etc/confd/conf.d/local-overrides.toml`:

```toml
[template]
src = "overrides.conf.tmpl"
dest = "/etc/myapp/overrides.conf"
mode = "0644"
keys = [
  "/myapp/overrides",
]

[backend]
backend = "file"
file = ["/etc/myapp/local-config.yaml"]
```

## Available Backend Options

The `[backend]` section accepts all backend configuration options. See [configuration-guide.md](configuration-guide.md) for the full list.

Common options:

| Option | Description |
|--------|-------------|
| `backend` | Backend type (required): `consul`, `etcd`, `vault`, `redis`, `zookeeper`, `dynamodb`, `ssm`, `secretsmanager`, `acm`, `imds`, `env`, `file` |
| `nodes` | Backend node addresses |
| `scheme` | URI scheme (`http` or `https`) |
| `auth_token` | Authentication token |
| `auth_type` | Auth method (Vault: `token`, `approle`, `userpass`, `kubernetes`) |
| `role_id` / `secret_id` | AppRole credentials (Vault) |
| `username` / `password` | Basic auth or userpass credentials |
| `client_cert` / `client_key` / `client_cakeys` | TLS client certificates |
| `client_insecure` | Skip TLS verification |
| `file` | File paths (file backend) |
| `table` | DynamoDB table name |
| `separator` | Key separator (Redis) |

## Client Caching

When multiple template resources specify identical backend configurations, confd automatically shares a single connection. The client cache hashes the backend config (type, nodes, auth credentials, TLS settings) and reuses existing clients for matching configurations.

This means you can add `[backend]` sections to many template resources without worrying about connection overhead. Ten templates pointing at the same Vault cluster use one Vault client.

Operational parameters like timeouts and retry settings are excluded from the cache key — two configs that differ only in timeouts share the same client.

## Architecture Patterns

### Separation of Concerns

The most common multi-backend pattern separates config from secrets:

| Data Type | Backend | Example |
|-----------|---------|---------|
| Application config | Consul, etcd, or file | Feature flags, service URLs, tuning parameters |
| Secrets | Vault or Secrets Manager | Database passwords, API keys, TLS certificates |
| Infrastructure metadata | IMDS or SSM | Instance ID, availability zone, instance tags |
| Environment-specific overrides | env or file | Local development settings |

### Migration Strategy

Per-resource backends enable gradual migration between backends. To migrate from etcd to Consul:

1. Start with etcd as the global backend
2. Add `[backend]` sections pointing to Consul on templates as you migrate their keys
3. Once all templates use Consul, switch the global backend

No downtime or big-bang cutover required.

### Environment Differences

To handle different backends per environment (dev vs staging vs prod), use your deployment tooling to template the confd config files:

**Helm values:**

```yaml
# values-dev.yaml
confd:
  backend: env

# values-prod.yaml
confd:
  backend: vault
  vaultAddr: https://vault.prod.internal:8200
```

**Docker entrypoint:**

```bash
#!/bin/bash
case "${APP_ENV}" in
  development) exec confd env --onetime ;;
  staging)     exec confd consul --node consul.staging:8500 --watch ;;
  production)  exec confd vault --node https://vault.prod:8200 --auth-type approle --watch ;;
esac
```

This keeps environment logic in the deployment layer where it belongs, while confd focuses on template processing.

## Limitations

- **Watch mode**: Per-resource backends that don't support watch (Vault, SSM, ACM, IMDS, DynamoDB, Secrets Manager) will **not** receive updates when the global backend uses `--watch`. The watch processor calls `WatchPrefix` on all backends; for non-watch backends, this blocks until shutdown without processing. For templates using these backends, use `--interval` mode instead, or run separate confd instances for watch-capable and non-watch backends.
- **No fallback chains**: If a per-resource backend is unreachable, confd does not automatically try another backend. Use `--failure-mode best-effort` to continue processing other templates.
- **No per-key backends**: Backend override is per-template, not per-key. If you need keys from different backends in a single template, split them into separate templates and use `include` to compose the output.
