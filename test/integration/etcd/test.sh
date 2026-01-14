#!/bin/bash
set -e

export HOSTNAME="localhost"
export ETCDCTL_API="3"

# Wait for etcd to be ready
wait_for_etcd() {
    local retries=30
    while ! etcdctl endpoint health --endpoints "$ETCD_ENDPOINT" > /dev/null 2>&1; do
        retries=$((retries - 1))
        if [[ $retries -eq 0 ]]; then
            echo "ERROR: etcd not ready after 30 seconds" >&2
            exit 1
        fi
        sleep 1
    done
}

wait_for_etcd

# Clean up any existing keys
etcdctl del "" --prefix --endpoints "$ETCD_ENDPOINT" > /dev/null 2>&1 || true

# Populate test data
etcdctl put /key foobar --endpoints "$ETCD_ENDPOINT"
etcdctl put /database/host 127.0.0.1 --endpoints "$ETCD_ENDPOINT"
etcdctl put /database/password p@sSw0rd --endpoints "$ETCD_ENDPOINT"
etcdctl put /database/port 3306 --endpoints "$ETCD_ENDPOINT"
etcdctl put /database/username confd --endpoints "$ETCD_ENDPOINT"
etcdctl put /upstream/app1 10.0.1.10:8080 --endpoints "$ETCD_ENDPOINT"
etcdctl put /upstream/app2 10.0.1.11:8080 --endpoints "$ETCD_ENDPOINT"
etcdctl put /nested/production/app1 10.0.1.10:8080 --endpoints "$ETCD_ENDPOINT"
etcdctl put /nested/production/app2 10.0.1.11:8080 --endpoints "$ETCD_ENDPOINT"
etcdctl put /nested/staging/app1 172.16.1.10:8080 --endpoints "$ETCD_ENDPOINT"
etcdctl put /nested/staging/app2 172.16.1.11:8080 --endpoints "$ETCD_ENDPOINT"

# Run confd
confd etcd --onetime --log-level debug --confdir ./test/integration/confdir --node "$ETCD_ENDPOINT" --watch
