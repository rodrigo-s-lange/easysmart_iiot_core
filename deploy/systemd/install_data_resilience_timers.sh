#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

sudo cp "$SCRIPT_DIR"/easysmart_iiot_backup.service /etc/systemd/system/
sudo cp "$SCRIPT_DIR"/easysmart_iiot_backup.timer /etc/systemd/system/
sudo cp "$SCRIPT_DIR"/easysmart_iiot_restore_drill.service /etc/systemd/system/
sudo cp "$SCRIPT_DIR"/easysmart_iiot_restore_drill.timer /etc/systemd/system/

sudo systemctl daemon-reload
sudo systemctl enable --now easysmart_iiot_backup.timer easysmart_iiot_restore_drill.timer

systemctl list-timers --all | grep -E 'easysmart_iiot_backup|easysmart_iiot_restore_drill' || true
