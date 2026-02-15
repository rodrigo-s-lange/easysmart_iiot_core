#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

STATE_DIR="$ROOT_DIR/deploy/state"
mkdir -p "$STATE_DIR"
ROLLBACK_FILE="$STATE_DIR/last_images.env"

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
GO_ROLLBACK_TAG="easysmart_iiot_core-go_api:rollback-${STAMP}"
BOT_ROLLBACK_TAG="easysmart_iiot_core-telegram_ops_bot:rollback-${STAMP}"

if docker image inspect easysmart_iiot_core-go_api:latest >/dev/null 2>&1; then
  docker tag easysmart_iiot_core-go_api:latest "$GO_ROLLBACK_TAG"
fi

if docker image inspect easysmart_iiot_core-telegram_ops_bot:latest >/dev/null 2>&1; then
  docker tag easysmart_iiot_core-telegram_ops_bot:latest "$BOT_ROLLBACK_TAG"
fi

cat > "$ROLLBACK_FILE" <<EOF
GO_ROLLBACK_TAG=${GO_ROLLBACK_TAG}
BOT_ROLLBACK_TAG=${BOT_ROLLBACK_TAG}
SAVED_AT_UTC=$(date -u +%FT%TZ)
EOF

echo "Saved rollback tags in $ROLLBACK_FILE"
docker compose up -d --build go_api telegram_ops_bot
echo "Deploy completed"
