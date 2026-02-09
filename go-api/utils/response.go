package utils

import (
	"encoding/json"
	"net/http"
)

// WriteJSON writes JSON response
func WriteJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

// WriteError writes error response
func WriteError(w http.ResponseWriter, status int, message string) {
	resp := map[string]string{"error": message}
	if reqID := w.Header().Get("X-Request-ID"); reqID != "" {
		resp["request_id"] = reqID
	}
	WriteJSON(w, status, resp)
}
