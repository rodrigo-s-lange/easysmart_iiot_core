#ifndef EDGE_STATES_H
#define EDGE_STATES_H

/*
 * Edge Runtime – State Machine Contract
 *
 * Este arquivo define as regras formais da máquina de estados do Edge Runtime.
 * Ele NÃO implementa lógica, apenas declara o que é permitido.
 *
 * Implementações (Zephyr ou não) DEVEM respeitar este contrato.
 */

#include "edge_types.h"

/* ============================================================
 * Transições de Estado Permitidas
 * ============================================================ */

/*
 * Estrutura que descreve uma transição válida entre estados.
 * Usada para validação, auditoria e certificação.
 */
typedef struct
{
    edge_state_t    from;
    edge_state_t    to;
    edge_authority_t authority;
} edge_state_transition_t;

/*
 * Tabela normativa de transições permitidas.
 * Qualquer transição fora desta tabela é inválida.
 */
static const edge_state_transition_t edge_allowed_transitions[] =
{
    /* INIT */
    { EDGE_STATE_INIT,  EDGE_STATE_RUN,   EDGE_AUTH_INTERNAL },
    { EDGE_STATE_INIT,  EDGE_STATE_SAFE,  EDGE_AUTH_INTERNAL },
    { EDGE_STATE_INIT,  EDGE_STATE_STOP,  EDGE_AUTH_INTERNAL },

    /* RUN */
    { EDGE_STATE_RUN,   EDGE_STATE_PAUSE, EDGE_AUTH_INTERNAL },
    { EDGE_STATE_RUN,   EDGE_STATE_FAULT, EDGE_AUTH_INTERNAL },

    /* PAUSE */
    { EDGE_STATE_PAUSE, EDGE_STATE_RUN,   EDGE_AUTH_INTERNAL },
    { EDGE_STATE_PAUSE, EDGE_STATE_SAFE,  EDGE_AUTH_INTERNAL },

    /* FAULT */
    { EDGE_STATE_FAULT, EDGE_STATE_PAUSE, EDGE_AUTH_INTERNAL },
    { EDGE_STATE_FAULT, EDGE_STATE_SAFE,  EDGE_AUTH_INTERNAL },
    { EDGE_STATE_FAULT, EDGE_STATE_STOP,  EDGE_AUTH_INTERNAL },

    /* SAFE */
    { EDGE_STATE_SAFE,  EDGE_STATE_STOP,  EDGE_AUTH_INTERNAL },
    { EDGE_STATE_SAFE,  EDGE_STATE_INIT,  EDGE_AUTH_INTERNAL },
};

/* ============================================================
 * Regras de Estado
 * ============================================================ */

/*
 * Estados que permitem execução de lógica do usuário
 */
static inline bool edge_state_allows_user_logic(edge_state_t state)
{
    return (state == EDGE_STATE_RUN);
}

/*
 * Estados que permitem alteração de configuração
 */
static inline bool edge_state_allows_reconfiguration(edge_state_t state)
{
    return (state == EDGE_STATE_INIT ||
            state == EDGE_STATE_PAUSE);
}

/*
 * Estados que permitem comunicação externa
 */
static inline bool edge_state_allows_gateway(edge_state_t state)
{
    return (state != EDGE_STATE_STOP);
}

/*
 * Estados considerados seguros para atualização
 */
static inline bool edge_state_allows_update(edge_state_t state)
{
    return (state == EDGE_STATE_PAUSE);
}

/* ============================================================
 * Validação de Transição
 * ============================================================ */

/*
 * Verifica se uma transição é permitida segundo o contrato.
 */
static inline bool edge_state_transition_allowed(edge_state_t from,
                                                  edge_state_t to,
                                                  edge_authority_t authority)
{
    for (uint32_t i = 0;
         i < (sizeof(edge_allowed_transitions) /
              sizeof(edge_allowed_transitions[0]));
         i++)
    {
        const edge_state_transition_t *t = &edge_allowed_transitions[i];

        if ((t->from == from) &&
            (t->to == to) &&
            (t->authority == authority))
        {
            return true;
        }
    }

    return false;
}

#endif /* EDGE_STATES_H */
