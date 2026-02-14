-- Usage:
-- psql -U admin -d iiot_platform -v user_email='admin@example.com' -f database/maintenance/promote_super_admin.sql

UPDATE users
SET role = 'super_admin',
    tenant_id = NULL,
    updated_at = NOW()
WHERE email = :'user_email';

SELECT user_id, email, role, tenant_id
FROM users
WHERE email = :'user_email';
