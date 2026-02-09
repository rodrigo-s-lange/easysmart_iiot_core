#!/bin/bash
# Backup EMQX Configuration (Data + Config)
# Run after any manual changes in Dashboard

set -e

BACKUP_DIR="backups/emqx"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

mkdir -p "$BACKUP_DIR"

echo "🔧 Backing up EMQX configuration..."

# 1. Backup config file
echo "📄 Backing up emqx.conf..."
cp emqx/etc/emqx.conf "$BACKUP_DIR/emqx.conf.$TIMESTAMP"

# 2. Backup EMQX data directory (contains rules, connectors, etc)
echo "📦 Backing up EMQX data..."
docker exec iiot_emqx tar czf /tmp/emqx_data.tar.gz /opt/emqx/data 2>/dev/null || true
docker cp iiot_emqx:/tmp/emqx_data.tar.gz "$BACKUP_DIR/emqx_data.$TIMESTAMP.tar.gz"
docker exec iiot_emqx rm /tmp/emqx_data.tar.gz

echo "✅ EMQX backup completed!"
echo "📁 Files:"
echo "   - $BACKUP_DIR/emqx.conf.$TIMESTAMP"
echo "   - $BACKUP_DIR/emqx_data.$TIMESTAMP.tar.gz"
echo ""
echo "🔄 To restore after restart:"
echo "   docker cp $BACKUP_DIR/emqx_data.$TIMESTAMP.tar.gz iiot_emqx:/tmp/"
echo "   docker exec iiot_emqx tar xzf /tmp/emqx_data.$TIMESTAMP.tar.gz -C /"
echo "   docker restart iiot_emqx"

