package middleware

import (
	"iiot-go-api/utils"
	"net/http"
)

func RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			permissions, ok := r.Context().Value("permissions").([]string)
			if !ok {
				utils.WriteError(w, http.StatusForbidden, "No permissions found")
				return
			}

			// Check if permission exists
			hasPermission := false
			for _, p := range permissions {
				if p == permission || p == "system:admin" {
					hasPermission = true
					break
				}
			}

			if !hasPermission {
				utils.WriteError(w, http.StatusForbidden, "Insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
