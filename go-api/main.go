package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"iiot-go-api/config"
	"iiot-go-api/database"
	"iiot-go-api/handlers"
	"iiot-go-api/middleware"
	"iiot-go-api/utils"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx := context.Background()

	// Structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// Load config
	cfg := config.Load()
	if cfg.JWTSecret == "" || cfg.JWTSecret == "change-this-in-production-please" {
		log.Fatal("JWT_SECRET must be set to a strong value (refusing to start with default/empty)")
	}
	if cfg.ManufacturingMasterKey == "" || cfg.ManufacturingMasterKey == "change-this-manufacturing-key" {
		log.Fatal("MANUFACTURING_MASTER_KEY must be set to a strong value (refusing to start with default/empty)")
	}

	// Connect to databases
	db, err := database.Connect(ctx, cfg.PostgresURL(), cfg.TimescaleURL(), cfg.RedisAddr(), cfg.RedisPassword)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	slog.Info("database_connections_established")

	// Initialize middlewares
	jwtMiddleware := middleware.NewJWTMiddleware(cfg.JWTSecret)
	apiKeyMiddleware := middleware.NewAPIKeyMiddleware(db.Postgres, db.Redis)
	tenantMiddleware := middleware.NewTenantContextMiddleware(db.Postgres)
	rateLimitAuth := middleware.NewRateLimitAuth(db.Redis, 10, 60) // 10 attempts per minute
	corsConfig := middleware.NewCORSConfig(cfg.CORSAllowedOrigins, cfg.CORSAllowedMethods, cfg.CORSAllowedHeaders)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(db.Postgres, db.Redis, cfg)
	deviceHandler := handlers.NewDeviceHandler(db.Postgres, db.Redis, cfg)
	telemetryHandler := handlers.NewTelemetryHandler(db.Postgres, db.Timescale, db.Redis, cfg)

	// Setup routes
	mux := http.NewServeMux()

	// Health checks
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) {
		utils.WriteJSON(w, http.StatusOK, map[string]string{
			"status":    "ok",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		checks := map[string]string{}
		ready := true

		if err := db.Postgres.Ping(ctx); err != nil {
			checks["postgres"] = "error"
			ready = false
		} else {
			checks["postgres"] = "ok"
		}

		if err := db.Timescale.Ping(ctx); err != nil {
			checks["timescale"] = "error"
			ready = false
		} else {
			checks["timescale"] = "ok"
		}

		if db.Redis != nil {
			if err := db.Redis.Ping(ctx).Err(); err != nil {
				checks["redis"] = "error"
				ready = false
			} else {
				checks["redis"] = "ok"
			}
		} else {
			checks["redis"] = "unavailable"
		}

		status := http.StatusOK
		result := "ok"
		if !ready {
			status = http.StatusServiceUnavailable
			result = "degraded"
		}

		utils.WriteJSON(w, status, map[string]interface{}{
			"status":    result,
			"checks":    checks,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		utils.WriteJSON(w, http.StatusOK, map[string]string{
			"status":    "ok",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	mux.Handle("GET /metrics", promhttp.Handler())

	// Auth endpoints (with rate limiting, no JWT auth required)
	mux.Handle("POST /api/auth/register", rateLimitAuth.Limit(http.HandlerFunc(authHandler.Register)))
	mux.Handle("POST /api/auth/login", rateLimitAuth.Limit(http.HandlerFunc(authHandler.Login)))
	mux.Handle("POST /api/auth/refresh", rateLimitAuth.Limit(http.HandlerFunc(authHandler.Refresh)))

	// Device bootstrap + secret (no auth required - devices poll this)
	mux.Handle("POST /api/devices/bootstrap", http.HandlerFunc(deviceHandler.Bootstrap))
	mux.Handle("POST /api/devices/secret", http.HandlerFunc(deviceHandler.GetSecret))

	// Device claim (requires JWT + permission)
	mux.Handle("POST /api/devices/claim",
		jwtMiddleware.Authenticate(
			middleware.RequirePermission("devices:provision")(
				http.HandlerFunc(deviceHandler.ClaimDevice),
			),
		),
	)

	// Device list (requires JWT + RLS)
	mux.Handle("GET /api/devices",
		jwtMiddleware.Authenticate(
			tenantMiddleware.SetContext(
				middleware.RequirePermission("devices:read")(
					http.HandlerFunc(deviceHandler.ListDevices),
				),
			),
		),
	)

	// Device reset (requires JWT + permission)
	mux.Handle("POST /api/devices/reset",
		jwtMiddleware.Authenticate(
			middleware.RequirePermission("devices:write")(
				http.HandlerFunc(deviceHandler.ResetDevice),
			),
		),
	)

	// Telemetry webhook (requires API key)
	mux.Handle("POST /api/telemetry",
		apiKeyMiddleware.Authenticate(
			http.HandlerFunc(telemetryHandler.Webhook),
		),
	)

	// Telemetry latest (public for now, can add auth later)
	mux.HandleFunc("GET /api/telemetry/latest", telemetryHandler.GetLatest)
	// Telemetry active slots (public for now, can add auth later)
	mux.HandleFunc("GET /api/telemetry/slots", telemetryHandler.GetActiveSlots)

	// Middleware chain
	handler := middleware.RequestID(
		middleware.Logging(
			middleware.Recover(
				corsConfig.Handle(mux),
			),
		),
	)

	// Start server
	addr := ":" + cfg.Port
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("server_start",
		slog.String("addr", addr),
	)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server_error", slog.Any("error", err))
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop

	slog.Info("server_shutdown_start")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ShutdownTimeoutSecs)*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server_shutdown_error", slog.Any("error", err))
	} else {
		slog.Info("server_shutdown_complete")
	}
}
