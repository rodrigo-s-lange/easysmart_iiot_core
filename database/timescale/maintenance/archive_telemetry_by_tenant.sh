#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$ROOT_DIR"

if [[ -f "$ROOT_DIR/.env" ]]; then
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
fi

TENANT_ID="${1:-}"
RETENTION_DAYS="${2:-90}"
ARCHIVE_DIR="${3:-$ROOT_DIR/backups/archive}"
TIMESCALE_CONTAINER="${TIMESCALE_CONTAINER:-iiot_timescaledb}"
TIMESCALE_USER="${TIMESCALE_USER:-admin}"
TIMESCALE_DB="${TIMESCALE_DB:-iiot_telemetry}"

if [[ -z "$TENANT_ID" ]]; then
  echo "Usage: $0 <tenant_id> [retention_days] [archive_dir]"
  exit 1
fi

mkdir -p "$ARCHIVE_DIR"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ARCHIVE_FILE="$ARCHIVE_DIR/telemetry_${TENANT_ID}_${STAMP}.csv"

SQL_EXPORT="\\copy (SELECT tenant_id, device_id, slot, value, timestamp FROM telemetry WHERE tenant_id='${TENANT_ID}'::uuid AND timestamp < NOW() - interval '${RETENTION_DAYS} days' ORDER BY timestamp) TO STDOUT WITH CSV HEADER"

docker exec -i "$TIMESCALE_CONTAINER" psql -v ON_ERROR_STOP=1 -U "$TIMESCALE_USER" -d "$TIMESCALE_DB" -c "$SQL_EXPORT" > "$ARCHIVE_FILE"

LINE_COUNT="$(wc -l < "$ARCHIVE_FILE" | tr -d ' ')"
if [[ "$LINE_COUNT" -le 1 ]]; then
  echo "No rows to archive for tenant ${TENANT_ID}."
  rm -f "$ARCHIVE_FILE"
  exit 0
fi

DELETED="$(docker exec -i "$TIMESCALE_CONTAINER" psql -v ON_ERROR_STOP=1 -U "$TIMESCALE_USER" -d "$TIMESCALE_DB" -Atqc "SELECT prune_telemetry_for_tenant('${TENANT_ID}'::uuid, ${RETENTION_DAYS}, 1000000);")"

echo "Archive file: $ARCHIVE_FILE"
echo "Deleted rows: ${DELETED}"
