# Estados do Edge Runtime

O Edge Runtime opera como uma **máquina de estados explícita**.
Nenhuma execução ocorre fora de um estado conhecido.
Nenhuma transição ocorre sem regras claras.

Estados não são apenas operacionais — são **contratuais e auditáveis**.

---

## Estados Definidos

### INIT

Estado inicial após boot ou reset.

Funções:
- Inicialização mínima de hardware
- Verificação de integridade do Core
- Descoberta e validação de Slots
- Health-check de energia, clock e memória
- Leitura de snapshots persistidos (se existirem)

Regras:
- Nenhuma saída física pode ser acionada
- Nenhuma lógica do usuário é executada
- Falhas em INIT levam diretamente a SAFE ou STOP

---

### RUN

Estado normal de operação.

Funções:
- Execução da lógica do usuário
- Processamento de eventos
- Controle de I/O
- Comunicação interna entre Core e Slots

Regras:
- Somente RUN permite controle ativo
- Entradas podem gerar eventos
- Saídas obedecem às regras de segurança
- Gateway **não pode forçar alterações críticas**

---

### PAUSE

Estado intermediário e controlado.

Funções:
- Congelar execução da lógica do usuário
- Manter último estado válido
- Permitir manutenção, update ou análise

Regras:
- Pode ser acionado a qualquer momento
- Não pode violar regras duras de Slots
- Saídas podem manter estado ou migrar para estado definido pelo usuário
- Necessário para atualizações

---

### FAULT

Estado de falha detectada.

Tipos de falha:
- Falha de Slot
- Falha lógica
- Falha elétrica
- Falha de comunicação interna
- Violação de regra de segurança

Funções:
- Registrar falha
- Executar política de falha definida pelo usuário
- Notificar gateway
- Decidir entre PAUSE, SAFE ou STOP

Regras:
- Nem toda falha é crítica
- Falhas podem ser locais ou globais
- Recuperação pode ou não ser automática

---

### SAFE

Estado de segurança controlada.

Funções:
- Levar o sistema para condição segura definida
- Manter serviços mínimos necessários
- Preservar logs e snapshots

Regras:
- SAFE é definido por política, não por padrão fixo
- Pode manter subsistemas ativos se permitido
- Nenhuma nova lógica é iniciada

---

### STOP

Estado terminal.

Funções:
- Desenergizar saídas
- Interromper execução
- Proteger o sistema

Regras:
- STOP ignora regras do usuário
- STOP prioriza segurança física
- STOP requer reset explícito para sair

---

## Transições Permitidas

| Origem | Destino | Condição |
|------|--------|---------|
| INIT | RUN | Health-check OK |
| INIT | SAFE | Falha recuperável |
| INIT | STOP | Falha crítica |
| RUN | PAUSE | Solicitação válida |
| RUN | FAULT | Falha detectada |
| PAUSE | RUN | Condições restauradas |
| FAULT | PAUSE | Falha tratável |
| FAULT | SAFE | Falha crítica |
| SAFE | STOP | Política exigir |
| SAFE | INIT | Reset autorizado |

---

## Autoridade de Transição

- **Core**: autoridade primária
- **Gateway**: autoridade condicionada
- **Usuário**: apenas via lógica permitida

Nenhuma entidade tem autoridade absoluta.

---

## Auditoria e Rastreamento

Toda transição deve gerar:
- Timestamp
- Estado anterior
- Estado novo
- Causa
- Origem (slot, core, gateway, energia)

Sem exceções.

---

## Princípio Central

> “Um CLP não falha quando entra em FAULT.  
> Ele falha quando entra em um estado indefinido.”

## State Invariants

- RUN só é permitido se:
  - Todos os slots críticos estiverem HEALTHY
  - Nenhuma falha de segurança estiver ativa

- STOP:
  - Ignora qualquer regra de slot
  - Sempre força o sistema para SAFE

- PAUSE:
  - Pode ser acionado a qualquer momento
  - Não pode violar regras hard de segurança

- INIT:
  - Executa Health Check obrigatório

| From | To    | Allowed | Condition                          |
|------|-------|---------|------------------------------------|
| INIT | RUN   | Yes     | Health Check OK                    |
| RUN  | PAUSE | Yes     | -                                  |
| RUN  | STOP  | Yes     | Always                             |
| FAULT| RUN   | No      | Must go through SAFE               |

