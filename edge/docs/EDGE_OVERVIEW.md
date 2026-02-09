# Edge Runtime – Visão Geral

O **Edge Runtime** é o núcleo de execução industrial responsável por controle local,
determinístico e seguro de processos, atuando entre o hardware físico e os sistemas
de nível superior (gateway, supervisório, nuvem).

Ele foi projetado para:

- Operar de forma autônoma
- Tolerar falhas de comunicação
- Permitir auditoria técnica e jurídica
- Escalar do dispositivo simples ao CLP modular avançado

---

## Posicionamento na Arquitetura

O Edge Runtime ocupa a camada **Edge / Control Plane**:

Backend / Cloud / SaaS
↑
Gateway (comunicação, OTA, políticas)
↑
Edge Runtime (este projeto)
↑
Hardware / Slots / I/O físico


O Edge **não depende** do gateway para operar.
O gateway **nunca executa lógica de controle**.

---

## Core vs Gateway

### Core Lógico (Edge Core)

- Executa a lógica de controle do usuário
- Gerencia estados operacionais
- Orquestra slots e eventos
- Mantém determinismo temporal
- Decide comportamento sob falha

### Gateway (fora do escopo)

- Comunicação externa
- Atualizações
- Telemetria
- Integração IIoT
- Observabilidade

> O gateway pode **monitorar e solicitar ações**,  
> mas o Core decide **se a ação é permitida**.

---

## Modelo de Execução

O Edge adota um modelo **híbrido**:

- **Event-driven** para respostas rápidas (interrupções, falhas, limites)
- **Cíclico (scan)** quando necessário para controle contínuo
- A escolha é **explícita**, nunca implícita

Determinismo só é assumido quando **garantido por contrato**.

---

## Estados do Sistema

O Edge opera com estados bem definidos:

- INIT
- RUN
- PAUSE
- FAULT
- STOP
- SAFE

Cada transição é controlada e auditável.
Nada muda de estado sem uma causa clara.

(Detalhado em `STATES.md`)

---

## Slots de Expansão

Toda funcionalidade de I/O e processamento periférico é modelada como **Slot**.

- Slots possuem contratos formais
- Slots podem falhar isoladamente
- Slots não violam segurança global
- Slots de expansão podem usar barramentos dedicados (ex: CAN-FD)

(Detalhado em `SLOTS_V1.md`)

---

## Segurança como Requisito de Projeto

Este runtime não confia:

- No usuário
- No gateway
- Na rede
- No hardware periférico
- Em dados externos

Tudo é validado.
Tudo é limitado.
Tudo é registrável.

Segurança funcional não é um módulo — é um **princípio transversal**.

---

## Escopo de Certificação

O Edge Runtime foi concebido para **não impedir** certificações como:

- IEC 61131 (em partes)
- IEC 61508 (conceitualmente)
- IEC 62443 (arquiteturalmente)

A certificação final depende:
- Do hardware
- Do processo
- Do uso final

Este projeto fornece **base técnica compatível**, não um selo automático.

---

## Frase Norteadora

> “Não é o Edge que decide o que é seguro.  
> Ele apenas garante que nada inseguro passe despercebido.”

## Non-Goals

O Edge NÃO é:

- Um PLC clássico com Ladder
- Um runtime genérico de scripts
- Um gateway de protocolo
- Um sistema de automação monolítico

Decisões rejeitadas explicitamente:
- Ladder / FBD / ST como linguagem primária
- Execução dinâmica de código do usuário
- Dependência direta de backend ou cloud