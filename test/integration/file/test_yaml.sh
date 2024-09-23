#!/bin/bash

export HOSTNAME="localhost"
mkdir -p backends/yaml
cat <<EOT >> backends/yaml/1
key: foobar
database:
  host: 127.0.0.1
  password: p@sSw0rd
  port: "3306"
  username: confd
EOT

cat <<EOT >> backends/yaml/2.yml
upstream:
  app1: 10.0.1.10:8080
  app2: 10.0.1.11:8080
EOT

cat <<EOT >> backends/yaml/3.yaml
nested:
  production:
    app1: 10.0.1.10:8080
    app2: 10.0.1.11:8080
  staging:
    app1: 172.16.1.10:8080
    app2: 172.16.1.11:8080
EOT

# Run confd
confd --onetime --log-level debug --confdir ./test/integration/confdir --backend file --file backends/yaml/ --watch

# Clean up after
rm -rf backends/yaml
