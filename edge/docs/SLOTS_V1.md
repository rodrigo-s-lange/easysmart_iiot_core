# Slots de Expansão – Especificação v1

Slots são a **unidade fundamental de extensibilidade** do Edge Runtime.
Todo I/O, periférico ou capacidade adicional existe como Slot.

Um Slot não é apenas hardware.
É um **contrato formal** entre o Core e uma funcionalidade.

---

## Objetivos do Modelo de Slots

- Isolar falhas
- Padronizar expansão
- Permitir auditoria
- Garantir segurança funcional
- Escalar do monolítico ao distribuído

---

## Classificação de Slots

### Slots Internos (v1–v99)

- Executam no mesmo SoC do Core
- Acesso direto ao HAL
- Potencialmente determinísticos
- Usados para:
  - I/O on-chip
  - Temporizadores
  - PWM
  - PCNT
  - ADC/DAC
  - Watchdogs

---

### Slots de Expansão (v100+)

- Executam fora do SoC principal
- Comunicação via barramento (ex: CAN-FD)
- Latência conhecida, porém não garantida como determinística
- Usados para:
  - Cartões de I/O
  - Módulos especializados
  - Expansões remotas

Slots de expansão **não podem ser assumidos como determinísticos**,
a menos que explicitamente declarado.

---

## Contrato Básico de um Slot

Todo Slot deve declarar:

- Identificador único
- Tipo (entrada, saída, sensor, atuador, lógico)
- Capacidades
- Limites operacionais
- Estados suportados
- Política de falha
- Regras de segurança

Sem contrato, não existe Slot.

---

## Ciclo de Vida de um Slot

1. Descoberta
2. Validação
3. Inicialização
4. Operação
5. Falha ou desligamento
6. Snapshot (se aplicável)

O Core nunca assume sucesso implícito.

---

## Comunicação Slot ↔ Core

Slots se comunicam com o Core via:

- Eventos
- Filas (queues)
- Mensagens estruturadas
- Sinais de falha

Comunicação direta entre Slots **não é permitida**.

---

## Falhas de Slot

Falhas podem ser:

- Locais (isoladas)
- Propagáveis
- Críticas

Cada Slot define:
- Como detecta falha
- Como reporta
- O impacto permitido no sistema

O Core decide a ação final.

---

## Segurança e Autoridade

- Slots **não controlam estados globais**
- Slots **não alteram políticas**
- Slots **não executam lógica de negócio**

Eles fornecem dados e executam ações dentro de limites.

---

## Slots e Programação do Usuário

O usuário:
- Configura Slots
- Define parâmetros
- Define políticas de falha

O usuário **não**:
- Altera drivers
- Altera HAL
- Altera regras duras de segurança

---

## Compatibilidade Futura

Esta é a versão **v1**.

Mudanças futuras devem:
- Ser compatíveis ou versionadas
- Não quebrar contratos existentes
- Preservar segurança

---

## Princípio Norteador

> “Um Slot poderoso sem limites é mais perigoso do que inútil.”

## Slot vs HAL

- O Slot NÃO acessa hardware diretamente
- O Slot consome serviços da HAL
- A HAL nunca conhece Slots

Relação:
HAL → fornece primitivas
Slot → encapsula lógica + regras
Core → orquestra

+-----------+
|  Core     |
+-----------+
     |
     v
+-----------+      +------+
|  Slot     | ---> | HAL  |
+-----------+      +------+

