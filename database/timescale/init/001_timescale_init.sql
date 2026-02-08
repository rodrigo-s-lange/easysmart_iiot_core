CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS telemetry (
    id BIGSERIAL,
    device_id UUID NOT NULL,
    slot SMALLINT NOT NULL,
    value JSONB NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, timestamp)
);

SELECT create_hypertable('telemetry', 'timestamp', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_telemetry_device_slot_ts
    ON telemetry (device_id, slot, timestamp DESC);

-- Retention policy: keep last 365 days
SELECT add_retention_policy('telemetry', INTERVAL '365 days', if_not_exists => TRUE);
