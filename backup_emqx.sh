#!/bin/bash
# Backup automático das configurações do EMQX

BACKUP_DIR="/home/rodrigo/easysmart_iiot_core/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Exportar configuração do EMQX
docker exec iiot_emqx emqx ctl data export > "$BACKUP_DIR/emqx_config_$TIMESTAMP.tar.gz"

# Manter apenas os 5 backups mais recentes
ls -t $BACKUP_DIR/emqx_config_*.tar.gz | tail -n +6 | xargs -r rm

echo "Backup criado: emqx_config_$TIMESTAMP.tar.gz"

