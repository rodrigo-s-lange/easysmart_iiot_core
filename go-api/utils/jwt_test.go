package utils

import (
	"testing"
	"time"
)

func TestGenerateAndValidateJWT(t *testing.T) {
	t.Parallel()

	secret := "test-jwt-secret"
	token, err := GenerateJWT(secret, "access", "user-1", "tenant-1", "user@example.com", "tenant_admin", []string{"devices:read"}, time.Hour)
	if err != nil {
		t.Fatalf("GenerateJWT error: %v", err)
	}

	claims, err := ValidateJWT(secret, token)
	if err != nil {
		t.Fatalf("ValidateJWT error: %v", err)
	}

	if claims.UserID != "user-1" {
		t.Fatalf("claims.UserID = %s, want user-1", claims.UserID)
	}
	if claims.TokenType != "access" {
		t.Fatalf("claims.TokenType = %s, want access", claims.TokenType)
	}
}

func TestValidateJWTWrongSecret(t *testing.T) {
	t.Parallel()

	token, err := GenerateJWT("secret-a", "access", "user-1", "tenant-1", "user@example.com", "tenant_admin", []string{"devices:read"}, time.Hour)
	if err != nil {
		t.Fatalf("GenerateJWT error: %v", err)
	}

	if _, err := ValidateJWT("secret-b", token); err == nil {
		t.Fatalf("ValidateJWT expected error with wrong secret")
	}
}
