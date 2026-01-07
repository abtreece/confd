# File Backend

The file backend enables confd to retrieve configuration data from local YAML or JSON files. This is useful for local development, testing, or deployments where configuration is mounted from ConfigMaps or secrets.

## Configuration

No authentication is required. The file backend reads files from the local filesystem.

## Supported File Formats

- **YAML** (`.yaml`, `.yml`, or no extension)
- **JSON** (`.json`)

Nested structures are flattened to key paths:

```yaml
# config.yaml
myapp:
  database:
    url: db.example.com
    user: admin
```

Becomes:
- `/myapp/database/url` = `db.example.com`
- `/myapp/database/user` = `admin`

## Options

| Flag | Description | Default |
|------|-------------|---------|
| `--file` | Path to file or directory (can be specified multiple times) | Required |
| `--filter` | Glob pattern to filter files | `*` |

## Basic Example

Create a YAML configuration file (`/etc/myapp/values.yaml`):

```yaml
myapp:
  database:
    url: db.example.com
    user: admin
    password: secret123
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
confd file --file /etc/myapp/values.yaml --onetime
```

## Advanced Example

### Multiple Files

Read from multiple configuration files:

```bash
confd file \
  --file /etc/myapp/defaults.yaml \
  --file /etc/myapp/overrides.yaml --onetime
```

### Directory with Filter

Read all YAML files from a directory:

```bash
confd file \
  --file /etc/myapp/config.d/ \
  --filter "*.yaml" --onetime
```

### JSON Configuration

```json
{
  "myapp": {
    "database": {
      "url": "db.example.com",
      "user": "admin"
    },
    "cache": {
      "host": "redis.example.com",
      "port": 6379
    }
  }
}
```

### Kubernetes ConfigMap Volume

Mount a ConfigMap as a file:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: myapp-config
data:
  config.yaml: |
    myapp:
      database:
        url: db.example.com
        user: admin
---
apiVersion: v1
kind: Pod
metadata:
  name: myapp
spec:
  containers:
  - name: myapp
    volumeMounts:
    - name: config
      mountPath: /etc/myapp
  volumes:
  - name: config
    configMap:
      name: myapp-config
```

## Watch Mode Support

Watch mode **is supported** for the file backend. confd uses filesystem notifications to detect changes.

```bash
confd file --file /etc/myapp/values.yaml --watch
```

When files are modified, created, or deleted, confd automatically re-renders templates.

## Data Types

The file backend handles various YAML/JSON data types:

| YAML/JSON Type | confd Value |
|----------------|-------------|
| String | As-is |
| Integer | String representation |
| Float | String representation |
| Boolean | `true` or `false` |
| Array | Indexed keys (`/path/0`, `/path/1`, etc.) |
| Object | Nested keys |

Example with arrays:

```yaml
myapp:
  servers:
    - host: server1.example.com
      port: 8080
    - host: server2.example.com
      port: 8081
```

Access in templates:

```
{{range gets "/myapp/servers/*"}}
server {{.Key}} = {{.Value}}
{{end}}
```

## Use Cases

The file backend is ideal for:

- **Local development** without running external services
- **Testing** confd templates before deployment
- **Kubernetes** deployments with ConfigMaps/Secrets mounted as files
- **Static configuration** that doesn't need a key-value store
