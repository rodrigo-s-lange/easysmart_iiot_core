#ifndef EDGE_ERRORS_H
#define EDGE_ERRORS_H

/*
 * Edge Runtime – Error and Fault Definitions
 *
 * Este arquivo define a taxonomia oficial de erros, falhas e violações
 * do Edge Runtime.
 *
 * Objetivo:
 * - Padronização
 * - Auditoria
 * - Segurança funcional
 * - Integração com logs, gateway e ML
 *
 * Nenhum erro é "genérico".
 * Todo erro carrega semântica.
 */

#include "edge_types.h"

/* ============================================================
 * Classes de Erro
 * ============================================================ */

typedef enum
{
    EDGE_ERROR_CLASS_NONE = 0,

    /* Erros operacionais (não são falhas de segurança) */
    EDGE_ERROR_CLASS_RUNTIME,
    EDGE_ERROR_CLASS_COMMUNICATION,
    EDGE_ERROR_CLASS_RESOURCE,

    /* Falhas funcionais */
    EDGE_ERROR_CLASS_FAULT,
    EDGE_ERROR_CLASS_SAFETY,

    /* Violações de contrato */
    EDGE_ERROR_CLASS_VIOLATION

} edge_error_class_t;

/* ============================================================
 * Código de Erro Padronizado
 * ============================================================ */

typedef struct
{
    edge_error_class_t class_id;
    edge_id_t          code;
} edge_error_code_t;

/* ============================================================
 * Erros Operacionais (Runtime)
 * ============================================================ */

#define EDGE_ERR_RUNTIME_TIMEOUT          0x0001
#define EDGE_ERR_RUNTIME_OVERFLOW         0x0002
#define EDGE_ERR_RUNTIME_UNDERFLOW        0x0003
#define EDGE_ERR_RUNTIME_INVALID_STATE    0x0004

/* ============================================================
 * Erros de Comunicação
 * ============================================================ */

#define EDGE_ERR_COMM_LOST                0x0101
#define EDGE_ERR_COMM_CRC                 0x0102
#define EDGE_ERR_COMM_PROTOCOL            0x0103

/* ============================================================
 * Erros de Recursos
 * ============================================================ */

#define EDGE_ERR_RESOURCE_MEMORY          0x0201
#define EDGE_ERR_RESOURCE_CPU             0x0202
#define EDGE_ERR_RESOURCE_QUEUE_FULL      0x0203

/* ============================================================
 * Falhas Funcionais
 * ============================================================ */

#define EDGE_FAULT_SLOT_FAILURE           0x1001
#define EDGE_FAULT_IO_FAILURE             0x1002
#define EDGE_FAULT_POWER_FAILURE          0x1003
#define EDGE_FAULT_CLOCK_FAILURE          0x1004

/* ============================================================
 * Falhas de Segurança
 * ============================================================ */

#define EDGE_FAULT_SAFETY_LIMIT           0x2001
#define EDGE_FAULT_SAFETY_OVERRIDE        0x2002
#define EDGE_FAULT_SAFETY_INTEGRITY       0x2003

/* ============================================================
 * Violações de Contrato
 * ============================================================ */

#define EDGE_VIOLATION_INVALID_TRANSITION 0xF001
#define EDGE_VIOLATION_UNAUTHORIZED       0xF002
#define EDGE_VIOLATION_INVALID_SLOT       0xF003
#define EDGE_VIOLATION_POLICY_BREACH      0xF004

/* ============================================================
 * Descritor Completo de Erro/Falha
 * ============================================================ */

typedef struct
{
    edge_error_code_t   error;
    edge_severity_t     severity;
    edge_origin_t       origin;
    edge_authority_t    authority;
    edge_fault_policy_t policy;
    edge_time_us_t      timestamp;
} edge_error_t;

/* ============================================================
 * Utilitários Semânticos
 * ============================================================ */

/* Indica se o erro é uma falha funcional */
static inline bool edge_error_is_fault(edge_error_class_t class_id)
{
    return (class_id == EDGE_ERROR_CLASS_FAULT ||
            class_id == EDGE_ERROR_CLASS_SAFETY);
}

/* Indica se o erro representa violação grave */
static inline bool edge_error_is_violation(edge_error_class_t class_id)
{
    return (class_id == EDGE_ERROR_CLASS_VIOLATION);
}

#endif /* EDGE_ERRORS_H */
