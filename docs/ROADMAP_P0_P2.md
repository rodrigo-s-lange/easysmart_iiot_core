# Roadmap P0-P2 (Backend)

Este roadmap organiza o trabalho para levar o backend do estado atual para produção com previsibilidade.

## Status atual (2026-02-14)
- P0.1 (suite mínima de testes): **reforçado parcialmente** com novos testes unitários em auth/rate-limit/telemetry.
- P0.1 pendente: integração com banco real para fechar RLS e fluxos completos de devices (`provision/claim/secret`).
- P0.2, P0.3, P0.4, P0.5: acompanhar execução conforme checkpoints abaixo.
- Validações operacionais mínimas adicionadas em scripts:
  - `scripts/audit/test_tenant_isolation.sh`
  - `scripts/audit/test_e2e_mqtt_ingest.sh`
  - `database/backup_restore.sh`
  - `scripts/audit/run_load_test.sh`

## P0 (bloqueia produção)

### 1) Suite mínima de testes backend (auth/devices/telemetry/RLS)
- Objetivo: cobrir fluxos críticos com testes automatizados.
- Critérios de aceite:
  - cobre `register/login/refresh`;
  - cobre `devices/provision`, `devices/claim`, `devices/secret` (one-time);
  - cobre webhook `telemetry` (válido, inválido, tenant mismatch, rate limit);
  - valida isolamento entre tenants (Postgres e TimescaleDB).

### 2) Normalizar versionamento de migrações
- Objetivo: remover ambiguidades de ordenação e risco de drift.
- Critérios de aceite:
  - numeração única e sequencial (sem duplicidade como `003_*`);
  - script único de apply em ordem;
  - banco limpo sobe sem ajuste manual.

### 3) Tabela de controle de migração + idempotência
- Objetivo: garantir execução confiável em novos ambientes.
- Critérios de aceite:
  - tabela de controle de migração implementada;
  - reexecução não quebra schema;
  - pipeline valida “migrate up” em banco vazio.

### 4) CI pipeline básico (gofmt/vet/test/build)
- Objetivo: bloquear regressões antes de merge.
- Critérios de aceite:
  - workflow executa `gofmt` (check), `go vet`, testes e build;
  - PR falha quando qualquer etapa falhar.

### 5) Observabilidade mínima + alertas críticos
- Objetivo: operar com detecção rápida de incidentes.
- Critérios de aceite:
  - métricas essenciais expostas e coletadas;
  - alertas mínimos: API down, DB down, spike de 500.

## P1 (estabilidade de produção)

### 6) Teste de carga básico (throughput MQTT/webhook)
- Objetivo: validar capacidade antes de produção.
- Critérios de aceite:
  - cenário de carga reproduzível;
  - relatório com limites observados.

### 7) Padronização de erros e contrato API
- Objetivo: previsibilidade para frontend e integrações.
- Critérios de aceite:
  - envelope de erro unificado;
  - status codes consistentes;
  - OpenAPI aderente ao comportamento real.

### 8) Versionamento da API (`/api/v1`)
- Objetivo: evoluir contrato sem quebra abrupta.
- Critérios de aceite:
  - endpoints `v1` disponíveis;
  - compatibilidade transitória definida;
  - plano de depreciação documentado.

### 9) Rotação de segredos documentada e testada
- Objetivo: reduzir risco operacional de credenciais estáticas.
- Escopo: `JWT_SECRET`, `MANUFACTURING_MASTER_KEY`, API keys.
- Critérios de aceite:
  - procedimento documentado;
  - rotação testada em ambiente de teste;
  - sem downtime não planejado.

### 10) Backup/restore automatizado
- Objetivo: garantir recuperação real.
- Critérios de aceite:
  - backup automatizado (Postgres e TimescaleDB);
  - restore executado com sucesso em ambiente limpo;
  - checklist de validação publicado.

### 11) Limpeza/auditoria sem conflito de FK
- Objetivo: permitir saneamento de dados sem quebrar integridade.
- Critérios de aceite:
  - procedimento de cleanup funcional;
  - sem violação de FK/triggers;
  - runbook operacional documentado.

## Plano de execução P1 (ordem recomendada)

### Sprint P1.1 (contrato + confiabilidade)
- Entregas:
  - padronização de erros HTTP (código + payload único);
  - revisão de aderência do `docs/openapi.yaml` ao comportamento real.
- Saída esperada:
  - frontend integra sem condicionais por endpoint;
  - exemplos e respostas de erro consistentes.

### Sprint P1.2 (capacidade + operação)
- Entregas:
  - teste de carga básico MQTT/webhook com cenário reproduzível;
  - relatório com throughput, latência e ponto de degradação.
- Saída esperada:
  - limites operacionais conhecidos antes de produção.

### Sprint P1.3 (segurança operacional)
- Entregas:
  - rotação de segredos (`JWT_SECRET`, `MANUFACTURING_MASTER_KEY`, API keys) com procedimento testado.
- Saída esperada:
  - rotação sem downtime não planejado.

### Sprint P1.4 (resiliência de dados)
- Entregas:
  - backup/restore automatizado de Postgres e TimescaleDB;
  - validação de restore em ambiente limpo.
- Saída esperada:
  - RPO/RTO validados com script e checklist.

### Sprint P1.5 (higiene de dados)
- Entregas:
  - rotina de cleanup/auditoria sem conflitos de FK.
- Saída esperada:
  - manutenção periódica de dados com integridade preservada.

## P2 (escala e maturidade)

### 12) Threat model e hardening avançado
- Objetivo: priorizar mitigação por risco real.
- Critérios de aceite:
  - threat model documentado;
  - plano de ação para riscos prioritários.

### 13) Staging espelhado + runbooks
- Objetivo: reduzir risco de mudanças em produção.
- Critérios de aceite:
  - ambiente staging funcional e próximo de produção;
  - runbooks para incidentes principais (API, DB, Redis, EMQX).
