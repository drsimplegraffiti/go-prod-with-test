// Package config centralizes all environment-driven configuration for the
// service. Values are read once at startup and passed down explicitly,
// rather than being read ad-hoc from os.Getenv throughout the codebase.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the API server.
type Config struct {
	Env             string
	Port            string
	DatabaseURL     string
	JWTSecret       string
	JWTAccessTTL    time.Duration
	JWTRefreshTTL   time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	BcryptCost      int
	RateLimitRPS    int
}

// Load builds a Config from environment variables, applying sane defaults
// for local development and returning an error if any required production
// value is missing.
func Load() (*Config, error) {
	cfg := &Config{
		Env:             getEnv("APP_ENV", "development"),
		Port:            getEnv("PORT", "8080"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://xybug:Bassguitar1@localhost:5432/goprod?sslmode=disable"),
		JWTSecret:       getEnv("JWT_SECRET", ""),
		JWTAccessTTL:    getDurationEnv("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL:   getDurationEnv("JWT_REFRESH_TTL", 7*24*time.Hour),
		ReadTimeout:     getDurationEnv("HTTP_READ_TIMEOUT", 5*time.Second),
		WriteTimeout:    getDurationEnv("HTTP_WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:     getDurationEnv("HTTP_IDLE_TIMEOUT", 120*time.Second),
		ShutdownTimeout: getDurationEnv("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second),
		BcryptCost:      getIntEnv("BCRYPT_COST", 12),
		RateLimitRPS:    getIntEnv("RATE_LIMIT_RPS", 10),
	}

	if cfg.JWTSecret == "" {
		if cfg.Env == "production" {
			return nil, fmt.Errorf("JWT_SECRET must be set in production")
		}
		cfg.JWTSecret = "dev-only-insecure-secret-change-me"
	}

	if len(cfg.JWTSecret) < 32 && cfg.Env == "production" {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters in production")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
