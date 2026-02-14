# Observability

Este documento cobre monitoramento e alertas do `easysmart_iiot_core` fora do contrato de API.

## Componentes
- `prometheus`: coleta métricas e avalia regras de alerta.
- `blackbox_exporter`: probe HTTP para `/health/live` e `/health/ready`.
- `alertmanager`: roteia alertas para webhook.
- `grafana`: dashboards.

## Endpoints
- Prometheus: `http://localhost:9090`
- Alertmanager: `http://localhost:9093`
- Grafana: `http://localhost:3002` (default: `admin/admin` se não configurado)

## Subir stack
```bash
docker compose up -d prometheus blackbox_exporter alertmanager grafana
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

## Canal de notificação
`alertmanager` usa webhook.

Defina no `.env`:
```bash
ALERT_WEBHOOK_URL=https://seu-endpoint-webhook
```

Exemplos de destino:
- Slack Incoming Webhook
- Discord Webhook
- NTFY / serviço interno
- Endpoint próprio para rotear para Telegram/Email

Se `ALERT_WEBHOOK_URL` estiver vazio, o alertmanager mantém receiver sink local (sem envio externo).

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
