CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS telemetry (
    id BIGSERIAL,
    tenant_id UUID NOT NULL,
    device_id UUID NOT NULL,
    slot SMALLINT NOT NULL,
    value JSONB NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, timestamp)
);

SELECT create_hypertable('telemetry', 'timestamp', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_telemetry_device_slot_ts
    ON telemetry (device_id, slot, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_telemetry_tenant_device_slot_ts
    ON telemetry (tenant_id, device_id, slot, timestamp DESC);

-- Retention policy: keep last 365 days
SELECT add_retention_policy('telemetry', INTERVAL '365 days', if_not_exists => TRUE);

-- Row Level Security (tenant isolation)
ALTER TABLE telemetry ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_telemetry ON telemetry
USING (
    tenant_id = current_setting('app.current_tenant_id', true)::uuid
    OR current_setting('app.current_user_role', true) = 'super_admin'
)
WITH CHECK (
    tenant_id = current_setting('app.current_tenant_id', true)::uuid
    OR current_setting('app.current_user_role', true) = 'super_admin'
);
