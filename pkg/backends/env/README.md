# Environment Variables Backend

The env backend enables confd to retrieve configuration data from environment variables. This is the simplest backend, requiring no external services or authentication.

## Configuration

No configuration is required. The env backend reads directly from the process environment.

## Key Mapping

Environment variable names are mapped to confd keys using the following transformation:

| confd key | Environment variable |
|-----------|---------------------|
| `/myapp/database/url` | `MYAPP_DATABASE_URL` |
| `/myapp/database/user` | `MYAPP_DATABASE_USER` |
| `/config/api/key` | `CONFIG_API_KEY` |

The transformation rules:
1. Remove the leading `/`
2. Replace `/` with `_`
3. Convert to uppercase

When confd retrieves values, it reverses this transformation to present keys in the standard `/path/format`.

## Options

The env backend has no backend-specific flags.

## Basic Example

Set environment variables:

```bash
export MYAPP_DATABASE_URL=db.example.com
export MYAPP_DATABASE_USER=admin
export MYAPP_DATABASE_PASSWORD=secret123
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
confd env --onetime
```

## Advanced Example

Using with Docker or Kubernetes:

**Docker:**

```bash
docker run -e MYAPP_DATABASE_URL=db.example.com \
           -e MYAPP_DATABASE_USER=admin \
           -v /etc/confd:/etc/confd \
           myapp-with-confd
```

**Kubernetes ConfigMap:**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: myapp-config
data:
  MYAPP_DATABASE_URL: "db.example.com"
  MYAPP_DATABASE_USER: "admin"
---
apiVersion: v1
kind: Pod
metadata:
  name: myapp
spec:
  containers:
  - name: myapp
    envFrom:
    - configMapRef:
        name: myapp-config
```

**Kubernetes Secrets:**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: myapp-secrets
type: Opaque
stringData:
  MYAPP_DATABASE_PASSWORD: "secret123"
---
apiVersion: v1
kind: Pod
metadata:
  name: myapp
spec:
  containers:
  - name: myapp
    envFrom:
    - secretRef:
        name: myapp-secrets
```

## Watch Mode Support

Watch mode is **not supported** for the env backend. Environment variables are only read at startup or when using interval mode.

For periodic updates:

```bash
confd env --interval 60
```

## Use Cases

The env backend is ideal for:

- **12-factor applications** following environment-based configuration
- **Container deployments** where config is injected via environment
- **Local development** with simple configuration needs
- **CI/CD pipelines** where secrets are passed as environment variables
- **Serverless functions** that receive configuration via environment
