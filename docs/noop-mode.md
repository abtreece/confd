# Noop Mode

When in noop mode target configuration files will not be modified. This is useful for testing configuration changes before applying them.

## Usage

### Command-line flag

```bash
confd env --noop
```

### Configuration file

```toml
noop = true
```

## Diff Output

Use `--diff` to see what changes would be made in unified diff format:

```bash
confd --noop --diff consul
```

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `--diff` | Show unified diff output | `false` |
| `--diff-context` | Lines of context around changes | `3` |
| `--color` | Colorize diff output (red for removals, green for additions) | `false` |

### Example with diff

```bash
confd --noop --diff --color consul --onetime
```

Output:

```diff
--- /etc/myapp/config.conf (current)
+++ /etc/myapp/config.conf (proposed)
@@ -1,5 +1,5 @@
 # Application Configuration
-server_name = "old-server"
+server_name = "new-server"
 port = 8080
-debug = false
+debug = true
 log_level = "info"
```

## Example without diff

Without `--diff`, confd only logs that files would be modified:

```bash
confd env --onetime --noop
```

Output:

```
2014-07-08T22:30:10-07:00 confd[16397]: INFO /tmp/myconfig.conf has md5sum c1924fc5c5f2698e2019080b7c043b7a should be 8e76340b541b8ee29023c001a5e4da18
2014-07-08T22:30:10-07:00 confd[16397]: WARNING Noop mode enabled /tmp/myconfig.conf will not be modified
```

## Use Cases

- **Pre-deployment validation**: Test configuration changes in a staging environment before production
- **CI/CD pipelines**: Verify that templates render correctly without modifying files
- **Debugging**: Understand what changes confd would make to configuration files
- **Change review**: Generate diffs for review before applying changes
