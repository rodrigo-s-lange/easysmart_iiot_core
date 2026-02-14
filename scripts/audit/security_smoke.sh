#!/usr/bin/env bash
set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://localhost:3001}"

json_get() {
  local key="$1"
  python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('$key',''))"
}

RAND="$(date +%s)"
EMAIL="security-$RAND@example.com"
PASSWORD="Abcdef1!"

echo "[1/4] Registering user for security smoke tests..."
RESP="$(curl -sS -X POST "$API_BASE_URL/api/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")"
TOKEN="$(echo "$RESP" | json_get access_token)"

if [[ -z "$TOKEN" ]]; then
  echo "FAILED: could not obtain token: $RESP"
  exit 1
fi

echo "[2/4] Validating method hardening (GET on POST endpoint => 405)..."
STATUS_METHOD="$(curl -sS -o /tmp/security_method_body.txt -w "%{http_code}" \
  -X GET "$API_BASE_URL/api/v1/devices/provision" \
  -H "Authorization: Bearer $TOKEN")"
if [[ "$STATUS_METHOD" != "405" ]]; then
  echo "FAILED: expected 405, got $STATUS_METHOD body=$(cat /tmp/security_method_body.txt)"
  exit 1
fi

echo "[3/4] Validating JWT tamper rejection..."
TAMPERED="${TOKEN%?}X"
STATUS_JWT="$(curl -sS -o /tmp/security_jwt_body.txt -w "%{http_code}" \
  -X GET "$API_BASE_URL/api/v1/devices" \
  -H "Authorization: Bearer $TAMPERED")"
if [[ "$STATUS_JWT" != "401" ]]; then
  echo "FAILED: expected 401, got $STATUS_JWT body=$(cat /tmp/security_jwt_body.txt)"
  exit 1
fi

echo "[4/4] Validating SQL-injection style input handling..."
STATUS_SQLI="$(curl -sS -o /tmp/security_sqli_body.txt -w "%{http_code}" \
  -G "$API_BASE_URL/api/v1/telemetry/latest" \
  -H "Authorization: Bearer $TOKEN" \
  --data-urlencode "device_label=' OR '1'='1" \
  --data-urlencode "slot=0")"
if [[ "$STATUS_SQLI" == "500" ]]; then
  echo "FAILED: SQLi-like input caused 500 body=$(cat /tmp/security_sqli_body.txt)"
  exit 1
fi

echo "OK: security smoke checks passed (method/JWT/SQLi-style input)"
