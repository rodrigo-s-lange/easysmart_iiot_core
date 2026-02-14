#!/usr/bin/env bash
set -euo pipefail

POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-iiot_postgres}"
TIMESCALE_CONTAINER="${TIMESCALE_CONTAINER:-iiot_timescaledb}"
POSTGRES_DB="${POSTGRES_DB:-iiot_platform}"
POSTGRES_USER="${POSTGRES_USER:-admin}"
TIMESCALE_DB="${TIMESCALE_DB:-iiot_telemetry}"
TIMESCALE_USER="${TIMESCALE_USER:-admin}"

TS_NOW="$(date +%s)"
TENANT_A="11111111-1111-1111-1111-111111111111"
TENANT_B="22222222-2222-2222-2222-222222222222"
USER_A="aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1"
USER_B="bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb2"
DEVICE_A="aaaaaaaa-1111-4444-8888-aaaaaaaaaaaa"
DEVICE_B="bbbbbbbb-2222-4444-8888-bbbbbbbbbbbb"
RLS_ROLE="rls_auditor"

echo "[1/4] Preparing deterministic multi-tenant fixtures..."
docker exec -i "$POSTGRES_CONTAINER" psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<SQL
INSERT INTO tenants (tenant_id, name, slug, status)
VALUES
  ('$TENANT_A', 'Audit Tenant A', 'audit-tenant-a', 'active'),
  ('$TENANT_B', 'Audit Tenant B', 'audit-tenant-b', 'active')
ON CONFLICT (tenant_id) DO NOTHING;

INSERT INTO users (user_id, tenant_id, email, password_hash, role, status)
VALUES
  ('$USER_A', '$TENANT_A', 'audit-a-$TS_NOW@example.com', crypt('Abcdef1!', gen_salt('bf', 12)), 'tenant_admin', 'active'),
  ('$USER_B', '$TENANT_B', 'audit-b-$TS_NOW@example.com', crypt('Abcdef1!', gen_salt('bf', 12)), 'tenant_admin', 'active')
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO devices (device_id, tenant_id, owner_user_id, device_label, secret_hash, status, claimed_at, created_at, updated_at)
VALUES
  ('$DEVICE_A', '$TENANT_A', '$USER_A', 'audit-device-a-$TS_NOW', crypt('secret-a', gen_salt('bf', 12)), 'active', NOW(), NOW(), NOW()),
  ('$DEVICE_B', '$TENANT_B', '$USER_B', 'audit-device-b-$TS_NOW', crypt('secret-b', gen_salt('bf', 12)), 'active', NOW(), NOW(), NOW())
ON CONFLICT (device_id) DO NOTHING;

DO \$\$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '$RLS_ROLE') THEN
    CREATE ROLE $RLS_ROLE LOGIN PASSWORD 'rls-auditor-pass' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
  END IF;
END\$\$;

GRANT USAGE ON SCHEMA public TO $RLS_ROLE;
GRANT SELECT ON devices TO $RLS_ROLE;
SQL

echo "[2/4] Seeding telemetry fixtures on TimescaleDB..."
docker exec -i "$TIMESCALE_CONTAINER" psql -v ON_ERROR_STOP=1 -U "$TIMESCALE_USER" -d "$TIMESCALE_DB" <<SQL
INSERT INTO telemetry (tenant_id, device_id, slot, value, timestamp)
VALUES
  ('$TENANT_A', '$DEVICE_A', 10, '{"value": 101}'::jsonb, NOW()),
  ('$TENANT_B', '$DEVICE_B', 10, '{"value": 202}'::jsonb, NOW())
ON CONFLICT DO NOTHING;

DO \$\$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '$RLS_ROLE') THEN
    CREATE ROLE $RLS_ROLE LOGIN PASSWORD 'rls-auditor-pass' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
  END IF;
END\$\$;

GRANT USAGE ON SCHEMA public TO $RLS_ROLE;
GRANT SELECT ON telemetry TO $RLS_ROLE;
SQL

echo "[3/4] Validating Postgres RLS isolation..."
PG_COUNTS="$(docker exec -i "$POSTGRES_CONTAINER" psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At <<SQL
SET ROLE $RLS_ROLE;
SET app.current_user_role = 'tenant_admin';
SET app.current_tenant_id = '$TENANT_A';
SELECT count(*) FROM devices WHERE device_id = '$DEVICE_A'::uuid;
SELECT count(*) FROM devices WHERE device_id = '$DEVICE_B'::uuid;
RESET ROLE;
SQL
)"
PG_NUMS="$(echo "$PG_COUNTS" | grep -E '^[0-9]+$' || true)"
PG_A_OWN="$(echo "$PG_NUMS" | sed -n '1p')"
PG_A_OTHER="$(echo "$PG_NUMS" | sed -n '2p')"

if [[ "$PG_A_OWN" != "1" || "$PG_A_OTHER" != "0" ]]; then
  echo "Postgres RLS FAILED: own=$PG_A_OWN other=$PG_A_OTHER"
  exit 1
fi

echo "[4/4] Validating Timescale RLS isolation..."
TS_COUNTS="$(docker exec -i "$TIMESCALE_CONTAINER" psql -v ON_ERROR_STOP=1 -U "$TIMESCALE_USER" -d "$TIMESCALE_DB" -At <<SQL
SET ROLE $RLS_ROLE;
SET app.current_user_role = 'tenant_admin';
SET app.current_tenant_id = '$TENANT_A';
SELECT count(*) FROM telemetry WHERE device_id = '$DEVICE_A'::uuid;
SELECT count(*) FROM telemetry WHERE device_id = '$DEVICE_B'::uuid;
RESET ROLE;
SQL
)"
TS_NUMS="$(echo "$TS_COUNTS" | grep -E '^[0-9]+$' || true)"
TS_A_OWN="$(echo "$TS_NUMS" | sed -n '1p')"
TS_A_OTHER="$(echo "$TS_NUMS" | sed -n '2p')"

if [[ "$TS_A_OWN" -lt "1" || "$TS_A_OTHER" != "0" ]]; then
  echo "Timescale RLS FAILED: own=$TS_A_OWN other=$TS_A_OTHER"
  exit 1
fi

echo "OK: Tenant isolation validated in Postgres and TimescaleDB"
