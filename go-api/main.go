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
	"github.com/redis/go-redis/v9"
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

type latestTelemetry struct {
	DeviceID  string          `json:"device_id"`
	Slot      int             `json:"slot"`
	Value     json.RawMessage `json:"value"`
	Timestamp string          `json:"timestamp"`
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

	rdb := newRedisClient(ctx)
	limiter := newRateLimiter(rdb)

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

		if limiter != nil {
			allowed, err := limiter.Allow(ctx, deviceToken, slot)
			if err != nil {
				if limiter.failOpen {
					log.Printf("rate limit check failed (fail-open): %v", err)
				} else {
					writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "Rate limiter unavailable"})
					return
				}
			} else if !allowed {
				log.Printf("rate_limit_exceeded device=%s slot=%d", deviceToken, slot)
				writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "Rate limit exceeded"})
				return
			}
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

		if rdb != nil {
			if err := cacheLatest(ctx, rdb, deviceID, slot, req.Payload, ts); err != nil {
				log.Printf("cache error: %v", err)
			}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success":   true,
			"device_id": deviceID,
			"slot":      slot,
		})
	})

	mux.HandleFunc("/api/telemetry/latest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "Method not allowed"})
			return
		}

		if rdb == nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "Cache unavailable"})
			return
		}

		deviceToken := r.URL.Query().Get("token")
		slotStr := r.URL.Query().Get("slot")
		if deviceToken == "" || slotStr == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "token and slot are required"})
			return
		}

		slot, err := strconv.Atoi(slotStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "Invalid slot"})
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

		value, err := getCachedLatest(ctx, rdb, deviceID, slot)
		if err != nil {
			if errors.Is(err, errCacheMiss) {
				writeJSON(w, http.StatusNotFound, errorResponse{Error: "No cached value"})
				return
			}
			log.Printf("cache error: %v", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "Internal server error"})
			return
		}

		writeJSON(w, http.StatusOK, value)
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

type rateLimiter struct {
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

func newRedisClient(ctx context.Context) *redis.Client {
	host := getEnv("REDIS_HOST", "redis")
	port := getEnv("REDIS_PORT", "6379")
	pass := getEnv("REDIS_PASSWORD", "")

	rdb := redis.NewClient(&redis.Options{
		Addr:     host + ":" + port,
		Password: pass,
		DB:       0,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("redis unavailable: %v", err)
		return nil
	}

	return rdb
}

func newRateLimiter(rdb *redis.Client) *rateLimiter {
	if rdb == nil {
		return nil
	}
	return &rateLimiter{
		rdb:          rdb,
		devicePerMin: getEnvInt64("RATE_LIMIT_DEVICE_PER_MIN", 12),
		devicePerSec: getEnvInt64("RATE_LIMIT_DEVICE_PER_SEC", 5),
		slotPerMin:   getEnvInt64("RATE_LIMIT_SLOT_PER_MIN", 12),
		failOpen:     getEnvBool("RATE_LIMIT_FAIL_OPEN", true),
	}
}

func (r *rateLimiter) Allow(ctx context.Context, token string, slot int) (bool, error) {
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

	sec, err := toInt64(values[0])
	if err != nil {
		return false, err
	}
	min, err := toInt64(values[1])
	if err != nil {
		return false, err
	}
	slotMin, err := toInt64(values[2])
	if err != nil {
		return false, err
	}

	if sec > r.devicePerSec || min > r.devicePerMin || slotMin > r.slotPerMin {
		return false, nil
	}
	return true, nil
}

var errCacheMiss = errors.New("cache miss")

func cacheLatest(ctx context.Context, rdb *redis.Client, deviceID string, slot int, payload json.RawMessage, ts time.Time) error {
	item := latestTelemetry{
		DeviceID:  deviceID,
		Slot:      slot,
		Value:     payload,
		Timestamp: ts.UTC().Format(time.RFC3339),
	}

	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}

	key := cacheKey(deviceID, slot)
	ttl := getEnvInt64("CACHE_TTL_SECONDS", 0)
	if ttl > 0 {
		return rdb.Set(ctx, key, raw, time.Duration(ttl)*time.Second).Err()
	}
	return rdb.Set(ctx, key, raw, 0).Err()
}

func getCachedLatest(ctx context.Context, rdb *redis.Client, deviceID string, slot int) (latestTelemetry, error) {
	key := cacheKey(deviceID, slot)
	raw, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return latestTelemetry{}, errCacheMiss
		}
		return latestTelemetry{}, err
	}

	var item latestTelemetry
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return latestTelemetry{}, err
	}
	return item, nil
}

func cacheKey(deviceID string, slot int) string {
	return "latest:device:" + deviceID + ":slot:" + strconv.Itoa(slot)
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

func getEnvInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func toInt64(v interface{}) (int64, error) {
	switch t := v.(type) {
	case int64:
		return t, nil
	case int:
		return int64(t), nil
	case string:
		return strconv.ParseInt(t, 10, 64)
	default:
		return 0, errors.New("invalid rate limit value")
	}
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
