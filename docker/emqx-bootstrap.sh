#!/bin/sh
set -eu

log() {
  printf '[emqx-bootstrap] %s\n' "$*"
}

require_env() {
  var_name="$1"
  eval "val=\${$var_name:-}"
  if [ -z "$val" ]; then
    log "missing required env: $var_name"
    exit 1
  fi
}

require_env POSTGRES_USER
require_env POSTGRES_PASSWORD
require_env POSTGRES_DB
require_env EMQX_DASHBOARD_USER
require_env EMQX_DASHBOARD_PASSWORD
require_env EMQX_WEBHOOK_API_KEY

EMQX_API_URL="${EMQX_API_URL:-http://emqx:18083/api/v5}"
EMQX_BOOTSTRAP_INTERVAL="${EMQX_BOOTSTRAP_INTERVAL:-60}"
PG_CONN="postgresql://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}?sslmode=disable"

wait_for_postgres() {
  until psql "$PG_CONN" -Atqc "SELECT 1" >/dev/null 2>&1; do
    log "waiting postgres..."
    sleep 2
  done
}

wait_for_emqx() {
  until curl -fsS "${EMQX_API_URL}/status" >/dev/null 2>&1; do
    log "waiting emqx..."
    sleep 2
  done
}

wait_for_go_api() {
  until curl -fsS "http://go_api:3001/health" >/dev/null 2>&1; do
    log "waiting go_api..."
    sleep 2
  done
}

upsert_api_key() {
  first_tenant="$(psql "$PG_CONN" -Atqc "SELECT tenant_id FROM tenants ORDER BY created_at ASC LIMIT 1;")"
  if [ -z "$first_tenant" ]; then
    log "no tenant found to scope webhook API key"
    return 1
  fi

  key_prefix="$(printf '%s' "$EMQX_WEBHOOK_API_KEY" | cut -c1-8)"
  escaped_key="$(printf "%s" "$EMQX_WEBHOOK_API_KEY" | sed "s/'/''/g")"

  psql "$PG_CONN" -v ON_ERROR_STOP=1 -Atqc "
WITH up AS (
  UPDATE api_keys
  SET key_hash = crypt('${escaped_key}', gen_salt('bf', 12)),
      key_prefix = '${key_prefix}',
      scopes = ARRAY['telemetry:write']::text[],
      status = 'active',
      tenant_id = '${first_tenant}'::uuid,
      revoked_at = NULL,
      expires_at = NULL
  WHERE name = 'emqx_webhook'
  RETURNING 1
)
INSERT INTO api_keys (
  key_id, tenant_id, user_id, name, key_hash, key_prefix, scopes, status, created_at
)
SELECT
  gen_random_uuid(),
  '${first_tenant}'::uuid,
  NULL,
  'emqx_webhook',
  crypt('${escaped_key}', gen_salt('bf', 12)),
  '${key_prefix}',
  ARRAY['telemetry:write']::text[],
  'active',
  NOW()
WHERE NOT EXISTS (SELECT 1 FROM up);
"
}

get_token() {
  token="$(
    curl -fsS -X POST "${EMQX_API_URL}/login" \
      -H "Content-Type: application/json" \
      -d "{\"username\":\"${EMQX_DASHBOARD_USER}\",\"password\":\"${EMQX_DASHBOARD_PASSWORD}\"}" \
    | sed -n 's/.*"token":"\([^"]*\)".*/\1/p'
  )"

  if [ -z "$token" ]; then
    log "failed to get EMQX token. Ensure dashboard user exists."
    return 1
  fi

  printf '%s' "$token"
}

upsert_connector() {
  token="$1"
  cat >/tmp/connector_post.json <<EOF
{
  "type": "http",
  "name": "api_webhook",
  "enable": true,
  "url": "http://go_api:3001",
  "headers": {
    "content-type": "application/json",
    "authorization": "Bearer ${EMQX_WEBHOOK_API_KEY}"
  },
  "connect_timeout": "15s",
  "pool_type": "random",
  "pool_size": 8,
  "enable_pipelining": 100,
  "ssl": {"enable": false}
}
EOF

  code="$(curl -s -o /tmp/connector_post.out -w "%{http_code}" \
    -X POST "${EMQX_API_URL}/connectors" \
    -H "Authorization: Bearer ${token}" \
    -H "Content-Type: application/json" \
    --data @/tmp/connector_post.json)"

  if [ "$code" = "201" ]; then
    return 0
  fi

  if [ "$code" = "400" ] && grep -q "ALREADY_EXISTS" /tmp/connector_post.out; then
    cat >/tmp/connector_put.json <<EOF
{
  "enable": true,
  "url": "http://go_api:3001",
  "headers": {
    "content-type": "application/json",
    "authorization": "Bearer ${EMQX_WEBHOOK_API_KEY}"
  },
  "connect_timeout": "15s",
  "pool_type": "random",
  "pool_size": 8,
  "enable_pipelining": 100,
  "ssl": {"enable": false}
}
EOF
    code_put="$(curl -s -o /tmp/connector_put.out -w "%{http_code}" \
      -X PUT "${EMQX_API_URL}/connectors/http:api_webhook" \
      -H "Authorization: Bearer ${token}" \
      -H "Content-Type: application/json" \
      --data @/tmp/connector_put.json)"
    [ "$code_put" = "200" ] || {
      log "connector update failed (code=${code_put}): $(cat /tmp/connector_put.out)"
      return 1
    }
    return 0
  fi

  log "connector create failed (code=${code}): $(cat /tmp/connector_post.out)"
  return 1
}

