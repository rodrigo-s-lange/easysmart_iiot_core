# IIoT Platform - Production Multi-Tenant Architecture

## ✅ Status: Sistema Base Funcional - Iniciando Melhorias Profissionais

**Última Atualização:** 2026-02-09 03:42 BRT  
**Progresso MVP:** 80% → 85% (base funcional, faltam refinamentos)

---

## 🎯 Missão Atual: Backend Profissional

### Fase A - Gaps Críticos (EM ANDAMENTO)
Corrigindo falhas que impedem produção profissional:
- [ ] Register endpoint
- [ ] Refresh token endpoint  
- [ ] Input validation
- [ ] Error handling estruturado
- [ ] Graceful shutdown
- [ ] CORS middleware

**Tempo estimado:** 6 horas  
**Prioridade:** P0 (Crítico)

---

## 🏗️ Arquitetura Atual

### Stack Tecnológico
```
Frontend:   Next.js (planejado)
API:        Go 1.21+ (Gin/net/http)
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

**users_v2** (User management + RBAC)
- user_id (UUID PK)
- tenant_id (FK, NULL for super_admin)
- email (unique), password_hash (bcrypt)
- role (super_admin/tenant_admin/tenant_user)
- status (active/suspended/deleted)
- last_login_at

**devices_v2** (Device lifecycle)
- device_id (UUID PK)
- tenant_id (FK)
- owner_user_id (FK)
- device_label (unique, public identifier)
- secret_hash (bcrypt, NULL when unclaimed)
- status (unclaimed/claimed/active/suspended/revoked)
- claimed_at, activated_at, last_seen_at
- firmware_version, hardware_revision
- metadata (JSONB)

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
FROM devices_v2
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
CREATE POLICY tenant_isolation_devices ON devices_v2
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

### Device Provisioning Flow

```
Factory Phase:
1. INSERT device (status=unclaimed, device_label="ESM-X1Y2Z3", secret_hash=NULL)
2. Flash firmware with device_label only

Device First Boot:
3. Device → API: GET /api/devices/bootstrap?device_label=ESM-X1Y2Z3
4. API → Device: 202 {status: "unclaimed", poll_interval: 30}
5. Device polls every 30s (LED blinks)

User Claims Device:
6. User → API: POST /api/devices/claim {device_label: "ESM-X1Y2Z3"} [JWT auth]
7. API → DB: claim_device() generates 64-char hex secret, stores bcrypt hash
8. API → Redis: SET claim:{device_id}:secret (5min TTL)
9. API → User: 200 {device_id, message: "Claimed"}

Device Retrieves Secret:
10. Device → API: GET /api/devices/bootstrap (next poll)
11. API → Device: 200 {status: "claimed", secret_url: "/api/devices/secret?token=..."}
12. Device → API: GET /api/devices/secret?token=...
13. API → Redis: GETDEL claim:{device_id}:secret (consume once)
14. API → Device: 200 {device_secret: "a1b2c3..."}
15. Device stores secret in flash (encrypted at rest)

MQTT Activation:
16. Device → EMQX: CONNECT (username=device_label, password=device_secret)
17. EMQX → DB: SELECT FROM emqx_auth_v2 WHERE username=...
18. EMQX: bcrypt verify → CONNACK success
19. Device → EMQX: PUBLISH tenants/{tenant_id}/devices/{device_id}/telemetry/slot/0
20. EMQX Rule Engine → Go API webhook → TimescaleDB
21. API updates device status='active', activated_at=NOW()
```

---

## 🧪 Testes Validados

### ✅ End-to-End Funcionando

1. **MQTT Local**
   ```bash
   mosquitto_pub -h 192.168.0.99 -p 1883 \
     -u "8f8835f1-70c3-4cbd-b4c0-9acb6826c641" \
     -P "8f8835f1-70c3-4cbd-b4c0-9acb6826c641" \
     -t "tenants/.../devices/.../telemetry/slot/0" \
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
   - Device antigo (token UUID) continua funcionando
   **Status:** ✅ Funciona

---

## 🔍 Análise Crítica do Backend

### ✅ O Que Está BOM

- Arquitetura modular (handlers/middleware/models)
- Security forte (bcrypt, JWT, RLS)
- Database schema profissional
- Rate limiting funcional
- Telemetry pipeline completa

