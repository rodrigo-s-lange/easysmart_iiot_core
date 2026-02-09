package models

import (
	"encoding/json"
	"time"
)

// User represents a user in the system
type User struct {
	UserID       string     `json:"user_id" db:"user_id"`
	TenantID     *string    `json:"tenant_id" db:"tenant_id"`
	Email        string     `json:"email" db:"email"`
	PasswordHash string     `json:"-" db:"password_hash"`
	Role         string     `json:"role" db:"role"`
	Status       string     `json:"status" db:"status"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
}

// Device represents a device in the system
type Device struct {
	DeviceID         string     `json:"device_id" db:"device_id"`
	TenantID         *string    `json:"tenant_id" db:"tenant_id"`
	OwnerUserID      *string    `json:"owner_user_id" db:"owner_user_id"`
	DeviceLabel      string     `json:"device_label" db:"device_label"`
	SecretHash       *string    `json:"-" db:"secret_hash"`
	Status           string     `json:"status" db:"status"`
	ClaimedAt        *time.Time `json:"claimed_at,omitempty" db:"claimed_at"`
	ActivatedAt      *time.Time `json:"activated_at,omitempty" db:"activated_at"`
	FirmwareVersion  *string    `json:"firmware_version,omitempty" db:"firmware_version"`
	HardwareRevision *string    `json:"hardware_revision,omitempty" db:"hardware_revision"`
	LastSeenAt       *time.Time `json:"last_seen_at,omitempty" db:"last_seen_at"`
	LastIP           *string    `json:"last_ip,omitempty" db:"last_ip"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

// JWTClaims represents JWT token claims
type JWTClaims struct {
	JTI         string   `json:"jti"`
	TokenType   string   `json:"token_type"`
	UserID      string   `json:"user_id"`
	TenantID    string   `json:"tenant_id"`
	Email       string   `json:"email"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	ExpiresAt   int64    `json:"exp"`
	IssuedAt    int64    `json:"iat"`
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RegisterResponse represents a registration response
type RegisterResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	User         User   `json:"user"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	User         User   `json:"user"`
}

// RefreshRequest represents a token refresh request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// RefreshResponse represents a token refresh response
type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// ClaimDeviceRequest represents a device claim request
type ClaimDeviceRequest struct {
	DeviceID  string `json:"device_id" validate:"required"`
	ClaimCode string `json:"claim_code" validate:"required"`
}

// ClaimDeviceResponse represents a device claim response
type ClaimDeviceResponse struct {
	DeviceID string `json:"device_id"`
	Message  string `json:"message"`
}

// BootstrapResponse represents a device bootstrap response
type BootstrapResponse struct {
	Status       string `json:"status"`
	DeviceID     string `json:"device_id,omitempty"`
	PollInterval int    `json:"poll_interval,omitempty"`
}

// BootstrapRequest represents device bootstrap polling
type BootstrapRequest struct {
	DeviceID  string `json:"device_id" validate:"required"`
	Timestamp string `json:"timestamp" validate:"required"`
	Signature string `json:"signature" validate:"required"`
}

// SecretRequest represents device secret retrieval
type SecretRequest struct {
	DeviceID  string `json:"device_id" validate:"required"`
	Timestamp string `json:"timestamp" validate:"required"`
	Signature string `json:"signature" validate:"required"`
}

// SecretResponse represents a secret retrieval response
type SecretResponse struct {
	DeviceSecret string `json:"device_secret"`
	ExpiresAt    string `json:"expires_at"`
}

// ResetDeviceRequest represents device reset request
type ResetDeviceRequest struct {
	DeviceID     string `json:"device_id" validate:"required"`
	Confirmation string `json:"confirmation" validate:"required"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// TelemetryRequest represents a telemetry webhook request
type TelemetryRequest struct {
	ClientID  string          `json:"clientid"`
	Topic     string          `json:"topic" validate:"required"`
	Payload   json.RawMessage `json:"payload" validate:"required"`
	Timestamp string          `json:"timestamp"`
}

// LatestTelemetry represents cached latest telemetry
type LatestTelemetry struct {
	DeviceID  string          `json:"device_id"`
	Slot      int             `json:"slot"`
	Value     json.RawMessage `json:"value"`
	Timestamp string          `json:"timestamp"`
}