upsert_action() {
  token="$1"
  cat >/tmp/action_post.json <<'EOF'
{
  "type": "http",
  "name": "send_to_api",
  "enable": true,
  "connector": "api_webhook",
  "parameters": {
    "method": "post",
    "path": "/api/telemetry",
    "body": "{\\"clientid\\":\\"${clientid}\\",\\"topic\\":\\"${topic}\\",\\"payload\\":${payload},\\"timestamp\\":\\"${timestamp}\\"}",
    "headers": {
      "content-type": "application/json",
      "authorization": "Bearer __WEBHOOK_KEY__"
    }
  }
}
EOF
  sed -i "s|__WEBHOOK_KEY__|${EMQX_WEBHOOK_API_KEY}|g" /tmp/action_post.json

  code="$(curl -s -o /tmp/action_post.out -w "%{http_code}" \
    -X POST "${EMQX_API_URL}/actions" \
    -H "Authorization: Bearer ${token}" \
    -H "Content-Type: application/json" \
    --data @/tmp/action_post.json)"

  if [ "$code" = "201" ]; then
    return 0
  fi

  if [ "$code" = "400" ] && grep -q "ALREADY_EXISTS" /tmp/action_post.out; then
    cat >/tmp/action_put.json <<'EOF'
{
  "enable": true,
  "connector": "api_webhook",
  "parameters": {
    "method": "post",
    "path": "/api/telemetry",
    "body": "{\\"clientid\\":\\"${clientid}\\",\\"topic\\":\\"${topic}\\",\\"payload\\":${payload},\\"timestamp\\":\\"${timestamp}\\"}",
    "headers": {
      "content-type": "application/json",
      "authorization": "Bearer __WEBHOOK_KEY__"
    }
  }
}
EOF
    sed -i "s|__WEBHOOK_KEY__|${EMQX_WEBHOOK_API_KEY}|g" /tmp/action_put.json
    code_put="$(curl -s -o /tmp/action_put.out -w "%{http_code}" \
      -X PUT "${EMQX_API_URL}/actions/http:send_to_api" \
      -H "Authorization: Bearer ${token}" \
      -H "Content-Type: application/json" \
      --data @/tmp/action_put.json)"
    [ "$code_put" = "200" ] || {
      log "action update failed (code=${code_put}): $(cat /tmp/action_put.out)"
      return 1
    }
    return 0
  fi

  log "action create failed (code=${code}): $(cat /tmp/action_post.out)"
  return 1
}

upsert_rule() {
  token="$1"
  cat >/tmp/rule_put.json <<'EOF'
{
  "name": "telemetry_ingest",
  "enable": true,
  "description": "Multi-tenant telemetry to Go API",
  "sql": "SELECT payload, clientid, topic, timestamp FROM \"tenants/+/devices/+/telemetry/slot/+\"",
  "actions": ["http:send_to_api"]
}
EOF

  code="$(curl -s -o /tmp/rule_put.out -w "%{http_code}" \
    -X PUT "${EMQX_API_URL}/rules/telemetry_ingest" \
    -H "Authorization: Bearer ${token}" \
    -H "Content-Type: application/json" \
    --data @/tmp/rule_put.json)"
  [ "$code" = "200" ] || {
    log "rule upsert failed (code=${code}): $(cat /tmp/rule_put.out)"
    return 1
  }
}

reconcile() {
  wait_for_postgres
  wait_for_emqx
  wait_for_go_api

  upsert_api_key
  token="$(get_token)"
  upsert_connector "$token"
  upsert_action "$token"
  upsert_rule "$token"
  log "reconcile ok"
}

log "starting loop (interval=${EMQX_BOOTSTRAP_INTERVAL}s)"
while true; do
  if ! reconcile; then
    log "reconcile failed, retrying in 5s"
    sleep 5
    continue
  fi
  sleep "$EMQX_BOOTSTRAP_INTERVAL"
done
