# EasySmart IIoT Core

Backend da plataforma IIoT: autenticação, provisionamento de devices, ingestão MQTT via webhook e persistência de telemetria.

## Stack
- EMQX 5.5 (broker MQTT)
- Go API (porta `3001`)
- PostgreSQL 16 (auth/devices)
- TimescaleDB 2.x (telemetria)
- Redis 7 (rate limit + cache latest)

## Serviços locais
- API: `http://localhost:3001`
- OpenAPI: `docs/openapi.yaml`
- Swagger UI: `http://localhost:8088` (serviço `swagger_ui`)
- EMQX Dashboard: `http://localhost:18083`
- Prometheus: `http://localhost:9090`
- Alertmanager: `http://localhost:9093`
- Grafana: `http://localhost:3002`

## Subir ambiente
```bash
docker-compose up -d
docker-compose ps
```

O serviço `emqx_bootstrap` reconcilia automaticamente (a cada 60s):
- API key de webhook em `api_keys` (Postgres);
- connector HTTP `api_webhook`;
- action HTTP `send_to_api`;
- rule `telemetry_ingest`.

Isso evita perda de ingestão MQTT após restart/reboot.

O serviço `telegram_ops_bot` oferece comandos operacionais no Telegram:
- `/health`
- `/status`
- `/metrics`
- `/logs api|emqx|postgres|timescale|redis`

Também envia notificações automáticas para:
- novo usuário cadastrado;
- novo dispositivo cadastrado.

Variável obrigatória no `.env`:
- `EMQX_WEBHOOK_API_KEY=<chave-longa-e-aleatoria>`

## Migrações (normalizado)
- Migrações ativas Postgres: `database/migrations` (`002` a `005`)
- Schema legado (não aplicar): `database/migrations/legacy/001_initial_schema.sql`
- Migrações Timescale incrementais: `database/timescale/migrations`

Comando único:
```bash
./database/migrate.sh --target all
```

Baseline (somente registrar como já aplicado, sem executar SQL):
```bash
./database/migrate.sh --target all --baseline
```

## Testes (item 1 reforçado)
Escopo atual coberto em `go test ./...`:
- auth: validação de email/senha, body inválido e refresh com token incorreto;
- middleware crítico: JWT, permissões, API key curta/inválida e comportamento seguro sem panic;
- telemetry: validações de leitura com contexto de tenant, parser de payload e utilitários;
- rate limit Redis: limites por dispositivo/segundo e por slot/minuto (com `miniredis`).

Arquivos de reforço adicionados:
- `go-api/handlers/auth_handler_test.go`
- `go-api/handlers/ratelimiter_test.go`
- `go-api/handlers/telemetry_utils_test.go`

Executar:
```bash
docker run --rm -v "$PWD/go-api":/src -w /src golang:1.22.4 sh -c "go test -p 1 ./..."
```

Observação: ainda faltam testes de integração completos para fechar 100% dos critérios do item 1 (especialmente RLS ponta a ponta e fluxos de `devices/provision|claim|secret` com banco real).

## Fluxo de provisionamento (atual)

### 1. Provisionamento direto (usuário autenticado)
`POST /api/v1/devices/provision`
- Requer JWT com `devices:provision`
- Cria device já associado ao tenant do usuário
- Retorna imediatamente:
  - `tenant_id`
  - `device_id`
  - `device_label` (username MQTT)
  - `device_secret` (password MQTT)
  - `broker`

### 2. Provisionamento por claim (legado, ainda suportado)
- `POST /api/v1/devices/claim`
- `POST /api/v1/devices/bootstrap`
- `POST /api/v1/devices/secret`

Observação: `POST /api/v1/devices/secret` é **one-time**. Se o secret expirar/não estiver mais no cache, não é reemitido automaticamente; é necessário reset + novo claim/provision.

## Publicação MQTT
Tópico esperado:
`tenants/<tenant_id>/devices/<device_id>/telemetry/slot/<n>`

Exemplo:
```bash
mosquitto_pub -h 192.168.0.99 -p 1883 \
  -u "<device_label>" \
  -P "<device_secret>" \
  -t "tenants/<tenant_id>/devices/<device_id>/telemetry/slot/0" \
  -m '{"value":23.5}'
```

## Segurança aplicada
- JWT obrigatório para endpoints de usuário
- `/api/v1/telemetry` protegido por API key
- `/api/v1/telemetry/latest` e `/api/v1/telemetry/slots` agora exigem JWT + `telemetry:read`
- Escopo por tenant aplicado nas consultas de leitura
- Endpoints sensíveis com método HTTP restrito (GET/POST explícitos, `405` para método inválido)
- Validação de tenant no tópico MQTT (tenant do tópico deve bater com tenant do device)
- Rate limit de auth resiliente a Redis indisponível (sem panic)
- Validação defensiva de API key curta (sem panic)
- Seletores de leitura de telemetria sem ambiguidade: aceitar **apenas um** entre `device_id` e `device_label`

## Endpoints principais
- Auth:
  - `POST /api/v1/auth/register`
  - `POST /api/v1/auth/login`
  - `POST /api/v1/auth/refresh`
- Devices:
  - `GET /api/v1/devices`
  - `POST /api/v1/devices/provision`
  - `POST /api/v1/devices/claim`
  - `POST /api/v1/devices/reset`
- Device bootstrap/secret:
  - `POST /api/v1/devices/bootstrap`
  - `POST /api/v1/devices/secret`
- Telemetry:
  - `POST /api/v1/telemetry`
  - `GET /api/v1/telemetry/latest`
  - `GET /api/v1/telemetry/slots`

Compatibilidade temporária:
- Prefixo legado `/api/*` ainda disponível durante transição para `/api/v1/*`.

## Observabilidade
- Health:
  - `GET /health`
  - `GET /health/live`
  - `GET /health/ready`
- Metrics:
  - `GET /metrics`
- Stack:
  - `prometheus` (coleta + regras de alerta)
  - `blackbox_exporter` (probe HTTP dos health endpoints)
  - `alertmanager` (roteamento de alertas por webhook)
  - `grafana` (dashboards provisionados)

Configuração detalhada:
- `docs/OBSERVABILITY.md`

## Banco e isolamento
- Postgres: dados de tenant/users/devices
- Timescale: telemetria com `tenant_id`
- Leituras de telemetria via API respeitam tenant do JWT

## Referências
- Histórico de mudanças: `CHANGELOG.md`
- Contrato REST: `docs/openapi.yaml`
- Roadmap técnico P0-P2: `docs/ROADMAP_P0_P2.md`
- Observabilidade (monitoramento/alertas): `docs/OBSERVABILITY.md`
- SLO/SLI por serviço: `docs/SLO_SLI.md`
- Runbooks operacionais: `docs/RUNBOOKS.md`
- Validação mínima para produção/auditoria: `docs/PRODUCTION_VALIDATION.md`
