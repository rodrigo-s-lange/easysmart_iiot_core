package middleware

import (
	"context"
	"testing"
)

func TestValidateAPIKeyRejectsShortKey(t *testing.T) {
	t.Parallel()

	m := &APIKeyMiddleware{}
	if _, err := m.validateAPIKey(context.Background(), "short"); err == nil {
		t.Fatalf("validateAPIKey expected error for short key")
	}
}
