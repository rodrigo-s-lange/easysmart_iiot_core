# IIoT Platform

Plataforma Industrial IoT com MQTT, PostgreSQL, Express API e Cloudflare Tunnel.

## Stack

- **PostgreSQL 16** (5432): Banco de dados particionado
- **Redis 7** (6379): Cache
- **EMQX 5.5.0** (1883, 8083, 8084, 18083): MQTT Broker
- **Express API** (3000): Webhooks de telemetria
- **Cloudflare Tunnel**: WSS via `mqtt.easysmart.com.br:443`

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

**API Health:**
```bash
curl http://localhost:3000/health
```

## Configuração do EMQX

### Rule Engine (via Dashboard)

**IMPORTANTE:** Rule Engine não persiste em `emqx.conf`. Configure via Dashboard:

1. Acesse: http://192.168.0.99:18083
2. Data Integration → Connectors → Create
   - Type: `HTTP Server`
   - Name: `api_webhook`
   - URL: `http://iiot_nextjs:3000`
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

## Troubleshooting

### Rule Engine Parou de Funcionar

1. Criar backup: `./backup_emqx.sh`
2. Verificar connector: Dashboard → Connectors → Status
3. Verificar rule: Dashboard → Rules → Metrics
4. Ver logs: `docker logs iiot_emqx --tail 50`
5. Restart: `docker-compose restart emqx`

### API Não Recebe Dados

```bash
# Ver logs da API
docker logs iiot_nextjs --tail 50

# Testar endpoint direto
curl -X POST http://localhost:3000/api/telemetry \
  -H "Content-Type: application/json" \
  -d '{"clientid":"test","topic":"devices/TOKEN/telemetry/slot/99","payload":{"value":1},"timestamp":"'$(date +%s)000'"}'

# Verificar banco
docker exec -it iiot_postgres psql -U admin -d iiot_platform -c \
  "SELECT * FROM telemetry WHERE slot=99;"
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
├── app/                    # Express API
│   ├── index.js
│   ├── package.json
│   └── Dockerfile
├── database/
│   ├── init/              # Schema inicial
│   └── maintenance/       # Scripts de manutenção
├── emqx/
│   ├── etc/
│   │   └── emqx.conf     # Config declarativa
│   └── certs/            # Certificados SSL
├── backups/              # Backups do EMQX
├── docker-compose.yml
├── backup_emqx.sh       # Backup manual
├── restore_emqx.sh      # Restore manual
└── README.md
```

## Segurança

- ✅ Autenticação MQTT via PostgreSQL
- ✅ SSL/TLS (Let's Encrypt)
- ✅ Cloudflare Tunnel (não expõe IP)
- ✅ Passwords em variáveis de ambiente
- ⚠️ Authorization: `allow all` (TODO: implementar ACLs)

## TODO

- [ ] ACLs por device/tenant
- [ ] Frontend dashboard (Next.js)
- [ ] Comandos bidirecionais (MQTT publish)
- [ ] Multi-tenancy
- [ ] Sparkplug B
- [ ] Grafana dashboards

## Logs Importantes

```bash
# EMQX
docker logs iiot_emqx --tail 100

# API
docker logs iiot_nextjs --tail 100

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

