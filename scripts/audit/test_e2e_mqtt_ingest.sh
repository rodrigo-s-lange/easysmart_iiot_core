#!/usr/bin/env bash
set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://localhost:3001}"
MQTT_HOST="${MQTT_HOST:-127.0.0.1}"
MQTT_PORT="${MQTT_PORT:-1883}"

json_get() {
  local key="$1"
  python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('$key',''))"
}

register_user() {
  local email="$1"
  local password="$2"
  curl -sS -X POST "$API_BASE_URL/api/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$email\",\"password\":\"$password\"}"
}

provision_device() {
  local token="$1"
  local label="$2"
  curl -sS -X POST "$API_BASE_URL/api/devices/provision" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "{\"device_label\":\"$label\"}"
}

assert_http_200() {
  local body="$1"
  local desc="$2"
  if echo "$body" | grep -q '"error"'; then
    echo "FAILED: $desc => $body"
    exit 1
  fi
}

RAND="$(date +%s)"
EMAIL_A="e2e-a-$RAND@example.com"
EMAIL_B="e2e-b-$RAND@example.com"
PASSWORD="Abcdef1!"


echo "[1/6] Registering tenant A and tenant B users..."
RESP_A="$(register_user "$EMAIL_A" "$PASSWORD")"
RESP_B="$(register_user "$EMAIL_B" "$PASSWORD")"

TOKEN_A="$(echo "$RESP_A" | json_get access_token)"
TOKEN_B="$(echo "$RESP_B" | json_get access_token)"
TENANT_A="$(echo "$RESP_A" | python3 -c 'import json,sys;print(json.load(sys.stdin)["user"]["tenant_id"])')"

if [[ -z "$TOKEN_A" || -z "$TOKEN_B" || -z "$TENANT_A" ]]; then
  echo "FAILED: could not obtain tokens/tenant_id"
  echo "RESP_A=$RESP_A"
  echo "RESP_B=$RESP_B"
  exit 1
fi

echo "[2/6] Provisioning device for tenant A..."
DEVICE_LABEL="e2e-device-$RAND"
PROV="$(provision_device "$TOKEN_A" "$DEVICE_LABEL")"
assert_http_200 "$PROV" "provision device"

DEVICE_ID="$(echo "$PROV" | json_get device_id)"
MQTT_USER="$(echo "$PROV" | json_get device_label)"
MQTT_PASS="$(echo "$PROV" | json_get device_secret)"

if [[ -z "$DEVICE_ID" || -z "$MQTT_USER" || -z "$MQTT_PASS" ]]; then
  echo "FAILED: provisioning response incomplete: $PROV"
  exit 1
fi

TOPIC="tenants/$TENANT_A/devices/$DEVICE_ID/telemetry/slot/7"
PAYLOAD='{"value": 4242}'

echo "[3/6] Publishing telemetry over MQTT..."
mosquitto_pub -h "$MQTT_HOST" -p "$MQTT_PORT" -u "$MQTT_USER" -P "$MQTT_PASS" -t "$TOPIC" -m "$PAYLOAD"

echo "[4/6] Waiting ingestion and validating Timescale record..."
FOUND=0
for _ in $(seq 1 20); do
  COUNT="$(docker exec -i iiot_timescaledb psql -U admin -d iiot_telemetry -Atqc "SELECT count(*) FROM telemetry WHERE device_id='$DEVICE_ID'::uuid AND slot=7;")"
  if [[ "$COUNT" =~ ^[0-9]+$ ]] && [[ "$COUNT" -ge 1 ]]; then
    FOUND=1
    break
  fi
  sleep 1
done

if [[ "$FOUND" != "1" ]]; then
  echo "FAILED: telemetry not found in Timescale for device=$DEVICE_ID"
  exit 1
fi

echo "[5/6] Validating API read for owner tenant..."
LATEST_A="$(curl -sS -G "$API_BASE_URL/api/telemetry/latest" \
  -H "Authorization: Bearer $TOKEN_A" \
  --data-urlencode "device_id=$DEVICE_ID" \
  --data-urlencode "slot=7")"
assert_http_200 "$LATEST_A" "tenant A read latest"

echo "[6/6] Validating isolation at API layer (tenant B must not read tenant A device)..."
LATEST_B="$(curl -sS -G "$API_BASE_URL/api/telemetry/latest" \
  -H "Authorization: Bearer $TOKEN_B" \
  --data-urlencode "device_id=$DEVICE_ID" \
  --data-urlencode "slot=7")"

if ! echo "$LATEST_B" | grep -q 'Device not found or inactive'; then
  echo "FAILED: tenant B unexpectedly accessed tenant A data => $LATEST_B"
  exit 1
fi

echo "OK: E2E MQTT ingestion and tenant isolation at API layer validated"
