package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iiot-go-api/config"
	"iiot-go-api/metrics"
	"iiot-go-api/models"
	"iiot-go-api/utils"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type TelemetryHandler struct {
	Postgres  *pgxpool.Pool
	Timescale *pgxpool.Pool
	Redis     *redis.Client
	Config    *config.Config
	Limiter   *RateLimiter
}

func NewTelemetryHandler(pg, ts *pgxpool.Pool, rdb *redis.Client, cfg *config.Config) *TelemetryHandler {
	var limiter *RateLimiter
	if rdb != nil {
		limiter = NewRateLimiter(rdb, cfg)
	}

	return &TelemetryHandler{
		Postgres:  pg,
		Timescale: ts,
		Redis:     rdb,
		Config:    cfg,
		Limiter:   limiter,
	}
}

// Webhook handles EMQX Rule Engine webhook
func (h *TelemetryHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	var req models.TelemetryRequest
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		metrics.TelemetryRejected("invalid_json")
		utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if err := json.Unmarshal(rawBody, &req); err != nil {
		sanitized := sanitizeJSONEscapes(rawBody)
		if err2 := json.Unmarshal(sanitized, &req); err2 != nil {
			log.Printf("telemetry invalid json body=%s", string(rawBody))
			metrics.TelemetryRejected("invalid_json")
			utils.WriteError(w, http.StatusBadRequest, "Invalid JSON body")
			return
		}
	}
	if err := utils.ValidateStruct(&req); err != nil {
		metrics.TelemetryRejected("validation")
		utils.WriteError(w, http.StatusBadRequest, utils.ValidationErrorMessage(err))
		return
	}

	topicTenantID, deviceToken, slot, err := parseTopic(req.Topic)
	if err != nil {
		metrics.TelemetryRejected("invalid_topic")
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Rate limit
	if h.Limiter != nil {
		allowed, err := h.Limiter.Allow(context.Background(), deviceToken, slot)
		if err != nil && !h.Config.RateLimitFailOpen {
			metrics.TelemetryRejected("rate_limiter_unavailable")
			utils.WriteError(w, http.StatusServiceUnavailable, "Rate limiter unavailable")
			return
		}
		if !allowed {
			log.Printf("rate_limit_exceeded device=%s slot=%d", deviceToken, slot)
			metrics.TelemetryRejected("rate_limit")
			utils.WriteError(w, http.StatusTooManyRequests, "Rate limit exceeded")
			return
		}
	}

	// Find device + tenant
	var deviceID string
	var tenantID string
	err = h.Postgres.QueryRow(context.Background(), `
		SELECT device_id, tenant_id
		FROM devices
		WHERE device_id = $1::uuid AND status IN ('active', 'claimed')
	`, deviceToken).Scan(&deviceID, &tenantID)

	if err != nil {
		metrics.TelemetryRejected("device_not_found")
		utils.WriteError(w, http.StatusNotFound, "Device not found or inactive")
		return
	}
	if tenantID == "" {
		metrics.TelemetryRejected("tenant_missing")
		utils.WriteError(w, http.StatusNotFound, "Device missing tenant")
		return
	}
	if topicTenantID != tenantID {
		metrics.TelemetryRejected("tenant_mismatch")
		utils.WriteError(w, http.StatusForbidden, "Topic tenant does not match device tenant")
		return
	}

	// Parse timestamp
	ts, err := parseTimestamp(req.Timestamp)
	if err != nil {
		metrics.TelemetryRejected("invalid_timestamp")
		utils.WriteError(w, http.StatusBadRequest, "Invalid timestamp")
		return
	}

	// Insert telemetry
	tx, err := h.Timescale.Begin(context.Background())
	if err != nil {
		log.Printf("timescale tx error: %v", err)
		metrics.TelemetryRejected("db_error")
		utils.WriteError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	defer tx.Rollback(context.Background())

	// Set tenant context for RLS on TimescaleDB (use set_config to allow parameters)
	_, err = tx.Exec(context.Background(), "SELECT set_config('app.current_tenant_id', $1, true)", tenantID)
	if err != nil {
		log.Printf("timescale set context error: %v", err)
		metrics.TelemetryRejected("db_error")
		utils.WriteError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	_, err = tx.Exec(context.Background(), "SELECT set_config('app.current_user_role', $1, true)", "service")
	if err != nil {
		log.Printf("timescale set context error: %v", err)
		metrics.TelemetryRejected("db_error")
		utils.WriteError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	_, err = tx.Exec(context.Background(), `
		INSERT INTO telemetry (tenant_id, device_id, slot, value, timestamp)
		VALUES ($1, $2, $3, $4, $5)
	`, tenantID, deviceID, slot, req.Payload, ts)

	if err != nil {
		log.Printf("insert error: %v", err)
		metrics.TelemetryRejected("db_error")
		utils.WriteError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if err := tx.Commit(context.Background()); err != nil {
		log.Printf("timescale commit error: %v", err)
		metrics.TelemetryRejected("db_error")
		utils.WriteError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	metrics.TelemetryIngested(strconv.Itoa(slot))

	// Update cache
	if h.Redis != nil {
		cacheLatest(context.Background(), h.Redis, deviceID, slot, req.Payload, ts, h.Config.CacheTTLSeconds)
	}

	// Update last_seen
	h.Postgres.Exec(context.Background(), `
		UPDATE devices SET last_seen_at = NOW(), status = 'active' WHERE device_id = $1
	`, deviceID)

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"device_id": deviceID,
		"slot":      slot,
	})
}

func sanitizeJSONEscapes(input []byte) []byte {
	out := make([]byte, 0, len(input))
	for i := 0; i < len(input); i++ {
		if input[i] == '\\' && i+1 < len(input) {
			next := input[i+1]
			// Keep only structural JSON escapes and drop all others.
			// EMQX templating may emit sequences like "\tenants/..." that break topic parsing.
			if next != '"' && next != '\\' && next != '/' && next != 'u' {
				continue
			}
		}
		out = append(out, input[i])
	}
	return out
}

// GetLatest handles latest telemetry retrieval
func (h *TelemetryHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := r.Context().Value("tenant_id").(string)
	if !ok || tenantID == "" {
		utils.WriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	deviceIDParam := r.URL.Query().Get("device_id")
	deviceLabel := r.URL.Query().Get("device_label")
	slotStr := r.URL.Query().Get("slot")

	if slotStr == "" {
		utils.WriteError(w, http.StatusBadRequest, "slot is required")
		return
	}
	if deviceIDParam == "" && deviceLabel == "" {
		utils.WriteError(w, http.StatusBadRequest, "device_id or device_label is required")
		return
	}
	if deviceIDParam != "" && deviceLabel != "" {
		utils.WriteError(w, http.StatusBadRequest, "provide only one of device_id or device_label")
		return
	}

	slot, err := strconv.Atoi(slotStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid slot")
		return
	}

	// Find device
	var deviceID string
	switch {
	case deviceIDParam != "":
		err = h.Postgres.QueryRow(context.Background(), `
			SELECT device_id
			FROM devices
			WHERE device_id = $1::uuid AND tenant_id = $2::uuid AND status IN ('active', 'claimed')
		`, deviceIDParam, tenantID).Scan(&deviceID)
	case deviceLabel != "":
		err = h.Postgres.QueryRow(context.Background(), `
			SELECT device_id
			FROM devices
			WHERE device_label = $1 AND tenant_id = $2::uuid AND status IN ('active', 'claimed')
		`, deviceLabel, tenantID).Scan(&deviceID)
	}

	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "Device not found or inactive")
		return
	}

	// Get from cache
	if h.Redis == nil {
		utils.WriteError(w, http.StatusServiceUnavailable, "Cache unavailable")
		return
	}

	key := fmt.Sprintf("latest:device:%s:slot:%d", deviceID, slot)
	raw, err := h.Redis.Get(context.Background(), key).Result()
	if err != nil {
		utils.WriteJSON(w, http.StatusOK, map[string]interface{}{})
		return
	}

	var value models.LatestTelemetry
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	utils.WriteJSON(w, http.StatusOK, value)
}

// GetActiveSlots returns slots that have cached latest values (Redis)
func (h *TelemetryHandler) GetActiveSlots(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := r.Context().Value("tenant_id").(string)
	if !ok || tenantID == "" {
		utils.WriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	deviceIDParam := r.URL.Query().Get("device_id")
	deviceLabel := r.URL.Query().Get("device_label")

	if deviceIDParam == "" && deviceLabel == "" {
		utils.WriteError(w, http.StatusBadRequest, "device_id or device_label is required")
		return
	}
	if deviceIDParam != "" && deviceLabel != "" {
		utils.WriteError(w, http.StatusBadRequest, "provide only one of device_id or device_label")
		return
	}

	// Find device
	var deviceID string
	var err error
	switch {
	case deviceIDParam != "":
		err = h.Postgres.QueryRow(context.Background(), `
			SELECT device_id
			FROM devices
			WHERE device_id = $1::uuid AND tenant_id = $2::uuid AND status IN ('active', 'claimed')
		`, deviceIDParam, tenantID).Scan(&deviceID)
	case deviceLabel != "":
		err = h.Postgres.QueryRow(context.Background(), `
			SELECT device_id
			FROM devices
			WHERE device_label = $1 AND tenant_id = $2::uuid AND status IN ('active', 'claimed')
		`, deviceLabel, tenantID).Scan(&deviceID)
	}
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "Device not found or inactive")
		return
	}

	if h.Redis == nil {
		utils.WriteError(w, http.StatusServiceUnavailable, "Cache unavailable")
		return
	}

	pattern := fmt.Sprintf("latest:device:%s:slot:*", deviceID)
	var cursor uint64
	slotSet := make(map[int]struct{})
	for {
		keys, next, err := h.Redis.Scan(context.Background(), cursor, pattern, 200).Result()
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Internal error")
			return
		}
		for _, key := range keys {
			parts := strings.Split(key, ":")
			if len(parts) == 0 {
				continue
			}
			slotStr := parts[len(parts)-1]
			slot, err := strconv.Atoi(slotStr)
			if err != nil {
				continue
			}
			slotSet[slot] = struct{}{}
		}
		if next == 0 {
			break
		}
		cursor = next
	}

	slots := make([]int, 0, len(slotSet))
	for s := range slotSet {
		slots = append(slots, s)
	}
	sort.Ints(slots)

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"device_id": deviceID,
		"slots":     slots,
	})
}

