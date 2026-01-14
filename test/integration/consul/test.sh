#!/bin/bash
set -e

export HOSTNAME="localhost"

# Wait for Consul to be ready
wait_for_consul() {
    local retries=30
    while ! curl -sf "http://$CONSUL_HOST:$CONSUL_PORT/v1/status/leader" > /dev/null 2>&1; do
        retries=$((retries - 1))
        if [[ $retries -eq 0 ]]; then
            echo "ERROR: Consul not ready after 30 seconds" >&2
            exit 1
        fi
        sleep 1
    done
}

wait_for_consul

# Clean up any existing keys
curl -sX DELETE "http://$CONSUL_HOST:$CONSUL_PORT/v1/kv/?recurse=true" > /dev/null || true

# Populate test data
curl -sX PUT "http://$CONSUL_HOST:$CONSUL_PORT/v1/kv/key" -d 'foobar'
curl -sX PUT "http://$CONSUL_HOST:$CONSUL_PORT/v1/kv/database/host" -d '127.0.0.1'
curl -sX PUT "http://$CONSUL_HOST:$CONSUL_PORT/v1/kv/database/password" -d 'p@sSw0rd'
curl -sX PUT "http://$CONSUL_HOST:$CONSUL_PORT/v1/kv/database/port" -d '3306'
curl -sX PUT "http://$CONSUL_HOST:$CONSUL_PORT/v1/kv/database/username" -d 'confd'
curl -sX PUT "http://$CONSUL_HOST:$CONSUL_PORT/v1/kv/upstream/app1" -d '10.0.1.10:8080'
curl -sX PUT "http://$CONSUL_HOST:$CONSUL_PORT/v1/kv/upstream/app2" -d '10.0.1.11:8080'
curl -sX PUT "http://$CONSUL_HOST:$CONSUL_PORT/v1/kv/nested/production/app1" -d '10.0.1.10:8080'
curl -sX PUT "http://$CONSUL_HOST:$CONSUL_PORT/v1/kv/nested/production/app2" -d '10.0.1.11:8080'
curl -sX PUT "http://$CONSUL_HOST:$CONSUL_PORT/v1/kv/nested/staging/app1" -d '172.16.1.10:8080'
curl -sX PUT "http://$CONSUL_HOST:$CONSUL_PORT/v1/kv/nested/staging/app2" -d '172.16.1.11:8080'

# Run confd
confd consul --onetime --log-level debug --confdir ./test/integration/confdir --node "$CONSUL_HOST:$CONSUL_PORT"
