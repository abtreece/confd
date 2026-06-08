#!/bin/bash
set -e

export HOSTNAME="localhost"
export PGPASSWORD="secret"

# Wait for postgres to be ready
wait_for_postgres() {
    local retries=30
    while ! docker compose -f test/docker-compose.yml exec -T postgres psql -U admin -d confd -c "SELECT 1" > /dev/null 2>&1; do
        retries=$((retries - 1))
        if [[ $retries -eq 0 ]]; then
            echo "ERROR: postgres not ready after 30 seconds" >&2
            exit 1
        fi
        sleep 1
    done
}

wait_for_postgres

# Clean up and setup table
docker compose -f test/docker-compose.yml exec -T postgres psql -U admin -d confd -c "DROP TABLE IF EXISTS confd_config;" > /dev/null
docker compose -f test/docker-compose.yml exec -T postgres psql -U admin -d confd -c "CREATE TABLE confd_config (key VARCHAR(255) PRIMARY KEY, value TEXT NOT NULL);" > /dev/null

# Populate test data
docker compose -f test/docker-compose.yml exec -T postgres psql -U admin -d confd -c "
INSERT INTO confd_config (key, value) VALUES 
('/key', 'foobar'),
('/database/host', '127.0.0.1'),
('/database/password', 'p@sSw0rd'),
('/database/port', '3306'),
('/database/username', 'confd'),
('/upstream/app1', '10.0.1.10:8080'),
('/upstream/app2', '10.0.1.11:8080'),
('/nested/production/app1', '10.0.1.10:8080'),
('/nested/production/app2', '10.0.1.11:8080'),
('/nested/staging/app1', '172.16.1.10:8080'),
('/nested/staging/app2', '172.16.1.11:8080');
" > /dev/null

# Run confd
confd postgres --onetime --log-level debug --confdir ./test/integration/shared/confdir --node "127.0.0.1:5432" --username "admin" --password "secret" --database "confd" --table "confd_config"
