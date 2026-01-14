#!/bin/bash
set -e

export HOSTNAME="localhost"
export KEY="foobar"
export DATABASE_HOST="127.0.0.1"
export DATABASE_PASSWORD="p@sSw0rd"
export DATABASE_PORT="3306"
export DATABASE_USERNAME="confd"
export UPSTREAM_APP1="10.0.1.10:8080"
export UPSTREAM_APP2="10.0.1.11:8080"
export NESTED_PRODUCTION_APP1="10.0.1.10:8080"
export NESTED_PRODUCTION_APP2="10.0.1.11:8080"
export NESTED_STAGING_APP1="app1=172.16.1.10:8080"
export NESTED_STAGING_APP2="app2=172.16.1.11:8080"

confd env --onetime --log-level debug --confdir ./test/integration/confdir --interval 5
