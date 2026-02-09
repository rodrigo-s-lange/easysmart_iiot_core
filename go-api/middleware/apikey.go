package middleware

import (
	"context"
	"encoding/json"
	"iiot-go-api/utils"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type APIKeyMiddleware struct {
	DB    *pgxpool.Pool
	Redis *redis.Client
}

type APIKeyData struct {
	KeyID    string   `json:"key_id"`
	TenantID string   `json:"tenant_id"`
	Scopes   []string `json:"scopes"`
}

func NewAPIKeyMiddleware(db *pgxpool.Pool, rdb *redis.Client) *APIKeyMiddleware {
	return &APIKeyMiddleware{DB: db, Redis: rdb}
}

func (m *APIKeyMiddleware) Authenticate(next http.Handler) http.Handler {
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

		apiKey := parts[1]
		keyData, err := m.validateAPIKey(r.Context(), apiKey)
		if err != nil {
			utils.WriteError(w, http.StatusUnauthorized, "Invalid API key")
			return
		}

		// Add key data to context
		ctx := r.Context()
		ctx = context.WithValue(ctx, "api_key_id", keyData.KeyID)
		ctx = context.WithValue(ctx, "tenant_id", keyData.TenantID)
		ctx = context.WithValue(ctx, "scopes", keyData.Scopes)

		// Update last_used async
		go m.updateLastUsed(keyData.KeyID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *APIKeyMiddleware) validateAPIKey(ctx context.Context, key string) (*APIKeyData, error) {
	// 1. Redis cache (hot path)
	if m.Redis != nil {
		cacheKey := "apikey:valid:" + key
		cached, err := m.Redis.Get(ctx, cacheKey).Result()
		if err == nil {
			var data APIKeyData
			if json.Unmarshal([]byte(cached), &data) == nil {
				return &data, nil
			}
		}
	}

	// 2. Database lookup (cold path)
	prefix := key[:8]
	var keyHash string
	var data APIKeyData

	err := m.DB.QueryRow(ctx, `
		SELECT key_id, key_hash, tenant_id, scopes
		FROM api_keys
		WHERE key_prefix = $1 AND status = 'active'
		LIMIT 1
	`, prefix).Scan(&data.KeyID, &keyHash, &data.TenantID, &data.Scopes)

	if err != nil {
		return nil, err
	}

	// 3. Bcrypt verify
	if err := bcrypt.CompareHashAndPassword([]byte(keyHash), []byte(key)); err != nil {
		return nil, err
	}

	// 4. Cache for 1 hour
	if m.Redis != nil {
		jsonData, _ := json.Marshal(data)
		m.Redis.Set(ctx, "apikey:valid:"+key, jsonData, time.Hour)
	}

	return &data, nil
}

func (m *APIKeyMiddleware) updateLastUsed(keyID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m.DB.Exec(ctx, "UPDATE api_keys SET last_used_at = NOW() WHERE key_id = $1", keyID)
}
