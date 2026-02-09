ALTER TABLE telemetry ADD COLUMN IF NOT EXISTS tenant_id UUID;

-- Backfill existing data to default tenant if no mapping is available
UPDATE telemetry
SET tenant_id = '00000000-0000-0000-0000-000000000001'
WHERE tenant_id IS NULL;

ALTER TABLE telemetry ALTER COLUMN tenant_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_telemetry_tenant_device_slot_ts
    ON telemetry (tenant_id, device_id, slot, timestamp DESC);

-- Enable RLS + policy for tenant isolation
ALTER TABLE telemetry ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_telemetry ON telemetry;
CREATE POLICY tenant_isolation_telemetry ON telemetry
USING (
    tenant_id = current_setting('app.current_tenant_id', true)::uuid
    OR current_setting('app.current_user_role', true) = 'super_admin'
)
WITH CHECK (
    tenant_id = current_setting('app.current_tenant_id', true)::uuid
    OR current_setting('app.current_user_role', true) = 'super_admin'
);
