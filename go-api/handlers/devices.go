package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"iiot-go-api/config"
	"iiot-go-api/models"
	"iiot-go-api/utils"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type DeviceHandler struct {
	DB     *pgxpool.Pool
	Redis  *redis.Client
	Config *config.Config
}

func NewDeviceHandler(db *pgxpool.Pool, rdb *redis.Client, cfg *config.Config) *DeviceHandler {
	return &DeviceHandler{DB: db, Redis: rdb, Config: cfg}
}

// ClaimDevice handles device claim requests (device_id + claim_code)
func (h *DeviceHandler) ClaimDevice(w http.ResponseWriter, r *http.Request) {
	var req models.ClaimDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := utils.ValidateStruct(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, utils.ValidationErrorMessage(err))
		return
	}
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.ClaimCode = strings.TrimSpace(req.ClaimCode)
	if req.DeviceID == "" || req.ClaimCode == "" {
		utils.WriteError(w, http.StatusBadRequest, "device_id and claim_code are required")
		return
	}

	tenantID := r.Context().Value("tenant_id").(string)
	userID := r.Context().Value("user_id").(string)

	ctx := context.Background()
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	defer tx.Rollback(ctx)

	var status string
	var claimCodeHash sql.NullString
	err = tx.QueryRow(ctx, `
		SELECT status, claim_code_hash
		FROM devices_v2
		WHERE device_id = $1::uuid
		FOR UPDATE
	`, req.DeviceID).Scan(&status, &claimCodeHash)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "Device not found")
		return
	}

	if status != "unclaimed" {
		utils.WriteError(w, http.StatusConflict, "Device already claimed")
		return
	}

	if !claimCodeHash.Valid {
		utils.WriteError(w, http.StatusConflict, "Device is missing claim code")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(claimCodeHash.String), []byte(req.ClaimCode)); err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Invalid claim code")
		return
	}

	// Generate device secret and hash
	deviceSecret, err := generateDeviceSecret()
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	secretHash, err := bcrypt.GenerateFromPassword([]byte(deviceSecret), 12)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Claim device
	_, err = tx.Exec(ctx, `
		UPDATE devices_v2
		SET tenant_id = $1::uuid,
			owner_user_id = $2::uuid,
			status = 'claimed',
			claimed_at = NOW(),
			secret_hash = $3,
			activated_at = NULL,
			secret_delivered_at = NULL
		WHERE device_id = $4::uuid
	`, tenantID, userID, string(secretHash), req.DeviceID)
	if err != nil {
		log.Printf("claim_device update failed: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Cache secret for one-time retrieval
	if h.Redis != nil {
		redisKey := fmt.Sprintf("claim:%s:secret", req.DeviceID)
		if err := h.Redis.Set(context.Background(), redisKey, deviceSecret, 5*time.Minute).Err(); err != nil {
			log.Printf("Failed to cache secret: %v", err)
		}
	}

	utils.WriteJSON(w, http.StatusOK, models.ClaimDeviceResponse{
		DeviceID: req.DeviceID,
		Message:  "Device claimed successfully. Device can now retrieve secret.",
	})
}

// Bootstrap handles device bootstrap polling (HMAC + timestamp)
func (h *DeviceHandler) Bootstrap(w http.ResponseWriter, r *http.Request) {
	var req models.BootstrapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := utils.ValidateStruct(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, utils.ValidationErrorMessage(err))
		return
	}
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.Signature = strings.TrimSpace(req.Signature)
	req.Timestamp = strings.TrimSpace(req.Timestamp)
	if req.DeviceID == "" || req.Signature == "" || req.Timestamp == "" {
		utils.WriteError(w, http.StatusBadRequest, "device_id, signature and timestamp are required")
		return
	}

	if !h.verifyHMAC(req.DeviceID, req.Timestamp, req.Signature) {
		utils.WriteError(w, http.StatusUnauthorized, "Invalid signature")
		return
	}

	if !h.verifyTimestamp(req.Timestamp) {
		utils.WriteError(w, http.StatusUnauthorized, "Invalid timestamp")
		return
	}

	var status string
	err := h.DB.QueryRow(context.Background(), `
		SELECT status
		FROM devices_v2
		WHERE device_id = $1::uuid
	`, req.DeviceID).Scan(&status)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "Device not found")
		return
	}

	// Update last seen
	h.DB.Exec(context.Background(), `
		UPDATE devices_v2
		SET last_seen_at = NOW()
		WHERE device_id = $1::uuid
	`, req.DeviceID)

	utils.WriteJSON(w, http.StatusOK, models.BootstrapResponse{
		Status:       status,
		DeviceID:     req.DeviceID,
		PollInterval: 60,
	})
}

