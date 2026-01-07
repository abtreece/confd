#!/bin/bash -x

export HOSTNAME="localhost"
export ETCDCTL_API="3"

etcdctl put /key foobar --endpoints $ETCD_ENDPOINT
etcdctl put /database/host 127.0.0.1 --endpoints $ETCD_ENDPOINT
etcdctl put /database/password p@sSw0rd --endpoints $ETCD_ENDPOINT
etcdctl put /database/port 3306 --endpoints $ETCD_ENDPOINT
etcdctl put /database/username confd --endpoints $ETCD_ENDPOINT
etcdctl put /upstream/app1 10.0.1.10:8080 --endpoints $ETCD_ENDPOINT
etcdctl put /upstream/app2 10.0.1.11:8080 --endpoints $ETCD_ENDPOINT
etcdctl put /nested/production/app1 10.0.1.10:8080 --endpoints $ETCD_ENDPOINT
etcdctl put /nested/production/app2 10.0.1.11:8080 --endpoints $ETCD_ENDPOINT
etcdctl put /nested/staging/app1 172.16.1.10:8080 --endpoints $ETCD_ENDPOINT
etcdctl put /nested/staging/app2 172.16.1.11:8080 --endpoints $ETCD_ENDPOINT

# Run confd
confd etcd --onetime --log-level debug --confdir ./test/integration/confdir --node $ETCD_ENDPOINT --watch
