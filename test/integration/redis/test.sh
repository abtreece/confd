#!/bin/bash

export HOSTNAME="localhost"

redis-cli -h $REDIS_HOST set /key foobar
redis-cli -h $REDIS_HOST set /database/host 127.0.0.1
redis-cli -h $REDIS_HOST set /database/password p@sSw0rd
redis-cli -h $REDIS_HOST set /database/port 3306
redis-cli -h $REDIS_HOST set /database/username confd
redis-cli -h $REDIS_HOST set /upstream/app1 10.0.1.10:8080
redis-cli -h $REDIS_HOST set /upstream/app2 10.0.1.11:8080
redis-cli -h $REDIS_HOST set /nested/production/app1 10.0.1.10:8080
redis-cli -h $REDIS_HOST set /nested/production/app2 10.0.1.11:8080
redis-cli -h $REDIS_HOST set /nested/staging/app1 172.16.1.10:8080
redis-cli -h $REDIS_HOST set /nested/staging/app2 172.16.1.11:8080


confd redis --onetime --log-level debug --confdir ./integration/confdir --interval 5 --node $REDIS_HOST:6379
if [ $? -ne 0 ]
then
        exit 1
fi

confd redis --onetime --log-level debug --confdir ./test/integration/confdir --interval 5 --node $REDIS_HOST:6379/0
if [ $? -ne 0 ]
then
        exit 1
fi
