#!/bin/sh
set -eu

WEBHOOK_URL="${ALERT_WEBHOOK_URL:-}"
if [ -z "$WEBHOOK_URL" ]; then
  # Receiver sink; use ALERT_WEBHOOK_URL in .env for real notifications.
  WEBHOOK_URL="http://127.0.0.1:65535/alerts"
fi

cat >/etc/alertmanager/alertmanager.yml <<EOF
global:
  resolve_timeout: 5m

route:
  receiver: default-webhook
  group_by: ["alertname", "severity"]
  group_wait: 15s
  group_interval: 1m
  repeat_interval: 30m

receivers:
  - name: default-webhook
    webhook_configs:
      - url: "${WEBHOOK_URL}"
        send_resolved: true
EOF

exec /bin/alertmanager \
  --config.file=/etc/alertmanager/alertmanager.yml \
  --storage.path=/alertmanager \
  --web.listen-address=:9093
