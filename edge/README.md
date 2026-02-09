# Edge Runtime – Industrial Control Core

Este diretório contém a **base do runtime Edge**, responsável pela execução determinística,
segura e auditável de controle industrial distribuído.

O Edge **não é um CLP tradicional**, mas um núcleo lógico orientado a eventos,
com forte separação entre:

- Lógica de controle (Core)
- Expansões de I/O (Slots)
- Comunicação e orquestração (Gateway – externo)
- Backend e nuvem (fora do escopo deste diretório)

## Princípios Fundamentais

- Segurança funcional acima de disponibilidade
- Determinismo explícito (não implícito)
- Separação dura entre lógica e infraestrutura
- Comportamento previsível sob falha
- Auditabilidade como requisito de projeto

## O que este diretório NÃO é

- Não é firmware específico de hardware
- Não é backend
- Não é gateway
- Não contém lógica de negócio de planta fabril
- Não substitui normas industriais — ele se ancora nelas

## Estrutura Geral

- `docs/`  
  Documentação normativa e arquitetural

- `core/`  
  Núcleo lógico e máquina de estados do sistema

- `slots/`  
  Contratos e definição de módulos de expansão

- `include/`  
  Headers públicos que definem os contratos do sistema

- `safety/`  
  Conceitos, regras e garantias de segurança funcional

## Filosofia

> Um sistema industrial não deve “tentar continuar”.
> Ele deve **saber exatamente quando pode continuar e quando deve parar**.

Todo o comportamento do Edge é definido de forma explícita.
Nada é implícito. Nada é mágico.

---

Status: **Em definição ativa (v0.x)**  
Este diretório é intencionalmente conservador.