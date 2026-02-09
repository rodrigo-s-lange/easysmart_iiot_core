package middleware

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TenantContextMiddleware struct {
	DB *pgxpool.Pool
}

func NewTenantContextMiddleware(db *pgxpool.Pool) *TenantContextMiddleware {
	return &TenantContextMiddleware{DB: db}
}

func (m *TenantContextMiddleware) SetContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := r.Context().Value("tenant_id").(string)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		role, _ := r.Context().Value("role").(string)

		// Start transaction for RLS
		tx, err := m.DB.Begin(r.Context())
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback(r.Context())

		// Set session variables (use set_config to allow parameters)
		_, err = tx.Exec(r.Context(), "SELECT set_config('app.current_tenant_id', $1, true)", tenantID)
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		_, err = tx.Exec(r.Context(), "SELECT set_config('app.current_user_role', $1, true)", role)
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		// Add transaction to context
		ctx := context.WithValue(r.Context(), "db_tx", tx)

		next.ServeHTTP(w, r.WithContext(ctx))

		// Commit transaction
		if err := tx.Commit(r.Context()); err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
		}
	})
}
