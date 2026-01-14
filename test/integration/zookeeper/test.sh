#!/bin/bash
set -e

export HOSTNAME="localhost"

# Wait for Zookeeper to be ready
# Note: Use 'srvr' instead of 'ruok' as newer Zookeeper versions disable 'ruok' by default
wait_for_zookeeper() {
    local retries=30
    while ! echo srvr | nc -w 1 "$ZOOKEEPER_NODE" 2181 2>/dev/null | grep -q "Zookeeper version"; do
        retries=$((retries - 1))
        if [[ $retries -eq 0 ]]; then
            echo "ERROR: Zookeeper not ready after 30 seconds" >&2
            exit 1
        fi
        sleep 1
    done
}

wait_for_zookeeper

# Feed zookeeper with test data
ZK_PATH="$(dirname "$0")"
(cd "$ZK_PATH" && go run main.go)

# Run confd
confd zookeeper --onetime --log-level debug --confdir ./test/integration/confdir --interval 5 --node "$ZOOKEEPER_NODE:2181" --watch
