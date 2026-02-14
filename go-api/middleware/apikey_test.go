package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateAPIKeyRejectsShortKey(t *testing.T) {
	t.Parallel()

	m := &APIKeyMiddleware{}
	if _, err := m.validateAPIKey(context.Background(), "short"); err == nil {
		t.Fatalf("validateAPIKey expected error for short key")
	}
}

func TestAPIKeyAuthenticateHeaderValidation(t *testing.T) {
	t.Parallel()

	m := &APIKeyMiddleware{}
	h := m.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	reqMissing := httptest.NewRequest(http.MethodPost, "/api/telemetry", nil)
	wMissing := httptest.NewRecorder()
	h.ServeHTTP(wMissing, reqMissing)
	if wMissing.Code != http.StatusUnauthorized {
		t.Fatalf("missing header status = %d, want %d", wMissing.Code, http.StatusUnauthorized)
	}

	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/telemetry", nil)
	reqInvalid.Header.Set("Authorization", "ApiKey abc")
	wInvalid := httptest.NewRecorder()
	h.ServeHTTP(wInvalid, reqInvalid)
	if wInvalid.Code != http.StatusUnauthorized {
		t.Fatalf("invalid format status = %d, want %d", wInvalid.Code, http.StatusUnauthorized)
	}
}
