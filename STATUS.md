# IIoT Platform - Production Multi-Tenant Architecture

## ✅ Status: Sistema Base Funcional (Auth + Provisionamento + Telemetria)

**Última Atualização:** 2026-02-10  
**Progresso MVP:** Fases A e B concluídas; faltam Device CRUD, testes e frontend

**Nota:** Este documento mistura estado atual e arquitetura-alvo.  
Seção "Fase A" indica o que está **implementado** vs **pendente**.

---

## 🎯 Missão Atual: Backend Profissional

### Fase A - Gaps Críticos (STATUS)
Corrigindo falhas que impedem produção profissional:
- [x] Register endpoint
- [x] Login endpoint
- [x] Refresh token endpoint (token_type + JTI blacklist)
- [x] JWT secret enforcement (fail startup if default/empty)
- [x] Rate limit auth (Redis, 10/min por IP)
- [x] CORS middleware (configurável por env)
- [x] Input validation (validator v10)
- [x] Error handling básico (request_id + panic recovery)
- [x] Graceful shutdown
- [x] **Isolamento no TimescaleDB**: tabela `telemetry` tem `tenant_id` e RLS por tenant.

### Observabilidade (STATUS)
- [x] Request ID (X-Request-ID)
- [x] Logs estruturados (slog)
- [x] Health checks (/health/live, /health/ready)

### Isolamento de Telemetria (STATUS)
- [x] TimescaleDB com `tenant_id` na tabela `telemetry`
- [x] RLS no TimescaleDB por `app.current_tenant_id`

**Tempo estimado:** 6 horas  
**Prioridade:** P0 (Crítico)

---

## 🏗️ Arquitetura Atual

### Stack Tecnológico
```
Frontend:   Next.js (planejado)
API:        Go 1.21+ (net/http)
Auth:       JWT (access + refresh tokens)
Database:   PostgreSQL 16 (auth/devices)
Telemetry:  TimescaleDB 2.14.2 (365 dias)
Cache:      Redis 7 (rate limit + sessions)
MQTT:       EMQX 5.5.0 (bcrypt auth)
Tunnel:     Cloudflare (WSS público)
```

### Topologia Multi-Tenant
```
┌─────────────────────────────────────────────────────────┐
│                    Cloudflare Tunnel                     │
│              mqtt.easysmart.com.br (WSS)                │
└──────────────────────┬──────────────────────────────────┘
                       │
                       ↓
┌─────────────────────────────────────────────────────────┐
│                     EMQX 5.5.0                          │
│  • Bcrypt Auth (PostgreSQL)                             │
│  • ACL Multi-Tenant (tenant_id scoped)                  │
│  • Rule Engine → Go API Webhook                         │
└──────────────────────┬──────────────────────────────────┘
                       │
                       ↓
┌─────────────────────────────────────────────────────────┐
│                    Go API (Port 3001)                   │
│  • JWT Middleware (users)                               │
│  • API Key Middleware (webhooks)                        │
│  • Tenant Context (RLS)                                 │
│  • Rate Limiting (Redis)                                │
└─────┬──────────────────────┬─────────────────────┬──────┘
      │                      │                     │
      ↓                      ↓                     ↓
┌──────────────┐   ┌──────────────────┐   ┌──────────────┐
│ PostgreSQL   │   │  TimescaleDB     │   │    Redis     │
│ (Auth/Meta)  │   │  (Telemetry)     │   │  (Cache/RL)  │
└──────────────┘   └──────────────────┘   └──────────────┘
```

---

## 🗄️ Database Schema (Production)

### Core Tables

**tenants** (Multi-tenancy root)
- tenant_id (UUID PK)
- name, slug (unique)
- status (active/suspended/deleted)
- quota_devices, quota_messages_per_hour
- created_at, updated_at

**users** (User management + RBAC)
- user_id (UUID PK)
- tenant_id (FK, NULL for super_admin)
- email (unique), password_hash (bcrypt)
- role (super_admin/tenant_admin/tenant_user)
- status (active/suspended/deleted)
- last_login_at

