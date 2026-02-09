# IIoT Platform - Production Multi-Tenant Architecture

## ✅ Status: FUNCIONANDO

Sistema migrado com sucesso para arquitetura multi-tenant production-ready com autenticação bcrypt, RBAC, e device lifecycle management.

---

## 🎯 O Que Foi Feito

### 1. Database Migration (002_production_multi_tenant.sql)
- ✅ Multi-tenancy: `tenants`, `users_v2`, `devices_v2`
- ✅ RBAC: `permissions`, `role_permissions` (13 permissões, 3 roles)
- ✅ Security: bcrypt para senhas/secrets/API keys
- ✅ Audit: `audit_log` com structured columns + JSONB
- ✅ Row-Level Security (RLS) policies
- ✅ Device lifecycle: unclaimed → claimed → active
- ✅ Backward compatibility: `devices_compat` view

### 2. EMQX Configuration
- ✅ Authentication: bcrypt via PostgreSQL (`emqx_auth_v2`)
- ✅ ACL: Multi-tenant topics via `emqx_acl_v2`
- ✅ Rule Engine: Webhook para Go API com API key
- ✅ Topics: `tenants/{tenant_id}/devices/{device_id}/telemetry/slot/{N}`

### 3. Go API (Modular Structure)
- ✅ JWT middleware (users)
- ✅ API key middleware (webhooks) - bcrypt + Redis cache
- ✅ Tenant context middleware (RLS)
- ✅ Permission check middleware
- ✅ Rate limiting (Redis-based)
- ✅ Telemetry handler (multi-tenant aware)

### 4. Data Migration
- ✅ Default tenant criado: `00000000-0000-0000-0000-000000000001`
- ✅ User migrado: `test@test.com` (tenant_admin)
- ✅ Device migrado: mantém token antigo funcionando
- ✅ EMQX auth backward compatible

---

## 🔑 Credenciais & Keys

### PostgreSQL
- **Host:** localhost:5432 / iiot_postgres:5432 (interno)
- **User:** admin
- **Database:** iiot_platform
- **Password:** (ver .env)

### TimescaleDB
- **Host:** localhost:5433 / iiot_timescaledb:5432 (interno)
- **Database:** iiot_telemetry

### EMQX Dashboard
- **URL:** http://192.168.0.99:18083
- **User:** admin
- **Password:** admin0039
- **API Key:** 8636e9c8ee0bd7cb
- **API Secret:** fOazE17vskHGXyqBUjkZtsJPS3vgkt4z2kuNxhtpy0A

### Go API
- **URL:** http://localhost:3001
- **Webhook API Key:** `emqxwh01_production_key_2026_secure`
- **JWT Secret:** (ver .env)

---

## 🧪 Como Testar

### 1. Testar MQTT (Device Antigo - Backward Compat)
```bash
mosquitto_pub -h 192.168.0.99 -p 1883 \
  -u "8f8835f1-70c3-4cbd-b4c0-9acb6826c641" \
  -P "8f8835f1-70c3-4cbd-b4c0-9acb6826c641" \
  -t "tenants/00000000-0000-0000-0000-000000000001/devices/cad2adb9-8b50-4e28-8735-40f2c444b77f/telemetry/slot/0" \
  -m '{"value":42.5,"unit":"celsius"}'
```

### 2. Verificar Dados no TimescaleDB
```bash
docker exec iiot_timescaledb psql -U admin -d iiot_telemetry \
  -c "SELECT * FROM telemetry ORDER BY timestamp DESC LIMIT 5;"
```

### 3. Verificar Cache Redis
```bash
docker exec iiot_redis redis-cli GET "latest:device:cad2adb9-8b50-4e28-8735-40f2c444b77f:slot:0"
```

### 4. Testar API Health
```bash
curl http://localhost:3001/health
```

---

## 🔄 Backup & Restore

### Backup Completo
```bash
./backup_full.sh
# Cria: backups/full_TIMESTAMP.tar.gz
```

### Backup EMQX (Após Mudanças no Dashboard)
```bash
./backup_emqx_config.sh
# Cria: backups/emqx/emqx_data.TIMESTAMP.tar.gz
```

### Restore EMQX (Após Restart)
```bash
./restore_emqx_config.sh
# Restaura: config + rules + connectors
```

---

## 📊 Estrutura Multi-Tenant

### MQTT Topics
```
tenants/{tenant_id}/devices/{device_id}/telemetry/slot/{N}
tenants/{tenant_id}/devices/{device_id}/commands/{type}
tenants/{tenant_id}/devices/{device_id}/events/{type}
tenants/{tenant_id}/devices/{device_id}/status
```

### Database Isolation
- Application-level filtering (WHERE tenant_id = ...)
- RLS policies (defense-in-depth)
- Tenant context via session variables

### RBAC Roles
| Role | Permissions |
|------|-------------|
| super_admin | All (cross-tenant) |
| tenant_admin | All except system:admin, tenants:write |
| tenant_user | devices + telemetry read/write only |

---

## 🚀 Próximos Passos (Go API Implementation)

### Phase 1: Security (P0) - 2 weeks
- [ ] JWT endpoints (login, refresh)
- [ ] Device claim endpoint
- [ ] Device bootstrap endpoint
- [ ] Device secret retrieval (Redis-backed)
- [ ] User password management (bcrypt)

### Phase 2: Multi-Tenancy (P1) - 1 week
- [ ] Tenant CRUD endpoints
- [ ] User tenant assignment
- [ ] Quota enforcement

### Phase 3: RBAC (P1) - 1 week
- [ ] Role assignment endpoints
- [ ] Permission enforcement in all routes
- [ ] Redis permission cache

### Phase 4: Audit & Compliance (P2) - 2 weeks
- [ ] Audit log query endpoints
- [ ] GDPR data export
- [ ] Data anonymization
- [ ] Retention policy enforcement

---

## 📁 Arquivos Importantes

### Criados/Modificados
```
database/migrations/002_production_multi_tenant.sql  # Migration completa
emqx/etc/emqx.conf                                   # EMQX config (bcrypt auth)
backup_full.sh                                       # Backup completo
backup_emqx_config.sh                               # Backup EMQX
restore_emqx_config.sh                              # Restore EMQX
STATUS.md                                            # Este arquivo
```

### Backups
```
backups/full_20260209_003556.tar.gz                # Backup pré-migration
backups/emqx/emqx_data.20260209_020643.tar.gz     # EMQX funcionando
```

---

## ⚠️ Importante

1. **EMQX perde configuração ao reiniciar** - SEMPRE rode `./restore_emqx_config.sh` após restart
2. **API Key tem 8 chars de prefix** - Formato: `prefixXX_rest_of_key`
3. **Backward compatibility** - Device antigo ainda funciona, mas novos devices usarão claim flow
4. **RLS requer transação** - Go API deve usar `SET LOCAL` dentro de transaction

---

## 🆘 Rollback

Se algo quebrar:
```bash
# 1. Restaurar EMQX
cp emqx/etc/emqx.conf.backup_20260209_004313 emqx/etc/emqx.conf
docker restart iiot_emqx

# 2. Restaurar PostgreSQL
tar -xzf backups/full_20260209_003556.tar.gz -C backups/
docker exec -i iiot_postgres psql -U admin -d iiot_platform < backups/full_20260209_003556/postgres_iiot_platform.sql

# 3. Restart services
docker-compose restart
```

---

## 📞 Suporte

Sistema configurado em: 2026-02-09
Última atualização: 2026-02-09 02:06:43 BRT
Status: ✅ Produção funcional (MVP 70% → 80%)

