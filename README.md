# IIoT Platform

Plataforma Industrial IoT focada em ingestão MQTT, persistência de telemetria e acesso em tempo real via WSS.

**Objetivo do projeto**
- Receber telemetria MQTT de devices.
- Entregar dados em tempo real (frontend via MQTT/WSS).
- Persistir histórico com timeseries (TimescaleDB).
- Manter autenticação/ACL seguras no EMQX.

## Arquitetura (visão rápida)

**Fluxo realtime (front)**
1. Device publica em `devices/<token>/telemetry/slot/<n>`.
2. EMQX distribui para subscribers (frontend via WSS).

**Fluxo de persistência**
1. EMQX Rule Engine envia webhook HTTP.
2. Go API valida e aplica rate limit (Redis).
3. Go API grava no TimescaleDB.
4. Go API atualiza cache do último valor no Redis.

**Importante:** O realtime é MQTT direto. O Go API só controla persistência.

## Stack e responsabilidades

- **EMQX 5.5.0** (1883, 8083, 8084, 18083): broker MQTT + Rule Engine.
- **PostgreSQL 16** (5432): auth, usuários, devices, comandos.
- **TimescaleDB 2.x** (5433): telemetria (time-series).
- **Redis 7** (6379): rate limit + cache do último valor.
- **Go API** (3001): webhook de ingestão e endpoints auxiliares.
- **Cloudflare Tunnel**: WSS externo (`mqtt.easysmart.com.br:443`).

## Serviços e portas

- EMQX Dashboard: `http://192.168.0.99:18083`
- MQTT TCP: `192.168.0.99:1883`
- MQTT WS/WSS: `192.168.0.99:8083` / `8084`
- Go API: `http://localhost:3001`
- PostgreSQL: `localhost:5432`
- TimescaleDB: `localhost:5433`
- Redis: `localhost:6379`

## Credenciais e ambiente

Arquivo `.env` (gitignored) contém:
- `POSTGRES_*`
- `TIMESCALE_*`
- `REDIS_PASSWORD`
- `EMQX_DASHBOARD_*`
- Rate limit e cache:
  - `RATE_LIMIT_DEVICE_PER_MIN=12`
  - `RATE_LIMIT_DEVICE_PER_SEC=5`
  - `RATE_LIMIT_SLOT_PER_MIN=12`
  - `RATE_LIMIT_FAIL_OPEN=true`
  - `CACHE_TTL_SECONDS=0` (0 = sem expiração)

## Quick start

```bash
# subir tudo
docker-compose up -d

# status
docker-compose ps

# logs
docker-compose logs -f
```

## MQTT

**Local (TCP)**
```bash
mosquitto_pub -h 192.168.0.99 -p 1883 \
  -u "TOKEN" -P "TOKEN" \
  -t "devices/TOKEN/telemetry/slot/0" \
  -m '{"value":25.5}'
```

**Internet (WSS)**
- Host: `mqtt.easysmart.com.br`
- Port: `443`
- Path: `/mqtt`
- Protocol: `mqtt`
- SSL: sim
- Username/Password: `device token`

## Go API

**Health**
```bash
curl http://localhost:3001/health
```

**Webhook (EMQX Rule Engine)**
`POST /api/telemetry`

**Cache do último valor**
`GET /api/telemetry/latest?token=TOKEN&slot=0`
- Se não houver cache: retorna `200` com `{}`.

## EMQX (auth + rule engine)

### Auth/ACL (emqx.conf)
- Autenticação via PostgreSQL.
- ACL por device: `devices/<token>/#`.
- `no_match = deny`.

Arquivo: `emqx/etc/emqx.conf`

### Rule Engine (configurar via Dashboard)
**Nota:** Rules não persistem no `emqx.conf`.

1. Dashboard → Data Integration → Connectors → Create
   - Type: `HTTP Server`
   - Name: `api_webhook`
   - URL: `http://iiot_go_api:3001`
   - Pool Size: `8`

2. Dashboard → Data Integration → Rules → Create
   - ID: `telemetry_to_api`
   - SQL:
     ```sql
     SELECT payload, clientid, topic, timestamp
     FROM "devices/+/telemetry/slot/+"
     ```

3. Action → HTTP Server
   - Connector: `api_webhook`
   - Method: `POST`
   - Path: `/api/telemetry`
   - Headers: `content-type: application/json`
   - Body:
     ```json
     {"clientid":"${clientid}","topic":"${topic}","payload":${payload},"timestamp":"${timestamp}"}
     ```

## Persistência

- Telemetria: TimescaleDB (`iiot_telemetry`)
- Auth/devices: PostgreSQL (`iiot_platform`)

### Retenção TimescaleDB
- **365 dias** por policy.

Para alterar:
```sql
SELECT remove_retention_policy('telemetry');
SELECT add_retention_policy('telemetry', INTERVAL '365 days');
```

## Rate limit

- Por device: 12 msg/min e 5 msg/s
- Por slot: 12 msg/min

Logs de bloqueio (Go API):
`rate_limit_exceeded device=<token> slot=<n>`

## Cache do último valor

- Mantém o último valor de cada device/slot no Redis.
- Serve para telas que precisam mostrar estado atual sem esperar publish.

## Operações comuns

**Reset mantendo device de teste**
```bash
docker exec -i iiot_postgres psql -U admin -d iiot_platform < database/maintenance/reset_keep_device.sql
```

**Logs**
```bash
# EMQX
docker logs iiot_emqx --tail 100

# Go API
docker logs iiot_go_api --tail 100

# PostgreSQL
docker logs iiot_postgres --tail 100

# TimescaleDB
docker logs iiot_timescaledb --tail 100
```

**Cloudflare Tunnel**
```bash
sudo systemctl status cloudflared
sudo journalctl -u cloudflared -f
```

## Troubleshooting

**API não recebe dados**
```bash
curl -X POST http://localhost:3001/api/telemetry \
  -H "Content-Type: application/json" \
  -d '{"clientid":"test","topic":"devices/TOKEN/telemetry/slot/99","payload":{"value":1},"timestamp":"'$(date +%s)000'"}'

docker exec -it iiot_timescaledb psql -U admin -d iiot_telemetry -c \
  "SELECT * FROM telemetry WHERE slot=99 ORDER BY timestamp DESC LIMIT 5;"
```

**WSS não conecta**
```bash
sudo systemctl status cloudflared
nslookup mqtt.easysmart.com.br

docker exec iiot_emqx emqx ctl listeners | grep wss
curl -I http://localhost:8083
```

## Estrutura do repositório

```
iiot_platform/
├── go-api/                 # Go API (ingestão + cache)
├── database/
│   ├── init/              # Schema inicial (Postgres)
│   ├── timescale/         # Init TimescaleDB
│   └── maintenance/       # Scripts de manutenção
├── emqx/
│   ├── etc/emqx.conf       # Config declarativa
│   └── certs/              # Certificados SSL
├── backups/                # Backups do EMQX
├── docker-compose.yml
├── backup_emqx.sh
├── restore_emqx.sh
├── CHANGELOG.md
└── README.md
```

## Estado atual (pontos críticos)

- Auth/ACL no EMQX via PostgreSQL com `${username}` nas queries.
- Realtime via MQTT/WSS direto do EMQX.
- Persistência via Go API → TimescaleDB.
- Rate limit e cache no Redis.
- Log rotation habilitado no Docker (EMQX/Go API).

## Próximos passos sugeridos

- Frontend dashboard (Next.js).
- Multi-tenant (ACLs por tenant).
- Comandos bidirecionais (MQTT publish).
- Grafana dashboards.

## Histórico

Veja `CHANGELOG.md`.
