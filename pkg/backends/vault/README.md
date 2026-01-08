# Vault Backend

The Vault backend enables confd to retrieve configuration data from [HashiCorp Vault](https://www.vaultproject.io/). It supports the KV secrets engine (both v1 and v2) and multiple authentication methods.

## Configuration

### Authentication Methods

Vault requires authentication. The `--auth-type` flag specifies which method to use.

#### Token Authentication

The simplest method using a Vault token directly.

```bash
confd vault --node http://127.0.0.1:8200 \
  --auth-type token --auth-token s.XXXXXXXXXXXX --onetime
```

#### AppRole Authentication

Recommended for machine-to-machine authentication.

```bash
confd vault --node http://127.0.0.1:8200 \
  --auth-type app-role --role-id <role-id> --secret-id <secret-id> --onetime
```

To use a custom mount path:

```bash
confd vault --node http://127.0.0.1:8200 \
  --auth-type app-role --role-id <role-id> --secret-id <secret-id> \
  --path my-approle --onetime
```

#### Kubernetes Authentication

For workloads running in Kubernetes. Automatically reads the service account JWT from `/var/run/secrets/kubernetes.io/serviceaccount/token`.

```bash
confd vault --node http://vault.vault:8200 \
  --auth-type kubernetes --role-id <vault-role> --onetime
```

See [kubernetes-auth.md](kubernetes-auth.md) for a detailed setup guide.

#### Username/Password Authentication

```bash
confd vault --node http://127.0.0.1:8200 \
  --auth-type userpass --username <user> --password <pass> --onetime
```

#### GitHub Authentication

```bash
confd vault --node http://127.0.0.1:8200 \
  --auth-type github --auth-token <github-token> --onetime
```

#### TLS Certificate Authentication

Uses client TLS certificates for authentication.

```bash
confd vault --node https://127.0.0.1:8200 \
  --auth-type cert \
  --client-cert /path/to/client.crt \
  --client-key /path/to/client.key \
  --client-ca-keys /path/to/ca.crt --onetime
```

#### App-ID Authentication (Deprecated)

Legacy authentication method, use AppRole instead.

```bash
confd vault --node http://127.0.0.1:8200 \
  --auth-type app-id --app-id <app-id> --user-id <user-id> --onetime
```

### TLS Configuration

For Vault servers using TLS:

```bash
confd vault --node https://vault.example.com:8200 \
  --auth-type token --auth-token s.XXXX \
  --client-cert /path/to/client.crt \
  --client-key /path/to/client.key \
  --client-ca-keys /path/to/ca.crt --onetime
```

## Options

| Flag | Description | Required |
|------|-------------|----------|
| `-n, --node` | Vault server address | Yes |
| `--auth-type` | Authentication method (token, app-role, kubernetes, userpass, github, cert, app-id) | Yes |
| `--auth-token` | Token for token/github auth | Depends on auth-type |
| `--role-id` | Role ID for app-role auth, or role name for kubernetes auth | Depends on auth-type |
| `--secret-id` | Secret ID for app-role auth | Depends on auth-type |
| `--username` | Username for userpass auth | Depends on auth-type |
| `--password` | Password for userpass auth | Depends on auth-type |
| `--app-id` | App ID for app-id auth (deprecated) | Depends on auth-type |
| `--user-id` | User ID for app-id auth (deprecated) | Depends on auth-type |
| `--path` | Custom mount path for auth method | No (defaults to auth method name) |
| `--client-cert` | Path to client certificate | No |
| `--client-key` | Path to client private key | No |
| `--client-ca-keys` | Path to CA certificate | No |

## Basic Example

Store secrets in Vault:

```bash
# Enable KV v2 secrets engine
vault secrets enable -path=myapp kv-v2

# Write secrets
vault kv put myapp/database url=db.example.com user=admin password=secret
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
confd vault --node http://127.0.0.1:8200 \
  --auth-type token --auth-token $(vault print token) --onetime
```

## Advanced Example

Using AppRole in a production environment with TLS:

```bash
# Create AppRole
vault auth enable approle
vault write auth/approle/role/confd \
  token_policies="confd-policy" \
  token_ttl=1h \
  token_max_ttl=4h

# Get credentials
vault read auth/approle/role/confd/role-id
vault write -f auth/approle/role/confd/secret-id

# Run confd
confd vault --node https://vault.example.com:8200 \
  --auth-type app-role \
  --role-id 12345678-1234-1234-1234-123456789012 \
  --secret-id abcdefgh-abcd-abcd-abcd-abcdefghijkl \
  --client-ca-keys /etc/ssl/certs/vault-ca.crt \
  --interval 60
```

## Watch Mode Support

Watch mode is **not supported** for the Vault backend. Use interval mode (`--interval`) for periodic polling.

## Per-Resource Backend Configuration

Instead of using the global backend, individual template resources can specify their own Vault backend configuration. This is especially useful for fetching secrets from Vault while using a different backend for application config.

Add a `[backend]` section to your template resource file:

```toml
[template]
src = "secrets.conf.tmpl"
dest = "/etc/myapp/secrets.conf"
mode = "0600"
keys = [
  "/secret/data/myapp",
]

[backend]
backend = "vault"
nodes = ["https://vault.example.com:8200"]
auth_type = "approle"
role_id = "my-role-id"
secret_id = "my-secret-id"
client_cakeys = "/path/to/ca.crt"
```

Available backend options:
- `backend` - Must be `"vault"`
- `nodes` - Array with Vault server address (only first is used)
- `auth_type` - Authentication method: `token`, `app-role`, `kubernetes`, `userpass`, `github`, `cert`, `app-id`
- `auth_token` - Token for token/github auth
- `role_id` - Role ID for app-role auth, or role name for kubernetes auth
- `secret_id` - Secret ID for app-role auth
- `username` - Username for userpass auth
- `password` - Password for userpass auth
- `app_id` - App ID for app-id auth (deprecated)
- `user_id` - User ID for app-id auth (deprecated)
- `path` - Custom mount path for auth method
- `client_cert` - Path to client certificate
- `client_key` - Path to client private key
- `client_cakeys` - Path to CA certificate

## KV Secrets Engine Versions

The Vault backend automatically detects whether you're using KV v1 or KV v2 secrets engine and handles the path differences accordingly. Secrets are flattened to individual key-value pairs for use in templates.
