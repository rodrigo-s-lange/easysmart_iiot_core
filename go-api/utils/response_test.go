package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteErrorStandardEnvelope(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	w.Header().Set("X-Request-ID", "req-123")
	WriteError(w, http.StatusNotFound, "resource not found")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	var body ErrorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if body.Code != "not_found" {
		t.Fatalf("code = %q, want %q", body.Code, "not_found")
	}
	if body.Message != "resource not found" {
		t.Fatalf("message = %q", body.Message)
	}
	if body.RequestID != "req-123" {
		t.Fatalf("request_id = %q", body.RequestID)
	}
}
