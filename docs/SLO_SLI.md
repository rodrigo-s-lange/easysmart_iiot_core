# SLO/SLI (SRE mínimo)

Este documento formaliza os objetivos iniciais de confiabilidade para operação B2B.

## Janela de medição
- Janela padrão: 30 dias.
- Ambientes: produção (`easysmart_iiot_core`).

## Serviço: Go API
### SLI 1 - Disponibilidade (readiness)
- Definição: proporção de sucesso do probe `/health/ready`.
- Métrica/Query:
```promql
avg_over_time(probe_success{job="go_api_ready"}[30d])
```
- SLO: `>= 99.9%`

### SLI 2 - Taxa de erro HTTP
- Definição: percentual de respostas HTTP 5xx.
- Métrica/Query:
```promql
sum(rate(http_requests_total{status=~"5.."}[5m]))
/
clamp_min(sum(rate(http_requests_total[5m])), 0.001)
```
- SLO: `< 1%` (rolling 5m, sustentado)

### SLI 3 - Latência p95 HTTP
- Definição: p95 de latência dos endpoints HTTP.
- Métrica/Query:
```promql
histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))
```
- SLO: `< 500ms`

## Serviço: Postgres
### SLI - Disponibilidade de rede
- Definição: sucesso do TCP probe.
- Query:
```promql
avg_over_time(probe_success{job="postgres_tcp"}[30d])
```
- SLO: `>= 99.95%`

## Serviço: TimescaleDB
### SLI - Disponibilidade de rede
- Definição: sucesso do TCP probe.
- Query:
```promql
avg_over_time(probe_success{job="timescaledb_tcp"}[30d])
```
- SLO: `>= 99.95%`

## Serviço: EMQX
### SLI - Disponibilidade de rede
- Definição: sucesso do TCP probe no broker MQTT.
- Query:
```promql
avg_over_time(probe_success{job="emqx_tcp"}[30d])
```
- SLO: `>= 99.95%`

## Serviço: Ingestão de telemetria
### SLI 1 - Taxa de ingestão bem-sucedida
- Definição: mensagens ingeridas por segundo.
- Query:
```promql
sum(rate(telemetry_ingested_total[5m]))
```

### SLI 2 - Taxa de rejeição relevante
- Definição: rejeições por erro real de pipeline (exclui tráfego inválido esperado de testes).
- Query:
```promql
sum(rate(telemetry_rejected_total{reason=~"db_error|tenant_mismatch|device_not_found|rate_limiter_unavailable"}[5m]))
```
- SLO: `< 1 req/s` sustentado.

## Alertas associados
Arquivo: `monitoring/prometheus/alerts.yml`
- `GoApiDown`
- `GoApiReadinessFailing`
- `GoApi5xxSpike`
- `GoApiHighLatencyP95`
- `GoApiHighErrorRate`
- `PostgresTcpDown`
- `TimescaleTcpDown`
- `EmqxTcpDown`
- `TelemetryIngestionFailureSpike`
