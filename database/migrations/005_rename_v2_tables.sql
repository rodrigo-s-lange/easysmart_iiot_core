DO $$
BEGIN
    -- Rename users_v2 -> users (preserve legacy users table if exists)
    IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = 'users_v2') THEN
        IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = 'users') THEN
            ALTER TABLE users RENAME TO users_legacy;
        END IF;
        ALTER TABLE users_v2 RENAME TO users;
    END IF;

    -- Rename devices_v2 -> devices (preserve legacy devices table if exists)
    IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = 'devices_v2') THEN
        IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = 'devices') THEN
            ALTER TABLE devices RENAME TO devices_legacy;
        END IF;
        ALTER TABLE devices_v2 RENAME TO devices;
    END IF;
END $$;

-- Ensure RLS stays enabled after rename
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE devices ENABLE ROW LEVEL SECURITY;

-- Refresh views to point to canonical tables
CREATE OR REPLACE VIEW devices_compat AS
SELECT
    device_id AS id,
    owner_user_id AS user_id,
    device_label::uuid AS token,
    metadata ->> 'name' AS name,
    metadata ->> 'hw_type' AS hw_type,
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
WHERE tenant_id = '00000000-0000-0000-0000-000000000001'::uuid;

CREATE OR REPLACE VIEW emqx_auth_v2 AS
SELECT
    device_label AS username,
    secret_hash AS password_hash,
    'bcrypt'::text AS password_hash_algorithm
FROM devices
WHERE status IN ('active','claimed') AND secret_hash IS NOT NULL;

CREATE OR REPLACE VIEW emqx_acl_v2 AS
SELECT
    device_label AS username,
    'allow'::text AS permission,
    'publish'::text AS action,
    (('tenants/'::text || tenant_id) || '/devices/'::text || device_id) || '/telemetry/#'::text AS topic
FROM devices
WHERE status IN ('active','claimed') AND secret_hash IS NOT NULL
UNION ALL
SELECT
    device_label AS username,
    'allow'::text AS permission,
    'subscribe'::text AS action,
    (('tenants/'::text || tenant_id) || '/devices/'::text || device_id) || '/telemetry/#'::text AS topic
FROM devices
WHERE status IN ('active','claimed') AND secret_hash IS NOT NULL
UNION ALL
SELECT
    device_label AS username,
    'allow'::text AS permission,
    'publish'::text AS action,
    (('tenants/'::text || tenant_id) || '/devices/'::text || device_id) || '/events/#'::text AS topic
FROM devices
WHERE status IN ('active','claimed') AND secret_hash IS NOT NULL
UNION ALL
SELECT
    device_label AS username,
    'allow'::text AS permission,
    'publish'::text AS action,
    (('tenants/'::text || tenant_id) || '/devices/'::text || device_id) || '/status'::text AS topic
FROM devices
WHERE status IN ('active','claimed') AND secret_hash IS NOT NULL
UNION ALL
SELECT
    device_label AS username,
    'allow'::text AS permission,
    'subscribe'::text AS action,
    (('tenants/'::text || tenant_id) || '/devices/'::text || device_id) || '/commands/#'::text AS topic
FROM devices
WHERE status IN ('active','claimed') AND secret_hash IS NOT NULL;
