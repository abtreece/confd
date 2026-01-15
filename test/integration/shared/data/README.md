# Centralized Test Data

This directory contains the canonical test data used by all integration tests.

## Files

- `test-data.json` - The master test data file in JSON format
- `load-consul.sh` - Load test data into Consul
- `load-redis.sh` - Load test data into Redis
- `load-etcd.sh` - Load test data into etcd
- `load-env.sh` - Export test data as environment variables (source this file)

## Test Data Structure

```json
{
  "key": "foobar",
  "database": {
    "host": "127.0.0.1",
    "password": "p@sSw0rd",
    "port": "3306",
    "username": "confd"
  },
  "upstream": {
    "app1": "10.0.1.10:8080",
    "app2": "10.0.1.11:8080"
  },
  "nested": {
    "production": {
      "app1": "10.0.1.10:8080",
      "app2": "10.0.1.11:8080"
    },
    "staging": {
      "app1": "172.16.1.10:8080",
      "app2": "172.16.1.11:8080"
    }
  }
}
```

## Usage

### Consul
```bash
export CONSUL_HOST=localhost CONSUL_PORT=8500
./test/integration/shared/data/load-consul.sh
```

### Redis
```bash
export REDIS_HOST=localhost
./test/integration/shared/data/load-redis.sh
```

### etcd
```bash
export ETCD_ENDPOINT=http://localhost:2379
./test/integration/shared/data/load-etcd.sh
```

### Environment Variables
```bash
source ./test/integration/shared/data/load-env.sh
# Sets: KEY, DATABASE_HOST, DATABASE_PORT, etc.
```
