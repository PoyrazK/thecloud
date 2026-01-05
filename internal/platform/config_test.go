package platform

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConfig_Defaults(t *testing.T) {
	// Ensure env vars are unset to test defaults
	os.Unsetenv("PORT")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("APP_ENV")

	cfg, err := NewConfig()
	assert.NoError(t, err)
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "postgres://cloud:cloud@localhost:5433/thecloud", cfg.DatabaseURL)
	assert.Equal(t, "development", cfg.Environment)
}

func TestNewConfig_EnvVars(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/testdb")
	os.Setenv("APP_ENV", "production")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("APP_ENV")
	}()

	cfg, err := NewConfig()
	assert.NoError(t, err)
	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "postgres://test:test@localhost:5432/testdb", cfg.DatabaseURL)
	assert.Equal(t, "production", cfg.Environment)
}