### ❌ GAPS CRÍTICOS (Impedem Produção)

1. **Falta Register Endpoint**
   - Não consegue criar usuários via API
   - **Prioridade:** P0

2. **Falta Refresh Token Endpoint**
   - JWT expira em 1h sem renovação
   - **Prioridade:** P0

3. **Falta Input Validation**
   - Aceita emails inválidos, senhas fracas
   - **Prioridade:** P0

4. **Error Handling Inconsistente**
   - Logs não estruturados
   - Debug impossível em produção
   - **Prioridade:** P0

5. **Sem Graceful Shutdown**
   - Perde conexões em restart
   - **Prioridade:** P0

6. **Sem CORS**
   - Frontend bloqueado
   - **Prioridade:** P0

### ⚠️ MELHORIAS IMPORTANTES

7. Health check superficial
8. Logging primitivo
9. Sem request ID tracing
10. Rate limiting parcial
11. Zero testes

---

## 📋 Roadmap de Desenvolvimento

### ✅ Concluído (Base MVP - 85%)

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

### 🚧 Em Andamento (Fase A - 6h)

- [ ] Register endpoint + validation
- [ ] Refresh token endpoint
- [ ] Input validation (go-playground/validator)
- [ ] Error handling estruturado
- [ ] Graceful shutdown
- [ ] CORS middleware

### 📅 Próximas Fases

**Fase B: Melhorias Importantes (P1) - 4h**
- [ ] Health check completo (live/ready probes)
- [ ] Structured logging (slog)
- [ ] Request ID tracing
- [ ] Rate limiting global
- [ ] Metrics (Prometheus)

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
| **Auth** | Register | ❌ 0% | ❌ Missing |
| **Auth** | Refresh Token | ❌ 0% | ❌ Missing |
| **Devices** | List (tenant-scoped) | ✅ 100% | ✅ Ready |
| **Devices** | Claim | ⚠️ 80% | 🔧 Needs review |
| **Devices** | Bootstrap | ⚠️ 80% | 🔧 Needs review |
| **Devices** | Secret Retrieval | ⚠️ 80% | 🔧 Needs review |
| **Devices** | CRUD | ❌ 0% | ❌ Missing |
| **Telemetry** | Webhook Ingestion | ✅ 100% | ✅ Ready |
| **Telemetry** | Latest Cache | ✅ 100% | ✅ Ready |
| **Telemetry** | Query API | ❌ 0% | ❌ Missing |
| **MQTT** | Auth (bcrypt) | ✅ 100% | ✅ Ready |
| **MQTT** | ACL (multi-tenant) | ✅ 100% | ✅ Ready |
| **MQTT** | WSS Público | ✅ 100% | ✅ Ready |
| **Rate Limit** | Device/Slot | ✅ 100% | ✅ Ready |
| **Rate Limit** | Global IP/User | ❌ 0% | ❌ Missing |
| **Observability** | Logs | ⚠️ 30% | ❌ Primitive |
| **Observability** | Metrics | ❌ 0% | ❌ Missing |
| **Observability** | Tracing | ❌ 0% | ❌ Missing |
| **Tests** | Unit | ❌ 0% | ❌ Missing |
| **Tests** | Integration | ❌ 0% | ❌ Missing |

**Score Total:** 85/100 (Base funcional, refinamentos pendentes)

---

## 🚀 Para Produção Piloto

### Checklist Mínimo (MVP)

- [x] Device conecta via MQTT ✅
- [x] Telemetry salva no TimescaleDB ✅
- [x] Multi-tenant isolation ✅
- [x] Rate limiting ✅
- [ ] Register/Refresh endpoints ❌ (Fase A)
- [ ] Input validation ❌ (Fase A)
- [ ] Error handling profissional ❌ (Fase A)
- [ ] Device provisioning completo ⚠️ (Fase C)
- [ ] Health checks ❌ (Fase B)
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
**Última Sessão:** 2026-02-09 03:42 BRT  
**Próxima Ação:** Fase A - Backend Profissional (6h)

---

## 📚 Arquivos Importantes

```
/home/rodrigo/iiot_platform/
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

**Sistema estável. Pronto para Fase A - Backend Profissional.** 🚀

