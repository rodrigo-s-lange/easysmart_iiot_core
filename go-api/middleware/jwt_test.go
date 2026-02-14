package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"iiot-go-api/utils"
)

func TestJWTMiddlewareAccessToken(t *testing.T) {
	t.Parallel()

	secret := "test-jwt-secret"
	token, err := utils.GenerateJWT(secret, "access", "u1", "t1", "user@example.com", "tenant_admin", []string{"devices:read"}, time.Hour)
	if err != nil {
		t.Fatalf("GenerateJWT error: %v", err)
	}

	mw := NewJWTMiddleware(secret)
	h := mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Context().Value("user_id"); got != "u1" {
			t.Fatalf("user_id in context = %v, want u1", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestJWTMiddlewareRejectsRefreshToken(t *testing.T) {
	t.Parallel()

	secret := "test-jwt-secret"
	token, err := utils.GenerateJWT(secret, "refresh", "u1", "t1", "user@example.com", "tenant_admin", []string{"devices:read"}, time.Hour)
	if err != nil {
		t.Fatalf("GenerateJWT error: %v", err)
	}

	mw := NewJWTMiddleware(secret)
	h := mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestJWTMiddlewareRejectsTamperedToken(t *testing.T) {
	t.Parallel()

	secret := "test-jwt-secret"
	token, err := utils.GenerateJWT(secret, "access", "u1", "t1", "user@example.com", "tenant_admin", []string{"devices:read"}, time.Hour)
	if err != nil {
		t.Fatalf("GenerateJWT error: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("invalid token format")
	}
	tampered := parts[0] + "." + parts[1] + ".tampered-signature"

	mw := NewJWTMiddleware(secret)
	h := mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req.Header.Set("Authorization", "Bearer "+tampered)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}
