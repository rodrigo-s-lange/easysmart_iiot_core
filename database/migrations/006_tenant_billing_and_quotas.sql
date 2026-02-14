ALTER TABLE tenants
  ADD COLUMN IF NOT EXISTS plan_type VARCHAR(20) NOT NULL DEFAULT 'starter'
    CHECK (plan_type IN ('starter', 'pro', 'enterprise')),
  ADD COLUMN IF NOT EXISTS billing_cycle VARCHAR(20) NOT NULL DEFAULT 'monthly'
    CHECK (billing_cycle IN ('monthly', 'annual')),
  ADD COLUMN IF NOT EXISTS quota_msgs_per_min INT NOT NULL DEFAULT 360
    CHECK (quota_msgs_per_min >= 0),
  ADD COLUMN IF NOT EXISTS quota_storage_mb INT NOT NULL DEFAULT 1000
    CHECK (quota_storage_mb >= 0),
  ADD COLUMN IF NOT EXISTS allow_overage BOOLEAN NOT NULL DEFAULT false;

-- 0 means unlimited devices.
ALTER TABLE tenants
  ALTER COLUMN quota_devices SET DEFAULT 0;

UPDATE tenants
SET quota_devices = 0
WHERE quota_devices = 1000;

-- Legacy field maintained for compatibility (not used for enforcement).
COMMENT ON COLUMN tenants.quota_messages_per_hour IS 'Legacy quota field, replaced by quota_msgs_per_min.';

CREATE TABLE IF NOT EXISTS tenant_usage_snapshots (
  snapshot_id BIGSERIAL PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
  period_start TIMESTAMPTZ NOT NULL,
  period_end TIMESTAMPTZ NOT NULL,
  messages_ingested BIGINT NOT NULL DEFAULT 0,
  storage_mb NUMERIC(12,2) NOT NULL DEFAULT 0,
  devices_total INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, period_start, period_end)
);

CREATE INDEX IF NOT EXISTS idx_tenant_usage_snapshots_tenant_period
  ON tenant_usage_snapshots (tenant_id, period_start DESC);
