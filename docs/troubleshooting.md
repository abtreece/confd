# Troubleshooting

This guide is organized by symptom. Find the message confd printed (or the behavior you are seeing), then follow the cause, how to confirm it, and the fix.

Every log line starts with a timestamp, the hostname, the program name with its PID, and the level (`DEBUG`, `INFO`, `WARN` or `ERROR`). Messages are shown here without that prefix, but the level is worth filtering on: everything that stops a template resource from being applied is logged at `ERROR`. Placeholders such as `<dest>` stand for values from your configuration. Errors that come from a backend's client library (etcd, Consul, Vault, Redis, the AWS SDK) are not quoted here: they appear after the confd text shown, and vary by backend and version.

## Start Here

Four commands answer most questions without touching a destination file. Each takes the same backend subcommand and flags as your normal invocation.

| Question | Command |
|----------|---------|
| Are my `conf.d/*.toml` files valid? | `confd --check-config <backend>` |
| Do my templates parse? | `confd --validate <backend>` |
| Can confd reach the backend and see my keys? | `confd --preflight <backend> ...` |
| What would confd write? | `confd --onetime --noop --diff <backend> ...` |

Things to know about them:

- `--check-config` and `--validate` never connect to a backend. They cannot catch connection, authentication or missing key problems, and they do not check whether the backend supports watch mode.
- `--validate` replaces `getv`, `gets`, `getvs`, `ls` and the other store functions with stubs that return empty values, so it finds syntax errors and unknown functions but not missing keys.
- `--preflight` only warns when a template resource matches no keys. `<template path>: no keys found for prefix <prefix>` is logged at warning level and the command still exits 0, so read its output rather than relying on the exit status.
- `--onetime` exits non-zero if any template resource fails, which makes it the quickest way to reproduce a problem from a shell. The status is usually 1, but when a `check_cmd` or `reload_cmd` fails confd exits with that command's own exit code.

Add `--log-level debug` to any of these to see each step: which resource files were found, the backend fetch, template cache hits and misses, stage file creation, and the full `check_cmd` and `reload_cmd` strings. See [Logging](logging.md) for levels and formats, and [Noop Mode](noop-mode.md) for diff output.

To look at the rendered output itself, run with `--keep-stage-file`. The staged file is left next to the destination as a hidden dotfile (for `dest = "/etc/myapp/app.conf"`, look for `/etc/myapp/.app.conf*`). A stage file only exists when the rendered output differs from the destination and passes `output_format` validation.

## confd Runs but Nothing Happens

### No template resources were loaded

```text
Cannot load template resources: config dir '<confdir>/conf.d': <reason>
```

```text
Found no templates
```

**Cause:** confd looks for template resources in `<confdir>/conf.d` and templates in `<confdir>/templates`. Either the directory does not exist (or is unreadable), or it contains no files ending in `toml`. Both are warnings, not errors: `--onetime` exits 0 having done nothing, and interval and watch mode keep running.

**Confirm:** run with `--log-level debug` and look for `Loading template resources from config dir <dir>` followed by one `Found template: <path>` line per resource file.

**Fix:** point `--confdir` at the directory that contains `conf.d/` and `templates/` (the default is `/etc/confd`), and make sure resource files end in `.toml`. In a container, check the volume mounts described in [Docker](docker.md).

### The destination is already up to date

At debug level:

```text
Target config <dest> in sync
```