// Helper functions
func parseTopic(topic string) (string, string, int, error) {
	// tenants/{tenant_id}/devices/{device_id}/telemetry/slot/{N}
	parts := strings.Split(topic, "/")
	if len(parts) != 7 {
		return "", "", 0, errors.New("invalid topic format")
	}
	if parts[0] != "tenants" || parts[2] != "devices" || parts[4] != "telemetry" || parts[5] != "slot" {
		return "", "", 0, errors.New("invalid topic format")
	}

	tenantID := parts[1]
	deviceID := parts[3]
	slot, err := strconv.Atoi(parts[6])
	if tenantID == "" || deviceID == "" || err != nil {
		return "", "", 0, errors.New("invalid topic format")
	}
	return tenantID, deviceID, slot, nil
}

func parseTimestamp(ts string) (time.Time, error) {
	if ts == "" {
		return time.Now().UTC(), nil
	}
	ms, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(ms).UTC(), nil
}

func cacheLatest(ctx context.Context, rdb *redis.Client, deviceID string, slot int, payload json.RawMessage, ts time.Time, ttlSeconds int64) {
	item := models.LatestTelemetry{
		DeviceID:  deviceID,
		Slot:      slot,
		Value:     payload,
		Timestamp: ts.UTC().Format(time.RFC3339),
	}

	raw, _ := json.Marshal(item)
	key := fmt.Sprintf("latest:device:%s:slot:%d", deviceID, slot)

	if ttlSeconds > 0 {
		rdb.Set(ctx, key, raw, time.Duration(ttlSeconds)*time.Second)
	} else {
		rdb.Set(ctx, key, raw, 0)
	}
}

