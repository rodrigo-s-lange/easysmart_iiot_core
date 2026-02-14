# Production Validation (Minimum Audit Set)

Este guia define o pacote mínimo auditável para entrada em produção do backend.

## Escopo mínimo
- Isolamento entre tenants (RLS em Postgres e TimescaleDB)
- Integração ponta a ponta MQTT -> EMQX -> Go API -> TimescaleDB
- Backup/restore validado em banco limpo
- Carga básica reproduzível com relatório

## Pré-requisitos
- Stack em execução: `docker-compose up -d`
- API saudável: `curl -s http://localhost:3001/health`
- Ferramentas no host: `bash`, `python3`, `mosquitto_pub`, `docker`

## 1) Isolamento de tenants (RLS)
Script:
```bash
./scripts/audit/test_tenant_isolation.sh
```

Valida:
- tenant A lê seu próprio `device`;
- tenant A não lê `device` do tenant B;
- tenant A lê sua telemetria;
- tenant A não lê telemetria do tenant B.

Saída esperada:
- `OK: Tenant isolation validated in Postgres and TimescaleDB`

## 2) E2E MQTT -> API -> DB
Script:
```bash
./scripts/audit/test_e2e_mqtt_ingest.sh
```

Valida:
- criação de dois tenants via API;
- provisionamento de device para tenant A;
- publish MQTT válido;
- persistência no TimescaleDB;
- leitura permitida para tenant A;
- leitura negada para tenant B.

Saída esperada:
- `OK: E2E MQTT ingestion and tenant isolation at API layer validated`

## 3) Backup e restore
### 3.1 Backup
```bash
./database/backup_restore.sh backup
```

### 3.2 Verify restore em bancos temporários
```bash
LATEST_BACKUP="$(ls -1dt backups/db/* | head -n1)"
./database/backup_restore.sh verify "$LATEST_BACKUP"
```

Valida:
- restauração em `iiot_platform_restore` e `iiot_telemetry_restore`;
- presença das tabelas críticas (`users`, `devices`, `telemetry`);
- contagem de registros após restore.

## 4) Carga básica reproduzível (k6)
Script:
```bash
RATE=40 DURATION=2m DEVICE_POOL_SIZE=40 ./scripts/audit/run_load_test.sh
```

Saída:
- resumo do k6 em `artifacts/load/k6-summary-<timestamp>.json`
- latência/erros no terminal.

Observação:
- o teste provisiona múltiplos devices para respeitar limites por device/slot.

## 5) Security smoke (hardening de borda)
Script:
```bash
./scripts/audit/security_smoke.sh
```

Valida:
- método inválido em endpoint sensível retorna `405`;
- JWT adulterado é rejeitado (`401`);
- entrada estilo SQL injection não causa `500`.

## SLO operacional inicial (ajustável)
- `RPO`: 24h (backup diário)
- `RTO`: 30min (restore + validação técnica)

Esses valores são ponto de partida e devem ser revisados após o primeiro ciclo de operação real.

## Evidências para auditoria
- logs de execução dos 4 scripts;
- diretório de backup utilizado;
- resumo k6 (`artifacts/load/*.json`);
- versão do commit validado (`git rev-parse --short HEAD`).
