package handlers

import (
	"context"
	"fmt"
	"iiot-go-api/config"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type TenantQuota struct {
	TenantID        string
	PlanType        string
	BillingCycle    string
	QuotaDevices    int
	QuotaMsgsPerMin int
	QuotaStorageMB  int
	AllowOverage    bool
}

func fetchTenantQuota(ctx context.Context, db *pgxpool.Pool, tenantID string) (*TenantQuota, error) {
	var q TenantQuota
	err := db.QueryRow(ctx, `
		SELECT tenant_id::text, plan_type, billing_cycle, quota_devices, quota_msgs_per_min, quota_storage_mb, allow_overage
		FROM tenants
		WHERE tenant_id = $1::uuid
	`, tenantID).Scan(&q.TenantID, &q.PlanType, &q.BillingCycle, &q.QuotaDevices, &q.QuotaMsgsPerMin, &q.QuotaStorageMB, &q.AllowOverage)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func enforceDeviceQuota(ctx context.Context, db *pgxpool.Pool, cfg *config.Config, tenantID, userID, userEmail string) (bool, error) {
	quota, err := fetchTenantQuota(ctx, db, tenantID)
	if err != nil {
		return false, err
	}
	if quota.QuotaDevices == 0 {
		return true, nil
	}

	var total int
	err = db.QueryRow(ctx, `SELECT COUNT(*) FROM devices WHERE tenant_id = $1::uuid`, tenantID).Scan(&total)
	if err != nil {
		return false, err
	}
	if total >= quota.QuotaDevices {
		recordQuotaExceeded(db, tenantID, userID, "quota.devices_exceeded", "quota_devices", map[string]interface{}{
			"quota_devices": quota.QuotaDevices,
			"devices_total": total,
			"user_email":    userEmail,
		})
		SendQuotaTelegramAsync(cfg,
			"[IIoT Core] Quota devices excedida",
			fmt.Sprintf("tenant=%s", tenantID),
			fmt.Sprintf("email=%s", userEmail),
			fmt.Sprintf("devices=%d quota=%d", total, quota.QuotaDevices),
		)
		return false, nil
	}

	return true, nil
}

func enforceTelemetryQuota(ctx context.Context, db *pgxpool.Pool, ts *pgxpool.Pool, rdb *redis.Client, cfg *config.Config, tenantID, deviceID string) (bool, int, error) {
	quota, err := fetchTenantQuota(ctx, db, tenantID)
	if err != nil {
		return false, 0, err
	}

	if rdb != nil && quota.QuotaMsgsPerMin > 0 {
		key := fmt.Sprintf("quota:tenant:%s:device:%s:m", tenantID, deviceID)
		count, err := rdb.Incr(ctx, key).Result()
		if err == nil {
			if count == 1 {
				_ = rdb.Expire(ctx, key, 60*time.Second).Err()
			}
			if int(count) > quota.QuotaMsgsPerMin {
				recordQuotaExceeded(db, tenantID, "", "quota.messages_exceeded", "quota_msgs_per_min", map[string]interface{}{
					"quota_msgs_per_min": quota.QuotaMsgsPerMin,
					"device_id":          deviceID,
					"count":              count,
				})
				SendQuotaTelegramAsync(cfg,
					"[IIoT Core] Quota msg/min excedida",
					fmt.Sprintf("tenant=%s", tenantID),
					fmt.Sprintf("device=%s", deviceID),
					fmt.Sprintf("count=%d quota=%d", count, quota.QuotaMsgsPerMin),
				)
				return false, quota.QuotaMsgsPerMin, nil
			}
		}
	}

	if quota.QuotaStorageMB > 0 {
		var storageBytes float64
		err := ts.QueryRow(ctx, `SELECT COALESCE(SUM(pg_column_size(value)),0)::float8 FROM telemetry WHERE tenant_id = $1::uuid`, tenantID).Scan(&storageBytes)
		if err == nil {
			storageMB := storageBytes / 1024.0 / 1024.0
			if storageMB >= float64(quota.QuotaStorageMB) {
				if quota.PlanType == "enterprise" && quota.AllowOverage {
					return true, quota.QuotaMsgsPerMin, nil
				}
				recordQuotaExceeded(db, tenantID, "", "quota.storage_exceeded", "quota_storage_mb", map[string]interface{}{
					"quota_storage_mb": quota.QuotaStorageMB,
					"storage_mb":       storageMB,
					"plan_type":        quota.PlanType,
					"allow_overage":    quota.AllowOverage,
				})
				SendQuotaTelegramAsync(cfg,
					"[IIoT Core] Quota storage excedida",
					fmt.Sprintf("tenant=%s", tenantID),
					fmt.Sprintf("plan=%s", quota.PlanType),
					fmt.Sprintf("storage=%.2fMB quota=%dMB", storageMB, quota.QuotaStorageMB),
				)
				return false, quota.QuotaMsgsPerMin, nil
			}
		}
	}

	return true, quota.QuotaMsgsPerMin, nil
}

func createUsageSnapshot(ctx context.Context, db, ts *pgxpool.Pool, tenantID string) error {
	start, end := currentMonthRange(time.Now().UTC())

	var messages int64
	_ = ts.QueryRow(ctx, `
		SELECT COALESCE(COUNT(*),0)
		FROM telemetry
		WHERE tenant_id = $1::uuid
		  AND timestamp >= $2
		  AND timestamp < $3
	`, tenantID, start, end).Scan(&messages)

	var devices int
	_ = db.QueryRow(ctx, `SELECT COUNT(*) FROM devices WHERE tenant_id = $1::uuid`, tenantID).Scan(&devices)

	var storageBytes float64
	_ = ts.QueryRow(ctx, `SELECT COALESCE(SUM(pg_column_size(value)),0)::float8 FROM telemetry WHERE tenant_id = $1::uuid`, tenantID).Scan(&storageBytes)
	storageMB := storageBytes / 1024.0 / 1024.0

	_, err := db.Exec(ctx, `
		INSERT INTO tenant_usage_snapshots (tenant_id, period_start, period_end, messages_ingested, storage_mb, devices_total)
		VALUES ($1::uuid, $2, $3, $4, $5, $6)
		ON CONFLICT (tenant_id, period_start, period_end)
		DO UPDATE SET
			messages_ingested = EXCLUDED.messages_ingested,
			storage_mb = EXCLUDED.storage_mb,
			devices_total = EXCLUDED.devices_total,
			created_at = NOW()
	`, tenantID, start, end, messages, storageMB, devices)
	if err != nil {
		return err
	}

	_, _ = db.Exec(ctx, `
		INSERT INTO audit_log (tenant_id, event_type, event_category, severity, actor_type, action, result, metadata)
		VALUES ($1::uuid, 'billing.snapshot_generated', 'billing', 'info', 'system', 'snapshot', 'success', $2::jsonb)
	`, tenantID, toJSONB(map[string]interface{}{
		"period_start": start,
		"period_end":   end,
		"messages":     messages,
		"devices":      devices,
		"storage_mb":   storageMB,
	}))

	return nil
}
