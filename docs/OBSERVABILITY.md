# Observability

Este documento cobre monitoramento e alertas do `easysmart_iiot_core` fora do contrato de API.

## Componentes
- `prometheus`: coleta métricas e avalia regras de alerta.
- `blackbox_exporter`: probe HTTP para `/health/live` e `/health/ready`.
- `alertmanager`: roteia alertas para webhook.
- `grafana`: dashboards.
- `telegram_ops_bot`: comandos operacionais e notificações de eventos.

## Endpoints
- Prometheus: `http://localhost:9090`
- Alertmanager: `http://localhost:9093`
- Grafana: `http://localhost:3002` (default: `admin/admin` se não configurado)

## Subir stack
```bash
docker compose up -d prometheus blackbox_exporter alertmanager grafana telegram_ops_bot
docker compose ps
```

## Alertas críticos implementados
Arquivo: `monitoring/prometheus/alerts.yml`

1. `GoApiDown` (critical)
- Regra: `up{job="go_api"} == 0` por `2m`
- Significado: Prometheus não consegue coletar `/metrics` da API.

2. `GoApiReadinessFailing` (critical)
- Regra: `probe_success{job="go_api_ready"} == 0` por `2m`
- Significado: `/health/ready` está falhando (dependências indisponíveis).

3. `GoApi5xxSpike` (warning)
- Regra: `sum(rate(http_requests_total{status=~"5.."}[5m])) > 0.1` por `5m`
- Significado: aumento sustentado de erros 5xx.

4. `GoApiHighLatencyP95` (warning)
- Regra: `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le)) > 0.5` por `10m`
- Significado: p95 de latência acima do objetivo.

5. `GoApiHighErrorRate` (critical)
- Regra: taxa de 5xx > 1% por 10m.
- Significado: violação de SLO de erro HTTP.

6. `PostgresTcpDown` / `TimescaleTcpDown` / `EmqxTcpDown` (critical)
- Regra: `probe_success == 0` em probes TCP por 2m.
- Significado: indisponibilidade de rede dos serviços críticos.

7. `TelemetryIngestionFailureSpike` (warning)
- Regra: rejeições de ingestão relevantes > 1 req/s por 10m.
- Significado: backlog/falha de pipeline de ingestão.

## Canal de notificação
`alertmanager` roteia alertas `critical` para Telegram (on-call primário).

Defina no `.env`:
```bash
TELEGRAM_BOT_TOKEN=<bot_token>
TELEGRAM_CHAT_ID=<chat_id>
```

Fallback opcional (webhook secundário):
```bash
ALERT_FALLBACK_WEBHOOK_URL=https://seu-endpoint-webhook
```

Compatibilidade: `ALERT_WEBHOOK_URL` (legado) ainda é aceito como fallback.

Sem Telegram configurado, o alertmanager usa receiver sink local (sem envio externo).

Janela de manutenção padrão:
- diária, `03:00-03:30 UTC`, com mute no canal Telegram para alertas críticos.

## SLO/SLI e runbooks
- SLO/SLI por serviço: `docs/SLO_SLI.md`
- Runbooks de incidente: `docs/RUNBOOKS.md`

## Telegram Ops Bot
Comandos no chat permitido:
- `/health` (live/ready)
- `/status` (containers críticos)
- `/metrics` (resumo ingest/reject)
- `/logs api|emqx|postgres|timescale|redis`

Notificações automáticas:
- novo usuário cadastrado;
- novo dispositivo cadastrado.

## Dashboards Grafana
Provisionados automaticamente:
- Datasource: Prometheus (`uid=prometheus`)
- Dashboard: `IIoT Core Overview`
  - HTTP Throughput
  - HTTP 5xx Rate
  - Telemetry Ingest vs Reject
  - API Readiness Probe

Arquivos:
- `monitoring/grafana/provisioning/datasources/prometheus.yml`
- `monitoring/grafana/provisioning/dashboards/dashboards.yml`
- `monitoring/grafana/dashboards/iiot-core-overview.json`

## Arquivos de configuração
- Prometheus scrape config: `monitoring/prometheus/prometheus.yml`
- Regras de alerta: `monitoring/prometheus/alerts.yml`
- Blackbox config: `monitoring/blackbox/config.yml`
- Alertmanager bootstrap: `monitoring/alertmanager/entrypoint.sh`

## Testes rápidos
1. Status dos alvos no Prometheus:
```bash
curl -s http://localhost:9090/api/v1/targets | grep -E '"health":"up"|go_api|go_api_ready'
```

2. Ver alertas ativos:
```bash
curl -s http://localhost:9090/api/v1/alerts
```

3. Listar regras:
```bash
curl -s http://localhost:9090/api/v1/rules
```

4. Verificação de ingestão:
```bash
curl -s http://localhost:3001/metrics | grep -E "telemetry_ingested_total|telemetry_rejected_total"
```
