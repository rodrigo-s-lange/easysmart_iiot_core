package middleware

import (
	"context"
	"fmt"
	"iiot-go-api/metrics"
	"iiot-go-api/utils"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimitAuth middleware for authentication endpoints
// Limits requests by IP address to prevent brute-force attacks
type RateLimitAuth struct {
	Redis       *redis.Client
	MaxAttempts int64
	WindowSecs  int64
}

func NewRateLimitAuth(redisClient *redis.Client, maxAttempts, windowSecs int64) *RateLimitAuth {
	return &RateLimitAuth{
		Redis:       redisClient,
		MaxAttempts: maxAttempts,
		WindowSecs:  windowSecs,
	}
}

func (rl *RateLimitAuth) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rl.Redis == nil {
			next.ServeHTTP(w, r)
			return
		}

		// Get client IP (handle X-Forwarded-For for proxies)
		clientIP := r.RemoteAddr
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Use only the first IP in the list
			parts := strings.Split(xff, ",")
			clientIP = strings.TrimSpace(parts[0])
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			clientIP = xri
		}

		// Remove port from IP address
		if host, _, err := net.SplitHostPort(clientIP); err == nil {
			clientIP = host
		}

		// Redis key for rate limiting
		key := fmt.Sprintf("rl:auth:%s", clientIP)
		ctx := context.Background()

		// Increment counter
		val, err := rl.Redis.Incr(ctx, key).Result()
		if err != nil {
			// On Redis error, fail open (allow request)
			next.ServeHTTP(w, r)
			return
		}

		// Set expiration on first request
		if val == 1 {
			rl.Redis.Expire(ctx, key, time.Duration(rl.WindowSecs)*time.Second)
		}

		// Check if limit exceeded
		if val > rl.MaxAttempts {
			metrics.AuthRateLimited(r.URL.Path)
			// Get TTL for Retry-After header
			ttl, _ := rl.Redis.TTL(ctx, key).Result()
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(ttl.Seconds())))
			utils.WriteError(w, http.StatusTooManyRequests, "Too many authentication attempts. Please try again later.")
			return
		}

		next.ServeHTTP(w, r)
	})
}
