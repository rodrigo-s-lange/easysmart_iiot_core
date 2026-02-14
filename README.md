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

## Testes (suite básica - P0)
Escopo inicial coberto em `go test ./...`:
- validação de auth (email/senha e JWT);
- middleware crítico (JWT, permissões, rate-limit auth nil-safe, API key curta);
- fluxo de devices (HMAC/timestamp e guardas de erro);
- parser e guardas de telemetry (topic/timestamp e endpoints com contexto de tenant).

Executar:
```bash
docker run --rm -v "$PWD/go-api":/src -w /src golang:1.22.4 sh -c "go test ./..."
```

## Fluxo de provisionamento (atual)

### 1. Provisionamento direto (usuário autenticado)
`POST /api/devices/provision`
- Requer JWT com `devices:provision`
- Cria device já associado ao tenant do usuário
- Retorna imediatamente:
  - `tenant_id`
  - `device_id`
  - `device_label` (username MQTT)
  - `device_secret` (password MQTT)
  - `broker`

### 2. Provisionamento por claim (legado, ainda suportado)
- `POST /api/devices/claim`
- `POST /api/devices/bootstrap`
- `POST /api/devices/secret`

Observação: `POST /api/devices/secret` é **one-time**. Se o secret expirar/não estiver mais no cache, não é reemitido automaticamente; é necessário reset + novo claim/provision.

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
- `/api/telemetry` protegido por API key
- `/api/telemetry/latest` e `/api/telemetry/slots` agora exigem JWT + `telemetry:read`
- Escopo por tenant aplicado nas consultas de leitura
- Validação de tenant no tópico MQTT (tenant do tópico deve bater com tenant do device)
- Rate limit de auth resiliente a Redis indisponível (sem panic)
- Validação defensiva de API key curta (sem panic)

## Endpoints principais
- Auth:
  - `POST /api/auth/register`
  - `POST /api/auth/login`
  - `POST /api/auth/refresh`
- Devices:
  - `GET /api/devices`
  - `POST /api/devices/provision`
  - `POST /api/devices/claim`
  - `POST /api/devices/reset`
- Device bootstrap/secret:
  - `POST /api/devices/bootstrap`
  - `POST /api/devices/secret`
- Telemetry:
  - `POST /api/telemetry`
  - `GET /api/telemetry/latest`
  - `GET /api/telemetry/slots`

## Observabilidade
- Health:
  - `GET /health`
  - `GET /health/live`
  - `GET /health/ready`
- Metrics:
  - `GET /metrics`

## Banco e isolamento
- Postgres: dados de tenant/users/devices
- Timescale: telemetria com `tenant_id`
- Leituras de telemetria via API respeitam tenant do JWT

## Referências
- Histórico de mudanças: `CHANGELOG.md`
- Contrato REST: `docs/openapi.yaml`
- Roadmap técnico P0-P2: `docs/ROADMAP_P0_P2.md`