**devices** (Device lifecycle)
- device_id (UUID PK)
- tenant_id (FK)
- owner_user_id (FK)
- device_label (unique, public identifier)
- secret_hash (bcrypt, NULL when unclaimed)
- status (unclaimed/claimed/active/suspended/revoked)
- claimed_at, activated_at, last_seen_at
- firmware_version, hardware_revision
- metadata (JSONB)

**Legado**
- `users_legacy` e `devices_legacy` preservam o schema antigo (não usados pela API).

**permissions** (RBAC permissions)
- permission_id (UUID PK)
- name (unique, e.g., "devices:read")
- description
- 13 permissions seeded

**role_permissions** (Role → Permission mapping)
- role (super_admin/tenant_admin/tenant_user)
- permission_id (FK)

**audit_log** (Compliance-ready)
- audit_id (UUID PK)
- tenant_id (FK, NULL for system events)
- user_id (FK)
- action (string)
- resource_type, resource_id
- error_code, error_message
- request_path, response_status, duration_ms
- metadata (JSONB)
- timestamp (indexed)

**api_keys** (Service authentication)
- key_id (UUID PK)
- tenant_id (FK)
- user_id (FK)
- name
- key_hash (bcrypt), key_prefix (first 8 chars)
- scopes (TEXT[])
- status (active/revoked)
- last_used_at
- expires_at

### Views (EMQX Integration)

**emqx_auth_v2** (Authentication)
```sql
SELECT device_label AS username,
       secret_hash AS password_hash,
       'bcrypt' AS password_hash_algorithm
FROM devices
WHERE status IN ('active', 'claimed') AND secret_hash IS NOT NULL
```

**emqx_acl_v2** (Authorization - Multi-tenant topics)
```sql
-- Publish telemetry
tenants/{tenant_id}/devices/{device_id}/telemetry/#

-- Subscribe telemetry (for monitoring)
tenants/{tenant_id}/devices/{device_id}/telemetry/#

-- Subscribe commands
tenants/{tenant_id}/devices/{device_id}/commands/#

-- Publish events
tenants/{tenant_id}/devices/{device_id}/events/#

-- Publish status
tenants/{tenant_id}/devices/{device_id}/status
```

### Row-Level Security (Defense-in-Depth)

```sql
CREATE POLICY tenant_isolation_devices ON devices
FOR ALL USING (
    tenant_id = current_setting('app.current_tenant_id', true)::uuid
    OR current_setting('app.current_user_role', true) = 'super_admin'
);
```

---

## 🔐 Security Architecture

### Authentication Layers

1. **Users (JWT)**
   - Access token: 1h expiration
   - Refresh token: 30 days expiration
   - Permissions embedded in token
   - Redis blacklist for logout

2. **Devices (MQTT)**
   - Username: device_label (UUID format)
   - Password: device_secret (bcrypt hashed)
   - EMQX validates via PostgreSQL query

3. **Services (API Keys)**
   - Bearer token authentication
   - Bcrypt hashed keys
   - Redis cache (1h TTL, 99% hit rate)
   - Scope-based permissions

### Device Provisioning Flow (Implementado - Opção A)

```
Factory Phase:
1. Device has: device_id (público), device_label (MQTT username), claim_code (privado)
2. DB: devices.status = unclaimed, claim_code_hash = bcrypt(claim_code)

Bootstrap (Device):
3. POST /api/devices/bootstrap
   body: {device_id, timestamp, signature=HMAC(MASTER_KEY, device_id:timestamp)}
4. API valida HMAC + janela de tempo, retorna status + poll_interval

Claim (Usuário logado):
5. POST /api/devices/claim {device_id, claim_code}
6. API valida claim_code_hash, grava tenant_id/owner_user_id
7. API gera device_secret e grava apenas secret_hash (bcrypt)
8. API cacheia secret 5min no Redis (one-time)

Secret Delivery (Device):
9. POST /api/devices/secret
   body: {device_id, timestamp, signature=HMAC(MASTER_KEY, device_id:timestamp)}
10. API valida HMAC + status=claimed
11. API retorna device_secret (uma vez) e grava secret_delivered_at

MQTT Activation:
12. Device → EMQX: CONNECT (username=device_label, password=device_secret)
13. Device → EMQX: PUBLISH tenants/{tenant_id}/devices/{device_id}/telemetry/slot/0
14. Go API marca status=active no primeiro telemetry
```

