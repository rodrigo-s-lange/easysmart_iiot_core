package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"iiot-go-api/config"
	"iiot-go-api/database"
	"iiot-go-api/handlers"
	"iiot-go-api/middleware"
	"iiot-go-api/utils"
)

func main() {
	ctx := context.Background()

	// Load config
	cfg := config.Load()

	// Connect to databases
	db, err := database.Connect(ctx, cfg.PostgresURL(), cfg.TimescaleURL(), cfg.RedisAddr(), cfg.RedisPassword)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	log.Println("✅ Database connections established")

	// Initialize middlewares
	jwtMiddleware := middleware.NewJWTMiddleware(cfg.JWTSecret)
	apiKeyMiddleware := middleware.NewAPIKeyMiddleware(db.Postgres, db.Redis)
	tenantMiddleware := middleware.NewTenantContextMiddleware(db.Postgres)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(db.Postgres, cfg)
	deviceHandler := handlers.NewDeviceHandler(db.Postgres, db.Redis)
	telemetryHandler := handlers.NewTelemetryHandler(db.Postgres, db.Timescale, db.Redis, cfg)

	// Setup routes
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		utils.WriteJSON(w, http.StatusOK, map[string]string{
			"status":    "ok",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Auth endpoints (no auth required)
	mux.HandleFunc("/api/auth/login", authHandler.Login)

	// Device bootstrap (no auth required - devices poll this)
	mux.HandleFunc("/api/devices/bootstrap", deviceHandler.Bootstrap)
	mux.HandleFunc("/api/devices/secret", deviceHandler.GetSecret)

	// Device claim (requires JWT + permission)
	mux.Handle("/api/devices/claim",
		jwtMiddleware.Authenticate(
			middleware.RequirePermission("devices:provision")(
				http.HandlerFunc(deviceHandler.ClaimDevice),
			),
		),
	)

	// Device list (requires JWT + RLS)
	mux.Handle("/api/devices",
		jwtMiddleware.Authenticate(
			tenantMiddleware.SetContext(
				middleware.RequirePermission("devices:read")(
					http.HandlerFunc(deviceHandler.ListDevices),
				),
			),
		),
	)

	// Telemetry webhook (requires API key)
	mux.Handle("/api/telemetry",
		apiKeyMiddleware.Authenticate(
			http.HandlerFunc(telemetryHandler.Webhook),
		),
	)

	// Telemetry latest (public for now, can add auth later)
	mux.HandleFunc("/api/telemetry/latest", telemetryHandler.GetLatest)

	// Logging middleware
	handler := loggingMiddleware(mux)

	// Start server
	addr := ":" + cfg.Port
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("🚀 Go API running on %s", addr)
	log.Fatal(server.ListenAndServe())
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
