# EasySmart IIoT Core

Backend da plataforma IIoT (auth, devices, ingestao MQTT, telemetria, observabilidade e operacao).

## Estado Atual
- API Go em producao local com prefixo estavel `/api/v1`.
- Persistencia separada:
  - PostgreSQL: tenants/users/devices/auditoria/billing.
  - TimescaleDB: telemetria (`telemetry`) com `tenant_id`.
- MQTT via EMQX 5.5 com webhook para API.
- Redis para cache/rate-limit/quotas.
- Observabilidade com Prometheus + Alertmanager + Grafana.
- Telegram operacional ativo para alertas e eventos.

## Servicos Locais
- API: `http://localhost:3001`
- OpenAPI: `docs/openapi.yaml`
- Swagger UI: `http://localhost:8088`
- EMQX Dashboard: `http://localhost:18083`
- Prometheus: `http://localhost:9090`
- Alertmanager: `http://localhost:9093`
- Grafana: `http://localhost:3002`

## Subida
```bash
docker compose up -d
docker compose ps
```

## Fluxos Implementados

### Auth
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`

### Devices
- Provisionamento direto (principal):
  - `POST /api/v1/devices/provision`
- Fluxo claim (legado suportado):
  - `POST /api/v1/devices/claim`
  - `POST /api/v1/devices/bootstrap`
  - `POST /api/v1/devices/secret` (one-time)
  - `POST /api/v1/devices/reset`

### Telemetry
- Ingestao: `POST /api/v1/telemetry`
- Leitura:
  - `GET /api/v1/telemetry/latest`
  - `GET /api/v1/telemetry/slots`

### Tenant Admin (super_admin)
- `GET /api/v1/tenants/{tenant_id}/quotas`
- `PATCH /api/v1/tenants/{tenant_id}/quotas`
- `GET /api/v1/tenants/{tenant_id}/usage`

## Seguranca Aplicada
- JWT para endpoints de usuario.
- API key para webhook de telemetria.
- Isolamento por tenant nas leituras de telemetria.
- Validacao tenant/topic/device no webhook MQTT.
- Rate-limit de auth e limites de telemetria.
- Quotas de billing por tenant (devices, msg/min por device, storage).
- Trilhas de auditoria em `audit_log`.

## Billing/Quotas
- Planos: `starter`, `pro`, `enterprise`.
- Ciclos: `monthly`, `annual`.
- Defaults:
  - `quota_devices=0` (ilimitado)
  - `quota_msgs_per_min=360`
  - `quota_storage_mb=1000`
- Bloqueios:
  - `starter/pro`: bloqueio duro ao exceder.
  - `enterprise`: pode permitir overage se `allow_overage=true`.

Detalhes: `docs/BILLING_QUOTAS.md`.

## Observabilidade e Telegram
- Alertas criticos no Telegram via Alertmanager.
- `telegram_ops_bot` com comandos:
  - `/health`
  - `/status`
  - `/metrics`
  - `/logs api|emqx|postgres|timescale|redis`
- Eventos operacionais enviados pelo backend:
  - `🧾 [USUARIO] Cadastro`
  - `📟 [DEVICE] Cadastro`
  - `🚨 [QUOTA] Evento`
- Formato padronizado com campos em ordem fixa e horario BR.
- Duplicidade desativada por padrao no watcher DB:
  - `TELEGRAM_WATCH_NOTIFY_EVENTS=false` (default).

Detalhes: `docs/OBSERVABILITY.md`.

## Migrações
- Postgres: `database/migrations` (`002` a `006`).
- Timescale: `database/timescale/migrations`.
- Legado: `database/migrations/legacy/` (nao aplicar em ambiente novo).

```bash
./database/migrate.sh --target all
```

## Resiliencia de Dados
- Backup diario + restore drill semanal.
- Sync offsite de backup (rsync/rclone) configuravel por `.env`.
- Politica de retencao/arquivamento por tenant.
- RPO/RTO por plano (documentado).

Detalhes: `docs/DATA_RESILIENCE.md`.

## Validacao Operacional
- Isolamento tenant: `scripts/audit/test_tenant_isolation.sh`
- E2E MQTT ingestao: `scripts/audit/test_e2e_mqtt_ingest.sh`
- Security smoke: `scripts/audit/security_smoke.sh`
- Carga basica: `scripts/audit/run_load_test.sh`

## Deploy seguro minimo
- Deploy com checkpoint para rollback:
  - `./scripts/ops/deploy_core.sh`
- Rollback manual:
  - `./scripts/ops/rollback_core.sh`
- CI basico em GitHub Actions:
  - `gofmt` check
  - `go vet`
  - `go test`
  - `go build`
  - validacao de `docker build`

Checklist para liberar producao:
1. CI mais recente do branch principal precisa estar `success`.
2. Offsite backup habilitado (`OFFSITE_BACKUP_MODE=rsync` ou `rclone`).
3. Timer de backup offsite ativo no systemd.

Comandos:
```bash
./scripts/ops/check_ci_status.sh
sudo systemctl status --no-pager easysmart_iiot_backup_offsite.timer
grep -E '^OFFSITE_BACKUP_MODE=' .env
```

## Referencias
- Contrato API: `docs/openapi.yaml`
- Changelog: `CHANGELOG.md`
- SLO/SLI: `docs/SLO_SLI.md`
- Runbooks: `docs/RUNBOOKS.md`
- Validacao para producao: `docs/PRODUCTION_VALIDATION.md`
