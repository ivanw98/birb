package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := load(envFrom(map[string]string{
		"DATABASE_URL":      "postgres://localhost/birb",
		"SUPABASE_JWKS_URL": "https://ref.supabase.co/auth/v1/.well-known/jwks.json",
	}))
	require.NoError(t, err)
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "authenticated", cfg.JWTAudience)
	assert.Equal(t, 25, cfg.MaxOpenConns)
	assert.Equal(t, 5, cfg.MaxIdleConns)
	assert.Equal(t, 30*time.Minute, cfg.ConnMaxLifetime)
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := load(envFrom(map[string]string{
		"PORT":                  "9090",
		"DATABASE_URL":          "postgres://db/x",
		"SUPABASE_JWKS_URL":     "https://j",
		"SUPABASE_JWT_ISSUER":   "https://ref.supabase.co/auth/v1",
		"SUPABASE_JWT_AUDIENCE": "custom",
		"DB_MAX_OPEN_CONNS":     "50",
		"DB_MAX_IDLE_CONNS":     "10",
		"STATIC_DIR":            "/app/web",
	}))
	require.NoError(t, err)
	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "custom", cfg.JWTAudience)
	assert.Equal(t, "https://ref.supabase.co/auth/v1", cfg.JWTIssuer)
	assert.Equal(t, 50, cfg.MaxOpenConns)
	assert.Equal(t, 10, cfg.MaxIdleConns)
	assert.Equal(t, "/app/web", cfg.StaticDir)
}

func TestLoadMissingRequired(t *testing.T) {
	_, err := load(envFrom(map[string]string{"PORT": "8080"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_URL")
	assert.Contains(t, err.Error(), "SUPABASE_JWKS_URL")
}

func TestAtoiOrFallback(t *testing.T) {
	assert.Equal(t, 25, atoiOr("", 25))
	assert.Equal(t, 25, atoiOr("garbage", 25))
	assert.Equal(t, 25, atoiOr("-3", 25), "non-positive falls back")
	assert.Equal(t, 42, atoiOr("42", 25))
}