**Cause:** the rendered output, file mode and ownership all match the destination, so confd does not write the file. `check_cmd` does not run when nothing changed, and `reload_cmd` runs only to retry a reload that failed on an earlier cycle (see [The reload command fails](#the-reload-command-fails)).

**Fix:** none needed. If you expected a change, the backend is returning the same values as before: check the keys and prefix (see [Missing key](#missing-key)).

### Changes in the backend are picked up slowly

**Cause:** without `--watch`, confd polls. The default `--interval` is 600 seconds.

**Fix:** lower `--interval`, or use `--watch` if your backend supports it (see [Watch Mode Problems](#watch-mode-problems)).

### Edits to a template resource are ignored

**Cause:** in interval and watch mode confd reads `conf.d/*.toml` once at startup. Changing a resource file, or adding one, has no effect on a running confd.

**Fix:** send `SIGHUP` to reload template resources, or restart confd. confd logs `Captured SIGHUP. Reloading configuration...` when it receives the signal. Settings from `confd.toml` and the command line, including the global backend, are only read at startup and need a restart. See [Service Deployment](service-deployment.md#sighup-reload).

## Template Resource Errors

### A resource file fails to load

```text
Failed to load template resource <path>: cannot process template resource <path>: <TOML error>
```

**Cause:** the file is not valid TOML, or a value has the wrong type (for example `keys = "/app"` instead of `keys = ["/app"]`, or `mode = 0644` instead of `mode = "0644"`).

**Confirm:** `confd --check-config <backend>` reports every resource file, either `OK: <path>` or `<path>: TOML parse error: <detail>` with a line number.

**Fix:** correct the file. [Template Resources](template-resources.md) lists every field and its type.

How much damage one bad file does depends on the mode:

- With `--onetime`, `--failure-mode` decides, as it does for errors during processing. Under `best-effort` (the default) confd processes the resources that did load. Under `fail-fast` it logs `Fail-fast mode: not processing any template resource after a load error` and processes nothing. Either way it exits non-zero, and the final error names each resource file that failed to load.
- In interval and watch mode (including `--batch-interval`), confd logs `Some template resources failed to load: <error>` and carries on with the resources that did load, whatever the failure mode. If none load, it logs `Failed to load any template resources: <error>`, shuts down gracefully and exits with status 1 and the error `no template resources could be loaded: <error>`, so a service manager records a failure. The same applies when a `SIGHUP` reload finds resource files and none of them load: confd stops rather than carrying on with the resources it had.

An empty or missing `conf.d` is not a load failure: see [No template resources were loaded](#no-template-resources-were-loaded).

### Other load-time errors

These are reported the same way, as `Failed to load template resource <path>: <error>`:

| Error | Cause |
|-------|-------|
| `empty src template` | `src` is missing from the `[template]` section. |
| `cannot find owner's UID: <detail>` | `owner` names a user that does not exist on this host. |
| `cannot find group's GID for group "<group>": <detail>` | `group` names a group that does not exist on this host. |
| `invalid min_reload_interval "<value>"`, `invalid debounce "<value>"`, `invalid check_cmd_timeout "<value>"`, `invalid reload_cmd_timeout "<value>"` | The value is not a Go duration such as `30s` or `1m`. |
| `invalid check_cmd template`, `invalid reload_cmd template` | The command is not a valid Go template. Only `{{.src}}` and (for `reload_cmd`) `{{.dest}}` are available. |
| `cannot create backend client for <path>: <detail>` | The resource's `[backend]` section could not be turned into a working client. See [Backend Connection Errors](#backend-connection-errors). |

### `--check-config` reports a field error

`--check-config` prints one line per problem in the form `<path>: <field>: <message>`:

| Message | Meaning |
|---------|---------|
| `required field is missing` | `src` or `dest` is not set. |
| `required field is missing or empty` | `keys` is not set or is an empty array. |
| `template file not found: <path>` | `src` does not exist under `<confdir>/templates`. |
| `destination directory does not exist: <dir>` | The parent directory of `dest` is missing. confd does not create it. |
| `must be a valid octal value (e.g., "0644"), got "<mode>"` | `mode` is not an octal string. |
| `unknown format: "<format>" (supported: json, yaml, yml, toml, xml)` | `output_format` is not one of the supported values. |
| `backend type is required when [backend] section is present` | The `[backend]` section has no `backend` key. |
| `unknown backend type: "<type>"` | The `backend` value is not a backend confd knows. |

### Paths resolve somewhere unexpected

- `src` is always relative to `<confdir>/templates`. `src = "nginx.conf.tmpl"` with the default `--confdir` means `/etc/confd/templates/nginx.conf.tmpl`. A missing template is reported as `missing template: <path>` with the full path confd looked for.
- `dest` is used exactly as written. A relative `dest` resolves against confd's working directory, which differs between an interactive shell and a service manager. Use absolute paths.
- `--resource` is relative to `<confdir>/conf.d` unless it is absolute. A wrong name gives `resource file not found: <path>`.
- `--config-file` defaults to `/etc/confd/confd.toml`. A missing config file is not an error and is not reported: confd silently falls back to flags and environment variables, so a typo in the path looks the same as having no config file. An unparseable one stops confd with `decode config file <path>: <detail>`.

## Template Errors

Template errors appear as an error log line and, in the default `best-effort` failure mode, only affect the resource that failed. The Go template engine supplies the template name, line number and the failing action, so read the whole line.

### Syntax error or unknown function

```text
unable to process template <path>: <parser error>
```

**Cause:** the template does not parse: an unclosed `{{`, a missing `{{end}}`, or a function name confd does not provide.

**Confirm:** `confd --validate <backend>` parses every template without a backend and reports `<path>: template parse error: <detail>` with the line number. Use `--resource <file>` to check a single resource.

**Fix:** correct the template. [Template Functions](templates.md) lists the available functions.

### Missing key

```text
execute template: <location>: error calling getv: key does not exist: <key>
```

**Cause:** `getv` (and `get`) fail when the key is not in the data confd fetched for this resource. The usual reasons:

1. **The key is not covered by `keys`.** confd only fetches what `keys` lists. A template that reads `/db/host` needs `/db` or `/db/host` in `keys`.
2. **Prefix confusion.** `prefix` is prepended to `keys` when confd queries the backend and stripped again before the template runs. With `prefix = "/myapp"` and `keys = ["/db"]`, confd fetches `/myapp/db/...` and the template must use `getv "/db/host"`, not `getv "/myapp/db/host"`.
3. **Two prefixes.** A global `--prefix` (or `prefix` in `confd.toml`) and a resource `prefix` are concatenated, global first: `production` plus `myapp` becomes `/production/myapp`.
4. **The key really is absent**, or the credentials confd uses cannot read it.

**Confirm:** `confd --preflight <backend> ...` reports per resource either `<template path>: <n> keys found` or the warning `<template path>: no keys found for prefix <prefix>`. The prefix it prints is the combined one confd actually uses. With `--log-level debug`, the `Backend fetch completed` record includes `value_count`, and the env, file and redis backends also log the fetched keys as `Key Map: ...`.

**Fix:** correct `keys` or `prefix`. If the key is legitimately optional, give `getv` a default (`{{getv "/db/port" "5432"}}`) or guard the block with `exists`.

### `getv`, `getvs`, `gets` and empty output

**Symptom:** the template renders without error but a section is empty.

**Cause:** the functions treat "nothing found" differently:

| Function | Argument | Nothing found |
|----------|----------|---------------|
| `getv` | exact key | error, unless a default is given |
| `get` | exact key | error |
| `getvs` | pattern | empty list, no error |
| `gets` | pattern | empty list, no error |
| `ls`, `lsdir` | path | empty list, no error |

A `range` over `getvs` or `gets` that matches nothing silently renders nothing. Patterns use shell-style matching in which `*` does not cross a `/`, so `/app/*` matches `/app/a` but not `/app/a/b`. A malformed pattern fails with `invalid pattern "<pattern>"`.

**Fix:** check the pattern against the keys actually fetched (see [Missing key](#missing-key)). `getv` returns a single string; to iterate use `getvs`, `gets` or `ls`.

### Errors from template functions

Function errors are reported as `execute template: <location>: error calling <function>: <detail>`. The detail text for confd's own functions:

| Detail | Cause |
|--------|-------|
| `unmarshal JSON object: ...` | `json` was given a value that is not a JSON object. |
| `unmarshal JSON array: ...` | `jsonArray` was given a value that is not a JSON array. |
| `division by zero`, `modulo by zero` | `div` or `mod` was called with a zero divisor. |
| `include cycle detected: <path> is already being processed` | Templates include each other in a loop. |
| `maximum include depth (10) exceeded` | Includes are nested more than 10 deep. |
| `include path "<name>" is outside template directory` | `include` may only reference files under `<confdir>/templates`. |

### Rendered output fails format validation

```text
output format validation failed (<format>): <detail>
```

**Cause:** the resource sets `output_format` and the rendered text is not valid in that format. The destination is left untouched.

**Fix:** inspect what was rendered. A stage file is not kept when validation fails, so temporarily remove `output_format` and run `confd --onetime --noop --diff <backend> ...` to see the output.

## Backend Connection Errors

confd reports backend failures in two places, with the client library's own error appended:

```text
create store client: initialize <backend> backend: <client error>
```

confd could not build the client at startup and exits. This is where bad addresses, unreadable certificates and failed logins show up for backends that connect eagerly.

```text
Backend fetch failed prefix=<prefix> key_count=<n> ... error=<client error>
fetch values from backend: <client error>
```

The client was created but a read failed. In interval and watch mode confd keeps running and tries again on the next cycle.

**Confirm:** `confd --preflight <backend> ...` runs a health check and a key read, then exits. A failed health check is reported as `global backend connectivity failed: <client error>`, or `per-resource backend connectivity failed for <src>: <client error>` for a resource with its own `[backend]` section. With `--metrics-addr` set, the `/ready` and `/ready/detailed` endpoints report backend health from a running confd.

**Fix:** work through the client error. Things to check on the confd side:

- Check the address confd is using. It logs `Backend source(s) set to <nodes>` at startup. If you passed no `--node`, consul, etcd, redis, vault and zookeeper fall back to a default address on `127.0.0.1`.
- `--backend-timeout` (default 30s) is the deadline confd puts on each key fetch, for every backend.
- The connection timeout and retry flags are **not** used by every backend. Changing one that your backend ignores has no effect:

  | Flag | Used by |
  |------|---------|
  | `--dial-timeout` | etcd, redis and zookeeper (connecting); imds (client setup); acm, dynamodb, secretsmanager and ssm (only the region lookup against instance metadata, and only when `AWS_REGION` is not set) |
  | `--read-timeout`, `--write-timeout` | redis |
  | `--retry-max-attempts`, `--retry-base-delay`, `--retry-max-delay` | redis |

  consul, vault, env and file use none of these.

  When one of these six flags is set for a backend that does not use it, confd logs a warning at startup and carries on:

  ```text
  --<flag> has no effect with the <backend> backend
  ```

  The setting is ignored, not rejected. The message names the flag even when the value came from its `CONFD_*` environment variable or its `confd.toml` key, so look in all three places. Remove the setting, or use `--backend-timeout`, which applies to every backend.
- TLS handshake errors come from the client library. For consul, etcd and vault, check `--client-ca-keys`, `--client-cert` and `--client-key`; for consul also check that `--scheme` matches what the agent serves.

Backend-specific messages that confd itself produces:

| Backend | Message | Cause and fix |
|---------|---------|---------------|
| etcd | `failed to read CA certificate file`, `failed to load client certificate key pair` | The file passed to `--client-ca-keys`, or the `--client-cert` / `--client-key` pair, is missing, unreadable or mismatched. See the [etcd README](../pkg/backends/etcd/README.md). |
| vault | `you have to set the auth type when using the vault backend` | `--auth-type` is required. |
| vault | `required parameter "<name>" is missing from configuration` | The chosen `--auth-type` needs a credential that was not supplied (for example `role-id` and `secret-id` for `app-role`). |
| vault | `vault <auth-type> login: <client error>` | Vault rejected the login. The client error that follows says why; check the credentials and, if the auth method is mounted somewhere other than its default, `--path`. See the [Vault README](../pkg/backends/vault/README.md). |
| redis | `failed to connect to any redis node after retries` | Every node failed; the per-node errors follow. See the [Redis README](../pkg/backends/redis/README.md). |
| ssm, secretsmanager, acm, dynamodb | `no AWS credentials found`, `failed to retrieve AWS credentials` | The AWS SDK credential chain found nothing usable. See the [SSM README](../pkg/backends/ssm/README.md) for the supported sources. |
| secretsmanager | `AWS region not found. ...` | No region is configured. Set `AWS_REGION` or a profile with a region. |
| dynamodb | `failed to describe table <table>` | confd describes the `--table` at startup and the call failed. Check the table name, then the client error that follows. |
| acm | `ACM_PASSPHRASE environment variable is required when exporting private keys` | `--acm-export-private-key` is set without a passphrase. |
| imds | `IMDS not available` | confd is not running on an EC2 instance, or IMDSv2 is unreachable from it. |
| file | `invalid file extension, YAML or JSON only` | `--file` matched a file whose extension is not `.yaml`, `.yml` or `.json` (a file with no extension is read as YAML). Use `--filter` to restrict which files in a directory are read. |

Authentication and permission denials from Consul ACLs, Vault policies, etcd auth and IAM are client errors appended to the messages above. confd does not interpret them.

## File and Permission Errors

confd renders to a temporary stage file in the **destination's directory**, sets its mode and ownership, then renames it over the destination. The user confd runs as therefore needs write access to that directory, not just to the destination file.

| Message | Cause | Fix |
|---------|-------|-----|
| `create stage file: <OS error>` | The destination directory does not exist or is not writable by confd. | Create the directory, or fix its ownership. confd does not create parent directories. |
| `failed to chown stage file: <OS error>` | The resource sets `uid`, `gid`, `owner` or `group` to something other than the user confd runs as, and confd lacks the privilege to change ownership. | Run confd as that user, run it with sufficient privilege, or remove the ownership fields. They default to confd's effective uid and gid. |
| `failed to chmod stage file: <OS error>` | The filesystem rejects the requested `mode`. | Check the mount (some volume types ignore or refuse mode changes). |
| `parse file mode "<mode>": <detail>` | `mode` is not a number. | Use an octal string such as `"0644"`. |
| `rename stage file to destination: <OS error>` | The rename over the destination failed. | Check the OS error. For a cross-device rename or a busy mount point, confd instead tries to write the destination in place; other rename errors are reported as shown. |
| `stat destination file: <OS error>` | confd cannot read the destination's metadata to compare it. | Give confd search permission on every directory in the path. |

If `mode` is not set, confd keeps the mode of an existing destination and uses `0644` for a new file.

A destination that confd rewrites on every run, with no change in content, usually means something else is changing its mode or ownership between runs. At info level confd logs why it considers the file different: `<dest> has mode <actual> should be <expected>`, `<dest> has UID <actual> should be <expected>`, `<dest> has GID <actual> should be <expected>`, `<dest> has size <actual> should be <expected>`, or `<dest> has md5sum <actual> should be <expected>`.

For container users and volume ownership, see the troubleshooting section of [Docker](docker.md#troubleshooting). For systemd sandboxing options that restrict writable paths, see [Service Deployment](service-deployment.md#security-hardening).

## check_cmd and reload_cmd Problems

Both commands run through `/bin/sh -c` (`cmd /C` on Windows) as the user confd runs as, with confd's environment. The log records look like this:

```text
Command failed command_type=check dest=<dest> timeout=30s exit_code=1 ... output=<output> error=<error>
```

The full command string is logged only at debug level, in the `Starting command execution` record, because commands can contain credentials. At other levels a command is identified by `command_type` (`check` or `reload`) and `dest`. On failure the command's combined stdout and stderr is included, truncated to 500 characters.

### The check command fails

```text
config check failed: <error>
```

**Cause:** `check_cmd` exited non-zero. confd discards the stage file and leaves the destination untouched, which is the point of a check command.

**Confirm:** read the `output` attribute of the `Command failed` record. To run the command by hand, get the exact string from `--log-level debug` and keep the rendered file with `--keep-stage-file`.

**Fix:** if the rendered config really is invalid, fix the template or the data. If the command itself is wrong, remember that `{{.src}}` is the path of the stage file, a hidden dotfile next to the destination, not the destination itself. Tools that locate included files relative to the config path, or that insist on a particular file name, may reject it.

### The reload command fails

```text
run command: <error>
```

**Cause:** `reload_cmd` exited non-zero. Read `exit_code` and `output` in the `Command failed` record. `exit_code=127` with a "command not found" message from the shell means the command is not on confd's `PATH`, which is often shorter under a service manager than in your shell: use absolute paths. A permission error in the output means the user confd runs as is not allowed to perform the reload.

**Retry:** the destination file has already been replaced when `reload_cmd` runs, so the application keeps running with its old configuration until a reload succeeds. confd remembers the failed reload and retries `reload_cmd` the next time it processes the resource, even though the destination is now in sync. It logs `Retrying reload_cmd for <dest> after an earlier failure` at info level, and a retry that fails is reported like the first failure. The retry stops after the first success.

- With `--interval` the retry runs on every cycle until it succeeds. With `--watch` the resource is only processed when a watched key changes, so the retry waits for the next change.
- A retry runs `reload_cmd` only: the file is not rendered again and `check_cmd` does not run. There is no stage file, so `{{.src}}` in `reload_cmd` is the destination path during a retry.
- The pending retry is held in memory. It is lost when confd restarts or reloads its template resources on `SIGHUP`, and a `--onetime` run cannot retry: it exits non-zero, and a second run finds the destination in sync and does not run `reload_cmd`. In those cases reload the application yourself, or remove the destination file so that confd regenerates it and reruns the commands.
- `--noop` and `--sync-only` never run `reload_cmd`, including retries.
- A failed retry is an error for that cycle. Under `--failure-mode fail-fast` in interval mode, a resource whose reload keeps failing therefore stops the resources after it on every cycle, and they are not rendered at all until the reload succeeds.

### A command times out

```text
Command timed out command_type=<type> dest=<dest> timeout=<timeout> ...
command timed out after <timeout>
```

**Cause:** the command ran longer than its timeout and was killed, along with its child processes. The defaults are 30s for `check_cmd` and 60s for `reload_cmd`.

**Fix:** raise `--check-cmd-timeout` or `--reload-cmd-timeout`, or set `check_cmd_timeout` / `reload_cmd_timeout` on the one resource that needs longer. A reload command must exit on its own: a command that starts a foreground process will always time out.

### The reload did not run

| Log line | Reason |
|----------|--------|
| `Target config <dest> in sync` (debug) | Nothing changed, so `check_cmd` does not run, and `reload_cmd` runs only to retry a reload that failed earlier. |
| `Reload throttled for <dest> (next allowed in <duration>)` | `min_reload_interval` has not elapsed since the last reload. The file was still updated; the reload for this change is skipped, not deferred. |
| `Noop mode enabled. <dest> will not be modified` | `--noop` is set (on the command line, as `CONFD_NOOP`, or `noop = true` in `confd.toml`). |
| none | `--sync-only` is set, which writes files but skips both commands. |

## Watch Mode Problems

### confd exits at startup with "watch mode not supported"

```text
check watch support: watch mode not supported for <backend> backend
```

```text
check watch support: watch mode not supported for <backend> backend (template resource <path>)
```

**Cause:** `--watch` (or `watch = true` in `confd.toml`, or `CONFD_WATCH`) is set and a backend in use cannot watch. The first form names the global backend. The second names a template resource whose own `[backend]` section uses a polling-only backend.

| Watch supported | Polling only |
|-----------------|--------------|
| consul, etcd, file, redis, zookeeper | acm, dynamodb, env, imds, secretsmanager, ssm, vault |

Every backend in use must support watching. confd refuses to start rather than run with some resources silently unwatched, and the check applies to `--onetime` and `--preflight` as well when `--watch` is set.

**Fix:** drop `--watch` and use `--interval` to poll. If only some resources use a polling-only backend, run those in a second confd instance in interval mode. See [Multi-Backend Architectures](multi-backend.md).

Two cases the startup check does not cover:

- `--check-config` and `--validate` exit before any backend client is created. They succeed for a configuration that would be rejected at startup. Use `--preflight` together with `--watch` to test the real combination.
- On `SIGHUP`, a resource that was changed to (or added with) a polling-only `[backend]` is not fatal. confd logs `watch mode not supported for <backend> backend (template resource <path>)` as an error, keeps running, and continues to watch the other resources, but that one resource is **not** watched and is not processed. Check the log after every reload.

### Changes are not picked up

**Cause and fix**, in the order worth checking:

1. **The change is outside what the resource watches.** A resource watches its `prefix` and `keys`. Changes elsewhere do not trigger it.
2. **Redis is not publishing keyspace notifications.** confd's Redis watch depends on them, and they have to be enabled on the Redis server. See [Watch Mode Support](../pkg/backends/redis/README.md#watch-mode-support) in the Redis README.
3. **The watch is failing.** When a watch call returns an error, confd logs it at error level and waits `--watch-error-backoff` (default 2s) before trying again. Look for error lines that repeat at about that spacing and treat them as a [backend connection error](#backend-connection-errors).
4. **The change is being delayed.** With `debounce` or `--debounce`, confd waits that long after a change before processing it. The wait is not extended by further changes: a key that changes continuously is processed about once per debounce period, not held back indefinitely. With `--batch-interval`, changes are processed when the interval expires. At debug level look for `Debouncing changes for <dest> (<duration>)` and `Queued change for batch processing: <dest>`.
5. **The change was processed but produced identical output.** See [The destination is already up to date](#the-destination-is-already-up-to-date).

### Too many renders or reloads, or high CPU

**Cause:** in watch mode every change under a watched prefix triggers a fetch and render for each resource watching it. A busy prefix, or a writer that updates many keys one at a time, causes a burst of work and possibly a burst of reloads.

**Confirm:** count `Target config <dest> has been updated` lines per minute, or with `--metrics-addr` set, watch `confd_template_process_total` and `confd_command_total`.

**Fix:**

- Set `debounce` on the resource, or `--debounce` globally, so a burst of changes is processed about once per debounce period instead of once per change.
- Use `--batch-interval` to process all changed resources together on a fixed interval.
- Set `min_reload_interval` on resources whose reload is expensive.
- Narrow `keys` so a resource does not watch more of the tree than its template reads.
- If the log shows a repeating watch error rather than real changes, raise `--watch-error-backoff` to slow the retry loop while you fix the underlying error.

`debounce`, `min_reload_interval` and batch processing are described with examples in [Template Resources](template-resources.md).

## Errors Are Logged but confd Keeps Running

**Cause:** this is the default. In interval and watch mode a failing resource is logged at error level and retried on the next cycle or change; other resources are unaffected. Only `--onetime` turns a processing error into a non-zero exit status; a long-running confd exits non-zero on its own only when [no resource file could be loaded](#a-resource-file-fails-to-load). While the backend is failing, the `/ready` endpoint (with `--metrics-addr` set) returns 503.

**Fix:** to stop a cycle at the first failure, use `--failure-mode fail-fast`; confd logs `Fail-fast mode: stopping after error in <dest>` and skips the remaining resources for that cycle. To detect failures from outside, alert on error-level log lines or, with `--metrics-addr` set, on `confd_template_process_total` with `status="error"` and on `confd_backend_errors_total`.

If the log contains `error dropped (errChan full) for <dest>: <error>`, confd's internal error queue was full when the error was raised. The error text is still in that line.

## Platform-Specific Issues

confd's behavior under systemd, Docker and Kubernetes is covered where those deployments are documented:

- **systemd:** unit files, `--systemd-notify`, watchdog failures, reload and shutdown timeouts are in [Service Deployment](service-deployment.md#troubleshooting).
- **Docker:** the non-root container user, volume mounts, reaching a backend on the host, and signals are in [Docker](docker.md#troubleshooting).
- **Kubernetes:** probes, sidecar layout and reloading after a ConfigMap update are in [Service Deployment](service-deployment.md#kubernetes-deployment) and [Docker](docker.md#kubernetes).

If `SIGHUP` or `SIGTERM` appear to be ignored, check the log for `Captured SIGHUP. Reloading configuration...` or the corresponding shutdown line. If it is absent, the signal did not reach the confd process.

## Getting More Help

If none of the above matches, collect the following before opening an issue:

- The output of `confd --version`.
- The full command line and `confd.toml`, with credentials removed.
- The template resource and template involved.
- Output from a run with `--log-level debug`. Debug output includes complete `check_cmd` and `reload_cmd` strings, and the env, file and redis backends log fetched values, so redact secrets before sharing it.
