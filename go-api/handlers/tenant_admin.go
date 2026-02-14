package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"iiot-go-api/config"
	"iiot-go-api/utils"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TenantAdminHandler struct {
	DB        *pgxpool.Pool
	Timescale *pgxpool.Pool
	Config    *config.Config
}

type TenantQuotaResponse struct {
	TenantID        string `json:"tenant_id"`
	PlanType        string `json:"plan_type"`
	BillingCycle    string `json:"billing_cycle"`
	QuotaDevices    int    `json:"quota_devices"`
	QuotaMsgsPerMin int    `json:"quota_msgs_per_min"`
	QuotaStorageMB  int    `json:"quota_storage_mb"`
	AllowOverage    bool   `json:"allow_overage"`
}

type TenantQuotaPatchRequest struct {
	PlanType        *string `json:"plan_type,omitempty"`
	BillingCycle    *string `json:"billing_cycle,omitempty"`
	QuotaDevices    *int    `json:"quota_devices,omitempty"`
	QuotaMsgsPerMin *int    `json:"quota_msgs_per_min,omitempty"`
	QuotaStorageMB  *int    `json:"quota_storage_mb,omitempty"`
	AllowOverage    *bool   `json:"allow_overage,omitempty"`
}

type TenantUsageResponse struct {
	TenantID           string  `json:"tenant_id"`
	MessagesLast60Min  int64   `json:"messages_last_60min"`
	DevicesTotal       int64   `json:"devices_total"`
	StorageMBEstimated float64 `json:"storage_mb_estimated"`
	PlanType           string  `json:"plan_type"`
	BillingCycle       string  `json:"billing_cycle"`
}

func NewTenantAdminHandler(db, ts *pgxpool.Pool, cfg *config.Config) *TenantAdminHandler {
	return &TenantAdminHandler{DB: db, Timescale: ts, Config: cfg}
}

func (h *TenantAdminHandler) GetTenantQuotas(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenant_id")
	if tenantID == "" {
		utils.WriteError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}

	var resp TenantQuotaResponse
	err := h.DB.QueryRow(context.Background(), `
		SELECT tenant_id::text, plan_type, billing_cycle, quota_devices, quota_msgs_per_min, quota_storage_mb, allow_overage
		FROM tenants
		WHERE tenant_id = $1::uuid
	`, tenantID).Scan(&resp.TenantID, &resp.PlanType, &resp.BillingCycle, &resp.QuotaDevices, &resp.QuotaMsgsPerMin, &resp.QuotaStorageMB, &resp.AllowOverage)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	utils.WriteJSON(w, http.StatusOK, resp)
}

func (h *TenantAdminHandler) PatchTenantQuotas(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenant_id")
	if tenantID == "" {
		utils.WriteError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}

	var req TenantQuotaPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.PlanType != nil {
		if *req.PlanType != "starter" && *req.PlanType != "pro" && *req.PlanType != "enterprise" {
			utils.WriteError(w, http.StatusBadRequest, "Invalid plan_type")
			return
		}
	}
	if req.BillingCycle != nil {
		if *req.BillingCycle != "monthly" && *req.BillingCycle != "annual" {
			utils.WriteError(w, http.StatusBadRequest, "Invalid billing_cycle")
			return
		}
	}
	if req.QuotaDevices != nil && *req.QuotaDevices < 0 {
		utils.WriteError(w, http.StatusBadRequest, "quota_devices must be >= 0")
		return
	}
	if req.QuotaMsgsPerMin != nil && *req.QuotaMsgsPerMin < 0 {
		utils.WriteError(w, http.StatusBadRequest, "quota_msgs_per_min must be >= 0")
		return
	}
	if req.QuotaStorageMB != nil && *req.QuotaStorageMB < 0 {
		utils.WriteError(w, http.StatusBadRequest, "quota_storage_mb must be >= 0")
		return
	}

	_, err := h.DB.Exec(context.Background(), `
		UPDATE tenants
		SET plan_type = COALESCE($2, plan_type),
			billing_cycle = COALESCE($3, billing_cycle),
			quota_devices = COALESCE($4, quota_devices),
			quota_msgs_per_min = COALESCE($5, quota_msgs_per_min),
			quota_storage_mb = COALESCE($6, quota_storage_mb),
			allow_overage = COALESCE($7, allow_overage),
			updated_at = NOW()
		WHERE tenant_id = $1::uuid
	`, tenantID, req.PlanType, req.BillingCycle, req.QuotaDevices, req.QuotaMsgsPerMin, req.QuotaStorageMB, req.AllowOverage)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	actorUserID, _ := r.Context().Value("user_id").(string)
	_, _ = h.DB.Exec(context.Background(), `
		INSERT INTO audit_log (tenant_id, user_id, event_type, event_category, severity, actor_type, actor_id, action, result, metadata)
		VALUES ($1::uuid, NULLIF($2,'')::uuid, 'quota.updated', 'billing', 'info', 'user', NULLIF($2,'')::uuid, 'update', 'success', $3::jsonb)
	`, tenantID, actorUserID, toJSONB(map[string]interface{}{
		"plan_type":          req.PlanType,
		"billing_cycle":      req.BillingCycle,
		"quota_devices":      req.QuotaDevices,
		"quota_msgs_per_min": req.QuotaMsgsPerMin,
		"quota_storage_mb":   req.QuotaStorageMB,
		"allow_overage":      req.AllowOverage,
	}))

	h.GetTenantQuotas(w, r)
}