// RateLimiter
type RateLimiter struct {
	rdb          *redis.Client
	devicePerMin int64
	devicePerSec int64
	slotPerMin   int64
	failOpen     bool
}

var rateLimitScript = redis.NewScript(`
local keys = KEYS
local expiries = ARGV
local counts = {}
for i = 1, #keys do
  local c = redis.call('INCR', keys[i])
  if c == 1 then
    redis.call('EXPIRE', keys[i], tonumber(expiries[i]))
  end
  counts[i] = c
end
return counts
`)

func NewRateLimiter(rdb *redis.Client, cfg *config.Config) *RateLimiter {
	return &RateLimiter{
		rdb:          rdb,
		devicePerMin: cfg.RateLimitDevicePerMin,
		devicePerSec: cfg.RateLimitDevicePerSec,
		slotPerMin:   cfg.RateLimitSlotPerMin,
		failOpen:     cfg.RateLimitFailOpen,
	}
}

func (r *RateLimiter) Allow(ctx context.Context, token string, slot int) (bool, error) {
	keyDevSec := "rl:dev:" + token + ":1"
	keyDevMin := "rl:dev:" + token + ":60"
	keySlotMin := "rl:dev:" + token + ":slot:" + strconv.Itoa(slot) + ":60"

	res, err := rateLimitScript.Run(ctx, r.rdb, []string{keyDevSec, keyDevMin, keySlotMin}, 1, 60, 60).Result()
	if err != nil {
		return false, err
	}

	values, ok := res.([]interface{})
	if !ok || len(values) != 3 {
		return false, errors.New("invalid rate limit response")
	}

	sec, _ := toInt64(values[0])
	min, _ := toInt64(values[1])
	slotMin, _ := toInt64(values[2])

	if sec > r.devicePerSec || min > r.devicePerMin || slotMin > r.slotPerMin {
		return false, nil
	}
	return true, nil
}

func toInt64(v interface{}) (int64, error) {
	switch t := v.(type) {
	case int64:
		return t, nil
	case int:
		return int64(t), nil
	default:
		return 0, errors.New("invalid type")
	}
}
