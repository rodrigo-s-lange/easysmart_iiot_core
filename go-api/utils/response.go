package utils

import (
	"encoding/json"
	"net/http"
)

type ErrorEnvelope struct {
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	RequestID string      `json:"request_id,omitempty"`
	Details   interface{} `json:"details,omitempty"`
}

// WriteJSON writes JSON response
func WriteJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func errorCodeFromStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	default:
		return "internal_error"
	}
}

// WriteError writes error response
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteErrorWithDetails(w, status, errorCodeFromStatus(status), message, nil)
}

// WriteErrorWithCode writes standardized error response with explicit code.
func WriteErrorWithCode(w http.ResponseWriter, status int, code, message string) {
	WriteErrorWithDetails(w, status, code, message, nil)
}

// WriteErrorWithDetails writes standardized error response with details payload.
func WriteErrorWithDetails(w http.ResponseWriter, status int, code, message string, details interface{}) {
	resp := ErrorEnvelope{
		Code:    code,
		Message: message,
		Details: details,
	}
	if reqID := w.Header().Get("X-Request-ID"); reqID != "" {
		resp.RequestID = reqID
	}
	WriteJSON(w, status, resp)
}
