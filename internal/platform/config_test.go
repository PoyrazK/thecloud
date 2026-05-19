package platform

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConfig(t *testing.T) {
	// Save original env and restore after test
	origPort := os.Getenv("PORT")
	origSecret := os.Getenv("STORAGE_SECRET")
	origDNSKey := os.Getenv("POWERDNS_API_KEY")
	defer func() {
		os.Setenv("PORT", origPort)
		os.Setenv("STORAGE_SECRET", origSecret)
		os.Setenv("POWERDNS_API_KEY", origDNSKey)
	}()

	t.Run("Default values", func(t *testing.T) {
		os.Unsetenv("PORT")
		os.Setenv("STORAGE_SECRET", "test-secret")
		os.Setenv("POWERDNS_API_KEY", "test-dns-key")
		cfg, err := NewConfig()
		require.NoError(t, err)
		assert.Equal(t, "8080", cfg.Port)
	})

	t.Run("Env override", func(t *testing.T) {
		os.Setenv("PORT", "9090")
		os.Setenv("STORAGE_SECRET", "test-secret")
		os.Setenv("POWERDNS_API_KEY", "test-dns-key")
		cfg, err := NewConfig()
		require.NoError(t, err)
		assert.Equal(t, "9090", cfg.Port)
	})

	t.Run("Missing POWERDNS_API_KEY fails", func(t *testing.T) {
		os.Setenv("STORAGE_SECRET", "test-secret")
		os.Unsetenv("POWERDNS_API_KEY")
		_, err := NewConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "POWERDNS_API_KEY")
	})
}

func TestGetEnv(t *testing.T) {
	t.Run("Existing env", func(t *testing.T) {
		err := os.Setenv("TEST_KEY", "test_value")
		require.NoError(t, err)
		defer func() { _ = os.Unsetenv("TEST_KEY") }()
		assert.Equal(t, "test_value", getEnv("TEST_KEY", "fallback"))
	})

	t.Run("Fallback value", func(t *testing.T) {
		err := os.Unsetenv("NON_EXISTENT_KEY")
		require.NoError(t, err)
		assert.Equal(t, "fallback", getEnv("NON_EXISTENT_KEY", "fallback"))
	})
}
