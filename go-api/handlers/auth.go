package handlers

import (
	"context"
	"encoding/json"
	"iiot-go-api/config"
	"iiot-go-api/models"
	"iiot-go-api/utils"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	DB     *pgxpool.Pool
	Config *config.Config
}

func NewAuthHandler(db *pgxpool.Pool, cfg *config.Config) *AuthHandler {
	return &AuthHandler{DB: db, Config: cfg}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Find user
	var user models.User
	err := h.DB.QueryRow(context.Background(), `
		SELECT user_id, tenant_id, email, password_hash, role, status
		FROM users_v2
		WHERE email = $1
	`, req.Email).Scan(&user.UserID, &user.TenantID, &user.Email, &user.PasswordHash, &user.Role, &user.Status)

	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	if user.Status != "active" {
		utils.WriteError(w, http.StatusForbidden, "Account is not active")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Get permissions
	permissions, err := h.getPermissions(user.Role)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Generate JWT
	tenantID := ""
	if user.TenantID != nil {
		tenantID = *user.TenantID
	}

	accessToken, err := utils.GenerateJWT(
		h.Config.JWTSecret,
		user.UserID,
		tenantID,
		user.Email,
		user.Role,
		permissions,
		h.Config.JWTAccessExpiration,
	)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	refreshToken, err := utils.GenerateJWT(
		h.Config.JWTSecret,
		user.UserID,
		tenantID,
		user.Email,
		user.Role,
		permissions,
		h.Config.JWTRefreshExpiration,
	)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Update last_login_at
	h.DB.Exec(context.Background(), "UPDATE users_v2 SET last_login_at = NOW() WHERE user_id = $1", user.UserID)

	utils.WriteJSON(w, http.StatusOK, models.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(h.Config.JWTAccessExpiration.Seconds()),
		User:         user,
	})
}

func (h *AuthHandler) getPermissions(role string) ([]string, error) {
	rows, err := h.DB.Query(context.Background(), `
		SELECT p.name
		FROM role_permissions rp
		JOIN permissions p ON rp.permission_id = p.permission_id
		WHERE rp.role = $1
	`, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []string
	for rows.Next() {
		var perm string
		if err := rows.Scan(&perm); err != nil {
			return nil, err
		}
		permissions = append(permissions, perm)
	}

	return permissions, nil
}
