# DNS SRV Records

SRV records can be used to declare the backend nodes; just use the `--srv-domain` flag.

## Supported Backends

SRV discovery is supported for node-based backends:

- **consul** - Consul agent discovery
- **etcd** - etcd cluster discovery
- **redis** - Redis server discovery
- **vault** - Vault server discovery
- **zookeeper** - Zookeeper ensemble discovery

SRV discovery is **not supported** for backends that don't use network nodes:

- **env** - Environment variables (no network connection)
- **file** - File-based configuration (no network connection)
- **acm** - AWS Certificate Manager (uses AWS API endpoints)
- **dynamodb** - AWS DynamoDB (uses AWS API endpoints)
- **ssm** - AWS Systems Manager Parameter Store (uses AWS API endpoints)
- **secretsmanager** - AWS Secrets Manager (uses AWS API endpoints)

## Examples

### etcd

```
dig SRV _etcd._tcp.confd.io
```

```
...
;; ANSWER SECTION:
_etcd._tcp.confd.io.	300	IN	SRV	1 100 4001 etcd.confd.io.
```

-

```
confd etcd --srv-domain confd.io
```

### consul

```
dig SRV _consul._tcp.confd.io
```

```
...
;; ANSWER SECTION:
_consul._tcp.confd.io.	300	IN	SRV	1 100 8500 consul.confd.io.
```

-

```
confd consul --srv-domain confd.io
```

### redis

```
dig SRV _redis._tcp.confd.io
```

```
...
;; ANSWER SECTION:
_redis._tcp.confd.io.	300	IN	SRV	1 100 6379 redis.confd.io.
```

-

```
confd redis --srv-domain confd.io
```

### zookeeper

```
dig SRV _zookeeper._tcp.confd.io
```

```
...
;; ANSWER SECTION:
_zookeeper._tcp.confd.io.	300	IN	SRV	1 100 2181 zookeeper.confd.io.
```

-

```
confd zookeeper --srv-domain confd.io
```

### vault

```
dig SRV _vault._tcp.confd.io
```

```
...
;; ANSWER SECTION:
_vault._tcp.confd.io.	300	IN	SRV	1 100 8200 vault.confd.io.
```

-

```
confd vault --srv-domain confd.io --auth-token <token>
```

## The backend scheme

By default the `scheme` is set to http; change it with the `--scheme` flag.

```
confd etcd --scheme https --srv-domain confd.io
```

Both the SRV domain and scheme can be configured in the confd configuration file. See the [Configuration Guide](configuration-guide.md) for more details.

## Advanced Usage

### Custom SRV Records

If you need to use a custom SRV record name instead of the default `_<backend>._tcp.<domain>` format, use the `--srv-record` flag:

```bash
confd etcd --srv-record _etcd-client._tcp.example.com --scheme https
```

This is useful when:
- Your DNS infrastructure uses non-standard SRV record naming
- You have multiple clusters and need to distinguish between them
- You're using a service discovery system with custom naming conventions

### Multiple Clusters

You can use different SRV records for different environments:

```bash
# Production cluster
confd etcd --srv-record _etcd-prod._tcp.example.com

# Staging cluster
confd etcd --srv-record _etcd-staging._tcp.example.com
```
