#!/bin/bash
# Restaurar configuração do EMQX do backup mais recente

BACKUP_DIR="/home/rodrigo/easysmart_iiot_core/backups"
LATEST_BACKUP=$(ls -t $BACKUP_DIR/emqx_config_*.tar.gz 2>/dev/null | head -1)

if [ -z "$LATEST_BACKUP" ]; then
    echo "Nenhum backup encontrado!"
    exit 1
fi

echo "Restaurando backup: $LATEST_BACKUP"
docker exec -i iiot_emqx emqx ctl data import < "$LATEST_BACKUP"
echo "Backup restaurado com sucesso!"
echo "Reinicie o EMQX: docker-compose restart emqx"

