# EasySmart IIoT Core - Status

**Atualizado em:** 2026-02-14

## Resumo Executivo
- Backend funcional com auth, provisionamento, ingestao MQTT e leitura de telemetria.
- Isolamento por tenant ativo em Postgres e Timescale.
- Observabilidade operacional ativa (Prometheus, Alertmanager, Grafana, Telegram).
- Quotas/billing implementados no backend.
- Automacao de backup/restore documentada e com timers.

## O que esta implementado

### API e Seguranca
- JWT com refresh e validacoes de permissao.
- API key para webhook do EMQX.
- Prefixo estavel `/api/v1` (legacy `/api` ainda compativel).
- Envelope de erro padronizado (`code`, `message`, `request_id`, `details`).

### Devices e MQTT
- Provisionamento direto (`/api/v1/devices/provision`).
- Claim/bootstrap/secret/reset (fluxo legado suportado).
- Secret one-time no endpoint de `secret`.
- Validacao tenant/topic/device no webhook de telemetria.

### Dados e Isolamento
- Postgres: tenants/users/devices/audit/api_keys/quotas.
- Timescale: `telemetry` com `tenant_id`.
- Leitura de telemetria por tenant do JWT.

### Billing e Limites
- Planos e ciclo de cobranca no tenant.
- Quotas:
  - devices
  - mensagens por minuto (por device)
  - storage por tenant
- Bloqueio duro para `starter/pro` e overage opcional em `enterprise`.
- Endpoints super_admin para quota/uso.

### Observabilidade e Operacao
- Health: `/health`, `/health/live`, `/health/ready`.
- Metrics: `/metrics`.
- Alertas de API/DB/ingestao.
- Telegram:
  - alertas criticos
  - eventos de cadastro de usuario/device
  - formato padronizado
  - horario BR na mensagem
  - deduplicacao: watcher DB desativado por padrao.

### Resiliencia
- Backup diario + restore drill semanal.
- Politica de retencao/arquivamento por tenant.
- RPO/RTO documentado por plano.

## Pendencias de maturidade (proximo ciclo)
- CI com gates completos (lint/vet/test/build + seguranca de supply chain).
- Staging espelhado com smoke E2E antes de producao.
- Teste de carga com alvo formal e relatorio versionado.
- Hardening de auditoria/compliance (processo LGPD operacional).
