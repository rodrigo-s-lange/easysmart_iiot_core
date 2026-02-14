package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequirePermission(t *testing.T) {
	t.Parallel()

	h := RequirePermission("devices:read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req = req.WithContext(context.WithValue(req.Context(), "permissions", []string{"devices:read"}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
	}

	reqDenied := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	reqDenied = reqDenied.WithContext(context.WithValue(reqDenied.Context(), "permissions", []string{"telemetry:read"}))
	wDenied := httptest.NewRecorder()
	h.ServeHTTP(wDenied, reqDenied)
	if wDenied.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", wDenied.Code, http.StatusForbidden)
	}
}
