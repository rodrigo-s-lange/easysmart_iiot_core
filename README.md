# IIoT Platform

Plataforma Industrial IoT focada em ingestão MQTT, persistência de telemetria e acesso em tempo real via WSS.

**Objetivo do projeto**
- Receber telemetria MQTT de devices.
- Entregar dados em tempo real (frontend via MQTT/WSS).
- Persistir histórico com timeseries (TimescaleDB).
- Manter autenticação/ACL seguras no EMQX.

## Arquitetura (visão rápida)

**Fluxo realtime (front)**
1. Device publica em `tenants/<tenant_id>/devices/<device_id>/telemetry/slot/<n>`.
2. EMQX distribui para subscribers (frontend via WSS).
3. Autenticação MQTT é por `device_label` + `device_secret` (ver seção MQTT).

**Fluxo de persistência**
1. EMQX Rule Engine envia webhook HTTP.
2. Go API valida e aplica rate limit (Redis).
3. Go API grava no TimescaleDB.
4. Go API atualiza cache do último valor no Redis.

**Importante:** O realtime é MQTT direto. O Go API só controla persistência.

## Stack e responsabilidades

- **EMQX 5.5.0** (1883, 8083, 8084, 18083): broker MQTT + Rule Engine.
- **PostgreSQL 16** (5432): auth, usuários e devices **com RLS por tenant**. Tabelas legadas foram renomeadas para `users_legacy` e `devices_legacy`.
- **TimescaleDB 2.x** (5433): telemetria (time-series).
- **Redis 7** (6379): rate limit + cache do último valor.
- **Go API** (3001): webhook de ingestão e endpoints auxiliares.
- **Cloudflare Tunnel**: WSS externo (`mqtt.easysmart.com.br:443`).
- **Frontend**: **Next.js + Tailwind + shadcn/ui** (dashboards por usuário/tenant).

## Frontend (decisão)

**Stack definida para o dashboard:**
- **Next.js** (rotas protegidas, SSR/CSR híbrido, auth por tenant).
- **Tailwind CSS** (UI rápida e consistente).
- **shadcn/ui** (componentes prontos e customizáveis).

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
  -u "DEVICE_LABEL" -P "DEVICE_SECRET" \
  -t "tenants/TENANT_ID/devices/DEVICE_ID/telemetry/slot/0" \
  -m '{"value":25.5}'
```

**Internet (WSS)**
- Host: `mqtt.easysmart.com.br`
- Port: `443`
- Path: `/mqtt`
- Protocol: `mqtt`
- SSL: sim
- Username: `device_label`
- Password: `device_secret`
- Topic: `tenants/<tenant_id>/devices/<device_id>/telemetry/slot/<n>`

**Subscribe (monitoramento)**
```bash
mosquitto_sub -h 192.168.0.99 -p 1883 \
  -u "DEVICE_LABEL" -P "DEVICE_SECRET" \
  -t "tenants/TENANT_ID/devices/DEVICE_ID/telemetry/#" -v
```

## Go API

**Health**
```bash
curl http://localhost:3001/health
curl http://localhost:3001/health/live
curl http://localhost:3001/health/ready
```

**Metrics (Prometheus)**
```bash
curl http://localhost:3001/metrics
```

**CORS (confirmado)**
- Implementado via middleware (`go-api/middleware/cors.go`).
- **Só ativa se** `CORS_ALLOWED_ORIGINS` estiver preenchido no `.env`.
- Se estiver vazio, a API **não** adiciona headers CORS (comportamento intencional).

Exemplo:
```
CORS_ALLOWED_ORIGINS=http://localhost:3000,https://app.example.com
CORS_ALLOWED_METHODS=GET,POST,PUT,DELETE,OPTIONS
CORS_ALLOWED_HEADERS=Authorization,Content-Type
```

**Webhook (EMQX Rule Engine)**
`POST /api/telemetry`

**Cache do último valor**
`GET /api/telemetry/latest?token=DEVICE_ID&slot=0`
- Se não houver cache: retorna `200` com `{}`.
**Nota:** o parâmetro `token` atualmente é o `device_id` (legado; renomear depois).

## EMQX (auth + rule engine)

### Auth/ACL (emqx.conf)
- Autenticação via PostgreSQL (views `emqx_auth_v2` e `emqx_acl_v2`).
- Username: `device_label`.
- ACL por device/tenant:
  - Publish: `tenants/<tenant_id>/devices/<device_id>/telemetry/#`
  - Publish: `tenants/<tenant_id>/devices/<device_id>/events/#`
  - Subscribe: `tenants/<tenant_id>/devices/<device_id>/commands/#`
  - Publish: `tenants/<tenant_id>/devices/<device_id>/status`
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
     FROM "tenants/+/devices/+/telemetry/slot/+"
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

