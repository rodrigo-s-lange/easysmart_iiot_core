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
LOG_FILE="$LOG_DIR/restore-drill-$STAMP.log"

notify_telegram() {
  local text="$1"
  if [[ -n "${TELEGRAM_BOT_TOKEN:-}" && -n "${TELEGRAM_CHAT_ID:-}" ]]; then
    curl -sS -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
      -d chat_id="${TELEGRAM_CHAT_ID}" \
      --data-urlencode text="$text" >/dev/null || true
  fi
}

LATEST_BACKUP="$(ls -1dt backups/db/* 2>/dev/null | head -n1 || true)"
if [[ -z "$LATEST_BACKUP" ]]; then
  echo "[$(date -u +%FT%TZ)] no backups available for restore drill" | tee "$LOG_FILE"
  echo "status=failed" > "$STATE_DIR/last_restore_drill.status"
  echo "timestamp=$(date -u +%FT%TZ)" >> "$STATE_DIR/last_restore_drill.status"
  echo "reason=no_backup_found" >> "$STATE_DIR/last_restore_drill.status"
  notify_telegram "[IIoT Core] Restore drill semanal FALHOU: nenhum backup encontrado"
  exit 1
fi

if {
  echo "[$(date -u +%FT%TZ)] starting weekly restore drill from ${LATEST_BACKUP}"
  ./database/backup_restore.sh verify "$LATEST_BACKUP"
  echo "[$(date -u +%FT%TZ)] restore drill ok"
} | tee "$LOG_FILE"; then
  echo "status=ok" > "$STATE_DIR/last_restore_drill.status"
  echo "timestamp=$(date -u +%FT%TZ)" >> "$STATE_DIR/last_restore_drill.status"
  echo "backup_dir=$LATEST_BACKUP" >> "$STATE_DIR/last_restore_drill.status"
  notify_telegram "[IIoT Core] Restore drill semanal concluido: ${STAMP}"
else
  echo "status=failed" > "$STATE_DIR/last_restore_drill.status"
  echo "timestamp=$(date -u +%FT%TZ)" >> "$STATE_DIR/last_restore_drill.status"
  echo "backup_dir=$LATEST_BACKUP" >> "$STATE_DIR/last_restore_drill.status"
  notify_telegram "[IIoT Core] Restore drill semanal FALHOU: ${STAMP}"
  exit 1
fi
