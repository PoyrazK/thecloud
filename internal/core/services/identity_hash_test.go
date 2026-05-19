package services

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeKeyHash(t *testing.T) {
	// Test that the hash function produces consistent output
	key := "test-api-key-123"
	hash1 := computeKeyHash(key)
	hash2 := computeKeyHash(key)

	// Should be deterministic
	require.NotEmpty(t, hash1)
	assert.Equal(t, hash1, hash2)

	// Different keys should produce different hashes
	differentHash := computeKeyHash("different-key")
	assert.NotEqual(t, hash1, differentHash)
}

func TestGetServerSecret(t *testing.T) {
	// Save original values
	origSecrets := os.Getenv("SECRETS_ENCRYPTION_KEY")
	origTestSecrets := os.Getenv("TEST_SECRETS")
	defer func() {
		if origSecrets != "" {
			os.Setenv("SECRETS_ENCRYPTION_KEY", origSecrets)
		} else {
			os.Unsetenv("SECRETS_ENCRYPTION_KEY")
		}
		os.Setenv("TEST_SECRETS", origTestSecrets)
	}()

	t.Run("WithEnvVar", func(t *testing.T) {
		os.Setenv("SECRETS_ENCRYPTION_KEY", "test-secret-key")
		os.Unsetenv("TEST_SECRETS")
		secret := getServerSecret()
		assert.Equal(t, "test-secret-key", secret)
	})

	t.Run("WithoutEnvVar_UsesTestFallback", func(t *testing.T) {
		os.Unsetenv("SECRETS_ENCRYPTION_KEY")
		os.Setenv("TEST_SECRETS", "test-only-secret")
		secret := getServerSecret()
		assert.Equal(t, "test-only-secret", secret)
	})
}
