package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimitAuthAllowsWhenRedisNil(t *testing.T) {
	t.Parallel()

	rl := NewRateLimitAuth(nil, 10, 60)
	h := rl.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}
