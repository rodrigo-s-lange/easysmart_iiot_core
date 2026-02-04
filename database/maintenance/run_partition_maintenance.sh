#!/bin/bash
# Script de manutenção mensal de partições
# Roda via cron todo dia 1º do mês

CONTAINER="iiot_postgres"
SQL_FILE="/tmp/create_future_partitions.sql"
LOG_FILE="/var/log/iiot_partition_maintenance.log"

echo "=== $(date '+%Y-%m-%d %H:%M:%S') - Iniciando manutenção de partições ===" >> $LOG_FILE

# Copia script SQL para container
docker cp /home/rodrigo/iiot_platform/database/maintenance/create_future_partitions.sql $CONTAINER:$SQL_FILE

# Executa script
docker exec $CONTAINER psql -U admin -d iiot_platform -f $SQL_FILE >> $LOG_FILE 2>&1

echo "=== Manutenção concluída ===" >> $LOG_FILE
echo "" >> $LOG_FILE