---

## 🧪 Testes Validados

### ✅ End-to-End Funcionando

1. **MQTT Local**
   ```bash
   mosquitto_pub -h 192.168.0.99 -p 1883 \
     -u "DEVICE_LABEL" \
     -P "DEVICE_SECRET" \
     -t "tenants/<tenant_id>/devices/<device_id>/telemetry/slot/0" \
     -m '{"value":42}'
   ```
   **Status:** ✅ Funciona

2. **MQTT Externo (WSS)**
   - Cloudflare Tunnel: mqtt.easysmart.com.br:443
   - Mobile Android (IoT MQTT Panel)
   - Publish + Subscribe funcionando
   **Status:** ✅ Funciona

3. **Telemetry Pipeline**
   - MQTT → EMQX → Go API Webhook → TimescaleDB
   - Redis cache atualizado
   - Device last_seen_at atualizado
   **Status:** ✅ Funciona

4. **Rate Limiting**
   - 12 msgs/min por device
   - 5 msgs/sec burst
   - 12 msgs/min por slot
   **Status:** ✅ Funciona

5. **Backward Compatibility**
   - Removido: fluxo antigo por token UUID não é mais garantido
   **Status:** ✅ Conforme docs atuais

---

## 🔍 Análise Crítica do Backend

### ✅ O Que Está BOM

- Arquitetura modular (handlers/middleware/models)
- Security forte (bcrypt, JWT, RLS)
- Database schema profissional
- Rate limiting funcional
- Telemetry pipeline completa

### ✅ GAPS CRÍTICOS (Todos resolvidos)

1. ~~Falta Register Endpoint~~ → Implementado (auth.go)
2. ~~Falta Refresh Token Endpoint~~ → Implementado com rotation + blacklist (auth.go)
3. ~~Falta Input Validation~~ → validator v10 + password strength
4. ~~Error Handling Inconsistente~~ → slog JSON + request_id + panic recovery
5. ~~Sem Graceful Shutdown~~ → Implementado (SIGTERM/SIGINT, timeout configurável)
6. ~~Sem CORS~~ → Middleware configurável via env

### ⚠️ MELHORIAS IMPORTANTES

7. Health check superficial
8. Logging primitivo
9. Sem request ID tracing
10. Rate limiting parcial
11. Zero testes

---

## 📋 Roadmap de Desenvolvimento

### ✅ Concluído (Base MVP + Fase A + Fase B)

- [x] Database migration multi-tenant
- [x] EMQX auth/ACL bcrypt
- [x] Go API estrutura modular
- [x] JWT middleware preparado
- [x] API key authentication
- [x] Tenant context middleware
- [x] Rate limiting Redis
- [x] Telemetry webhook
- [x] MQTT WSS público (Cloudflare)
- [x] Mobile teste Android
- [x] Backward compatibility

### ✅ Concluído (Fase A - Backend Profissional)

- [x] Register endpoint + validation
- [x] Refresh token endpoint (rotation + JTI blacklist)
- [x] Input validation (go-playground/validator v10)
- [x] Error handling estruturado (slog + request_id + panic recovery)
- [x] Graceful shutdown (SIGTERM/SIGINT)
- [x] CORS middleware (configurável via env)

### 📅 Próximas Fases

**Fase B: Melhorias Importantes (P1) - Concluída**
- [x] Health check completo (live/ready probes)
- [x] Structured logging (slog JSON)
- [x] Request ID tracing (X-Request-ID)
- [x] Rate limiting auth (10/min por IP)
- [x] Metrics (Prometheus /metrics)

**Fase C: Device Provisioning (P0) - 4h**
- [ ] Revisar/completar claim flow
- [ ] GET /api/devices/:id
- [ ] PUT /api/devices/:id
- [ ] DELETE /api/devices/:id (soft delete)
- [ ] POST /api/devices/:id/unclaim

