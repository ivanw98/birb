// Package config loads runtime configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config is the resolved service configuration.
type Config struct {
	Port            string
	DatabaseURL     string
	JWKSURL         string
	JWTIssuer       string
	JWTAudience     string
	StaticDir       string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ShutdownTimeout time.Duration
}

// Load reads configuration from the process environment.
func Load() (Config, error) { return load(os.Getenv) }

// load is the testable core, parameterised by a getenv function.
func load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Port:            orDefault(getenv("PORT"), "8080"),
		DatabaseURL:     getenv("DATABASE_URL"),
		JWKSURL:         getenv("SUPABASE_JWKS_URL"),
		JWTIssuer:       getenv("SUPABASE_JWT_ISSUER"),
		JWTAudience:     orDefault(getenv("SUPABASE_JWT_AUDIENCE"), "authenticated"),
		StaticDir:       getenv("STATIC_DIR"),
		MaxOpenConns:    atoiOr(getenv("DB_MAX_OPEN_CONNS"), 25),
		MaxIdleConns:    atoiOr(getenv("DB_MAX_IDLE_CONNS"), 5),
		ConnMaxLifetime: 30 * time.Minute,
		ShutdownTimeout: 15 * time.Second,
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.JWKSURL == "" {
		missing = append(missing, "SUPABASE_JWKS_URL")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %v", missing)
	}
	return cfg, nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func atoiOr(v string, def int) int {
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return n
	}
	return def
}
