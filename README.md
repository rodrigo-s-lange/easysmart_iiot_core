# IIoT Platform

Plataforma Industrial IoT com MQTT, PostgreSQL, TimescaleDB e Cloudflare Tunnel.

## Stack

- **PostgreSQL 16** (5432): Auth, usuários, devices, comandos
- **TimescaleDB 2.x** (5433): Telemetria (time-series)
- **Redis 7** (6379): Cache
- **EMQX 5.5.0** (1883, 8083, 8084, 18083): MQTT Broker
- **Go API** (3001): Ingestão principal (TimescaleDB)
- **Cloudflare Tunnel**: WSS via `mqtt.easysmart.com.br:443`

## Quebra Temporal (MVP vs Próximas Mudanças)

**MVP Funcionando Hoje**
- Ingestão MQTT com EMQX e autenticação via PostgreSQL
- Telemetria persistida em TimescaleDB
- Go API recebendo webhook do Rule Engine
- WSS via Cloudflare Tunnel
- Backup/restore do EMQX

**Vamos Começar a Mexer (Próxima Fase)**
- Fortalecer auth (hash forte + tokens separados das credenciais)
- ACLs por device/tenant no EMQX
- Autenticação e rate limit na API
- Segredos fora do Git (env/secrets)
- Melhorias de escala: batch inserts + fila
- Retenção/compressão no TimescaleDB

## Quick Start

```bash
# Subir stack
docker-compose up -d

# Verificar status
docker-compose ps

# Ver logs
docker-compose logs -f
```

## Acessos

**EMQX Dashboard:**
- Local: http://192.168.0.99:18083
- Login: `admin` / `admin0039`

**MQTT Local:**
```bash
mosquitto_pub -h 192.168.0.99 -p 1883 \
  -u "TOKEN" -P "TOKEN" \
  -t "devices/TOKEN/telemetry/slot/0" \
  -m '{"value":25.5}'
```

**MQTT via Internet (WSS):**
- Host: `mqtt.easysmart.com.br`
- Port: `443`
- Path: `/mqtt`
- SSL: ✓
- Username/Password: device token

**PostgreSQL:**
```bash
docker exec -it iiot_postgres psql -U admin -d iiot_platform
```

**TimescaleDB (telemetria):**
```bash
docker exec -it iiot_timescaledb psql -U admin -d iiot_telemetry
```

**API Health:**
```bash
curl http://localhost:3001/health
```

## Configuração do EMQX

### Rule Engine (via Dashboard)

**IMPORTANTE:** Rule Engine não persiste em `emqx.conf`. Configure via Dashboard:

1. Acesse: http://192.168.0.99:18083
2. Data Integration → Connectors → Create
   - Type: `HTTP Server`
   - Name: `api_webhook`
   - URL: `http://iiot_go_api:3001`
  - Pool Size: `8`

3. Data Integration → Rules → Create
   - ID: `telemetry_to_api`
   - SQL:
     ```sql
     SELECT payload, clientid, topic, timestamp
     FROM "devices/+/telemetry/slot/+"
     ```

4. Add Action → HTTP Server
   - Connector: `api_webhook`
   - Method: `POST`
   - Path: `/api/telemetry`
   - Headers: `content-type: application/json`
   - Body:
     ```json
     {"clientid":"${clientid}","topic":"${topic}","payload":${payload},"timestamp":"${timestamp}"}
     ```

### Backup e Restore

**CRÍTICO:** Faça backup antes de `docker-compose down` ou `docker volume rm`!

**Criar backup:**
```bash
./backup_emqx.sh
```

**Restaurar último backup:**
```bash
./restore_emqx.sh
docker-compose restart emqx
```

**Backups ficam em:** `backups/emqx_config_YYYYMMDD_HHMMSS.tar.gz`

## SSL/TLS (Let's Encrypt)

**Certificados:**
- Domínio: `mqtt.easysmart.com.br`
- Localização: `/etc/letsencrypt/live/mqtt.easysmart.com.br/`
- Renovação: Automática (certbot timer)

**Verificar renovação:**
```bash
sudo systemctl status certbot.timer
```

## Cloudflare Tunnel

**Status:**
```bash
sudo systemctl status cloudflared
```

**Logs:**
```bash
sudo journalctl -u cloudflared -f
```

**Reiniciar:**
```bash
sudo systemctl restart cloudflared
```

**Config:** `~/.cloudflared/config.yml`

## Manutenção Automática

### Partições PostgreSQL

- **Criação**: Automática todo dia 1º às 02:00
- **Mantém**: 3 meses futuros
- **Log**: `/var/log/iiot_partition_maintenance.log`

**Executar manualmente:**
```bash
./database/maintenance/run_partition_maintenance.sh
```

**Ver partições:**
```bash
docker exec -it iiot_postgres psql -U admin -d iiot_platform -c \
  "SELECT tablename FROM pg_tables WHERE tablename LIKE 'telemetry_%' ORDER BY tablename;"
```

### Retenção TimescaleDB

- **Retenção atual**: 90 dias (telemetria)

