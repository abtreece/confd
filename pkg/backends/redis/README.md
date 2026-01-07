# Redis Backend

The Redis backend enables confd to retrieve configuration data from [Redis](https://redis.io/). It supports string keys, hash fields, and pattern-based key retrieval.

## Configuration

### Basic Connection

Connect to Redis without authentication:

```bash
confd redis --node 127.0.0.1:6379 --onetime
```

### Authentication

#### Password

```bash
confd redis --node 127.0.0.1:6379 --password secret --onetime
```

Or via environment variable:

```bash
export REDIS_PASSWORD=secret
confd redis --node 127.0.0.1:6379 --onetime
```

### Database Selection

Specify a database number (default is 0):

```bash
confd redis --node 127.0.0.1:6379/4 --onetime
```

### Unix Socket

Connect via Unix socket:

```bash
confd redis --node /var/run/redis/redis.sock --onetime
```

### Key Separator

By default, confd uses `/` as the key separator. Use `--separator` to change this:

```bash
confd redis --node 127.0.0.1:6379 --separator : --onetime
```

This transforms `/myapp/database/url` to `myapp:database:url` when querying Redis.

## Options

| Flag | Description | Default |
|------|-------------|---------|
| `-n, --node` | Redis server address (host:port or socket path) | - |
| `--password` | Redis password | - |
| `--separator` | Character to replace `/` in keys | `/` |

## Data Types

The Redis backend supports multiple data types:

| Redis Type | confd Behavior |
|------------|----------------|
| String | Returns value directly |
| Hash | Returns all fields as nested keys |
| Keys (pattern) | Scans matching keys |

### String Keys

```bash
redis-cli SET /myapp/database/url "db.example.com"
```

Access as `/myapp/database/url`.

### Hash Fields

```bash
redis-cli HSET /myapp/database url "db.example.com" user "admin" password "secret"
```

Access fields as `/myapp/database/url`, `/myapp/database/user`, etc.

## Basic Example

Set keys in Redis:

```bash
redis-cli SET /myapp/database/url "db.example.com"
redis-cli SET /myapp/database/user "admin"
redis-cli SET /myapp/database/password "secret123"
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
confd redis --node 127.0.0.1:6379 --onetime
```

## Advanced Example

### Using Hash for Grouped Config

Store related config in a hash:

```bash
redis-cli HSET /myapp/database url "db.example.com" user "admin" password "secret"
redis-cli HSET /myapp/cache host "redis.example.com" port "6379"
```

Template:

```
[database]
url = {{getv "/myapp/database/url"}}
user = {{getv "/myapp/database/user"}}

[cache]
host = {{getv "/myapp/cache/host"}}
port = {{getv "/myapp/cache/port"}}
```

### Using Custom Separator

If your Redis keys use `:` as separator:

```bash
redis-cli SET myapp:database:url "db.example.com"
```

```bash
confd redis --node 127.0.0.1:6379 --separator : --onetime
```

confd will transform `/myapp/database/url` to `myapp:database:url`.

### Redis Sentinel (Manual)

Connect to a Redis instance behind Sentinel by specifying the master's address:

```bash
# Get master address from Sentinel
redis-cli -h sentinel1.example.com -p 26379 SENTINEL get-master-addr-by-name mymaster

# Use that address with confd
confd redis --node <master-ip>:<master-port> --watch
```

## Watch Mode Support

Watch mode **is supported** for the Redis backend. confd uses Redis keyspace notifications via PubSub.

```bash
confd redis --node 127.0.0.1:6379 --watch
```

### Enable Keyspace Notifications

Redis must be configured to emit keyspace notifications:

```bash
redis-cli CONFIG SET notify-keyspace-events AKE
```

Or in `redis.conf`:

```
notify-keyspace-events AKE
```

- `A` - Alias for all events
- `K` - Keyspace events
- `E` - Keyevent events

confd watches for these events: `set`, `del`, `append`, `rename_from`, `rename_to`, `expire`, `incrby`, `incrbyfloat`, `hset`, `hincrby`, `hincrbyfloat`, `hdel`.

## Connection Behavior

- **Connection timeout**: 1 second
- **Automatic reconnection**: Connections are tested with PING before use
- **Multiple nodes**: Tries each node in order until one connects (no clustering support)
