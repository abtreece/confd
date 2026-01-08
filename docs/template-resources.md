# Template Resources

Template resources are written in TOML and define a single template resource.
Template resources are stored under the `/etc/confd/conf.d` directory by default.

### Required

* `dest` (string) - The target file.
* `keys` (array of strings) - An array of keys.
* `src` (string) - The relative path of a [configuration template](templates.md).

### Optional

* `gid` (int) - The gid that should own the file. Defaults to the effective gid.
* `mode` (string) - The permission mode of the file.
* `uid` (int) - The uid that should own the file. Defaults to the effective uid.
* `reload_cmd` (string) - The command to reload config. Use `{{.src}}` to reference the rendered source template, or `{{.dest}}` to reference the destination file.
* `check_cmd` (string) - The command to check config. Use `{{.src}}` to reference the rendered source template.
* `prefix` (string) - The string to prefix to keys. When a global prefix is also set in `confd.toml`, the prefixes are concatenated (e.g., global `production` + resource `myapp` = `/production/myapp`).

### Per-Resource Backend Configuration

Template resources can optionally specify their own backend configuration using a `[backend]` section. This allows different templates to fetch data from different backends within a single confd instance.

When a `[backend]` section is present, it overrides the global backend configured via the command line. If no `[backend]` section is present, the global backend is used.

**Backend Options:**

* `backend` (string) - The backend type: `consul`, `etcd`, `vault`, `redis`, `zookeeper`, `dynamodb`, `ssm`, `secretsmanager`, `acm`, `env`, or `file`.
* `nodes` (array of strings) - Backend node addresses.
* `scheme` (string) - URL scheme (`http` or `https`).
* `client_cert` (string) - Path to client certificate file.
* `client_key` (string) - Path to client key file.
* `client_cakeys` (string) - Path to CA certificate file.
* `client_insecure` (bool) - Skip TLS verification.
* `basic_auth` (bool) - Enable basic authentication.
* `username` (string) - Username for authentication.
* `password` (string) - Password for authentication.
* `auth_token` (string) - Authentication token (for Vault).
* `auth_type` (string) - Authentication type (for Vault): `token`, `app-id`, `userpass`, or `approle`.

See the backend-specific documentation for additional options.

### Notes

When using the `reload_cmd` feature it's important that the command exits on its own. The reload
command is not managed by confd, and will block the configuration run until it exits.

## Example

```TOML
[template]
src = "nginx.conf.tmpl"
dest = "/etc/nginx/nginx.conf"
uid = 0
gid = 0
mode = "0644"
keys = [
  "/nginx",
]
check_cmd = "/usr/sbin/nginx -t -c {{.src}}"
reload_cmd = "/usr/sbin/service nginx restart"
```

## Example with Per-Resource Backend

This example fetches secrets from Vault while the main application config comes from the global backend (e.g., Consul):

```TOML
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
