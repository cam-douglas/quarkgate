package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds runtime configuration from environment variables.
type Config struct {
	ListenAddr       string
	DatabaseURL      string
	RedisURL         string
	VaultKEK         string
	CreditUSDMicro   int64
	DriversPath      string
	NodePath         string
	ConnectTimeout   time.Duration
	StreamIdleSec    int
	StreamMaxSec     int
	HoldTTLMinutes   int
	PlatformMargin   float64
	ReconcileAutoFix bool
	WorkerMaxRetries int
	BulkheadPerProvider int
	WorkerMetricsAddr string
}

func Load() Config {
	return Config{
		ListenAddr:     env("LISTEN_ADDR", ":8080"),
		DatabaseURL:    env("DATABASE_URL", "postgres://quarkgate:quarkgate@localhost:5433/quarkgate?sslmode=disable"),
		RedisURL:       env("REDIS_URL", "redis://localhost:6380/0"),
		VaultKEK:       env("VAULT_KEK", "dev-only-32-byte-key-change-me!!"),
		CreditUSDMicro: int64(envInt("CREDIT_USD_MICRO", 10000)),
		DriversPath:    env("DRIVERS_PATH", "drivers"),
		NodePath:       env("NODE_PATH", "node"),
		ConnectTimeout: time.Duration(envInt("CONNECT_TIMEOUT_SEC", 5)) * time.Second,
		StreamIdleSec:  envInt("STREAM_IDLE_SEC", 30),
		StreamMaxSec:   envInt("STREAM_MAX_SEC", 1800),
		HoldTTLMinutes:      envInt("HOLD_TTL_MINUTES", 15),
		PlatformMargin:      envFloat("PLATFORM_MARGIN", 0.05),
		ReconcileAutoFix:    envBool("RECONCILE_AUTO_FIX", false),
		WorkerMaxRetries:    envInt("WORKER_MAX_RETRIES", 5),
		BulkheadPerProvider: envInt("BULKHEAD_PER_PROVIDER", 0),
		WorkerMetricsAddr:   env("WORKER_METRICS_ADDR", ":9091"),
	}
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "1" || v == "true" || v == "TRUE"
	}
	return fallback
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return fallback
}
