#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

ROLLBACK_FILE="$ROOT_DIR/deploy/state/last_images.env"
if [[ ! -f "$ROLLBACK_FILE" ]]; then
  echo "Missing rollback state file: $ROLLBACK_FILE"
  exit 1
fi

# shellcheck disable=SC1090
source "$ROLLBACK_FILE"

if [[ -n "${GO_ROLLBACK_TAG:-}" ]]; then
  docker image inspect "$GO_ROLLBACK_TAG" >/dev/null 2>&1 || { echo "Rollback image not found: $GO_ROLLBACK_TAG"; exit 1; }
  docker tag "$GO_ROLLBACK_TAG" easysmart_iiot_core-go_api:latest
  echo "Tagged go_api rollback image: $GO_ROLLBACK_TAG"
fi

if [[ -n "${BOT_ROLLBACK_TAG:-}" ]]; then
  docker image inspect "$BOT_ROLLBACK_TAG" >/dev/null 2>&1 || { echo "Rollback image not found: $BOT_ROLLBACK_TAG"; exit 1; }
  docker tag "$BOT_ROLLBACK_TAG" easysmart_iiot_core-telegram_ops_bot:latest
  echo "Tagged telegram_ops_bot rollback image: $BOT_ROLLBACK_TAG"
fi

docker compose up -d go_api telegram_ops_bot
echo "Rollback completed using images saved at ${SAVED_AT_UTC:-unknown}"
