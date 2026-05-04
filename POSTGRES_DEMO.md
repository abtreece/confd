# PostgreSQL Backend: The Ultimate Masterclass Demo

This guide is the absolute ultimate demonstration of `confd` with the PostgreSQL backend. It tests **everything**:
1. Continuous Daemon mode (`--interval`).
2. `confd` safety mechanisms (`check_cmd`, `reload_cmd`).
3. Complex Template Functions (`getv`, `gets`, `getenv`, `datetime`).
4. **PostgreSQL Validation Triggers** (preventing bad data at the database level).
5. **PostgreSQL Audit Triggers** (tracking every single change).

---

## 1. Environment Setup

```bash
cd /path/to/confd

# Start PostgreSQL using the test suite docker-compose
docker compose -f test/docker-compose.yml up -d postgres
sleep 5

# Create our demo workspace
mkdir -p /tmp/confd-demo/conf.d
mkdir -p /tmp/confd-demo/templates
mkdir -p /tmp/confd-demo/output
```

---

## 2. Advanced Database: Validation & Auditing

We will inject a schema that not only logs history, but actively **blocks** developers from inserting invalid configuration (like a port > 65535 or a malformed IP).

```bash
docker compose -f test/docker-compose.yml exec -T postgres psql -U admin -d confd -c "
-- 1. Tables
CREATE TABLE IF NOT EXISTS confd_config (key VARCHAR(255) PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS config_audit (id SERIAL PRIMARY KEY, key VARCHAR(255), old_value TEXT, new_value TEXT, action VARCHAR(50), changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, changed_by VARCHAR(255));

-- 2. Audit Function
CREATE OR REPLACE FUNCTION log_audit() RETURNS TRIGGER AS \$\$
BEGIN
    IF (TG_OP = 'UPDATE') THEN INSERT INTO config_audit (key, old_value, new_value, action, changed_by) VALUES (OLD.key, OLD.value, NEW.value, 'UPDATE', current_user);
    ELSIF (TG_OP = 'INSERT') THEN INSERT INTO config_audit (key, old_value, new_value, action, changed_by) VALUES (NEW.key, NULL, NEW.value, 'INSERT', current_user);
    END IF; RETURN NULL;
END; \$\$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_audit AFTER INSERT OR UPDATE ON confd_config FOR EACH ROW EXECUTE FUNCTION log_audit();

-- 3. VALIDATION Function (The Bouncer)
CREATE OR REPLACE FUNCTION validate_config() RETURNS TRIGGER AS \$\$
BEGIN
    -- Prevent invalid ports
    IF NEW.key = '/app/database/port' AND (NEW.value::int < 1 OR NEW.value::int > 65535) THEN
        RAISE EXCEPTION 'Invalid port number: %', NEW.value;
    END IF;
    -- Prevent empty upstream IPs
    IF NEW.key LIKE '/app/upstreams/%' AND length(NEW.value) < 7 THEN
        RAISE EXCEPTION 'Invalid IP Address length for %', NEW.key;
    END IF;
    RETURN NEW;
END; \$\$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_validation BEFORE INSERT OR UPDATE ON confd_config FOR EACH ROW EXECUTE FUNCTION validate_config();

-- 4. Initial Valid Data
INSERT INTO confd_config (key, value) VALUES 
  ('/app/database/host', 'db-master.internal'),
  ('/app/database/port', '5432'),
  ('/app/feature_flags/new_ui', 'true'),
  ('/app/feature_flags/maintenance', 'false'),
  ('/app/upstreams/web_01', '10.0.0.11:80'),
  ('/app/upstreams/web_02', '10.0.0.12:80');
"
```

---

## 3. Confd: Advanced Configuration

We will define a TOML file that uses `check_cmd` to test the configuration *before* applying it, and `reload_cmd` to simulate restarting a service.

### The Resource Definition (`conf.d/app.toml`)
```bash
cat << 'EOF' > /tmp/confd-demo/conf.d/app.toml
[template]
src = "app.conf.tmpl"
dest = "/tmp/confd-demo/output/app.conf"
keys = [
    "/app"
]

# confd will run this command on a temporary file. If it returns an error, the config is REJECTED!
# We simulate a syntax check (the file must contain the word 'Syntax:OK' to be valid)
check_cmd = "grep -q 'Syntax:OK' {{.src}}"

# If the check passes, confd runs this to reload the service
reload_cmd = "echo '====> [APP RELOADED] The new configuration is applied! <===='"
EOF
```

### The Template (`templates/app.conf.tmpl`)
This template uses every trick in the book.

```bash
cat << 'EOF' > /tmp/confd-demo/templates/app.conf.tmpl
# Generated at: {{datetime}}
# Node Env: {{getenv "USER"}}

# To pass the confd check_cmd, the following line is required based on maintenance mode:
{{if eq (getv "/app/feature_flags/maintenance") "true"}}
Syntax:INVALID (System is in maintenance)
{{else}}
Syntax:OK
{{end}}

[database_connection]
endpoint = {{getv "/app/database/host"}}:{{getv "/app/database/port"}}

[load_balancer_pool]
{{range gets "/app/upstreams/*"}}
server {{base .Key}} address={{.Value}};
{{end}}
EOF
```

---

## 4. Run Confd in Daemon Mode (Background)

