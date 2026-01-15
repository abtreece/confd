# Quick Start Guide

Before we begin be sure to [download and install confd](installation.md).

## Select a backend

confd supports the following backends:

| Backend | Description | Watch Support | Documentation |
|---------|-------------|---------------|---------------|
| [env](../pkg/backends/env/README.md) | Environment variables | No | [Details](../pkg/backends/env/README.md) |
| [file](../pkg/backends/file/README.md) | Local YAML/JSON files | Yes | [Details](../pkg/backends/file/README.md) |
| [etcd](../pkg/backends/etcd/README.md) | etcd v3 key-value store | Yes | [Details](../pkg/backends/etcd/README.md) |
| [consul](../pkg/backends/consul/README.md) | HashiCorp Consul KV | Yes | [Details](../pkg/backends/consul/README.md) |
| [vault](../pkg/backends/vault/README.md) | HashiCorp Vault secrets | No | [Details](../pkg/backends/vault/README.md) |
| [redis](../pkg/backends/redis/README.md) | Redis key-value store | Yes | [Details](../pkg/backends/redis/README.md) |
| [zookeeper](../pkg/backends/zookeeper/README.md) | Apache ZooKeeper | Yes | [Details](../pkg/backends/zookeeper/README.md) |
| [dynamodb](../pkg/backends/dynamodb/README.md) | AWS DynamoDB | No | [Details](../pkg/backends/dynamodb/README.md) |
| [ssm](../pkg/backends/ssm/README.md) | AWS Systems Manager Parameter Store | No | [Details](../pkg/backends/ssm/README.md) |
| [secretsmanager](../pkg/backends/secretsmanager/README.md) | AWS Secrets Manager | No | [Details](../pkg/backends/secretsmanager/README.md) |
| [acm](../pkg/backends/acm/README.md) | AWS Certificate Manager | No | [Details](../pkg/backends/acm/README.md) |
| [imds](../pkg/backends/imds/README.md) | AWS EC2 Instance Metadata Service | No | [Details](../pkg/backends/imds/README.md) |

This quick start uses the **env** and **file** backends which require no external services. For production use cases, see the backend-specific documentation linked above.

## Quick Start with Environment Variables

The env backend is the simplest way to get started.

### 1. Set environment variables

```bash
export MYAPP_DATABASE_URL=db.example.com
export MYAPP_DATABASE_USER=admin
```

### 2. Create the confdir

```bash
sudo mkdir -p /etc/confd/{conf.d,templates}
```

### 3. Create a template resource config

`/etc/confd/conf.d/myconfig.toml`:

```toml
[template]
src = "myconfig.conf.tmpl"
dest = "/tmp/myconfig.conf"
keys = [
  "/myapp/database",
]
```

### 4. Create the source template

`/etc/confd/templates/myconfig.conf.tmpl`:

```
[myconfig]
database_url = {{getv "/myapp/database/url"}}
database_user = {{getv "/myapp/database/user"}}
```

### 5. Run confd

```bash
confd env --onetime
```

Check the output:

```bash
cat /tmp/myconfig.conf
```

## Quick Start with File Backend

The file backend reads configuration from local YAML or JSON files.

### 1. Create a YAML configuration file

`myapp.yaml`:

```yaml
myapp:
  database:
    url: db.example.com
    user: admin
```

### 2. Create the confdir

```bash
sudo mkdir -p /etc/confd/{conf.d,templates}
```

### 3. Create a template resource config

`/etc/confd/conf.d/myconfig.toml`:

```toml
[template]
src = "myconfig.conf.tmpl"
dest = "/tmp/myconfig.conf"
keys = [
  "/myapp/database",
]
```

### 4. Create the source template

`/etc/confd/templates/myconfig.conf.tmpl`:

```
[myconfig]
database_url = {{getv "/myapp/database/url"}}
database_user = {{getv "/myapp/database/user"}}
```

### 5. Run confd

```bash
confd file --file myapp.yaml --onetime
```

## Running Modes

confd supports two modes of operation:

### One-time mode

Process templates once and exit:

```bash
confd env --onetime
```

### Daemon mode with interval polling

Poll the backend at regular intervals:

```bash
confd env --interval 60
```

### Watch mode (supported backends only)

Watch for changes in real-time (etcd, consul, redis, zookeeper, file):

```bash
confd file --file myapp.yaml --watch
```

## Next Steps

- **Template Functions**: See [templates.md](templates.md) for available template functions
- **Configuration**: See [configuration-guide.md](configuration-guide.md) for all configuration options
- **Backend Details**: Click the backend links above for authentication, advanced options, and examples

## Advanced Example

This example manages nginx config files using etcd or consul.

### Add keys (etcd)

```bash
etcdctl put /myapp/subdomain myapp
etcdctl put /myapp/upstream/app1 "10.0.1.100:80"
etcdctl put /myapp/upstream/app2 "10.0.1.101:80"
```

### Add keys (consul)

```bash
consul kv put myapp/subdomain myapp
consul kv put myapp/upstream/app1 "10.0.1.100:80"
consul kv put myapp/upstream/app2 "10.0.1.101:80"
```

### Create template resource

`/etc/confd/conf.d/myapp-nginx.toml`:

```toml
[template]
prefix = "/myapp"
src = "nginx.tmpl"
dest = "/tmp/myapp.conf"
owner = "nginx"
mode = "0644"
keys = [
  "/subdomain",
  "/upstream",
]
check_cmd = "/usr/sbin/nginx -t -c {{.src}}"
reload_cmd = "/usr/sbin/service nginx reload"
```

### Create the source template

`/etc/confd/templates/nginx.tmpl`:

```
upstream {{getv "/subdomain"}} {
{{range getvs "/upstream/*"}}
    server {{.}};
{{end}}
}

server {
    server_name  {{getv "/subdomain"}}.example.com;
    location / {
        proxy_pass        http://{{getv "/subdomain"}};
        proxy_redirect    off;
        proxy_set_header  Host             $host;
        proxy_set_header  X-Real-IP        $remote_addr;
        proxy_set_header  X-Forwarded-For  $proxy_add_x_forwarded_for;
   }
}
```

### Run confd

```bash
# etcd
confd etcd --node http://127.0.0.1:2379 --watch

# consul
confd consul --node 127.0.0.1:8500 --watch
```