// GetSecret handles secret retrieval (one-time use) via HMAC + timestamp
func (h *DeviceHandler) GetSecret(w http.ResponseWriter, r *http.Request) {
	var req models.SecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := utils.ValidateStruct(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, utils.ValidationErrorMessage(err))
		return
	}
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.Signature = strings.TrimSpace(req.Signature)
	req.Timestamp = strings.TrimSpace(req.Timestamp)
	if req.DeviceID == "" || req.Signature == "" || req.Timestamp == "" {
		utils.WriteError(w, http.StatusBadRequest, "device_id, signature and timestamp are required")
		return
	}

	if !h.verifyHMAC(req.DeviceID, req.Timestamp, req.Signature) {
		utils.WriteError(w, http.StatusUnauthorized, "Invalid signature")
		return
	}

	if !h.verifyTimestamp(req.Timestamp) {
		utils.WriteError(w, http.StatusUnauthorized, "Invalid timestamp")
		return
	}

	// Ensure device is claimed
	var status string
	err := h.DB.QueryRow(context.Background(), `
		SELECT status
		FROM devices_v2
		WHERE device_id = $1::uuid
	`, req.DeviceID).Scan(&status)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "Device not found")
		return
	}
	if status != "claimed" {
		utils.WriteError(w, http.StatusConflict, "Device is not ready for secret retrieval")
		return
	}

	if h.Redis == nil {
		utils.WriteError(w, http.StatusServiceUnavailable, "Cache unavailable")
		return
	}

	redisKey := fmt.Sprintf("claim:%s:secret", req.DeviceID)

	// Get and delete atomically
	secret, err := h.Redis.GetDel(context.Background(), redisKey).Result()
	if err != nil || secret == "" {
		// Re-issue new secret if cache is missing
		secret, err = generateDeviceSecret()
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Internal error")
			return
		}
		secretHash, err := bcrypt.GenerateFromPassword([]byte(secret), 12)
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Internal error")
			return
		}
		_, err = h.DB.Exec(context.Background(), `
			UPDATE devices_v2
			SET secret_hash = $1, secret_delivered_at = NOW()
			WHERE device_id = $2::uuid
		`, string(secretHash), req.DeviceID)
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Internal error")
			return
		}
	} else {
		h.DB.Exec(context.Background(), `
			UPDATE devices_v2
			SET secret_delivered_at = NOW()
			WHERE device_id = $1::uuid
		`, req.DeviceID)
	}

	expiresAt := time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339)
	utils.WriteJSON(w, http.StatusOK, models.SecretResponse{
		DeviceSecret: secret,
		ExpiresAt:    expiresAt,
	})
}

// ResetDevice resets a claimed/active device back to unclaimed
func (h *DeviceHandler) ResetDevice(w http.ResponseWriter, r *http.Request) {
	var req models.ResetDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := utils.ValidateStruct(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, utils.ValidationErrorMessage(err))
		return
	}
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if req.DeviceID == "" || req.Confirmation != "RESET" {
		utils.WriteError(w, http.StatusBadRequest, "device_id and confirmation are required")
		return
	}

	userID := r.Context().Value("user_id").(string)
	tenantID := r.Context().Value("tenant_id").(string)
	role := r.Context().Value("role").(string)

	var tag any
	var err error
	if role == "super_admin" || role == "tenant_admin" {
		tag, err = h.DB.Exec(context.Background(), `
			UPDATE devices_v2
			SET tenant_id = NULL,
				owner_user_id = NULL,
				status = 'unclaimed',
				claimed_at = NULL,
				activated_at = NULL,
				secret_hash = NULL,
				secret_delivered_at = NULL
			WHERE device_id = $1::uuid AND tenant_id = $2::uuid
		`, req.DeviceID, tenantID)
	} else {
		tag, err = h.DB.Exec(context.Background(), `
			UPDATE devices_v2
			SET tenant_id = NULL,
				owner_user_id = NULL,
				status = 'unclaimed',
				claimed_at = NULL,
				activated_at = NULL,
				secret_hash = NULL,
				secret_delivered_at = NULL
			WHERE device_id = $1::uuid AND tenant_id = $2::uuid AND owner_user_id = $3::uuid
		`, req.DeviceID, tenantID, userID)
	}

	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	affected := int64(0)
	if ct, ok := tag.(interface{ RowsAffected() int64 }); ok {
		affected = ct.RowsAffected()
	}
	if affected == 0 {
		utils.WriteError(w, http.StatusNotFound, "Device not found or not authorized")
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "Device reset to unclaimed",
	})
}

// ListDevices handles device listing (simple version without RLS)
func (h *DeviceHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value("tenant_id").(string)

	rows, err := h.DB.Query(context.Background(), `
		SELECT device_id, device_label, status, firmware_version, last_seen_at, created_at
		FROM devices_v2
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	defer rows.Close()

	devices := []models.Device{}
	for rows.Next() {
		var d models.Device
		rows.Scan(&d.DeviceID, &d.DeviceLabel, &d.Status, &d.FirmwareVersion, &d.LastSeenAt, &d.CreatedAt)
		devices = append(devices, d)
	}

	utils.WriteJSON(w, http.StatusOK, devices)
}

func (h *DeviceHandler) verifyHMAC(deviceID, timestamp, signature string) bool {
	msg := deviceID + ":" + timestamp
	mac := hmac.New(sha256.New, []byte(h.Config.ManufacturingMasterKey))
	_, _ = mac.Write([]byte(msg))
	expected := mac.Sum(nil)

	decodedSig, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	return hmac.Equal(decodedSig, expected)
}

func (h *DeviceHandler) verifyTimestamp(ts string) bool {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return false
	}
	now := time.Now().UTC()
	if t.After(now.Add(time.Duration(h.Config.BootstrapMaxSkewSecs) * time.Second)) {
		return false
	}
	if now.Sub(t) > time.Duration(h.Config.BootstrapMaxSkewSecs)*time.Second {
		return false
	}
	return true
}

func generateDeviceSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