We launch `confd` in the background (`&`). It will poll PostgreSQL every **3 secondes**.

```bash
./bin/confd postgres \
  --confdir /tmp/confd-demo \
  --node "127.0.0.1:5432" \
  --username "admin" \
  --password "secret" \
  --database "confd" \
  --table "confd_config" \
  --interval 3 &

CONFD_PID=$!
```

*Wait 3 seconds... You should see `INFO Target config /tmp/confd-demo/output/app.conf has been updated` and `====> [APP RELOADED]... <====` in your terminal logs!*

---

## 5. The Ultimate Tests!

Keep an eye on the background `confd` logs in your terminal while you run these SQL commands in a new terminal tab!

### Test A: Database Validation Blocking Bad Data (SQL Error)
Let's try to set the port to `99999`. The PostgreSQL trigger should block it instantly.
```bash
docker compose -f test/docker-compose.yml exec -T postgres psql -U admin -d confd -c "
UPDATE confd_config SET value = '99999' WHERE key = '/app/database/port';
"
```
*(Result: You will get a SQL ERROR: `Invalid port number: 99999`. Confd won't even see the change).*

### Test B: Confd Safety Blocking Bad Config (Confd Error)
Let's activate maintenance mode. Our template is designed to write `Syntax:INVALID` if maintenance is true. `confd` will run the `check_cmd`, it will fail, and it will **REFUSE** to apply the config!
```bash
docker compose -f test/docker-compose.yml exec -T postgres psql -U admin -d confd -c "
UPDATE confd_config SET value = 'true' WHERE key = '/app/feature_flags/maintenance';
"
```
*(Result: Look at your confd logs! It says `ERROR check command failed` and the old `app.conf` remains completely untouched! No service interruption).*

### Test C: A Valid Update & Auto-Reload
Let's put maintenance back to false, and add a new upstream node.
```bash
docker compose -f test/docker-compose.yml exec -T postgres psql -U admin -d confd -c "
UPDATE confd_config SET value = 'false' WHERE key = '/app/feature_flags/maintenance';
INSERT INTO confd_config (key, value) VALUES ('/app/upstreams/web_03', '10.0.0.13:80');
"
```
*(Result: Within 3 seconds, `confd` will see the change, the `check_cmd` will pass, and you will see `====> [APP RELOADED]... <====` in the logs!)*

You can verify the final file:
```bash
cat /tmp/confd-demo/output/app.conf
```

---

## 6. Audit Trail Verification

Let's look at the database to see all the historical changes tracked by our trigger:

```bash
docker compose -f test/docker-compose.yml exec -T postgres psql -U admin -d confd -c "
SELECT action, key, old_value, new_value, changed_at FROM config_audit ORDER BY changed_at DESC LIMIT 5;
"
```

---

## 7. Advanced: Using SQL Views Instead of Tables

`confd` only runs `SELECT key, value FROM <table>`. This means `<table>` can actually be a **PostgreSQL VIEW** that aggregates data from multiple business tables!

### Step 1: Create Business Tables and a View
Let's simulate a company that has separate tables for servers and app settings, and create a View that unifies them into the `confd` format:

```bash
docker compose -f test/docker-compose.yml exec -T postgres psql -U admin -d confd -c "
CREATE TABLE internal_servers (id SERIAL, hostname VARCHAR(50), ip VARCHAR(50), role VARCHAR(50));
INSERT INTO internal_servers (hostname, ip, role) VALUES ('db-prod-1', '10.0.1.100', 'database'), ('web-prod-1', '10.0.1.10', 'web');

CREATE TABLE app_settings (app_name VARCHAR(50), is_maintenance BOOLEAN);
INSERT INTO app_settings (app_name, is_maintenance) VALUES ('my_app', false);

-- The Magic View!
CREATE VIEW confd_business_view AS 
SELECT '/app/database/host' AS key, ip AS value FROM internal_servers WHERE role = 'database'
UNION ALL
SELECT '/app/database/port' AS key, '5432' AS value
UNION ALL
SELECT '/app/upstreams/' || hostname AS key, ip || ':80' AS value FROM internal_servers WHERE role = 'web'
UNION ALL
SELECT '/app/feature_flags/maintenance' AS key, is_maintenance::text AS value FROM app_settings WHERE app_name = 'my_app';
"
```

### Step 2: Run Confd against the View
First, kill the existing `confd` process if it's still running:
```bash
pkill confd
```

Now, point `confd` to the View using `--table confd_business_view` (and `--onetime` just to see the result immediately):
```bash
./bin/confd postgres \
  --confdir /tmp/confd-demo \
  --node "127.0.0.1:5432" \
  --username "admin" \
  --password "secret" \
  --database "confd" \
  --table "confd_business_view" \
  --onetime
```

### Step 3: Verify the output
Look at the generated config. It was built using data from the `internal_servers` and `app_settings` tables via the View!
```bash
cat /tmp/confd-demo/output/app.conf
```

---

## 8. Stop Everything (Cleanup)

To stop the background `confd` daemon and clean up the docker environment:

```bash
# Kill the confd background process
kill $CONFD_PID

# Remove the docker containers
docker compose -f test/docker-compose.yml down -v

# Delete the demo workspace
rm -rf /tmp/confd-demo
```