## Provisionamento (implementado - Opção A)

**Objetivo:** devices saem de fábrica com `device_id` público e prova de autenticidade via HMAC. Eles sobem na rede como **unclaimed** e só passam a publicar após o usuário **reclamar** o device com um **claim_code** físico.

**Fluxo implementado (HMAC + claim_code):**
1. **Bootstrap (device)**  
   `POST /api/devices/bootstrap`  
   Body:
   ```json
   {
     "device_id": "UUID",
     "timestamp": "2026-02-09T06:03:22Z",
     "signature": "HMAC_SHA256(MANUFACTURING_MASTER_KEY, device_id:timestamp)"
   }
   ```
   Retorna `status` e `poll_interval`.

2. **Claim (usuário logado)**  
   `POST /api/devices/claim`  
   Body:
   ```json
   {
     "device_id": "UUID",
     "claim_code": "CODE-IMPRESSO"
   }
   ```
   Backend valida `claim_code_hash`, gera `device_secret` e grava apenas `secret_hash` (bcrypt). Status → `claimed`.

3. **Entrega do secret (device)**  
   `POST /api/devices/secret` com HMAC + timestamp.  
   Retorna `device_secret` (uma vez). Status permanece `claimed`.

4. **Primeira conexão MQTT**  
   Ao publicar telemetry, o backend marca status `active`.  
   - MQTT `username = device_label`  
   - MQTT `password = device_secret`  
   - Topic usa `tenant_id` + `device_id`

## Próximas implementações (curto prazo)

1. **Segurança operacional**
   - Rotação de `JWT_SECRET` e `MANUFACTURING_MASTER_KEY`.
   - Backups automáticos + teste de restore.
   - Alertas para falhas de bridge/DB.

2. **Provisionamento: testes E2E + reset**
   - Testes end-to-end do fluxo completo.
   - Endpoint de reset do device com auditoria.

**Segurança:**
- `MANUFACTURING_MASTER_KEY` fica **apenas** no `.env` (não vai para o banco).
- `claim_code` **nunca** é salvo em texto; só `claim_code_hash`.
- `device_secret` nunca é salvo em texto no banco.
- Assinaturas têm janela de tempo (default 5 min).

**Observação importante (IDs):**
- `device_id` é o identificador do device usado nos tópicos MQTT.
- `device_label` é o **username** MQTT (credencial pública/token).
- O device precisa ter **ambos** gravados (ex.: durante fabricação).

**Reset do device (usuário autorizado):**
`POST /api/devices/reset` com `confirmation: "RESET"`  
Reseta para `unclaimed` e limpa secrets/tenant/owner.

**Migração para devices existentes:**
- `claim_code_hash` é preenchido com `device_label` (temporário).  
  Troque por claim_code real em produção.

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
  -d '{"clientid":"test","topic":"tenants/TENANT_ID/devices/DEVICE_ID/telemetry/slot/99","payload":{"value":1},"timestamp":"'$(date +%s)000'"}'

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

- Frontend dashboard (Next.js + Tailwind + shadcn/ui).
- Refinar multi-tenant (permissões e auditoria).
- Comandos bidirecionais (MQTT publish).
- Grafana dashboards.

## Histórico

Veja `CHANGELOG.md`.
