#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if [[ -f "$ROOT_DIR/.env" ]]; then
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
fi

MODE="${OFFSITE_BACKUP_MODE:-disabled}" # disabled|rsync|rclone
BACKUP_ROOT="${BACKUP_ROOT:-$ROOT_DIR/backups/db}"
LATEST_BACKUP="${1:-$(ls -1dt "$BACKUP_ROOT"/* 2>/dev/null | head -n1 || true)}"

if [[ -z "$LATEST_BACKUP" || ! -d "$LATEST_BACKUP" ]]; then
  echo "No backup directory to sync"
  exit 1
fi

notify_telegram() {
  local text="$1"
  if [[ -n "${TELEGRAM_BOT_TOKEN:-}" && -n "${TELEGRAM_CHAT_ID:-}" ]]; then
    curl -sS -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
      -d chat_id="${TELEGRAM_CHAT_ID}" \
      --data-urlencode text="$text" >/dev/null || true
  fi
}

echo "Sync mode: $MODE"
echo "Backup dir: $LATEST_BACKUP"

case "$MODE" in
  disabled)
    echo "Offsite sync disabled (OFFSITE_BACKUP_MODE=disabled)"
    exit 0
    ;;
  rsync)
    : "${OFFSITE_RSYNC_TARGET:?OFFSITE_RSYNC_TARGET is required for rsync mode}"
    rsync -az --delete "$LATEST_BACKUP"/ "$OFFSITE_RSYNC_TARGET"/"$(basename "$LATEST_BACKUP")"/
    ;;
  rclone)
    : "${OFFSITE_RCLONE_REMOTE:?OFFSITE_RCLONE_REMOTE is required for rclone mode}"
    rclone sync "$LATEST_BACKUP" "${OFFSITE_RCLONE_REMOTE}/$(basename "$LATEST_BACKUP")"
    ;;
  *)
    echo "Invalid OFFSITE_BACKUP_MODE: $MODE"
    exit 1
    ;;
esac

echo "Offsite sync completed"
notify_telegram "[IIoT Core] Offsite backup sync concluido: $(basename "$LATEST_BACKUP") mode=${MODE}"
