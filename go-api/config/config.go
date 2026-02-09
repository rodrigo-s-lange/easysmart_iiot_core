package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port string
	
	// PostgreSQL (auth database)
	PostgresHost     string
	PostgresPort     string
	PostgresDB       string
	PostgresUser     string
	PostgresPassword string
	
	// TimescaleDB (telemetry)
	TimescaleHost     string
	TimescalePort     string
	TimescaleDB       string
	TimescaleUser     string
	TimescalePassword string
	
	// Redis
	RedisHost     string
	RedisPort     string
	RedisPassword string
	
	// JWT
	JWTSecret            string
	JWTAccessExpiration  time.Duration
	JWTRefreshExpiration time.Duration
	
	// Rate Limit
	RateLimitDevicePerMin int64
	RateLimitDevicePerSec int64
	RateLimitSlotPerMin   int64
	RateLimitFailOpen     bool
	
	// Cache
	CacheTTLSeconds int64

	// CORS
	CORSAllowedOrigins string
	CORSAllowedMethods string
	CORSAllowedHeaders string

	// Manufacturing / provisioning
	ManufacturingMasterKey string
	BootstrapMaxSkewSecs   int64

	// Server shutdown
	ShutdownTimeoutSecs int64
}

func Load() *Config {
	return &Config{
		Port: getEnv("PORT", "3001"),
		
		PostgresHost:     getEnv("POSTGRES_HOST", "postgres"),
		PostgresPort:     getEnv("POSTGRES_PORT", "5432"),
		PostgresDB:       getEnv("POSTGRES_DB", "iiot_platform"),
		PostgresUser:     getEnv("POSTGRES_USER", "admin"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", "0039"),
		
		TimescaleHost:     getEnv("TIMESCALE_HOST", getEnv("POSTGRES_HOST", "timescaledb")),
		TimescalePort:     getEnv("TIMESCALE_PORT", getEnv("POSTGRES_PORT", "5432")),
		TimescaleDB:       getEnv("TIMESCALE_DB", "iiot_telemetry"),
		TimescaleUser:     getEnv("TIMESCALE_USER", getEnv("POSTGRES_USER", "admin")),
		TimescalePassword: getEnv("TIMESCALE_PASSWORD", getEnv("POSTGRES_PASSWORD", "0039")),
		
		RedisHost:     getEnv("REDIS_HOST", "redis"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		
		JWTSecret:            getEnv("JWT_SECRET", "change-this-in-production-please"),
		JWTAccessExpiration:  time.Hour,
		JWTRefreshExpiration: 30 * 24 * time.Hour,
		
		RateLimitDevicePerMin: getEnvInt64("RATE_LIMIT_DEVICE_PER_MIN", 12),
		RateLimitDevicePerSec: getEnvInt64("RATE_LIMIT_DEVICE_PER_SEC", 5),
		RateLimitSlotPerMin:   getEnvInt64("RATE_LIMIT_SLOT_PER_MIN", 12),
		RateLimitFailOpen:     getEnvBool("RATE_LIMIT_FAIL_OPEN", true),
		
		CacheTTLSeconds: getEnvInt64("CACHE_TTL_SECONDS", 0),

		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", ""),
		CORSAllowedMethods: getEnv("CORS_ALLOWED_METHODS", "GET,POST,PUT,DELETE,OPTIONS"),
		CORSAllowedHeaders: getEnv("CORS_ALLOWED_HEADERS", "Authorization,Content-Type"),

		ManufacturingMasterKey: getEnv("MANUFACTURING_MASTER_KEY", "change-this-manufacturing-key"),
		BootstrapMaxSkewSecs:   getEnvInt64("BOOTSTRAP_MAX_SKEW_SECS", 300),

		ShutdownTimeoutSecs: getEnvInt64("SHUTDOWN_TIMEOUT_SECS", 30),
	}
}

func (c *Config) PostgresURL() string {
	return "postgres://" + c.PostgresUser + ":" + c.PostgresPassword + "@" + c.PostgresHost + ":" + c.PostgresPort + "/" + c.PostgresDB
}

func (c *Config) TimescaleURL() string {
	return "postgres://" + c.TimescaleUser + ":" + c.TimescalePassword + "@" + c.TimescaleHost + ":" + c.TimescalePort + "/" + c.TimescaleDB
}

func (c *Config) RedisAddr() string {
	return c.RedisHost + ":" + c.RedisPort
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
