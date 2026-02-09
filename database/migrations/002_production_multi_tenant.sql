-- ============================================================================
-- IIoT Platform - Production Multi-Tenant Schema
-- Version: 2.0.0
-- Date: 2026-02-09
-- 
-- This migration adds multi-tenant architecture while maintaining
-- compatibility with existing single-tenant schema.
-- ============================================================================

BEGIN;

-- ============================================================================
-- EXTENSIONS
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================================
-- NEW TABLES: TENANTS
-- ============================================================================

CREATE TABLE tenants (
    tenant_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'deleted')),
    quota_devices INT NOT NULL DEFAULT 1000,
    quota_messages_per_hour INT NOT NULL DEFAULT 100000,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_tenants_slug ON tenants(slug);
CREATE INDEX idx_tenants_status ON tenants(status) WHERE status = 'active';

COMMENT ON TABLE tenants IS 'Multi-tenant organizations (Phase 2)';

-- ============================================================================
-- NEW TABLES: USERS (multi-tenant version)
-- ============================================================================

CREATE TYPE user_role AS ENUM ('super_admin', 'tenant_admin', 'tenant_user');

CREATE TABLE users (
    user_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role user_role NOT NULL DEFAULT 'tenant_user',
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'deleted')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}'::jsonb,
    
    CONSTRAINT chk_super_admin_no_tenant CHECK (
        (role = 'super_admin' AND tenant_id IS NULL) OR 
        (role != 'super_admin' AND tenant_id IS NOT NULL)
    )
);

