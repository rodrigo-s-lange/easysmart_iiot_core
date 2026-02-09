package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("expired token")
)

// GenerateJWT generates a JWT token
func GenerateJWT(secret string, userID, tenantID, email, role string, permissions []string, expiration time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
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

// ValidateJWT validates a JWT token
func ValidateJWT(secret, tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Check expiration
		if exp, ok := claims["exp"].(float64); ok {
			if time.Unix(int64(exp), 0).Before(time.Now()) {
				return nil, ErrExpiredToken
			}
		}
		return claims, nil
	}

	return nil, ErrInvalidToken
}
