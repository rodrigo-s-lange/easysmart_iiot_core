#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ -f "$ROOT_DIR/.env" ]]; then
  # shellcheck disable=SC1090
  source "$ROOT_DIR/.env"
fi

POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-iiot_postgres}"
TIMESCALE_CONTAINER="${TIMESCALE_CONTAINER:-iiot_timescaledb}"
POSTGRES_USER="${POSTGRES_USER:-admin}"
POSTGRES_DB="${POSTGRES_DB:-iiot_platform}"
TIMESCALE_USER="${TIMESCALE_USER:-admin}"
TIMESCALE_DB="${TIMESCALE_DB:-iiot_telemetry}"
BACKUP_ROOT="${BACKUP_ROOT:-$ROOT_DIR/backups/db}"

usage() {
  cat <<USAGE
Usage:
  ./database/backup_restore.sh backup
  ./database/backup_restore.sh restore <backup_dir>
  ./database/backup_restore.sh verify <backup_dir>

Notes:
- backup_dir must contain postgres.sql and timescale.sql.
- restore/verify use temporary databases: iiot_platform_restore / iiot_telemetry_restore.
USAGE
}

require_file() {
  local file="$1"
  if [[ ! -f "$file" ]]; then
    echo "Missing required file: $file"
    exit 1
  fi
}

backup() {
  mkdir -p "$BACKUP_ROOT"
  local ts out
  ts="$(date -u +%Y%m%dT%H%M%SZ)"
  out="$BACKUP_ROOT/$ts"
  mkdir -p "$out"

  echo "Backing up Postgres database: $POSTGRES_DB"
  docker exec -i "$POSTGRES_CONTAINER" pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" > "$out/postgres.sql"

  echo "Backing up Timescale database: $TIMESCALE_DB"
  docker exec -i "$TIMESCALE_CONTAINER" pg_dump -U "$TIMESCALE_USER" -d "$TIMESCALE_DB" > "$out/timescale.sql"

  local pg_rows ts_rows
  pg_rows="$(docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT (SELECT count(*) FROM users)::text || ',' || (SELECT count(*) FROM devices)::text;")"
  ts_rows="$(docker exec -i "$TIMESCALE_CONTAINER" psql -U "$TIMESCALE_USER" -d "$TIMESCALE_DB" -Atqc "SELECT count(*) FROM telemetry;")"

  sha256sum "$out/postgres.sql" "$out/timescale.sql" > "$out/SHA256SUMS"

  cat > "$out/MANIFEST.txt" <<MANIFEST
created_at_utc=$ts
postgres_db=$POSTGRES_DB
timescale_db=$TIMESCALE_DB
postgres_rows_users_devices=$pg_rows
timescale_rows_telemetry=$ts_rows
MANIFEST

  echo "Backup completed: $out"
}

restore_to_temp() {
  local backup_dir="$1"
  local pg_sql="$backup_dir/postgres.sql"
  local ts_sql="$backup_dir/timescale.sql"

  require_file "$pg_sql"
  require_file "$ts_sql"

  echo "Restoring Postgres backup into iiot_platform_restore"
  docker exec -i "$POSTGRES_CONTAINER" psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres <<SQL
DROP DATABASE IF EXISTS iiot_platform_restore;
CREATE DATABASE iiot_platform_restore;
SQL
  docker exec -i "$POSTGRES_CONTAINER" psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d iiot_platform_restore < "$pg_sql"

  echo "Restoring Timescale backup into iiot_telemetry_restore"
  docker exec -i "$TIMESCALE_CONTAINER" psql -v ON_ERROR_STOP=1 -U "$TIMESCALE_USER" -d postgres <<SQL
DROP DATABASE IF EXISTS iiot_telemetry_restore;
CREATE DATABASE iiot_telemetry_restore;
SQL
  docker exec -i "$TIMESCALE_CONTAINER" psql -v ON_ERROR_STOP=1 -U "$TIMESCALE_USER" -d iiot_telemetry_restore < "$ts_sql"
}

verify_restore() {
  local backup_dir="$1"
  restore_to_temp "$backup_dir"

  local pg_ok ts_ok
  pg_ok="$(docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d iiot_platform_restore -Atqc "SELECT (to_regclass('public.users') IS NOT NULL)::int + (to_regclass('public.devices') IS NOT NULL)::int;")"
  ts_ok="$(docker exec -i "$TIMESCALE_CONTAINER" psql -U "$TIMESCALE_USER" -d iiot_telemetry_restore -Atqc "SELECT (to_regclass('public.telemetry') IS NOT NULL)::int;")"

  if [[ "$pg_ok" != "2" || "$ts_ok" != "1" ]]; then
    echo "Restore verification failed: users/devices/telemetry tables not found"
    exit 1
  fi

  local pg_counts ts_count
  pg_counts="$(docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d iiot_platform_restore -Atqc "SELECT (SELECT count(*) FROM users)::text || ',' || (SELECT count(*) FROM devices)::text;")"
  ts_count="$(docker exec -i "$TIMESCALE_CONTAINER" psql -U "$TIMESCALE_USER" -d iiot_telemetry_restore -Atqc "SELECT count(*) FROM telemetry;")"

  echo "Restore verified successfully"
  echo "iiot_platform_restore users,devices=$pg_counts"
  echo "iiot_telemetry_restore telemetry=$ts_count"
}

cmd="${1:-}"
case "$cmd" in
  backup)
    backup
    ;;
  restore)
    [[ $# -eq 2 ]] || { usage; exit 1; }
    restore_to_temp "$2"
    ;;
  verify)
    [[ $# -eq 2 ]] || { usage; exit 1; }
    verify_restore "$2"
    ;;
  *)
    usage
    exit 1
    ;;
esac
