#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if [[ -f "$ROOT_DIR/.env" ]]; then
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
fi

LOG_DIR="$ROOT_DIR/backups/ops_logs"
STATE_DIR="$ROOT_DIR/backups/ops_state"
mkdir -p "$LOG_DIR" "$STATE_DIR"

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
LOG_FILE="$LOG_DIR/backup-$STAMP.log"

notify_telegram() {
  local text="$1"
  if [[ -n "${TELEGRAM_BOT_TOKEN:-}" && -n "${TELEGRAM_CHAT_ID:-}" ]]; then
    curl -sS -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
      -d chat_id="${TELEGRAM_CHAT_ID}" \
      --data-urlencode text="$text" >/dev/null || true
  fi
}

{
  echo "[$(date -u +%FT%TZ)] starting daily backup"
  ./database/backup_restore.sh backup
  LATEST_BACKUP="$(ls -1dt backups/db/* | head -n1)"
  echo "[$(date -u +%FT%TZ)] backup ok: ${LATEST_BACKUP}"
  echo "[$(date -u +%FT%TZ)] syncing backup offsite (if enabled)"
  ./scripts/ops/sync_backup_offsite.sh "$LATEST_BACKUP"
  echo "status=ok" > "$STATE_DIR/last_backup.status"
  echo "timestamp=$(date -u +%FT%TZ)" >> "$STATE_DIR/last_backup.status"
  echo "backup_dir=$LATEST_BACKUP" >> "$STATE_DIR/last_backup.status"
  echo "offsite_mode=${OFFSITE_BACKUP_MODE:-disabled}" >> "$STATE_DIR/last_backup.status"
} | tee "$LOG_FILE"

notify_telegram "[IIoT Core] Backup diario concluido: ${STAMP}"
