# Segurança Funcional – Edge Runtime

Segurança não é uma feature.
Segurança é uma **condição necessária de existência** do Edge Runtime.

Este diretório documenta os princípios que orientam decisões técnicas,
mesmo quando não explicitamente codificados.

---

## Princípio Zero: Não Confiar

O Edge Runtime não confia em:

- Entradas externas
- Slots
- Gateway
- Usuário
- Rede
- Energia
- Hardware periférico

Tudo é validado.
Tudo é limitado.
Tudo pode falhar.

---

## Segurança vs Disponibilidade

Em conflito entre:

- Continuar operando
- Garantir segurança

**Segurança sempre vence**.

Disponibilidade é desejável.
Segurança é obrigatória.

---

## Falhas e Comportamento Seguro

Falhas são classificadas e tratadas conforme:

- Origem
- Impacto
- Política definida
- Estado atual do sistema

Nem toda falha leva a STOP.
Mas toda falha gera consequência.

---

## Autoridade e Limites

- O usuário define regras **dentro de limites**
- O gateway solicita ações **condicionadas**
- O Core decide **sempre**

Não existe override absoluto.

---

## Slots e Segurança

Slots:
- Não executam decisões globais
- Não alteram políticas
- Não mascaram falhas
- Não operam fora de contrato

Um Slot inseguro é isolado ou removido.

---

## Atualizações

Atualizações:
- Só ocorrem em PAUSE
- Nunca em RUN
- São auditáveis
- Podem ser revertidas

---

## Auditoria

Eventos relevantes devem registrar:

- O que aconteceu
- Quando aconteceu
- Por que aconteceu
- Quem originou

Logs são parte da segurança.

---

## Normas e Referências

Este projeto é **compatível em conceito** com:

- IEC 61508 – Segurança funcional
- IEC 61131 – Controle programável
- IEC 62443 – Segurança cibernética

A conformidade formal depende do processo e aplicação final.

---

## Princípio Final

> “Um sistema industrial não é seguro porque nunca falha.  
> Ele é seguro porque sabe exatamente como falhar.”