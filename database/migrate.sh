#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

TARGET="all"
BASELINE="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)
      TARGET="${2:-}"
      shift 2
      ;;
    --baseline)
      BASELINE="true"
      shift
      ;;
    *)
      echo "Unknown option: $1" >&2
      echo "Usage: ./database/migrate.sh [--target postgres|timescale|all] [--baseline]" >&2
      exit 1
      ;;
  esac
done

if [[ -f "${ROOT_DIR}/.env" ]]; then
  # shellcheck disable=SC1091
  source "${ROOT_DIR}/.env"
fi

POSTGRES_USER="${POSTGRES_USER:-admin}"
POSTGRES_DB="${POSTGRES_DB:-iiot_platform}"
TIMESCALE_USER="${TIMESCALE_USER:-admin}"
TIMESCALE_DB="${TIMESCALE_DB:-iiot_telemetry}"

apply_dir() {
  local container="$1"
  local db_user="$2"
  local db_name="$3"
  local migrations_dir="$4"
  local table_name="$5"

  docker exec -i "${container}" psql -v ON_ERROR_STOP=1 -U "${db_user}" -d "${db_name}" <<SQL
CREATE TABLE IF NOT EXISTS ${table_name} (
  id BIGSERIAL PRIMARY KEY,
  version TEXT NOT NULL,
  filename TEXT NOT NULL UNIQUE,
  checksum TEXT NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
SQL

  while IFS= read -r file; do
    local filename version checksum existing
    filename="$(basename "${file}")"
    version="${filename%%_*}"
    checksum="$(sha256sum "${file}" | awk '{print $1}')"

    existing="$(docker exec "${container}" psql -U "${db_user}" -d "${db_name}" -Atqc "SELECT checksum FROM ${table_name} WHERE filename='${filename}' LIMIT 1;")"

    if [[ -n "${existing}" ]]; then
      if [[ "${existing}" != "${checksum}" ]]; then
        echo "Checksum mismatch for ${filename} (${table_name})" >&2
        exit 1
      fi
      echo "skip ${filename} (already applied)"
      continue
    fi

    if [[ "${BASELINE}" == "true" ]]; then
      echo "baseline ${filename}"
      docker exec "${container}" psql -v ON_ERROR_STOP=1 -U "${db_user}" -d "${db_name}" \
        -c "INSERT INTO ${table_name}(version, filename, checksum) VALUES ('${version}','${filename}','${checksum}');"
      continue
    fi

    echo "apply ${filename}"
    docker exec -i "${container}" psql -v ON_ERROR_STOP=1 -U "${db_user}" -d "${db_name}" < "${file}"
    docker exec "${container}" psql -v ON_ERROR_STOP=1 -U "${db_user}" -d "${db_name}" \
      -c "INSERT INTO ${table_name}(version, filename, checksum) VALUES ('${version}','${filename}','${checksum}');"
  done < <(find "${migrations_dir}" -maxdepth 1 -type f -name '*.sql' | sort)
}

if [[ "${TARGET}" == "postgres" || "${TARGET}" == "all" ]]; then
  apply_dir "iiot_postgres" "${POSTGRES_USER}" "${POSTGRES_DB}" "${ROOT_DIR}/database/migrations" "schema_migrations"
fi

if [[ "${TARGET}" == "timescale" || "${TARGET}" == "all" ]]; then
  apply_dir "iiot_timescaledb" "${TIMESCALE_USER}" "${TIMESCALE_DB}" "${ROOT_DIR}/database/timescale/migrations" "timescale_schema_migrations"
fi
