package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type telemetryRequest struct {
	ClientID  string          `json:"clientid"`
	Topic     string          `json:"topic"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp string          `json:"timestamp"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func main() {
	ctx := context.Background()

	authPool, err := pgxpool.New(ctx, buildPostgresURL())
	if err != nil {
		log.Fatalf("postgres connection error: %v", err)
	}
	defer authPool.Close()

	telemetryPool, err := pgxpool.New(ctx, buildTimescaleURL())
	if err != nil {
		log.Fatalf("timescale connection error: %v", err)
	}
	defer telemetryPool.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":    "ok",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	mux.HandleFunc("/api/telemetry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "Method not allowed"})
			return
		}

		var req telemetryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "Invalid JSON body"})
			return
		}

		deviceToken, slot, err := parseTopic(req.Topic)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}

		deviceID, err := findDeviceID(ctx, authPool, deviceToken)
		if err != nil {
			if errors.Is(err, errDeviceNotFound) {
				writeJSON(w, http.StatusNotFound, errorResponse{Error: "Device not found or inactive"})
				return
			}
			log.Printf("db error: %v", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "Internal server error"})
			return
		}

		ts, err := parseTimestamp(req.Timestamp)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "Invalid timestamp"})
			return
		}

		if err := insertTelemetry(ctx, telemetryPool, deviceID, slot, req.Payload, ts); err != nil {
			log.Printf("insert error: %v", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "Internal server error"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success":   true,
			"device_id": deviceID,
			"slot":      slot,
		})
	})

	addr := ":" + getEnv("PORT", "3001")
	server := &http.Server{
		Addr:         addr,
		Handler:      logRequest(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Go API running on %s", addr)
	log.Fatal(server.ListenAndServe())
}

func buildPostgresURL() string {
	host := getEnv("POSTGRES_HOST", "postgres")
	port := getEnv("POSTGRES_PORT", "5432")
	db := getEnv("POSTGRES_DB", "iiot_platform")
	user := getEnv("POSTGRES_USER", "admin")
	pass := getEnv("POSTGRES_PASSWORD", "0039")
	return "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + db
}

func buildTimescaleURL() string {
	host := getEnv("TIMESCALE_HOST", getEnv("POSTGRES_HOST", "timescaledb"))
	port := getEnv("TIMESCALE_PORT", getEnv("POSTGRES_PORT", "5432"))
	db := getEnv("TIMESCALE_DB", getEnv("POSTGRES_DB", "iiot_telemetry"))
	user := getEnv("TIMESCALE_USER", getEnv("POSTGRES_USER", "admin"))
	pass := getEnv("TIMESCALE_PASSWORD", getEnv("POSTGRES_PASSWORD", "0039"))
	return "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + db
}

func parseTopic(topic string) (string, int, error) {
	parts := strings.Split(topic, "/")
	if len(parts) < 5 {
		return "", 0, errors.New("Invalid topic format")
	}
	deviceToken := parts[1]
	slot, err := strconv.Atoi(parts[4])
	if deviceToken == "" || err != nil {
		return "", 0, errors.New("Invalid topic format")
	}
	return deviceToken, slot, nil
}

var errDeviceNotFound = errors.New("device not found")

func findDeviceID(ctx context.Context, pool *pgxpool.Pool, token string) (string, error) {
	var id string
	err := pool.QueryRow(ctx,
		"SELECT id FROM devices WHERE token::text = $1 AND status = 'active'",
		token,
	).Scan(&id)
	if err != nil {
		return "", errDeviceNotFound
	}
	return id, nil
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

func insertTelemetry(ctx context.Context, pool *pgxpool.Pool, deviceID string, slot int, payload json.RawMessage, ts time.Time) error {
	_, err := pool.Exec(ctx,
		"INSERT INTO telemetry (device_id, slot, value, timestamp) VALUES ($1, $2, $3, $4)",
		deviceID, slot, payload, ts,
	)
	return err
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
