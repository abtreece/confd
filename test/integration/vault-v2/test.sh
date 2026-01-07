#!/bin/bash

export HOSTNAME="localhost"

vault secrets enable -version 2 -path kv-v2 kv

# vault kv put kv-v2/exists key=foobar
vault kv put kv-v2/database host=127.0.0.1 port=3306 username=confd password=p@sSw0rd
vault kv put kv-v2/upstream app1=10.0.1.10:8080 app2=10.0.1.11:8080
vault kv put kv-v2/nested/production app1=10.0.1.10:8080 app2=10.0.1.11:8080
vault kv put kv-v2/nested/staging app1=172.16.1.10:8080 app2=172.16.1.11:8080

# Run confd
confd vault --onetime --log-level debug \
      --confdir ./test/integration/confdir \
      --auth-type token \
      --auth-token $VAULT_TOKEN \
      --prefix "/kv-v2" \
      --node $VAULT_ADDR

# Disable kv-v1 secrets for next tests
vault secrets disable kv-v2