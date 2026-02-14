CREATE TABLE IF NOT EXISTS tenant_telemetry_retention_policy (
    tenant_id UUID PRIMARY KEY,
    retention_days INT NOT NULL CHECK (retention_days > 0),
    archive_before_delete BOOLEAN NOT NULL DEFAULT true,
    archive_bucket TEXT,
    enabled BOOLEAN NOT NULL DEFAULT true,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tenant_retention_enabled
    ON tenant_telemetry_retention_policy (enabled)
    WHERE enabled = true;

-- Prunes old telemetry for one tenant in bounded batches.
CREATE OR REPLACE FUNCTION prune_telemetry_for_tenant(
    p_tenant_id UUID,
    p_retention_days INT,
    p_batch_size INT DEFAULT 50000
)
RETURNS INT AS $$
DECLARE
    v_deleted INT;
BEGIN
    IF p_retention_days <= 0 THEN
        RAISE EXCEPTION 'retention_days must be > 0';
    END IF;

    WITH doomed AS (
        SELECT ctid
        FROM telemetry
        WHERE tenant_id = p_tenant_id
          AND timestamp < NOW() - make_interval(days => p_retention_days)
        LIMIT p_batch_size
    )
    DELETE FROM telemetry t
    USING doomed d
    WHERE t.ctid = d.ctid;

    GET DIAGNOSTICS v_deleted = ROW_COUNT;
    RETURN v_deleted;
END;
$$ LANGUAGE plpgsql;

-- Applies retention across tenants (policy-driven, with default fallback).
CREATE OR REPLACE FUNCTION prune_telemetry_all_tenants(
    p_default_retention_days INT DEFAULT 365,
    p_batch_size INT DEFAULT 50000
)
RETURNS TABLE (tenant_id UUID, deleted_rows INT) AS $$
DECLARE
    v_tenant UUID;
    v_retention INT;
    v_deleted INT;
BEGIN
    FOR v_tenant IN
        SELECT DISTINCT t.tenant_id
        FROM telemetry t
    LOOP
        SELECT rp.retention_days
          INTO v_retention
        FROM tenant_telemetry_retention_policy rp
        WHERE rp.tenant_id = v_tenant
          AND rp.enabled = true;

        IF v_retention IS NULL THEN
            v_retention := p_default_retention_days;
        END IF;

        v_deleted := prune_telemetry_for_tenant(v_tenant, v_retention, p_batch_size);
        tenant_id := v_tenant;
        deleted_rows := v_deleted;
        RETURN NEXT;
    END LOOP;
END;
$$ LANGUAGE plpgsql;