func (h *TenantAdminHandler) GetTenantUsage(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenant_id")
	if tenantID == "" {
		utils.WriteError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}

	ctx := context.Background()

	var planType, billingCycle string
	err := h.DB.QueryRow(ctx, `SELECT plan_type, billing_cycle FROM tenants WHERE tenant_id = $1::uuid`, tenantID).Scan(&planType, &billingCycle)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	var devicesTotal int64
	_ = h.DB.QueryRow(ctx, `SELECT COUNT(*) FROM devices WHERE tenant_id = $1::uuid`, tenantID).Scan(&devicesTotal)

	var messagesLast60m int64
	_ = h.Timescale.QueryRow(ctx, `SELECT COALESCE(COUNT(*),0) FROM telemetry WHERE tenant_id = $1::uuid AND timestamp >= NOW() - interval '60 minutes'`, tenantID).Scan(&messagesLast60m)

	var storageBytes float64
	_ = h.Timescale.QueryRow(ctx, `SELECT COALESCE(SUM(pg_column_size(value)),0)::float8 FROM telemetry WHERE tenant_id = $1::uuid`, tenantID).Scan(&storageBytes)

	resp := TenantUsageResponse{
		TenantID:           tenantID,
		MessagesLast60Min:  messagesLast60m,
		DevicesTotal:       devicesTotal,
		StorageMBEstimated: math.Round((storageBytes/1024.0/1024.0)*100) / 100,
		PlanType:           planType,
		BillingCycle:       billingCycle,
	}
	_ = createUsageSnapshot(ctx, h.DB, h.Timescale, tenantID)
	utils.WriteJSON(w, http.StatusOK, resp)
}

func toJSONB(v interface{}) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func SendQuotaTelegramAsync(cfg *config.Config, title string, lines ...string) {
	if cfg == nil {
		return
	}
	msg := title
	if len(lines) > 0 {
		msg = fmt.Sprintf("%s\n%s", title, strings.Join(lines, "\n"))
	}
	go func() {
		_ = utils.SendTelegramMessage(context.Background(), cfg.TelegramBotToken, cfg.TelegramChatID, msg)
	}()
}

func recordQuotaExceeded(db *pgxpool.Pool, tenantID, userID, eventType, reason string, metadata map[string]interface{}) {
	if db == nil || tenantID == "" {
		return
	}
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata["reason"] = reason
	metadata["event_type"] = eventType

	_, _ = db.Exec(context.Background(), `
		INSERT INTO audit_log (tenant_id, user_id, event_type, event_category, severity, actor_type, actor_id, action, result, metadata, timestamp)
		VALUES ($1::uuid, NULLIF($2,'')::uuid, $3, 'billing', 'warning', 'system', NULLIF($2,'')::uuid, 'enforce_quota', 'blocked', $4::jsonb, NOW())
	`, tenantID, userID, eventType, toJSONB(metadata))
}

func currentMonthRange(now time.Time) (time.Time, time.Time) {
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	return start, end
}
