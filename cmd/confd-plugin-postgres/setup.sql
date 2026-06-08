-- Confd Plugin Demo: Database Setup
-- This script creates the table, triggers, and initial data.

DROP TABLE IF EXISTS confd_config CASCADE;
DROP TABLE IF EXISTS config_audit CASCADE;

-- Core config table
CREATE TABLE confd_config (
    key VARCHAR(255) PRIMARY KEY,
    value TEXT
);

-- Audit trail table
CREATE TABLE config_audit (
    id SERIAL,
    action VARCHAR(10),
    key VARCHAR(255),
    old_value TEXT,
    new_value TEXT,
    changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    changed_by VARCHAR(50)
);

-- Trigger 1: Validation (reject invalid port numbers)
CREATE OR REPLACE FUNCTION validate_config() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.key = '/app/database/port' AND (NEW.value::int < 1 OR NEW.value::int > 65535) THEN
        RAISE EXCEPTION 'Invalid port number: %', NEW.value;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_validation
    BEFORE INSERT OR UPDATE ON confd_config
    FOR EACH ROW EXECUTE FUNCTION validate_config();

-- Trigger 2: Audit trail (log every change)
CREATE OR REPLACE FUNCTION log_audit() RETURNS TRIGGER AS $$
BEGIN
    IF (TG_OP = 'UPDATE') THEN
        INSERT INTO config_audit (key, old_value, new_value, action)
        VALUES (OLD.key, OLD.value, NEW.value, 'UPDATE');
    ELSIF (TG_OP = 'INSERT') THEN
        INSERT INTO config_audit (key, old_value, new_value, action)
        VALUES (NEW.key, NULL, NEW.value, 'INSERT');
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_audit
    AFTER INSERT OR UPDATE ON confd_config
    FOR EACH ROW EXECUTE FUNCTION log_audit();

-- Trigger 3: Real-time NOTIFY (pushes events to confd via LISTEN/NOTIFY)
CREATE OR REPLACE FUNCTION notify_confd() RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('confd_update', NEW.key || '=' || NEW.value);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_notify
    AFTER INSERT OR UPDATE ON confd_config
    FOR EACH ROW EXECUTE FUNCTION notify_confd();

-- Seed initial configuration values
INSERT INTO confd_config (key, value) VALUES
    ('/app/database/host', 'db-master.internal'),
    ('/app/database/port', '5432'),
    ('/app/feature_flags/maintenance', 'false');
