package utils

import (
	"context"
	"errors"
	"fmt"
	"iiot-go-api/models"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrInvalidToken    = errors.New("invalid token")
	ErrExpiredToken    = errors.New("expired token")
	ErrBlacklistedToken = errors.New("token has been revoked")
)

// GenerateJWT generates a JWT token with a unique JTI
func GenerateJWT(secret string, userID, tenantID, email, role string, permissions []string, expiration time.Duration) (string, error) {
	now := time.Now()
	jti := uuid.New().String() // Generate unique token ID

	claims := jwt.MapClaims{
		"jti":         jti,
		"user_id":     userID,
		"tenant_id":   tenantID,
		"email":       email,
		"role":        role,
		"permissions": permissions,
		"exp":         now.Add(expiration).Unix(),
		"iat":         now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateJWT validates a JWT token and returns typed claims
func ValidateJWT(secret, tokenString string) (*models.JWTClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if mapClaims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Check expiration
		if exp, ok := mapClaims["exp"].(float64); ok {
			if time.Unix(int64(exp), 0).Before(time.Now()) {
				return nil, ErrExpiredToken
			}
		}

		// Convert MapClaims to JWTClaims
		claims, err := parseJWTClaims(mapClaims)
		if err != nil {
			return nil, err
		}

		return claims, nil
	}

	return nil, ErrInvalidToken
}

// IsTokenBlacklisted checks if a token JTI is blacklisted
func IsTokenBlacklisted(redisClient *redis.Client, jti string) (bool, error) {
	if jti == "" {
		return false, nil // No JTI = old token, allow for backward compatibility
	}

	ctx := context.Background()
	key := fmt.Sprintf("jwt:blacklist:%s", jti)
	
	exists, err := redisClient.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}

	return exists > 0, nil
}

// BlacklistToken adds a token JTI to the blacklist with TTL
func BlacklistToken(redisClient *redis.Client, jti string, ttl time.Duration) error {
	if jti == "" {
		return nil // No JTI to blacklist
	}

	ctx := context.Background()
	key := fmt.Sprintf("jwt:blacklist:%s", jti)
	
	return redisClient.Set(ctx, key, "1", ttl).Err()
}

// parseJWTClaims converts jwt.MapClaims to models.JWTClaims
func parseJWTClaims(m jwt.MapClaims) (*models.JWTClaims, error) {
	claims := &models.JWTClaims{}

	// JTI (optional for backward compatibility, but should be present)
	if jti, ok := m["jti"].(string); ok {
		claims.JTI = jti
	}

	// Required fields
	userID, ok := m["user_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing user_id claim")
	}
	claims.UserID = userID

	email, ok := m["email"].(string)
	if !ok {
		return nil, fmt.Errorf("missing email claim")
	}
	claims.Email = email

	role, ok := m["role"].(string)
	if !ok {
		return nil, fmt.Errorf("missing role claim")
	}
	claims.Role = role

	// Optional fields
	if tenantID, ok := m["tenant_id"].(string); ok {
		claims.TenantID = tenantID
	}

	// Permissions (array)
	if perms, ok := m["permissions"].([]interface{}); ok {
		permissions := make([]string, len(perms))
		for i, p := range perms {
			if perm, ok := p.(string); ok {
				permissions[i] = perm
			}
		}
		claims.Permissions = permissions
	}

	// Timestamps
	if exp, ok := m["exp"].(float64); ok {
		claims.ExpiresAt = int64(exp)
	}
	if iat, ok := m["iat"].(float64); ok {
		claims.IssuedAt = int64(iat)
	}

	return claims, nil
}

