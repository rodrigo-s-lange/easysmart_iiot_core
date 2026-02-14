package middleware

import (
	"iiot-go-api/utils"
	"net/http"
	"strings"
)

// RequireMethods enforces an allow-list of HTTP methods for a route.
// OPTIONS is always allowed for CORS preflight compatibility.
func RequireMethods(methods ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(methods))
	for _, m := range methods {
		allowed[strings.ToUpper(strings.TrimSpace(m))] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			if _, ok := allowed[r.Method]; !ok {
				w.Header().Set("Allow", strings.Join(methods, ", "))
				utils.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