CREATE INDEX idx_users_v2_email ON users(email);
CREATE INDEX idx_users_v2_tenant ON users(tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX idx_users_v2_role ON users(role);

COMMENT ON TABLE users IS 'Multi-tenant users with RBAC (Phase 2)';

-- ============================================================================
-- NEW TABLES: DEVICES (multi-tenant version)
-- ============================================================================

CREATE TYPE device_status AS ENUM ('unclaimed', 'claimed', 'active', 'suspended', 'revoked');

CREATE TABLE devices (
    device_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    owner_user_id UUID REFERENCES users(user_id) ON DELETE SET NULL,
    
    -- Public identifier (printed on device label)
    device_label VARCHAR(50) UNIQUE NOT NULL,
    
    -- Authentication secret (hashed with bcrypt)
    secret_hash VARCHAR(255),
    
    -- Lifecycle
    status device_status NOT NULL DEFAULT 'unclaimed',
    claimed_at TIMESTAMPTZ,
    activated_at TIMESTAMPTZ,
    
    -- Metadata
    firmware_version VARCHAR(50),
    hardware_revision VARCHAR(50),
    last_seen_at TIMESTAMPTZ,
    last_ip INET,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb,
    
    -- Constraints
    CONSTRAINT chk_claimed_has_tenant CHECK (
        (status = 'unclaimed' AND tenant_id IS NULL) OR
        (status != 'unclaimed' AND tenant_id IS NOT NULL)
    ),
    CONSTRAINT chk_claimed_has_secret CHECK (
        (status = 'unclaimed' AND secret_hash IS NULL) OR
        (status != 'unclaimed' AND secret_hash IS NOT NULL)
    )
);

CREATE INDEX idx_devices_v2_tenant ON devices(tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX idx_devices_v2_owner ON devices(owner_user_id) WHERE owner_user_id IS NOT NULL;
CREATE INDEX idx_devices_v2_status ON devices(status);
CREATE INDEX idx_devices_v2_label ON devices(device_label);

COMMENT ON TABLE devices IS 'Multi-tenant devices with claim/unclaim lifecycle (Phase 2)';

-- ============================================================================
-- PERMISSIONS & RBAC
-- ============================================================================

CREATE TABLE permissions (
    permission_id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE role_permissions (
    role user_role NOT NULL,
    permission_id INT REFERENCES permissions(permission_id) ON DELETE CASCADE,
    PRIMARY KEY (role, permission_id)
);

-- Seed permissions
INSERT INTO permissions (name, description) VALUES
    ('devices:read', 'View devices'),
    ('devices:write', 'Create/update devices'),
    ('devices:provision', 'Claim/unclaim devices'),
    ('devices:delete', 'Delete devices'),
    ('telemetry:read', 'Read telemetry data'),
    ('telemetry:write', 'Write telemetry data'),
    ('users:read', 'View users'),
    ('users:write', 'Create/update users'),
    ('users:delete', 'Delete users'),
    ('tenants:read', 'View tenant info'),
    ('tenants:write', 'Update tenant settings'),
    ('audit:read', 'Read audit logs'),
    ('system:admin', 'Full system access');

-- Seed role permissions
INSERT INTO role_permissions (role, permission_id) 
SELECT 'super_admin', permission_id FROM permissions;

INSERT INTO role_permissions (role, permission_id)
SELECT 'tenant_admin', permission_id FROM permissions 
WHERE name NOT IN ('system:admin', 'tenants:write');

INSERT INTO role_permissions (role, permission_id)
SELECT 'tenant_user', permission_id FROM permissions
WHERE name IN ('devices:read', 'devices:write', 'devices:provision', 'telemetry:read', 'telemetry:write');

-- ============================================================================
-- AUDIT LOG
-- ============================================================================

CREATE TABLE audit_log (
    audit_id BIGSERIAL PRIMARY KEY,
    tenant_id UUID REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(user_id) ON DELETE SET NULL,
    device_id UUID REFERENCES devices(device_id) ON DELETE SET NULL,
    
    event_type VARCHAR(100) NOT NULL,
    event_category VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'info',
    
    actor_type VARCHAR(20) NOT NULL,
    actor_id UUID,
    
    resource_type VARCHAR(50),
    resource_id UUID,
    
    action VARCHAR(100) NOT NULL,
    result VARCHAR(20) NOT NULL,
    
    ip_address INET,
    user_agent TEXT,
    
    -- Structured columns for common queries
    error_code VARCHAR(50),
    error_message TEXT,
    request_path VARCHAR(255),
    request_method VARCHAR(10),
    response_status INT,
    duration_ms INT,
    
    -- JSONB only for rare metadata
    metadata JSONB DEFAULT '{}'::jsonb,
    
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_tenant ON audit_log(tenant_id, timestamp DESC);
CREATE INDEX idx_audit_user ON audit_log(user_id, timestamp DESC);
CREATE INDEX idx_audit_device ON audit_log(device_id, timestamp DESC);
CREATE INDEX idx_audit_event_type ON audit_log(event_type, timestamp DESC);
CREATE INDEX idx_audit_timestamp ON audit_log(timestamp DESC);
CREATE INDEX idx_audit_errors ON audit_log(error_code, timestamp DESC) WHERE error_code IS NOT NULL;
CREATE INDEX idx_audit_slow_requests ON audit_log(duration_ms DESC) WHERE duration_ms > 1000;

COMMENT ON TABLE audit_log IS 'Audit trail for compliance (LGPD/GDPR)';

-- ============================================================================
-- API KEYS
-- ============================================================================

CREATE TABLE api_keys (
    key_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(user_id) ON DELETE CASCADE,
    
    name VARCHAR(100) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(20) NOT NULL,
    
    scopes TEXT[] NOT NULL DEFAULT '{}',
    
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_prefix ON api_keys(key_prefix) WHERE status = 'active';

COMMENT ON TABLE api_keys IS 'API keys for service-to-service authentication';

-- ============================================================================
-- EMQX VIEWS (CORRECTED)
-- ============================================================================

-- Authentication view (replaces old emqx_auth logic)
CREATE OR REPLACE VIEW emqx_auth_v2 AS
SELECT 
    device_label AS username,
    secret_hash AS password_hash,
    'bcrypt' AS password_hash_algorithm
FROM devices
WHERE status IN ('active', 'claimed')
AND secret_hash IS NOT NULL;

COMMENT ON VIEW emqx_auth_v2 IS 'EMQX authentication query (multi-tenant)';

-- ACL view (multi-tenant topics)
CREATE OR REPLACE VIEW emqx_acl_v2 AS
SELECT
    device_label AS username,
    'allow' AS permission,
    'publish' AS action,
    'tenants/' || tenant_id || '/devices/' || device_id || '/telemetry/#' AS topic
FROM devices
WHERE status IN ('active', 'claimed')
AND tenant_id IS NOT NULL

UNION ALL

SELECT
    device_label AS username,
    'allow' AS permission,
    'publish' AS action,
    'tenants/' || tenant_id || '/devices/' || device_id || '/events/#' AS topic
FROM devices
WHERE status IN ('active', 'claimed')

UNION ALL

SELECT
    device_label AS username,
    'allow' AS permission,
    'subscribe' AS action,
    'tenants/' || tenant_id || '/devices/' || device_id || '/commands/#' AS topic
FROM devices
WHERE status IN ('active', 'claimed')

UNION ALL

SELECT
    device_label AS username,
    'allow' AS permission,
    'publish' AS action,
    'tenants/' || tenant_id || '/devices/' || device_id || '/status' AS topic
FROM devices
WHERE status IN ('active', 'claimed');

COMMENT ON VIEW emqx_acl_v2 IS 'EMQX ACL query (tenant-scoped topics)';

-- ============================================================================
-- ROW-LEVEL SECURITY (Phase 2)
-- ============================================================================

ALTER TABLE devices ENABLE ROW LEVEL SECURITY;
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_devices ON devices
    FOR ALL
    USING (
        tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_setting('app.current_user_role', true) = 'super_admin'
    );

CREATE POLICY tenant_isolation_users ON users
    FOR ALL
    USING (
        tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_setting('app.current_user_role', true) = 'super_admin'
    );

CREATE POLICY tenant_isolation_audit ON audit_log
    FOR SELECT
    USING (
        tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_setting('app.current_user_role', true) = 'super_admin'
    );

-- ============================================================================
-- TRIGGERS
-- ============================================================================

CREATE OR REPLACE FUNCTION update_updated_at_v2()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_tenants_updated_at BEFORE UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_v2();

CREATE TRIGGER trg_users_v2_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_v2();

CREATE TRIGGER trg_devices_v2_updated_at BEFORE UPDATE ON devices
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_v2();

-- Audit trigger
CREATE OR REPLACE FUNCTION audit_device_lifecycle()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO audit_log (tenant_id, device_id, event_type, event_category, actor_type, action, result, metadata)
        VALUES (NEW.tenant_id, NEW.device_id, 'device.created', 'device', 'system', 'create', 'success', 
                jsonb_build_object('device_label', NEW.device_label, 'status', NEW.status));
        RETURN NEW;
    ELSIF TG_OP = 'UPDATE' THEN
        IF OLD.status != NEW.status THEN
            INSERT INTO audit_log (tenant_id, device_id, event_type, event_category, actor_type, action, result, metadata)
            VALUES (NEW.tenant_id, NEW.device_id, 'device.status_changed', 'device', 'system', 'update', 'success',
                    jsonb_build_object('old_status', OLD.status, 'new_status', NEW.status));
        END IF;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        INSERT INTO audit_log (tenant_id, device_id, event_type, event_category, actor_type, action, result, metadata)
        VALUES (OLD.tenant_id, OLD.device_id, 'device.deleted', 'device', 'system', 'delete', 'success',
                jsonb_build_object('device_label', OLD.device_label));
        RETURN OLD;
    END IF;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_audit_device_lifecycle
    AFTER INSERT OR UPDATE OR DELETE ON devices
    FOR EACH ROW EXECUTE FUNCTION audit_device_lifecycle();

-- ============================================================================
-- HELPER FUNCTIONS
-- ============================================================================

-- Claim device (atomic operation)
CREATE OR REPLACE FUNCTION claim_device(
    p_device_label VARCHAR,
    p_tenant_id UUID,
    p_user_id UUID
)
RETURNS TABLE (
    device_id UUID,
    device_secret TEXT,
    success BOOLEAN,
    error_message TEXT
) AS $$
DECLARE
    v_device_id UUID;
    v_device_secret TEXT;
    v_current_status device_status;
BEGIN
    -- Lock device row
    SELECT d.device_id, d.status INTO v_device_id, v_current_status
    FROM devices d
    WHERE d.device_label = p_device_label
    FOR UPDATE;

    IF v_device_id IS NULL THEN
        RETURN QUERY SELECT NULL::UUID, NULL::TEXT, FALSE, 'Device not found';
        RETURN;
    END IF;

    IF v_current_status != 'unclaimed' THEN
        RETURN QUERY SELECT NULL::UUID, NULL::TEXT, FALSE, 'Device already claimed';
        RETURN;
    END IF;

    -- Generate secret (32 bytes = 64 hex chars)
    v_device_secret := encode(gen_random_bytes(32), 'hex');

    -- Update device
    UPDATE devices SET
        tenant_id = p_tenant_id,
        owner_user_id = p_user_id,
        secret_hash = crypt(v_device_secret, gen_salt('bf', 12)),
        status = 'claimed',
        claimed_at = NOW(),
        updated_at = NOW()
    WHERE devices.device_id = v_device_id;

    -- Return plaintext secret (only time it exists)
    RETURN QUERY SELECT v_device_id, v_device_secret, TRUE, NULL::TEXT;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION claim_device IS 'Claim device and generate secret (Phase 1)';

-- ============================================================================
-- DATA MIGRATION: Create default tenant and migrate existing data
-- ============================================================================

-- Create default tenant
INSERT INTO tenants (tenant_id, name, slug, status)
VALUES ('00000000-0000-0000-0000-000000000001', 'Default Tenant', 'default', 'active')
ON CONFLICT DO NOTHING;

-- Optional migration from legacy tables (if they exist)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = 'users_legacy') THEN
        INSERT INTO users (user_id, tenant_id, email, password_hash, role, status, created_at, metadata)
        SELECT
            id AS user_id,
            '00000000-0000-0000-0000-000000000001' AS tenant_id,
            email,
            password_hash,
            'tenant_admin'::user_role AS role,
            CASE status
                WHEN 'active' THEN 'active'
                WHEN 'suspended' THEN 'suspended'
                WHEN 'deleted' THEN 'deleted'
            END AS status,
            created_at,
            jsonb_build_object('plan', plan, 'max_devices', max_devices, 'retention_days', retention_days)
        FROM users_legacy
        ON CONFLICT (email) DO NOTHING;
    END IF;

    IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = 'devices_legacy') THEN
        INSERT INTO devices (device_id, tenant_id, owner_user_id, device_label, secret_hash, status, firmware_version, last_seen_at, created_at, activated_at, metadata)
        SELECT
            id AS device_id,
            '00000000-0000-0000-0000-000000000001' AS tenant_id,
            user_id AS owner_user_id,
            token::text AS device_label,
            crypt(token::text, gen_salt('bf', 12)) AS secret_hash,
            'active'::device_status AS status,
            firmware_version,
            last_seen AS last_seen_at,
            created_at,
            created_at AS activated_at,
            jsonb_build_object('name', name, 'hw_type', hw_type) || COALESCE(metadata, '{}'::jsonb)
        FROM devices_legacy
        ON CONFLICT (device_label) DO NOTHING;
    END IF;
END $$;

-- ============================================================================
-- COMPATIBILITY VIEWS (temporary)
-- ============================================================================

-- Allow old queries to still work during transition
CREATE OR REPLACE VIEW devices_compat AS
SELECT 
    device_id AS id,
    owner_user_id AS user_id,
    device_label::uuid AS token,
    metadata->>'name' AS name,
    metadata->>'hw_type' AS hw_type,
    firmware_version,
    CASE status
        WHEN 'active' THEN 'active'
        WHEN 'claimed' THEN 'active'
        ELSE 'offline'
    END AS status,
    last_seen_at AS last_seen,
    metadata,
    created_at,
    updated_at
FROM devices
WHERE tenant_id = '00000000-0000-0000-0000-000000000001';

COMMENT ON VIEW devices_compat IS 'Compatibility view for old queries (remove in Phase 3)';

COMMIT;

-- ============================================================================
-- POST-MIGRATION NOTES
-- ============================================================================

-- Next steps:
-- 1. Update EMQX config to use emqx_auth_v2 and emqx_acl_v2
-- 2. Restart EMQX: docker-compose restart iiot_emqx
-- 3. Update Go API to use devices, users, tenants
-- 4. Test device claim flow
-- 5. After validation, drop old tables (migration 003)
