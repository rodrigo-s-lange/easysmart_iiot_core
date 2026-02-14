package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"iiot-go-api/config"
	"iiot-go-api/utils"
)

func testAuthHandler() *AuthHandler {
	return &AuthHandler{
		Config: &config.Config{
			JWTSecret:            "test-jwt-secret",
			JWTAccessExpiration:  time.Hour,
			JWTRefreshExpiration: 24 * time.Hour,
		},
	}
}

func TestRegisterRejectsInvalidEmail(t *testing.T) {
	t.Parallel()

	h := testAuthHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"email":"invalid","password":"Abcdef1!"}`))
	w := httptest.NewRecorder()

	h.Register(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRegisterRejectsWeakPassword(t *testing.T) {
	t.Parallel()

	h := testAuthHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"email":"user@example.com","password":"weak"}`))
	w := httptest.NewRecorder()

	h.Register(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestLoginRejectsInvalidBody(t *testing.T) {
	t.Parallel()

	h := testAuthHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{`))
	w := httptest.NewRecorder()

	h.Login(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRefreshRejectsAccessToken(t *testing.T) {
	t.Parallel()

	h := testAuthHandler()
	accessToken, err := utils.GenerateJWT(
		h.Config.JWTSecret,
		"access",
		"user-1",
		"tenant-1",
		"user@example.com",
		"tenant_admin",
		[]string{"telemetry:read"},
		time.Hour,
	)
	if err != nil {
		t.Fatalf("GenerateJWT error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", strings.NewReader(`{"refresh_token":"`+accessToken+`"}`))
	w := httptest.NewRecorder()

	h.Refresh(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}
