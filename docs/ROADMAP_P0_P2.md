# Roadmap P0-P2 (Backend)

Este roadmap organiza o trabalho para levar o backend do estado atual para produção com previsibilidade.

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