**Fase D: Testes (P2) - 8h**
- [ ] Unit tests (handlers/middleware)
- [ ] Integration tests (auth/provisioning)
- [ ] Load tests

**Fase E: Frontend Dashboard (P2) - 40h**
- [ ] Next.js 14 + TypeScript
- [ ] Login/Register UI
- [ ] Device management
- [ ] Telemetry charts
- [ ] User management
- [ ] Tenant admin

---

## 🛠️ Scripts Utilitários

### Backup & Restore

**Backup Completo:**
```bash
./backup_full.sh
# Cria: backups/full_TIMESTAMP.tar.gz
```

**Backup EMQX (Após mudanças no Dashboard):**
```bash
./backup_emqx_config.sh
# Cria: backups/emqx/emqx_data.TIMESTAMP.tar.gz
```

**Restore EMQX (Após restart):**
```bash
./restore_emqx_config.sh
# Restaura: config + rules + connectors
```

### Testes Rápidos

**Teste MQTT Local:**
```bash
mosquitto_pub -h 192.168.0.99 -p 1883 \
  -u "8f8835f1-70c3-4cbd-b4c0-9acb6826c641" \
  -P "8f8835f1-70c3-4cbd-b4c0-9acb6826c641" \
  -t "tenants/00000000-0000-0000-0000-000000000001/devices/cad2adb9-8b50-4e28-8735-40f2c444b77f/telemetry/slot/0" \
  -m '{"value":42.5}'
```

**Verificar TimescaleDB:**
```bash
docker exec iiot_timescaledb psql -U admin -d iiot_telemetry \
  -c "SELECT * FROM telemetry ORDER BY timestamp DESC LIMIT 5;"
```

**Limpar Rate Limit (Debug):**
```bash
docker exec iiot_redis redis-cli --no-auth-warning \
  KEYS "rl:dev:*" | xargs docker exec -i iiot_redis redis-cli --no-auth-warning DEL
```

---

## 🔑 Credenciais & Endpoints

### PostgreSQL
- **Host:** localhost:5432 (iiot_postgres:5432 interno)
- **User:** admin
- **Database:** iiot_platform
- **Password:** (ver .env)

### TimescaleDB
- **Host:** localhost:5433 (iiot_timescaledb:5432 interno)
- **Database:** iiot_telemetry

### EMQX Dashboard
- **URL:** http://192.168.0.99:18083
- **User:** admin
- **Password:** admin0039

### EMQX MQTT (Público)
- **WSS:** mqtt.easysmart.com.br:443
- **Path:** /mqtt
- **Protocol:** WebSocket-SSL

### Go API
- **URL:** http://localhost:3001
- **Webhook API Key:** `emqxwh01_production_key_2026_secure`

### Device Teste
- **Username/Password:** `8f8835f1-70c3-4cbd-b4c0-9acb6826c641`
- **Device ID:** `cad2adb9-8b50-4e28-8735-40f2c444b77f`
- **Tenant ID:** `00000000-0000-0000-0000-000000000001`

---

## 📊 Métricas de Progresso

### Funcionalidades Implementadas

