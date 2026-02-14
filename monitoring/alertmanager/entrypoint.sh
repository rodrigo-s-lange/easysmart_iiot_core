#!/bin/sh
set -eu

TELEGRAM_BOT_TOKEN="${TELEGRAM_BOT_TOKEN:-}"
TELEGRAM_CHAT_ID="${TELEGRAM_CHAT_ID:-}"
FALLBACK_WEBHOOK_URL="${ALERT_FALLBACK_WEBHOOK_URL:-${ALERT_WEBHOOK_URL:-}}"

# Sink receiver for environments without external integration.
if [ -z "$FALLBACK_WEBHOOK_URL" ]; then
  FALLBACK_WEBHOOK_URL="http://127.0.0.1:65535/alerts"
fi

build_telegram_receiver() {
  cat <<EOF
  - name: telegram-critical
    telegram_configs:
      - bot_token: "${TELEGRAM_BOT_TOKEN}"
        chat_id: ${TELEGRAM_CHAT_ID}
        parse_mode: "Markdown"
        message: |
          *[{{ .Status | toUpper }}]* {{ .CommonLabels.alertname }}
          Severity: {{ .CommonLabels.severity }}
          {{- if .CommonAnnotations.summary }}
          Summary: {{ .CommonAnnotations.summary }}
          {{- end }}
          {{- if .CommonAnnotations.description }}
          Description: {{ .CommonAnnotations.description }}
          {{- end }}
          StartsAt: {{ (index .Alerts 0).StartsAt }}
        send_resolved: true
EOF
}

build_fallback_receiver() {
  cat <<EOF
  - name: fallback-webhook
    webhook_configs:
      - url: "${FALLBACK_WEBHOOK_URL}"
        send_resolved: true
EOF
}

if [ -n "$TELEGRAM_BOT_TOKEN" ] && [ -n "$TELEGRAM_CHAT_ID" ]; then
  ROUTE_BLOCK=$(cat <<'EOF'
route:
  receiver: fallback-webhook
  group_by: ["alertname", "severity"]
  group_wait: 15s
  group_interval: 1m
  repeat_interval: 30m
  routes:
    - receiver: telegram-critical
      matchers:
        - severity="critical"
      continue: true
      mute_time_intervals: ["daily_maintenance"]
EOF
)
else
  ROUTE_BLOCK=$(cat <<'EOF'
route:
  receiver: fallback-webhook
  group_by: ["alertname", "severity"]
  group_wait: 15s
  group_interval: 1m
  repeat_interval: 30m
EOF
)
fi

cat >/etc/alertmanager/alertmanager.yml <<EOF
global:
  resolve_timeout: 5m

$ROUTE_BLOCK

time_intervals:
  - name: daily_maintenance
    time_intervals:
      - times:
          - start_time: "03:00"
            end_time: "03:30"
        weekdays: ["monday:friday", "saturday", "sunday"]
        location: "UTC"

receivers:
$(build_fallback_receiver)
$(if [ -n "$TELEGRAM_BOT_TOKEN" ] && [ -n "$TELEGRAM_CHAT_ID" ]; then build_telegram_receiver; fi)
EOF

exec /bin/alertmanager \
  --config.file=/etc/alertmanager/alertmanager.yml \
  --storage.path=/alertmanager \
  --web.listen-address=:9093
