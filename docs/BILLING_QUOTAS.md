# Billing e Quotas por Tenant

Este documento define o comportamento atual de quotas para a fase inicial SaaS.

## Planos
- `starter`
- `pro`
- `enterprise`

## Ciclo de cobrança
- `monthly`
- `annual`

## Quotas atuais
- `quota_devices`
  - `0` significa ilimitado.
- `quota_msgs_per_min`
  - limite por **device** dentro do tenant (janela de 60s).
  - padrão inicial: `360`.
- `quota_storage_mb`
  - limite de armazenamento estimado de telemetria por tenant.
  - padrão inicial: `1000 MB`.

## Regras de enforcement
- Provision/claim de device:
  - bloqueia com `429` se `quota_devices` for excedido.
- Ingestão de telemetria:
  - bloqueia com `429` se exceder `quota_msgs_per_min` por device.
  - para limite de storage:
    - `starter` e `pro`: bloqueio duro.
    - `enterprise`: permitido somente quando `allow_overage=true`.

## Auditoria para cobrança/disputa
Eventos gravados em `audit_log`:
- `quota.devices_exceeded`
- `quota.messages_exceeded`
- `quota.storage_exceeded`
- `quota.updated`
- `billing.snapshot_generated`

Snapshots de uso:
- tabela `tenant_usage_snapshots`
- atualizada durante consulta de uso (`/api/v1/tenants/{tenant_id}/usage`).

## Notificação operacional
Quando há bloqueio de quota, o backend notifica Telegram (se configurado):
- inclui tenant e contexto do bloqueio.
- para bloqueios de provision/claim inclui `email` do usuário autenticado.

## Endpoints (super_admin)
- `GET /api/v1/tenants/{tenant_id}/quotas`
- `PATCH /api/v1/tenants/{tenant_id}/quotas`
- `GET /api/v1/tenants/{tenant_id}/usage`

Permissão requerida:
- `system:admin`

## Bootstrap de super admin (quando necessário)
Se não houver usuário com role `super_admin`, promova um usuário existente:

```bash
docker exec -i iiot_postgres psql -U admin -d iiot_platform -c "\
UPDATE users SET role='super_admin', tenant_id=NULL, updated_at=NOW() \
WHERE email='seu-email@dominio.com';"
```

Referência SQL: `database/maintenance/promote_super_admin.sql`.
