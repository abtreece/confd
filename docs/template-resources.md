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
* `output_format` (string) - Validate the rendered output as a specific format. Supported formats: `json`, `yaml`, `yml`, `toml`, `xml`. If validation fails, the template processing aborts and the destination file is not updated.
* `min_reload_interval` (string) - Minimum time between reload command executions for this resource. Uses Go duration format (e.g., `30s`, `1m`, `500ms`). If changes occur more frequently, reloads are throttled and a warning is logged.
* `debounce` (string) - Wait for changes to settle before processing in watch mode. Uses Go duration format (e.g., `2s`, `500ms`). After detecting a change, confd waits this duration before processing. Additional changes during this period reset the timer. Useful for reducing unnecessary reloads when multiple keys change in rapid succession.

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

## Example with Output Format Validation

This example validates that the rendered template is valid JSON before writing to the destination:

```TOML
[template]
src = "config.json.tmpl"
dest = "/etc/myapp/config.json"
mode = "0644"
keys = [
  "/myapp/config",
]
output_format = "json"
reload_cmd = "/usr/bin/systemctl reload myapp"
```

If the template renders invalid JSON, confd will log an error and skip updating the destination file. This prevents configuration errors from propagating to your application.

## Example with Rate Limiting

This example limits reload commands to at most once per 30 seconds, even if the configuration changes more frequently:

```TOML
[template]
src = "nginx.conf.tmpl"
dest = "/etc/nginx/nginx.conf"
keys = [
  "/nginx",
]
reload_cmd = "/usr/sbin/service nginx reload"
min_reload_interval = "30s"
```

If a change is detected within 30 seconds of the last reload, confd will log a warning and skip the reload command. This is useful for expensive reload operations or services that need time to stabilize after a reload.

## Example with Debouncing (Watch Mode)

This example waits for changes to settle for 2 seconds before processing, useful in watch mode when multiple keys may change in rapid succession:

```TOML
[template]
src = "app.conf.tmpl"
dest = "/etc/myapp/app.conf"
keys = [
  "/myapp/config",
]
reload_cmd = "/usr/bin/systemctl reload myapp"
debounce = "2s"
```

When running in watch mode (`--watch`), if multiple keys under `/myapp/config` change within 2 seconds, confd will only process the template once after all changes have settled. This reduces unnecessary template renders and reload commands.

You can also set a global debounce for all templates using the `--debounce` command-line flag:

```bash
confd --watch --debounce 2s consul
```

Per-resource debounce settings override the global setting.

## Example with Batch Processing (Watch Mode)

Batch processing collects changes from multiple templates and processes them together after a batch interval. This is useful when you have many templates and want to reduce the overhead of processing each change individually.

```bash
confd --watch --batch-interval 5s consul
```

When running in watch mode with `--batch-interval`, all template changes are collected and processed together every 5 seconds. This differs from debouncing:

- **Debouncing**: Per-template, waits for a single template's changes to settle
- **Batch Processing**: Global, collects changes from all templates and processes them together

Batch processing is especially useful when:
- You have many templates that may change simultaneously
- Template processing is expensive and you want to minimize processing overhead
- Multiple related keys change in quick succession across different templates

Note: When batch processing is enabled, individual template debounce settings are still respected within the batch window.
