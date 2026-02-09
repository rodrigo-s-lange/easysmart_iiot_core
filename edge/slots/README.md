# Slots – Visão Geral de Implementação

Este diretório define o **modelo conceitual e estrutural** dos Slots,
não implementações específicas de hardware.

Slots são tratados como **módulos controlados**, nunca como extensões livres.

---

## O que é um Slot

Um Slot é uma unidade isolada que:

- Interage com hardware ou lógica periférica
- Fornece dados ao Core
- Executa ações sob comando do Core
- Opera dentro de limites explícitos

Slots **não são plugins dinâmicos**.
Eles são componentes registrados e validados.

---

## Tipos Comuns de Slots

- Entradas digitais
- Entradas rápidas (quando suportado)
- Saídas digitais
- PWM / Analógico
- Sensores (temperatura, pressão, etc)
- Slots lógicos (PID, filtros, ML local)

---

## Slots Internos vs Expansão

### Internos
- Executam no mesmo RTOS
- Usam drivers Zephyr
- Podem participar de caminhos determinísticos

### Expansão
- Comunicação serializada
- Latência não determinística
- Devem declarar limitações explícitas

---

## Responsabilidades de um Slot

Cada Slot deve:

- Declarar capacidades
- Declarar limites
- Declarar política de falha
- Implementar callbacks bem definidos
- Ser observável e auditável

---

## O que um Slot NÃO pode fazer

- Alterar estados globais
- Criar threads sem autorização
- Bloquear o Core
- Acessar hardware fora do seu escopo
- Bypassar políticas de segurança

---

## Integração com o Core

Slots interagem com o Core por meio de:

- APIs formais
- Estruturas de dados versionadas
- Eventos e mensagens

Nenhum Slot conhece a lógica do usuário.

---

## Versionamento

Slots possuem:
- Versão de contrato
- Versão de implementação

Compatibilidade é obrigatória.

---

## Filosofia

> “Slots existem para ampliar capacidades,  
> não para ampliar riscos.”