#ifndef EDGE_SLOTS_H
#define EDGE_SLOTS_H

/*
 * Edge Runtime – Slot Contract
 *
 * Este arquivo define o contrato formal de um Slot.
 * Slots são unidades isoladas, versionadas e auditáveis.
 *
 * Este contrato é independente de RTOS e HAL.
 */

#include "edge_types.h"
#include "edge_errors.h"

/* ============================================================
 * Versão do Contrato de Slot
 * ============================================================ */

#define EDGE_SLOT_CONTRACT_VERSION  1

/* ============================================================
 * Tipo de Slot
 * ============================================================ */

typedef enum
{
    EDGE_SLOT_TYPE_INPUT = 0,
    EDGE_SLOT_TYPE_OUTPUT,
    EDGE_SLOT_TYPE_SENSOR,
    EDGE_SLOT_TYPE_ACTUATOR,
    EDGE_SLOT_TYPE_LOGIC,
    EDGE_SLOT_TYPE_ML
} edge_slot_type_t;

/* ============================================================
 * Classe de Execução do Slot
 * ============================================================ */

typedef enum
{
    EDGE_SLOT_EXEC_EVENT_DRIVEN = 0,
    EDGE_SLOT_EXEC_CYCLIC,
    EDGE_SLOT_EXEC_HYBRID
} edge_slot_exec_model_t;

/* ============================================================
 * Capacidades do Slot
 * ============================================================ */

typedef struct
{
    edge_flags_t           flags;          /* determinístico, safety, etc */
    edge_slot_exec_model_t exec_model;
    uint32_t               max_frequency_hz;
    uint32_t               min_latency_us;
} edge_slot_capabilities_t;

/* ============================================================
 * Configuração Base do Slot
 * ============================================================ */

typedef struct
{
    edge_slot_id_t         slot_id;
    edge_slot_type_t       type;
    uint32_t               version;
    edge_fault_policy_t    fault_policy;
} edge_slot_config_t;

/* ============================================================
 * Callbacks Obrigatórios
 * ============================================================ */

/*
 * Inicialização do Slot
 * Executada em INIT
 */
typedef edge_result_t (*edge_slot_init_fn)(const edge_slot_config_t *cfg);

/*
 * Execução principal
 * Chamada conforme modelo de execução declarado
 */
typedef edge_result_t (*edge_slot_exec_fn)(edge_time_us_t now);

/*
 * Tratamento de falha
 */
typedef void (*edge_slot_fault_fn)(const edge_error_t *error);

/*
 * Snapshot do estado interno do Slot
 */
typedef void (*edge_slot_snapshot_fn)(void *buffer, uint32_t buffer_size);

/* ============================================================
 * Descritor de Slot
 * ============================================================ */

typedef struct
{
    edge_slot_config_t        config;
    edge_slot_capabilities_t  caps;

    edge_slot_init_fn         init;
    edge_slot_exec_fn         exec;
    edge_slot_fault_fn        on_fault;
    edge_slot_snapshot_fn     snapshot;
} edge_slot_t;

/* ============================================================
 * Regras Semânticas
 * ============================================================ */

/*
 * Slot pode executar lógica do usuário?
 */
static inline bool edge_slot_allows_user_logic(const edge_slot_t *slot)
{
    return (slot->config.type == EDGE_SLOT_TYPE_LOGIC ||
            slot->config.type == EDGE_SLOT_TYPE_ML);
}

/*
 * Slot é considerado determinístico?
 */
static inline bool edge_slot_is_deterministic(const edge_slot_t *slot)
{
    return edge_flag_is_set(slot->caps.flags, EDGE_FLAG_DETERMINISTIC);
}

/*
 * Slot é crítico de segurança?
 */
static inline bool edge_slot_is_safety_critical(const edge_slot_t *slot)
{
    return edge_flag_is_set(slot->caps.flags, EDGE_FLAG_SAFETY_CRITICAL);
}

#endif /* EDGE_SLOTS_H */
