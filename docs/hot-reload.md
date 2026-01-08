# Hot Reload Configuration

Starting from version 0.20.0, confd supports hot reloading of its configuration without requiring a process restart. This allows you to dynamically reconfigure confd during runtime.

## Triggering a Reload

Send a `SIGHUP` signal to the running confd process:

```bash
# Find the process ID
pidof confd

# Send SIGHUP
kill -SIGHUP $(pidof confd)
```

## Reloadable Settings

The following settings can be changed via hot reload:

| Setting | Reloadable | Notes |
|---------|------------|-------|
| `interval` | ✅ Yes | Takes effect on next polling cycle |
| `log-level` | ✅ Yes | Takes effect immediately |
| `log-format` | ✅ Yes | Takes effect immediately |
| `prefix` | ✅ Yes | New prefix is used for subsequent template processing |
| `noop` | ✅ Yes | Takes effect immediately |
| `sync-only` | ✅ Yes | Takes effect immediately |
| Template resources (conf.d/*.toml) | ✅ Yes | Picked up on next processing cycle |
| `backend` (type) | ❌ No | Requires process restart |
| `watch` | ❌ No | Requires process restart |
| Backend connection settings | ❌ No | Requires process restart |

## Template Resource Hot Reload

You can add, modify, or remove template resource files in the `conf.d/` directory and they will be picked up after sending `SIGHUP`:

```bash
# Add new template resource
cp new-service.toml /etc/confd/conf.d/
kill -SIGHUP $(pidof confd)

# Modify existing template resource
vim /etc/confd/conf.d/nginx.toml
kill -SIGHUP $(pidof confd)

# Remove template resource
rm /etc/confd/conf.d/old-service.toml
kill -SIGHUP $(pidof confd)
```

Template resources are automatically loaded from the `conf.d/` directory on each processing cycle, so changes are picked up even without SIGHUP in interval mode.

## Reload Behavior

When confd receives `SIGHUP`:

1. **Parse Configuration**: Reads and parses the config file (`confd.toml`)
2. **Validate Changes**: Ensures no non-reloadable settings were modified
3. **Apply Settings**: Updates reloadable settings in memory
4. **Log Changes**: Reports what was changed
5. **Continue Processing**: Resumes normal operation with new settings

Example log output:

```
INFO: Received SIGHUP, reloading configuration
INFO: Configuration validated successfully
INFO: Applied changes:
INFO:   - log_level: info -> debug
INFO:   - interval: 600 -> 300
INFO:   - prefix: /myapp -> /newapp
INFO: Configuration reload complete
```

## Error Handling

If the configuration reload fails (e.g., trying to change a non-reloadable setting):

- An error message is logged
- The existing configuration is kept
- confd continues running normally with the old configuration

Example error:

```
INFO: Received SIGHUP, reloading configuration
ERROR: Configuration reload failed: watch mode cannot be changed via reload
ERROR: Keeping existing configuration
```

## Use Cases

### Dynamic Log Level Changes

Increase logging verbosity for debugging without restart:

```toml
# Change in /etc/confd/confd.toml
log-level = "debug"
```

```bash
kill -SIGHUP $(pidof confd)
```

### Interval Adjustment

Reduce polling interval during critical updates:

```toml
# Change in /etc/confd/confd.toml
interval = 60  # Poll every minute instead of every 10 minutes
```

```bash
kill -SIGHUP $(pidof confd)
```

### Adding Services

Deploy new services without confd downtime:

```bash
# Add new template
cat > /etc/confd/conf.d/newservice.toml << 'EOF'
[template]
src = "newservice.conf.tmpl"
dest = "/etc/newservice/config.conf"
keys = ["/newservice"]
reload_cmd = "systemctl reload newservice"
EOF

# Reload confd
kill -SIGHUP $(pidof confd)
```

## Best Practices

1. **Test Configuration**: Validate your config changes before sending SIGHUP
2. **Monitor Logs**: Watch confd logs during reload to confirm success
3. **Avoid Frequent Reloads**: While safe, excessive reloads should be avoided
4. **Use with Caution**: Understand which settings are reloadable before modifying config

## Limitations

- Cannot change backend type (etcd, consul, etc.) without restart
- Cannot enable/disable watch mode without restart
- Backend connection parameters (nodes, certificates) require restart
- The reload only affects confd's own configuration, not the backend data
