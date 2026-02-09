package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"iiot-go-api/utils"
)

// Recover catches panics and returns a JSON 500.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic", slog.Any("error", rec), slog.String("stack", string(debug.Stack())))
				utils.WriteError(w, http.StatusInternalServerError, "Internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
