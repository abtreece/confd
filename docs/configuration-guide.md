# Configuration Guide

The confd configuration file is written in [TOML](https://github.com/mojombo/toml)
and loaded from `/etc/confd/confd.toml` by default. You can specify the config file via the `--config-file` command line flag.

> Note: You can use confd without a configuration file. See [Command Line Flags](command-line-flags.md).

Optional:

* `backend` (string) - The backend to use. ("etcd")
* `client_cakeys` (string) - The client CA key file.
* `client_cert` (string) - The client cert file.
* `client_key` (string) - The client key file.
* `confdir` (string) - The path to confd configs. ("/etc/confd")
* `interval` (int) - The backend polling interval in seconds. (600)
* `log-format` (string) - format of log messages ("text" or "json")
* `log-level` (string) - level which confd should log messages ("info")
* `nodes` (array of strings) - List of backend nodes. (["http://127.0.0.1:4001"])
* `noop` (bool) - Enable noop mode. Process all template resources; skip target update.
* `prefix` (string) - The string to prefix to keys. This prefix is concatenated with any prefix set in template resource files (e.g., global `production` + resource `myapp` = `/production/myapp`). ("/")
* `scheme` (string) - The backend URI scheme. ("http" or "https")
* `srv_domain` (string) - The name of the resource record.
* `srv_record` (string) - The SRV record to search for backends nodes.
* `sync-only` (bool) - sync without check_cmd and reload_cmd.
* `watch` (bool) - Enable watch support.
* `auth_token` (string) - Auth bearer token to use.
* `auth_type` (string) - Vault auth backend type to use.
* `basic_auth` (bool) - Use Basic Auth to authenticate (consul and etcd backends only).
* `table` (string) - The name of the DynamoDB table (dynamodb backend only).
* `separator` (string) - The separator to replace '/' with when looking up keys in the backend, prefixed '/' will also be removed (redis backend only).
* `username` (string) - The username to authenticate as (vault and etcd backends only).
* `password` (string) - The password to authenticate with (vault and etcd backends only).
* `app_id` (string) - Vault app-id to use with the app-id backend (vault backend with auth-type=app-id only).
* `user_id` (string) - Vault user-id to use with the app-id backend (vault backend with auth-type=app-id only).
* `role_id` (string) - Vault role-id to use with the AppRole, Kubernetes backends (vault backend with auth-type=app-role or auth-type=kubernetes only).
* `secret_id` (string) - Vault secret-id to use with the AppRole backend (vault backend with auth-type=app-role only).
* `file` (array of strings) - The YAML file to watch for changes (file backend only).
* `filter` (string) - Files filter (file backend only) (default "*").
* `path` (string) - Vault mount path of the auth method (vault backend only).

Example:

```TOML
backend = "etcd"
client_cert = "/etc/confd/ssl/client.crt"
client_key = "/etc/confd/ssl/client.key"
confdir = "/etc/confd"
log-level = "debug"
interval = 600
nodes = [
  "http://127.0.0.1:4001",
]
noop = false
prefix = "/production"
scheme = "https"
srv_domain = "etcd.example.com"
```
