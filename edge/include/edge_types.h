#ifndef EDGE_TYPES_H
#define EDGE_TYPES_H

/*
 * Edge Runtime – Core Types
 *
 * Este arquivo define os tipos fundamentais do Edge Runtime.
 * Ele NÃO depende de RTOS, HAL ou hardware específico.
 *
 * Tudo aqui é contrato.
 */

#include <stdint.h>
#include <stdbool.h>

/* ============================================================
 * Versionamento do Contrato
 * ============================================================ */

#define EDGE_API_VERSION_MAJOR  1
#define EDGE_API_VERSION_MINOR  0
#define EDGE_API_VERSION_PATCH  0

/* ============================================================
 * Tipos Fundamentais
 * ============================================================ */

/* Identificadores explícitos */
typedef uint32_t edge_id_t;
typedef uint32_t edge_slot_id_t;
typedef uint32_t edge_event_id_t;

/* Tempo em microssegundos (base determinística) */
typedef uint64_t edge_time_us_t;

/* ============================================================
 * Níveis de Severidade
 * ============================================================ */

typedef enum
{
    EDGE_SEVERITY_INFO = 0,
    EDGE_SEVERITY_WARNING,
    EDGE_SEVERITY_ALARM,
    EDGE_SEVERITY_FAULT,
    EDGE_SEVERITY_CRITICAL
} edge_severity_t;

/* ============================================================
 * Autoridade da Ação
 * ============================================================ */

typedef enum
{
    EDGE_AUTH_INTERNAL = 0,   /* Core */
    EDGE_AUTH_SLOT,           /* Slot */
    EDGE_AUTH_GATEWAY,        /* Gateway */
    EDGE_AUTH_USER             /* Lógica do usuário */
} edge_authority_t;

/* ============================================================
 * Origem de Evento ou Falha
 * ============================================================ */

typedef enum
{
    EDGE_ORIGIN_CORE = 0,
    EDGE_ORIGIN_SLOT,
    EDGE_ORIGIN_GATEWAY,
    EDGE_ORIGIN_POWER,
    EDGE_ORIGIN_UNKNOWN
} edge_origin_t;

/* ============================================================
 * Estados Operacionais do Sistema
 * (definição semântica completa em STATES.md)
 * ============================================================ */

typedef enum
{
    EDGE_STATE_INIT = 0,
    EDGE_STATE_RUN,
    EDGE_STATE_PAUSE,
    EDGE_STATE_FAULT,
    EDGE_STATE_SAFE,
    EDGE_STATE_STOP
} edge_state_t;

/* ============================================================
 * Resultado de Operação
 * ============================================================ */

typedef enum
{
    EDGE_RESULT_OK = 0,
    EDGE_RESULT_DENIED,
    EDGE_RESULT_INVALID,
    EDGE_RESULT_TIMEOUT,
    EDGE_RESULT_UNSUPPORTED,
    EDGE_RESULT_ERROR
} edge_result_t;

/* ============================================================
 * Flags de Comportamento
 * ============================================================ */

typedef uint32_t edge_flags_t;

#define EDGE_FLAG_NONE              0x00000000
#define EDGE_FLAG_DETERMINISTIC     0x00000001
#define EDGE_FLAG_NON_DETERMINISTIC 0x00000002
#define EDGE_FLAG_SAFETY_CRITICAL   0x00000004
#define EDGE_FLAG_AUDIT_REQUIRED    0x00000008

/* ============================================================
 * Evento Genérico
 * ============================================================ */

typedef struct
{
    edge_event_id_t   id;
    edge_time_us_t    timestamp;
    edge_severity_t   severity;
    edge_origin_t     origin;
    edge_authority_t  authority;
    edge_flags_t      flags;
} edge_event_t;

/* ============================================================
 * Snapshot de Estado
 * ============================================================ */

typedef struct
{
    edge_state_t     state;
    edge_time_us_t   timestamp;
    edge_flags_t     flags;
} edge_state_snapshot_t;

/* ============================================================
 * Política de Falha (genérica)
 * ============================================================ */

typedef enum
{
    EDGE_FAULT_IGNORE = 0,
    EDGE_FAULT_PAUSE,
    EDGE_FAULT_SAFE,
    EDGE_FAULT_STOP
} edge_fault_policy_t;

/* ============================================================
 * Descritor Genérico de Falha
 * ============================================================ */

typedef struct
{
    edge_id_t             code;
    edge_severity_t       severity;
    edge_origin_t         origin;
    edge_fault_policy_t   policy;
    edge_time_us_t        timestamp;
} edge_fault_t;

/* ============================================================
 * Utilitários Básicos
 * ============================================================ */

/* Comparação segura de flags */
static inline bool edge_flag_is_set(edge_flags_t flags, edge_flags_t mask)
{
    return ((flags & mask) == mask);
}

#endif /* EDGE_TYPES_H */
