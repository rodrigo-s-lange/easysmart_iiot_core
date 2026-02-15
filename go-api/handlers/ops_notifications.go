package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"iiot-go-api/config"
	"iiot-go-api/utils"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func sendTelegramAndAuditAsync(
	db *pgxpool.Pool,
	cfg *config.Config,
	tenantID string,
	userID string,
	eventType string,
	message string,
	metadata string,
) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result := "skipped"
		if cfg != nil && cfg.TelegramBotToken != "" && cfg.TelegramChatID != "" {
			if err := utils.SendTelegramMessage(ctx, cfg.TelegramBotToken, cfg.TelegramChatID, message); err != nil {
				result = "failed"
			} else {
				result = "success"
			}
		}

		if db == nil {
			return
		}

		_, _ = db.Exec(ctx, `
			INSERT INTO audit_log (
				tenant_id, user_id, event_type, event_category, severity,
				actor_type, action, result, resource_type, metadata, timestamp
			)
			VALUES (
				NULLIF($1,'')::uuid, NULLIF($2,'')::uuid, $3, 'operations', 'info',
				'system', 'telegram_notify', $4, 'telegram', $5::jsonb, NOW()
			)
		`, tenantID, userID, eventType, result, metadata)
	}()
}

func notifyUserRegistered(
	db *pgxpool.Pool,
	cfg *config.Config,
	userID string,
	tenantID string,
	email string,
	role string,
) {
	emittedAt := formatOpsTimeNow()
	msg := buildOpsTelegramMessage(
		"🧾 [USUARIO] Cadastro",
		map[string]string{
			"email":     email,
			"role":      role,
			"user_id":   userID,
			"tenant_id": tenantID,
			"horario":   emittedAt,
		},
	)
	metaBytes, _ := json.Marshal(map[string]string{
		"email":      email,
		"role":       role,
		"user_id":    userID,
		"tenant_id":  tenantID,
		"emitted_at": emittedAt,
	})
	sendTelegramAndAuditAsync(db, cfg, tenantID, userID, "ops.user_registered_notified", msg, string(metaBytes))
}

func notifyDeviceCreated(
	db *pgxpool.Pool,
	cfg *config.Config,
	userID string,
	tenantID string,
	userEmail string,
	deviceID string,
	deviceLabel string,
	source string,
) {
	emittedAt := formatOpsTimeNow()
	msg := buildOpsTelegramMessage(
		"📟 [DEVICE] Cadastro",
		map[string]string{
			"tenant_id":    tenantID,
			"user_email":   userEmail,
			"device_id":    deviceID,
			"device_label": deviceLabel,
			"source":       source,
			"horario":      emittedAt,
		},
	)
	metaBytes, _ := json.Marshal(map[string]string{
		"tenant_id":    tenantID,
		"user_email":   userEmail,
		"device_id":    deviceID,
		"device_label": deviceLabel,
		"source":       source,
		"emitted_at":   emittedAt,
	})
	meta := string(metaBytes)
	sendTelegramAndAuditAsync(db, cfg, tenantID, userID, "ops.device_created_notified", msg, meta)
}

func formatOpsTimeNow() string {
	brt := time.FixedZone("BRT", -3*60*60)
	return time.Now().In(brt).Format("02/01/2006 15:04:05")
}

func buildOpsTelegramMessage(title string, fields map[string]string) string {
	keys := []string{
		"email",
		"role",
		"user_id",
		"tenant_id",
		"user_email",
		"device_id",
		"device_label",
		"source",
		"status",
		"quota",
		"request_id",
		"horario",
	}

	lines := []string{title}
	for _, k := range keys {
		v, ok := fields[k]
		if !ok || strings.TrimSpace(v) == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("• %s: %s", k, v))
	}

	return strings.Join(lines, "\n")
}
