package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"iiot-go-api/config"
	"iiot-go-api/utils"
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
	msg := fmt.Sprintf(
		"Novo usuario cadastrado\n- email: %s\n- role: %s\n- user_id: %s\n- tenant_id: %s",
		email, role, userID, tenantID,
	)
	metaBytes, _ := json.Marshal(map[string]string{
		"email":     email,
		"role":      role,
		"user_id":   userID,
		"tenant_id": tenantID,
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
	msg := fmt.Sprintf(
		"Novo dispositivo cadastrado\n- tenant_id: %s\n- user_email: %s\n- device_id: %s\n- device_label: %s\n- source: %s",
		tenantID, userEmail, deviceID, deviceLabel, source,
	)
	metaBytes, _ := json.Marshal(map[string]string{
		"tenant_id":    tenantID,
		"user_email":   userEmail,
		"device_id":    deviceID,
		"device_label": deviceLabel,
		"source":       source,
	})
	meta := string(metaBytes)
	sendTelegramAndAuditAsync(db, cfg, tenantID, userID, "ops.device_created_notified", msg, meta)
}
