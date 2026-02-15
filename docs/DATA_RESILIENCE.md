# Data Resilience

Este documento define operação de backup/restore, retenção/arquivamento de telemetria e RPO/RTO por plano.

## 1) Backup agendado + restore drill semanal

### Scripts operacionais
- Backup diário: `scripts/ops/run_backup_daily.sh`
- Restore drill semanal: `scripts/ops/run_restore_drill_weekly.sh`
- Sync offsite: `scripts/ops/sync_backup_offsite.sh`

Os scripts usam `database/backup_restore.sh` e gravam estado em:
- `backups/ops_state/last_backup.status`
- `backups/ops_state/last_restore_drill.status`

Se Telegram estiver configurado (`TELEGRAM_BOT_TOKEN` e `TELEGRAM_CHAT_ID`), enviam notificação de sucesso/falha.

### Offsite backup (essencial)
Configuração por `.env`:
- `OFFSITE_BACKUP_MODE=disabled|rsync|rclone`
- `OFFSITE_RSYNC_TARGET=user@backup-host:/srv/easysmart/iiot_core` (quando `rsync`)
- `OFFSITE_RCLONE_REMOTE=remote:bucket/path` (quando `rclone`)

Comportamento:
- `run_backup_daily.sh` executa backup local e depois tenta sync offsite.
- Se `OFFSITE_BACKUP_MODE=disabled`, o passo offsite é ignorado sem erro.

### Agendamento systemd (produção)
Arquivos:
- `deploy/systemd/easysmart_iiot_backup.service`
- `deploy/systemd/easysmart_iiot_backup.timer`
- `deploy/systemd/easysmart_iiot_backup_offsite.service`
- `deploy/systemd/easysmart_iiot_backup_offsite.timer`
- `deploy/systemd/easysmart_iiot_restore_drill.service`
- `deploy/systemd/easysmart_iiot_restore_drill.timer`

Instalação:
```bash
./deploy/systemd/install_data_resilience_timers.sh
```

Agenda padrão:
- Backup diário: `03:05 UTC`
- Offsite diário: `03:15 UTC`
- Restore drill semanal: `domingo 03:20 UTC`

## 2) Retenção/arquivamento de telemetria por tenant

### Política por tenant (Timescale)
Migração: `database/timescale/migrations/003_tenant_retention_policy.sql`

Tabela:
- `tenant_telemetry_retention_policy`
  - `tenant_id`
  - `retention_days`
  - `archive_before_delete`
  - `archive_bucket`
  - `enabled`

Funções:
- `prune_telemetry_for_tenant(tenant_id, retention_days, batch_size)`
- `prune_telemetry_all_tenants(default_retention_days, batch_size)`

Exemplo de configuração de tenant:
```sql
INSERT INTO tenant_telemetry_retention_policy (tenant_id, retention_days, archive_before_delete, enabled)
VALUES ('83409caf-43f8-40b3-8ffe-32b8f0c16a94', 180, true, true)
ON CONFLICT (tenant_id)
DO UPDATE SET retention_days = EXCLUDED.retention_days,
              archive_before_delete = EXCLUDED.archive_before_delete,
              enabled = EXCLUDED.enabled,
              updated_at = NOW();
```

### Arquivamento por tenant (manual/cron)
Script:
```bash
./database/timescale/maintenance/archive_telemetry_by_tenant.sh <tenant_id> [retention_days] [archive_dir]
```

Fluxo:
1. Exporta CSV de dados antigos para `backups/archive/`.
2. Executa prune no Timescale para o tenant.

## 3) RPO/RTO por plano comercial

| Plano | RPO | RTO | Backup | Restore drill |
|------|-----|-----|--------|---------------|
| Starter | 24h | 8h | diário | semanal |
| Pro | 4h | 2h | diário + incremental | semanal |
| Enterprise | 1h | 30m | contínuo/incremental | semanal + teste mensal completo |

Observações:
- Valores acima são compromisso operacional alvo e devem ser refletidos em contrato comercial/SLA.
- Mudança de plano requer revisão de custo de armazenamento, janelas de manutenção e política de retenção.

## 4) Checklist operacional

### Diário
- Verificar status do backup do dia (`last_backup.status`).
- Confirmar tamanho/crescimento de `backups/db`.

### Semanal
- Verificar resultado do restore drill (`last_restore_drill.status`).
- Validar alertas no Telegram em caso de falha.

### Mensal
- Revisar retenção por tenant e volume de arquivamento.
- Revisar aderência de RPO/RTO por plano.
