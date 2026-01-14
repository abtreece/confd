#!/bin/bash
set -e

export HOSTNAME="localhost"

# Wait for Zookeeper to be ready
wait_for_zookeeper() {
    local retries=30
    while ! echo ruok | nc -w 1 "$ZOOKEEPER_NODE" 2181 2>/dev/null | grep -q imok; do
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
