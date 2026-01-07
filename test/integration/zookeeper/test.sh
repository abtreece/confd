#!/bin/bash

export HOSTNAME="localhost"

# feed zookeeper
export ZK_PATH="`dirname \"$0\"`"
sh -c "cd $ZK_PATH; go run main.go"

# Run confd
confd zookeeper --onetime --log-level debug --confdir ./test/integration/confdir --interval 5 --node $ZOOKEEPER_NODE:2181 --watch
