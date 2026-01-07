#!/bin/bash

export HOSTNAME="localhost"

vault secrets enable -version 1 -path kv-v1 kv

#vault write kv-v1/key key=foobar
vault write kv-v1/database host=127.0.0.1 port=3306 username=confd password=p@sSw0rd
vault write kv-v1/upstream app1=10.0.1.10:8080 app2=10.0.1.11:8080
vault write kv-v1/nested/production app1=10.0.1.10:8080 app2=10.0.1.11:8080
vault write kv-v1/nested/staging app1=172.16.1.10:8080 app2=172.16.1.11:8080

# Run confd
confd vault --onetime --log-level debug \
      --confdir ./test/integration/confdir \
      --auth-type token \
      --auth-token $VAULT_TOKEN \
      --prefix "kv-v1" \
      --node $VAULT_ADDR

# Disable kv-v1 secrets for next tests
vault secrets disable kv-v1
