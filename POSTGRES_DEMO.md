# PostgreSQL Backend: Ultimate Demo & Workflow

This guide provides a comprehensive demonstration of `confd` using the new PostgreSQL backend. It showcases advanced database features (triggers, audit logs, constraints), complex template generation, integration testing, and a full environment cleanup.

---

## 1. Setup the Environment

We will use `docker-compose` from the testing suite to spin up our PostgreSQL instance.

```bash
# Navigate to the confd root directory
cd /path/to/confd

# Start the PostgreSQL test environment
docker compose -f test/docker-compose.yml up -d postgres

# Wait a few seconds for the database to be ready
sleep 5
```

---

## 2. Advanced Database Schema & Constraints

We will create a robust database structure containing:
1. **`confd_config`**: The main configuration table.
2. **`config_audit`**: An audit table to track all historical changes.
3. **Triggers & Functions**: Automatic logging of changes and validations.

Execute the following command to inject the full schema:

```bash
docker compose -f test/docker-compose.yml exec -T postgres psql -U admin -d confd -c "
-- 1. Main Configuration Table
CREATE TABLE IF NOT EXISTS confd_config (
    key VARCHAR(255) PRIMARY KEY,
    value TEXT NOT NULL
);

-- 2. Audit Log Table
CREATE TABLE IF NOT EXISTS config_audit (
    id SERIAL PRIMARY KEY,
    key VARCHAR(255) NOT NULL,
    old_value TEXT,
    new_value TEXT,
    action VARCHAR(50),
    changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    changed_by VARCHAR(255)
);

-- 3. Audit Trigger Function
CREATE OR REPLACE FUNCTION log_config_change()
RETURNS TRIGGER AS \$\$
BEGIN
    IF (TG_OP = 'UPDATE' AND OLD.value <> NEW.value) THEN
        INSERT INTO config_audit (key, old_value, new_value, action, changed_by)
        VALUES (OLD.key, OLD.value, NEW.value, 'UPDATE', current_user);
    ELSIF (TG_OP = 'INSERT') THEN
        INSERT INTO config_audit (key, old_value, new_value, action, changed_by)
        VALUES (NEW.key, NULL, NEW.value, 'INSERT', current_user);
    ELSIF (TG_OP = 'DELETE') THEN
        INSERT INTO config_audit (key, old_value, new_value, action, changed_by)
        VALUES (OLD.key, OLD.value, NULL, 'DELETE', current_user);
    END IF;
    RETURN NULL;
END;
\$\$ LANGUAGE plpgsql;

-- 4. Apply the Trigger
DROP TRIGGER IF EXISTS audit_config_changes ON confd_config;
CREATE TRIGGER audit_config_changes
AFTER INSERT OR UPDATE OR DELETE ON confd_config
FOR EACH ROW EXECUTE FUNCTION log_config_change();

-- 5. Insert Advanced Mock Data
INSERT INTO confd_config (key, value) VALUES 
  ('/app/database/host', 'db-master.internal'),
  ('/app/database/port', '5432'),
  ('/app/feature_flags/new_ui', 'true'),
  ('/app/upstreams/web_01', '10.0.0.11:80'),
  ('/app/upstreams/web_02', '10.0.0.12:80'),
  ('/app/upstreams/web_03', '10.0.0.13:80');
"
```

---

## 3. Confd Templates Configuration

We will simulate a complex application load balancer configuration (like NGINX).

```bash
# Create local workspace
mkdir -p /tmp/confd-demo/conf.d
mkdir -p /tmp/confd-demo/templates
mkdir -p /tmp/confd-demo/output

# Create the TOML resource definition
cat << 'EOF' > /tmp/confd-demo/conf.d/app.toml
[template]
src = "app.conf.tmpl"
dest = "/tmp/confd-demo/output/app.conf"
keys = [
    "/app/database",
    "/app/upstreams",
    "/app/feature_flags"
]
EOF

# Create the complex Go Template
cat << 'EOF' > /tmp/confd-demo/templates/app.conf.tmpl
# ==========================================
# AUTO-GENERATED CONFIGURATION
# ==========================================

[database_connection]
endpoint = {{getv "/app/database/host"}}:{{getv "/app/database/port"}}

[features]
# The UI flag is currently: {{getv "/app/feature_flags/new_ui"}}

[load_balancer_pool]
{{range gets "/app/upstreams/*"}}
server {{base .Key}} address={{.Value}};
{{end}}
EOF
```

---

## 4. Run Confd

Execute `confd` using the `postgres` backend to generate our file.

```bash
./bin/confd postgres \
  --confdir /tmp/confd-demo \
  --node "127.0.0.1:5432" \
  --username "admin" \
  --password "secret" \
  --database "confd" \
  --table "confd_config" \
  --onetime \
  --log-level debug

# View the beautifully generated configuration
cat /tmp/confd-demo/output/app.conf
```

---

## 5. Live Updates & Audit Verification

Let's modify a value in the database, re-run `confd`, and check the audit logs!

```bash
# 1. Update the database (Simulating a scale-up)
docker compose -f test/docker-compose.yml exec -T postgres psql -U admin -d confd -c "
UPDATE confd_config SET value = '10.0.0.99:80' WHERE key = '/app/upstreams/web_02';
INSERT INTO confd_config (key, value) VALUES ('/app/upstreams/web_04', '10.0.0.14:80');
"

# 2. Re-run confd to sync changes
./bin/confd postgres \
  --confdir /tmp/confd-demo \
  --node "127.0.0.1:5432" \
  --username "admin" \
  --password "secret" \
  --onetime

# 3. Check the Audit Table to see who changed what!
docker compose -f test/docker-compose.yml exec -T postgres psql -U admin -d confd -c "
SELECT action, key, old_value, new_value, changed_by, changed_at FROM config_audit ORDER BY changed_at DESC LIMIT 5;
"
```

---

## 6. Running the Test Suite (Continuous Integration)

To ensure the PostgreSQL backend is robust and conforms to `confd` standards, you can run the integration tests. This executes a battery of tests against the container.

```bash
# Run the integration suite for all backends (including PostgreSQL)
make integration
```
*(Note: If you run this on macOS, the `check.sh` validation script might throw a `conditional binary operator` warning due to the old macOS default bash, but the generation works perfectly.)*

---

## 7. Complete Cleanup

Once you are done playing with the demo, here is how to completely clean up your environment, stop the databases, and delete the temporary files.

```bash
# 1. Stop and remove the Docker containers and networks
docker compose -f test/docker-compose.yml down -v

# 2. Remove any rogue manual containers (from earlier demos)
docker rm -f confd-postgres || true

# 3. Delete the temporary confd directory
rm -rf /tmp/confd-demo

# 4. Clean the bin directory (Optional)
make clean
```
