# Runbooks de Incidente

## Política de on-call (atual)
- Canal primário: Telegram (`critical`).
- Primeiro acionado: Rodrigo (sem notificação automática ao cliente).
- Janela de manutenção: 03:00-03:30 UTC (alertas críticos mutados nesse período).

## 1) API Down (`GoApiDown`)
### Sintoma
- Alerta `GoApiDown` ativo.
- `curl http://localhost:3001/health` falha.

### Diagnóstico rápido
```bash
docker compose ps go_api
docker logs iiot_go_api --tail 200
curl -s -i http://localhost:3001/health/live
curl -s -i http://localhost:3001/health/ready
```

### Ação
```bash
docker compose up -d --build go_api
```

### Validação
```bash
curl -s http://localhost:3001/health
curl -s http://localhost:3001/metrics | head
```

### Escalonar se
- não recuperar em 10 min;
- crash loop persistente;
- erro de migração/secret obrigatório no startup.

---

## 2) DB Down (`PostgresTcpDown` ou `TimescaleTcpDown`)
### Sintoma
- Alertas de TCP probe ou `GoApiReadinessFailing`.

### Diagnóstico rápido
```bash
docker compose ps postgres timescaledb
docker logs iiot_postgres --tail 200
docker logs iiot_timescaledb --tail 200
curl -s http://localhost:3001/health/ready
```

### Ação
```bash
docker compose restart postgres timescaledb
```
Se API continuar degraded:
```bash
docker compose restart go_api
```

### Validação
```bash
curl -s http://localhost:3001/health/ready
```
Esperado: `status=ok`.

### Escalonar se
- banco não sobe por corrupção/storage cheio;
- necessidade de restore;
- repetição do incidente no mesmo dia.

---

## 3) Backlog/Falha de Ingestão (`TelemetryIngestionFailureSpike`)
### Sintoma
- aumento sustentado de `telemetry_rejected_total` por `db_error`, `tenant_mismatch`, `device_not_found`, `rate_limiter_unavailable`.

### Diagnóstico rápido
```bash
docker logs iiot_go_api --since 15m | tail -200
docker logs iiot_emqx --since 15m | tail -200
curl -s http://localhost:3001/metrics | grep -E "telemetry_ingested_total|telemetry_rejected_total"
```

### Ação por causa
- `db_error`: validar Timescale/Postgres e readiness.
- `rate_limiter_unavailable`: validar Redis e conectividade.
- `tenant_mismatch` / `device_not_found`: revisar credenciais MQTT/device_label/topic.

Comandos úteis:
```bash
docker compose ps redis timescaledb postgres go_api emqx
docker compose restart redis
```

### Validação
- queda da taxa de rejeição no Prometheus/Grafana;
- novos inserts em Timescale:
```bash
docker exec -i iiot_timescaledb psql -U admin -d iiot_telemetry -c "SELECT count(*) FROM telemetry;"
```

### Escalonar se
- rejeição > 10 min após ação corretiva;
- impacto em múltiplos tenants;
- suspeita de regressão após deploy.

---

## Pós-incidente (obrigatório)
1. Registrar timeline (início, detecção, mitigação, recuperação).
2. Registrar tenant(s) impactado(s) e janela.
3. Abrir ação corretiva com causa raiz e prevenção.

---

## 4) Rollback manual de deploy (Go API / Telegram bot)
### Objetivo
Voltar rapidamente para as imagens anteriores quando o deploy degradar o serviço.

### Pré-requisito
Sempre fazer deploy usando:
```bash
./scripts/ops/deploy_core.sh
```
Esse comando salva as imagens anteriores em `deploy/state/last_images.env`.

### Rollback
```bash
./scripts/ops/rollback_core.sh
docker compose ps go_api telegram_ops_bot
```

### Validação pós-rollback
```bash
curl -s -i http://localhost:3001/health
curl -s -i http://localhost:3001/health/ready
```

### Escalonar se
- não existir `deploy/state/last_images.env`;
- imagem anterior também falhar;
- erro persistir após rollback.
