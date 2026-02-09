#!/bin/bash
# Restore EMQX Configuration (Data + Config)
# Run after EMQX restart or container recreation

set -e

BACKUP_DIR="backups/emqx"

# Find latest backup
LATEST_DATA=$(ls -t "$BACKUP_DIR"/emqx_data.*.tar.gz 2>/dev/null | head -1)
LATEST_CONF=$(ls -t "$BACKUP_DIR"/emqx.conf.* 2>/dev/null | head -1)

if [ -z "$LATEST_DATA" ] || [ -z "$LATEST_CONF" ]; then
    echo "❌ No backups found in $BACKUP_DIR"
    exit 1
fi

echo "🔄 Restoring EMQX configuration..."
echo "📦 Data: $LATEST_DATA"
echo "📄 Config: $LATEST_CONF"

# 1. Restore config
echo "📄 Restoring emqx.conf..."
cp "$LATEST_CONF" emqx/etc/emqx.conf

# 2. Restore data
echo "📦 Restoring EMQX data..."
docker cp "$LATEST_DATA" iiot_emqx:/tmp/emqx_data.tar.gz
docker exec iiot_emqx tar xzf /tmp/emqx_data.tar.gz -C /
docker exec iiot_emqx rm /tmp/emqx_data.tar.gz

# 3. Restart EMQX
echo "🔄 Restarting EMQX..."
docker restart iiot_emqx

echo ""
echo "✅ EMQX configuration restored!"
echo "⏳ Waiting 10s for EMQX to start..."
sleep 10

echo "🧪 Testing EMQX..."
curl -s http://192.168.0.99:18083 > /dev/null && echo "✅ EMQX Dashboard is up!" || echo "❌ EMQX Dashboard is down"

