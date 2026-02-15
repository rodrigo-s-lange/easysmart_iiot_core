# Roadmap P0-P2 (Atualizado)

## Status atual
- **P0 principal concluido** no escopo do core:
  - suite base de testes + scripts de validacao operacional;
  - migracoes normalizadas e controle por `schema_migrations`;
  - observabilidade minima com alertas criticos;
  - backup/restore e rotina operacional documentados;
  - quotas/billing com enforcement e auditoria.
- **P1 em andamento** para maturidade de operacao B2B.

## P0 (Concluido)
1. Testes e validacao minima
- unitarios e scripts E2E/isolamento/seguranca/carga basica.

2. Migracoes e baseline
- sequenciamento estabilizado;
- script unico `database/migrate.sh`.

3. Observabilidade minima
- `/metrics`, probes health/live/ready;
- Prometheus + Alertmanager + Grafana;
- alertas API/DB/ingestao.

4. Resiliencia inicial de dados
- backup diario e restore drill semanal;
- politica de retencao por tenant;
- RPO/RTO inicial documentado.

5. Billing e quotas
- limites por tenant e bloqueio em runtime;
- endpoints super_admin de quota/uso;
- trilha de auditoria e notificacao operacional.

## P1 (Proximo ciclo recomendado)
1. CI/CD com gates fortes
- lint/vet/test/build + scans de dependencia/imagem;
- rollback automatizado em deploy.

2. Staging espelhado
- ambiente espelho de producao;
- smoke E2E antes de promover release.

3. Teste de carga formalizado
- cenarios padrao (volumetria por plano);
- SLO de latencia/erro com relatorio versionado por release.

4. Governanca de API e contrato
- manter OpenAPI 100% aderente ao runtime;
- politica de deprecacao de endpoints legacy `/api`.

## P2 (Maturidade avancada)
1. Compliance operacional (LGPD/auditoria)
- processo de export/delete por titular;
- checklist de evidencias para auditoria externa.

2. Hardening e threat model formal
- mapa de risco por componente;
- backlog de mitigacoes com prioridade por impacto.
