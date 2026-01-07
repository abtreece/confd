#!/bin/bash

export HOSTNAME="localhost"
mkdir -p backends/json
cat <<EOT >> backends/json/1.json
{
  "key": "foobar",
  "database": {
    "host": "127.0.0.1",
    "password": "p@sSw0rd",
    "port": "3306",
    "username": "confd"
  }
}
EOT

cat <<EOT >> backends/json/2.json
{
  "upstream": {
    "app1": "10.0.1.10:8080",
    "app2": "10.0.1.11:8080"
  }
}
EOT

cat <<EOT >> backends/json/3.json
{
  "nested": {
    "production": {
      "app1": "10.0.1.10:8080",
      "app2": "10.0.1.11:8080"
    },
    "staging": {
      "app1": "172.16.1.10:8080",
      "app2": "172.16.1.11:8080"
    }
  }
}
EOT

# Run confd
confd file --onetime --log-level debug --confdir ./test/integration/confdir --file backends/json/ --watch

# Clean up after
rm -rf backends/json
