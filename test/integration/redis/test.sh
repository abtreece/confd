#!/bin/bash
set -e

export HOSTNAME="localhost"

# Wait for Redis to be ready
wait_for_redis() {
    local retries=30
    while ! redis-cli -h "$REDIS_HOST" ping > /dev/null 2>&1; do
        retries=$((retries - 1))
        if [[ $retries -eq 0 ]]; then
            echo "ERROR: Redis not ready after 30 seconds" >&2
            exit 1
        fi
        sleep 1
    done
}

wait_for_redis

# Clean up any existing keys
redis-cli -h "$REDIS_HOST" FLUSHDB > /dev/null

# Populate test data
redis-cli -h "$REDIS_HOST" set /key foobar
redis-cli -h "$REDIS_HOST" set /database/host 127.0.0.1
redis-cli -h "$REDIS_HOST" set /database/password p@sSw0rd
redis-cli -h "$REDIS_HOST" set /database/port 3306
redis-cli -h "$REDIS_HOST" set /database/username confd
redis-cli -h "$REDIS_HOST" set /upstream/app1 10.0.1.10:8080
redis-cli -h "$REDIS_HOST" set /upstream/app2 10.0.1.11:8080
redis-cli -h "$REDIS_HOST" set /nested/production/app1 10.0.1.10:8080
redis-cli -h "$REDIS_HOST" set /nested/production/app2 10.0.1.11:8080
redis-cli -h "$REDIS_HOST" set /nested/staging/app1 172.16.1.10:8080
redis-cli -h "$REDIS_HOST" set /nested/staging/app2 172.16.1.11:8080

# Test with host:port format
confd redis --onetime --log-level debug --confdir ./test/integration/confdir --interval 5 --node "$REDIS_HOST:$REDIS_PORT"

# Test with host:port/db format
confd redis --onetime --log-level debug --confdir ./test/integration/confdir --interval 5 --node "$REDIS_HOST:$REDIS_PORT/0"
