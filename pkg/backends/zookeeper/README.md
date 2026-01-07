# Zookeeper Backend

The Zookeeper backend enables confd to retrieve configuration data from [Apache ZooKeeper](https://zookeeper.apache.org/), a distributed coordination service.

## Configuration

### Basic Connection

Connect to ZooKeeper:

```bash
confd zookeeper --node 127.0.0.1:2181 --onetime
```

Multiple nodes for high availability:

```bash
confd zookeeper \
  --node zk1.example.com:2181 \
  --node zk2.example.com:2181 \
  --node zk3.example.com:2181 --onetime
```

## Options

| Flag | Description | Default |
|------|-------------|---------|
| `-n, --node` | ZooKeeper server address (can be specified multiple times) | - |

Note: ZooKeeper authentication (SASL/Kerberos) is not currently supported by this backend.

## Basic Example

Create znodes in ZooKeeper:

```bash
# Using zkCli.sh
zkCli.sh create /myapp ""
zkCli.sh create /myapp/database ""
zkCli.sh create /myapp/database/url "db.example.com"
zkCli.sh create /myapp/database/user "admin"
zkCli.sh create /myapp/database/password "secret123"
```

Or using the ZooKeeper shell:

```
[zk: localhost:2181(CONNECTED) 0] create /myapp ""
[zk: localhost:2181(CONNECTED) 1] create /myapp/database ""
[zk: localhost:2181(CONNECTED) 2] create /myapp/database/url "db.example.com"
[zk: localhost:2181(CONNECTED) 3] create /myapp/database/user "admin"
[zk: localhost:2181(CONNECTED) 4] create /myapp/database/password "secret123"
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
confd zookeeper --node 127.0.0.1:2181 --onetime
```

## Advanced Example

### Service Discovery Pattern

Store service endpoints in ZooKeeper:

```bash
zkCli.sh create /services ""
zkCli.sh create /services/api ""
zkCli.sh create /services/api/node1 "10.0.1.100:8080"
zkCli.sh create /services/api/node2 "10.0.1.101:8080"
zkCli.sh create /services/api/node3 "10.0.1.102:8080"
```

Template for load balancer config:

```
upstream api {
{{range gets "/services/api/*"}}
    server {{.Value}};
{{end}}
}
```

### Hierarchical Configuration

```bash
# Environment-specific config
zkCli.sh create /config ""
zkCli.sh create /config/production ""
zkCli.sh create /config/production/database ""
zkCli.sh create /config/production/database/host "prod-db.example.com"
zkCli.sh create /config/production/database/port "5432"
```

Template resource with prefix:

```toml
[template]
prefix = "/config/production"
src = "app.conf.tmpl"
dest = "/etc/app/config.conf"
keys = [
  "/database",
]
```

### Kubernetes Deployment

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: myapp
spec:
  containers:
  - name: myapp
    command:
    - confd
    - zookeeper
    - --node=zk-0.zk-headless.default.svc:2181
    - --node=zk-1.zk-headless.default.svc:2181
    - --node=zk-2.zk-headless.default.svc:2181
    - --watch
```

## Watch Mode Support

Watch mode **is supported** for the ZooKeeper backend. confd uses ZooKeeper's native watch mechanism for real-time updates.

```bash
confd zookeeper --node 127.0.0.1:2181 --watch
```

confd watches:
- **Data changes**: `NodeDataChanged` events on leaf nodes
- **Child changes**: `NodeChildrenChanged` events on parent nodes

When any watched znode changes, confd re-renders affected templates.

## ZooKeeper Data Model

ZooKeeper uses a hierarchical namespace similar to a filesystem:

```
/
├── myapp
│   ├── database
│   │   ├── url = "db.example.com"
│   │   ├── user = "admin"
│   │   └── password = "secret"
│   └── cache
│       ├── host = "redis.example.com"
│       └── port = "6379"
```

- **Znodes** can have both data and children
- **Leaf znodes** (no children) store the actual configuration values
- **Parent znodes** are traversed recursively to find all values

## Connection Behavior

- **Session timeout**: 1 second
- **Automatic reconnection**: ZooKeeper client handles reconnection automatically
- **Ensemble support**: Specify multiple nodes for automatic failover
