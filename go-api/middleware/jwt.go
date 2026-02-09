package middleware

import (
	"context"
	"iiot-go-api/utils"
	"net/http"
	"strings"
)

type JWTMiddleware struct {
	Secret string
}

func NewJWTMiddleware(secret string) *JWTMiddleware {
	return &JWTMiddleware{Secret: secret}
}

func (m *JWTMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			utils.WriteError(w, http.StatusUnauthorized, "Missing authorization header")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			utils.WriteError(w, http.StatusUnauthorized, "Invalid authorization format")
			return
		}

		claims, err := utils.ValidateJWT(m.Secret, parts[1])
		if err != nil {
			utils.WriteError(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		// Add claims to context
		ctx := r.Context()
		ctx = context.WithValue(ctx, "jwt_claims", claims)
		ctx = context.WithValue(ctx, "user_id", claims["user_id"].(string))
		ctx = context.WithValue(ctx, "tenant_id", claims["tenant_id"].(string))
		ctx = context.WithValue(ctx, "role", claims["role"].(string))

		permissions := []string{}
		if perms, ok := claims["permissions"].([]interface{}); ok {
			for _, p := range perms {
				if str, ok := p.(string); ok {
					permissions = append(permissions, str)
				}
			}
		}
		ctx = context.WithValue(ctx, "permissions", permissions)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
