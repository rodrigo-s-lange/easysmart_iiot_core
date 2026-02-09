package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"iiot-go-api/config"
	"iiot-go-api/models"
	"iiot-go-api/utils"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	DB     *pgxpool.Pool
	Redis  *redis.Client
	Config *config.Config
}

func NewAuthHandler(db *pgxpool.Pool, redisClient *redis.Client, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		DB:     db,
		Redis:  redisClient,
		Config: cfg,
	}
}

// Register creates a new user account
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate email format
	if !isValidEmail(req.Email) {
		utils.WriteError(w, http.StatusBadRequest, "Invalid email format")
		return
	}

	// Validate password strength
	if err := validatePassword(req.Password); err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Normalize email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	ctx := context.Background()
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	defer tx.Rollback(ctx)

	// Check if email already exists
	var exists bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users_v2 WHERE email = $1)", req.Email).Scan(&exists)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	if exists {
		utils.WriteError(w, http.StatusConflict, "Email already registered")
		return
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Check if this is the first user (should become super_admin)
	var userCount int
	err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM users_v2").Scan(&userCount)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	role := "tenant_user"
	var tenantID *string

	if userCount == 0 {
		// First user becomes super_admin with no tenant
		role = "super_admin"
		tenantID = nil
	} else {
		// Create or get tenant for new user
		// For now, create a personal tenant per user
		// In production, users would join existing tenants via invitation
		tenantUUID := uuid.New().String()
		tenantSlug := fmt.Sprintf("tenant_%s", tenantUUID[:8])

		_, err = tx.Exec(ctx, `
			INSERT INTO tenants (tenant_id, name, slug, status)
			VALUES ($1, $2, $3, 'active')
		`, tenantUUID, req.Email, tenantSlug)
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Internal error")
			return
		}

		tenantID = &tenantUUID
		role = "tenant_admin" // User is admin of their own tenant
	}

	// Create user
	userID := uuid.New().String()
	err = tx.QueryRow(ctx, `
		INSERT INTO users_v2 (user_id, tenant_id, email, password_hash, role, status, email_verified)
		VALUES ($1, $2, $3, $4, $5, 'active', true)
		RETURNING user_id
	`, userID, tenantID, req.Email, string(passwordHash), role).Scan(&userID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Get permissions
	permissions, err := h.getPermissions(role)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Generate JWT tokens
	tenantIDStr := ""
	if tenantID != nil {
		tenantIDStr = *tenantID
	}

	accessToken, err := utils.GenerateJWT(
		h.Config.JWTSecret,
		userID,
		tenantIDStr,
		req.Email,
		role,
		permissions,
		h.Config.JWTAccessExpiration,
	)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	refreshToken, err := utils.GenerateJWT(
		h.Config.JWTSecret,
		userID,
		tenantIDStr,
		req.Email,
		role,
		permissions,
		h.Config.JWTRefreshExpiration,
	)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Return response
	utils.WriteJSON(w, http.StatusCreated, models.RegisterResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(h.Config.JWTAccessExpiration.Seconds()),
		User: models.User{
			UserID:   userID,
			TenantID: tenantID,
			Email:    req.Email,
			Role:     role,
			Status:   "active",
		},
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Normalize email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

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

// Refresh generates new access token from refresh token with revocation
// Refresh generates new access token from refresh token
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req models.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate and parse refresh token
	claims, err := utils.ValidateJWT(h.Config.JWTSecret, req.RefreshToken)
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}

	// Check if token is blacklisted (already used)
	blacklisted, err := utils.IsTokenBlacklisted(h.Redis, claims.JTI)
	if err != nil {
		// Log error but continue (fail-open for availability)
		log.Printf("Error checking token blacklist: %v", err)
	}
	if blacklisted {
		utils.WriteError(w, http.StatusUnauthorized, "Refresh token already used")
		return
	}

	// Verify user still exists and is active, and fetch current role/tenant/email
	var status string
	var role string
	var tenantID *string
	var email string
	err = h.DB.QueryRow(context.Background(),
		"SELECT status, role, tenant_id, email FROM users_v2 WHERE user_id = $1",
		claims.UserID,
	).Scan(&status, &role, &tenantID, &email)
	if err != nil {
		if err == pgx.ErrNoRows {
			utils.WriteError(w, http.StatusUnauthorized, "User not found")
		} else {
			utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		}
		return
	}

	if status != "active" {
		utils.WriteError(w, http.StatusForbidden, "Account is not active")
		return
	}

	// Get fresh permissions (in case they changed)
	permissions, err := h.getPermissions(role)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	tenantIDStr := ""
	if tenantID != nil {
		tenantIDStr = *tenantID
	}

	// Generate new access token
	accessToken, err := utils.GenerateJWT(
		h.Config.JWTSecret,
		claims.UserID,
		tenantIDStr,
		email,
		role,
		permissions,
		h.Config.JWTAccessExpiration,
	)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Generate new refresh token (token rotation)
	newRefreshToken, err := utils.GenerateJWT(
		h.Config.JWTSecret,
		claims.UserID,
		tenantIDStr,
		email,
		role,
		permissions,
		h.Config.JWTRefreshExpiration,
	)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Blacklist the old refresh token to prevent reuse
	ttl := time.Until(time.Unix(claims.ExpiresAt, 0))
	if ttl > 0 {
		if err := utils.BlacklistToken(h.Redis, claims.JTI, ttl); err != nil {
			// Log error but continue (fail-open for availability)
			log.Printf("Error blacklisting token: %v", err)
		}
	}

	utils.WriteJSON(w, http.StatusOK, models.RefreshResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int64(h.Config.JWTAccessExpiration.Seconds()),
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

// isValidEmail checks if email format is valid
func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// validatePassword checks password strength
func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case 'A' <= char && char <= 'Z':
			hasUpper = true
		case 'a' <= char && char <= 'z':
			hasLower = true
		case '0' <= char && char <= '9':
			hasNumber = true
		case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;:,.<>?", char):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}
	if !hasNumber {
		return fmt.Errorf("password must contain at least one number")
	}
	if !hasSpecial {
		return fmt.Errorf("password must contain at least one special character")
	}

	return nil
}
