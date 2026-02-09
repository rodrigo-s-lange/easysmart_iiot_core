#!/bin/bash
# Backup completo do IIoT Platform
# Versão: 1.0.0

set -e

BACKUP_DIR="backups/full_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$BACKUP_DIR"

echo "🔄 Starting full backup..."

# 1. PostgreSQL (auth database)
echo "📦 Backing up PostgreSQL (iiot_platform)..."
docker exec iiot_postgres pg_dump -U admin iiot_platform > "$BACKUP_DIR/postgres_iiot_platform.sql"

# 2. TimescaleDB (telemetry)
echo "📦 Backing up TimescaleDB (iiot_telemetry)..."
docker exec iiot_timescaledb pg_dump -U admin iiot_telemetry > "$BACKUP_DIR/timescale_iiot_telemetry.sql"

# 3. EMQX config
echo "📦 Backing up EMQX config..."
cp -r emqx/etc "$BACKUP_DIR/emqx_etc"
cp -r emqx/data "$BACKUP_DIR/emqx_data" 2>/dev/null || true

# 4. Go API
echo "📦 Backing up Go API..."
cp -r go-api "$BACKUP_DIR/go-api"

# 5. Environment
echo "📦 Backing up .env..."
cp .env "$BACKUP_DIR/env"

# 6. Docker Compose
echo "📦 Backing up docker-compose.yml..."
cp docker-compose.yml "$BACKUP_DIR/docker-compose.yml"

# 7. Database schemas
echo "📦 Backing up database schemas..."
cp -r database "$BACKUP_DIR/database"

# 8. Redis dump (if exists)
echo "📦 Backing up Redis..."
docker exec iiot_redis redis-cli --no-auth-warning -a "${REDIS_PASSWORD:-}" SAVE 2>/dev/null || true
docker cp iiot_redis:/data/dump.rdb "$BACKUP_DIR/redis_dump.rdb" 2>/dev/null || true

# 9. Create manifest
cat > "$BACKUP_DIR/MANIFEST.txt" << EOF
IIoT Platform Backup
====================
Date: $(date)
Hostname: $(hostname)
Containers:
$(docker ps --format "table {{.Names}}\t{{.Status}}" | grep iiot)

Database Sizes:
$(docker exec iiot_postgres psql -U admin -d iiot_platform -c "SELECT pg_size_pretty(pg_database_size('iiot_platform')) as size;")
$(docker exec iiot_timescaledb psql -U admin -d iiot_telemetry -c "SELECT pg_size_pretty(pg_database_size('iiot_telemetry')) as size;")

Files:
$(ls -lh "$BACKUP_DIR")
EOF

# 10. Compress
echo "🗜️  Compressing backup..."
tar -czf "$BACKUP_DIR.tar.gz" -C backups "$(basename $BACKUP_DIR)"
rm -rf "$BACKUP_DIR"

echo "✅ Backup completed: $BACKUP_DIR.tar.gz"
echo "📊 Backup size: $(du -h "$BACKUP_DIR.tar.gz" | cut -f1)"