Para alterar:
```sql
SELECT remove_retention_policy('telemetry');
SELECT add_retention_policy('telemetry', INTERVAL '180 days');
```

### Reset de Dados (mantendo device de teste)

```bash
docker exec -i iiot_postgres psql -U admin -d iiot_platform < database/maintenance/reset_keep_device.sql
```

## Troubleshooting

### Rule Engine Parou de Funcionar

1. Criar backup: `./backup_emqx.sh`
2. Verificar connector: Dashboard → Connectors → Status
3. Verificar rule: Dashboard → Rules → Metrics
4. Ver logs: `docker logs iiot_emqx --tail 50`
5. Restart: `docker-compose restart emqx`

### API Não Recebe Dados

```bash
# Testar endpoint direto
curl -X POST http://localhost:3001/api/telemetry \
  -H "Content-Type: application/json" \
  -d '{"clientid":"test","topic":"devices/TOKEN/telemetry/slot/99","payload":{"value":1},"timestamp":"'$(date +%s)000'"}'

# Verificar telemetria (TimescaleDB)
docker exec -it iiot_timescaledb psql -U admin -d iiot_telemetry -c \
  "SELECT * FROM telemetry WHERE slot=99 ORDER BY timestamp DESC LIMIT 5;"
```

### WSS Não Conecta

```bash
# Verificar Cloudflare Tunnel
sudo systemctl status cloudflared

# Verificar DNS
nslookup mqtt.easysmart.com.br

# Verificar listener WSS EMQX
docker exec iiot_emqx emqx ctl listeners | grep wss

# Testar local
curl -I http://localhost:8083
```

## Estrutura

```
iiot_platform/
├── go-api/                 # Go API (opcional)
│   ├── main.go
│   ├── go.mod
│   └── Dockerfile
├── database/
│   ├── init/              # Schema inicial
│   ├── timescale/         # Init do TimescaleDB
│   └── maintenance/       # Scripts de manutenção
├── emqx/
│   ├── etc/
│   │   └── emqx.conf     # Config declarativa
│   └── certs/            # Certificados SSL
├── backups/              # Backups do EMQX
├── docker-compose.yml
├── backup_emqx.sh       # Backup manual
├── restore_emqx.sh      # Restore manual
├── CHANGELOG.md         # Histórico de mudanças
└── README.md
```

## Segurança

- ✅ Autenticação MQTT via PostgreSQL
- ✅ SSL/TLS (Let's Encrypt)
- ✅ Cloudflare Tunnel (não expõe IP)
- ✅ Secrets em `.env` (gitignored)
- ✅ ACLs por device (EMQX Authorization via PostgreSQL)

## TODO

- [ ] ACLs por tenant (expansão)
- [ ] Frontend dashboard (Next.js)
- [ ] Comandos bidirecionais (MQTT publish)
- [ ] Multi-tenancy
- [ ] Sparkplug B
- [ ] Grafana dashboards

## Estado Atual (Importante para Continuidade)

**Resumo do que foi implementado**
- ACLs por device no EMQX via PostgreSQL: cada device só publica/assina `devices/<token>/#`
- Autenticação MQTT via PostgreSQL (token como username/password)
- `no_match = deny` em authorization
- Logs do Docker com rotação (`max-size=10m`, `max-file=5`) no EMQX e API
- `docker-compose.yml` usa `.env` via `env_file`

**Arquivos-chave**
- EMQX auth/ACL: `emqx/etc/emqx.conf`
- Compose/log-rotation/env: `docker-compose.yml`
- Secrets: `.env` (gitignored)

**Comportamento esperado**
- `mosquitto_sub` fica bloqueado aguardando mensagens (normal). Saída com `Ctrl+C`.
- WSS via Cloudflare está OK (handshake HTTP 101 com `Sec-WebSocket-Protocol: mqtt`).

**Nota sobre EMQX + PostgreSQL**
- As queries em `emqx.conf` usam `${username}` dentro do SQL, sem aspas.
- Esse formato está funcionando no ambiente atual. Alterações aqui podem quebrar auth.

**Comandos de validação**
```bash
# Teste MQTT local
mosquitto_sub -h 192.168.0.99 -p 1883 \
  -u "TOKEN" -P "TOKEN" \
  -t "devices/TOKEN/#" -v

# Ver logs recentes
docker logs iiot_emqx --since 5m

# Reiniciar EMQX
docker-compose restart emqx
```

## Logs Importantes

```bash
# EMQX
docker logs iiot_emqx --tail 100

# API
docker logs iiot_go_api --tail 100

# PostgreSQL
docker logs iiot_postgres --tail 100

# Cloudflare Tunnel
sudo journalctl -u cloudflared -n 100
```

## Performance

**Testado:**
- ✅ Restart completo (configs persistem)
- ✅ Autenticação negativa (rejeita senhas erradas)
- ✅ Multi-slot (string, number, object)
- ✅ WSS via Internet
- ✅ Webhook end-to-end (MQTT → API → PostgreSQL)

**Capacidade:**
- EMQX: 1M+ conexões simultâneas
- PostgreSQL: Particionamento automático
- API: Pool de 20 conexões