| Módulo | Funcionalidade | Status | Produção |
|--------|----------------|--------|----------|
| **Auth** | Login JWT | ✅ 100% | ✅ Ready |
| **Auth** | Register | ✅ 100% | ✅ Ready |
| **Auth** | Refresh Token (rotation + blacklist) | ✅ 100% | ✅ Ready |
| **Devices** | List (tenant-scoped) | ✅ 100% | ✅ Ready |
| **Devices** | Claim | ✅ 100% | ✅ Ready |
| **Devices** | Bootstrap | ✅ 100% | ✅ Ready |
| **Devices** | Secret Retrieval | ✅ 100% | ✅ Ready |
| **Devices** | CRUD (get/update/delete) | ❌ 0% | ❌ Missing |
| **Telemetry** | Webhook Ingestion | ✅ 100% | ✅ Ready |
| **Telemetry** | Latest Cache | ✅ 100% | ✅ Ready |
| **Telemetry** | Active Slots | ✅ 100% | ✅ Ready |
| **Telemetry** | Query API (histórico) | ❌ 0% | ❌ Missing |
| **MQTT** | Auth (bcrypt) | ✅ 100% | ✅ Ready |
| **MQTT** | ACL (multi-tenant) | ✅ 100% | ✅ Ready |
| **MQTT** | WSS Público | ✅ 100% | ✅ Ready |
| **Rate Limit** | Device/Slot (telemetry) | ✅ 100% | ✅ Ready |
| **Rate Limit** | Auth endpoints (IP) | ✅ 100% | ✅ Ready |
| **Observability** | Logs (slog JSON) | ✅ 100% | ✅ Ready |
| **Observability** | Metrics (Prometheus) | ✅ 100% | ✅ Ready |
| **Observability** | Request ID tracing | ✅ 100% | ✅ Ready |
| **Observability** | Distributed tracing | ❌ 0% | ❌ Missing |
| **Tests** | Unit | ❌ 0% | ❌ Missing |
| **Tests** | Integration | ❌ 0% | ❌ Missing |

**Score Total:** removido (sem critério objetivo)

---

## 🚀 Para Produção Piloto

### Checklist Mínimo (MVP)

- [x] Device conecta via MQTT ✅
- [x] Telemetry salva no TimescaleDB ✅
- [x] Multi-tenant isolation ✅
- [x] Rate limiting ✅
- [x] Register/Refresh endpoints ✅ (Fase A)
- [x] Input validation ✅ (Fase A)
- [x] Error handling profissional ✅ (Fase A)
- [x] Device provisioning (bootstrap/claim/secret/reset) ✅
- [x] Health checks ✅ (Fase B)
- [ ] Device CRUD completo ❌ (Fase C)
- [ ] Frontend básico ❌ (Fase E)

**Tempo para MVP completo:** ~20 horas de dev restantes

---

## 🆘 Troubleshooting

### MQTT Não Conecta

1. Verificar EMQX rodando: `docker ps | grep emqx`
2. Verificar auth view: `SELECT * FROM emqx_auth_v2 WHERE username='...'`
3. Verificar logs EMQX: `docker logs iiot_emqx --tail 50`

### Telemetry Não Chega

1. Verificar Go API: `curl http://localhost:3001/health`
2. Verificar webhook EMQX: Dashboard → Rules → send_to_api
3. Verificar logs Go API: `docker logs iiot_go_api --tail 50`
4. Limpar rate limit: (comando acima)

### Rate Limit Ativo

1. Limpar cache Redis: `docker exec iiot_redis redis-cli FLUSHDB`
2. Ou aguardar 1 minuto para resetar

### WSS Externo Não Funciona

1. Verificar tunnel: `ps aux | grep cloudflared`
2. Verificar DNS: `nslookup mqtt.easysmart.com.br`
3. Testar HTTPS: `curl -I https://mqtt.easysmart.com.br`

---

## 📞 Informações do Sistema

**Servidor:** 192.168.0.99  
**OS:** Ubuntu 24 (provável)  
**Docker Compose:** Sim  
**Cloudflare Tunnel:** Ativo (mqtt.easysmart.com.br)  
**Última Sessão:** 2026-02-10 03:42 BRT  
**Próxima Ação:** Fase C - Device CRUD + Fase E - Frontend

---

## 📚 Arquivos Importantes

```
/home/rodrigo/easysmart_iiot_core/
├── database/migrations/
│   ├── 001_initial_schema.sql
│   └── 002_production_multi_tenant.sql
├── go-api/
│   ├── main.go
│   ├── config/config.go
│   ├── handlers/*.go
│   ├── middleware/*.go
│   └── models/models.go
├── emqx/etc/emqx.conf
├── docker-compose.yml
├── .env
├── backup_full.sh
├── backup_emqx_config.sh
├── restore_emqx_config.sh
├── STATUS.md (este arquivo)
└── README.md
```

---

**Sistema estável. Fases A e B concluídas. Próximo: Device CRUD e Frontend.**
