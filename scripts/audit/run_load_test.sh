#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
API_BASE_URL="${API_BASE_URL:-http://localhost:3001}"
RATE="${RATE:-40}"
DURATION="${DURATION:-2m}"
PRE_ALLOCATED_VUS="${PRE_ALLOCATED_VUS:-80}"
MAX_VUS="${MAX_VUS:-600}"
DEVICE_POOL_SIZE="${DEVICE_POOL_SIZE:-$RATE}"

if [[ -f "$ROOT_DIR/.env" ]]; then
  # shellcheck disable=SC1090
  source "$ROOT_DIR/.env"
fi

if [[ -z "${EMQX_WEBHOOK_API_KEY:-}" ]]; then
  echo "EMQX_WEBHOOK_API_KEY is required in .env"
  exit 1
fi

json_get() {
  local key="$1"
  python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('$key',''))"
}

RAND="$(date +%s)"
EMAIL="load-$RAND@example.com"
PASSWORD="Abcdef1!"

echo "[1/3] Creating tenant user and provisioned device for load test..."
AUTH_RESP="$(curl -sS -X POST "$API_BASE_URL/api/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")"

ACCESS_TOKEN="$(echo "$AUTH_RESP" | json_get access_token)"
TENANT_ID="$(echo "$AUTH_RESP" | python3 -c 'import json,sys;print(json.load(sys.stdin)["user"]["tenant_id"])')"

if [[ -z "$ACCESS_TOKEN" || -z "$TENANT_ID" ]]; then
  echo "Failed to create load-test user: $AUTH_RESP"
  exit 1
fi

DEVICE_IDS=()
for i in $(seq 1 "$DEVICE_POOL_SIZE"); do
  PROV_RESP="$(curl -sS -X POST "$API_BASE_URL/api/devices/provision" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"device_label\":\"load-device-$RAND-$i\"}")"
  DEVICE_ID="$(echo "$PROV_RESP" | json_get device_id)"
  if [[ -z "$DEVICE_ID" ]]; then
    echo "Failed to provision load-test device #$i: $PROV_RESP"
    exit 1
  fi
  DEVICE_IDS+=("$DEVICE_ID")
done
DEVICE_IDS_CSV="$(IFS=,; echo "${DEVICE_IDS[*]}")"

echo "[2/3] Running k6 load test (rate=${RATE}/s, duration=$DURATION, devices=${DEVICE_POOL_SIZE})..."
mkdir -p "$ROOT_DIR/artifacts/load"
chmod 777 "$ROOT_DIR/artifacts/load" || true
OUT_JSON="$ROOT_DIR/artifacts/load/k6-summary-${RAND}.json"

API_BASE_DOCKER="${API_BASE_URL/localhost/host.docker.internal}"


docker run --rm \
  --add-host=host.docker.internal:host-gateway \
  -v "$ROOT_DIR/scripts/audit:/scripts:ro" \
  -v "$ROOT_DIR/artifacts/load:/out" \
  -e API_BASE_URL="$API_BASE_DOCKER" \
  -e API_KEY="$EMQX_WEBHOOK_API_KEY" \
  -e TENANT_ID="$TENANT_ID" \
  -e DEVICE_IDS="$DEVICE_IDS_CSV" \
  -e RATE="$RATE" \
  -e DURATION="$DURATION" \
  -e PRE_ALLOCATED_VUS="$PRE_ALLOCATED_VUS" \
  -e MAX_VUS="$MAX_VUS" \
  grafana/k6:0.53.0 run /scripts/load_telemetry_k6.js --summary-export "/out/$(basename "$OUT_JSON")"

echo "[3/3] Load test completed"
echo "Summary: $OUT_JSON"
