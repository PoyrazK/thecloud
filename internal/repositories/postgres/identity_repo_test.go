//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentityRepository_Integration(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	repo := NewIdentityRepository(db)
	ctx := setupTestUser(t, db)

	// Cleanup
	_, _ = db.Exec(context.Background(), "DELETE FROM api_keys")

	// Create a test user for API keys
	userID := uuid.New()
	userRepo := NewUserRepo(db)
	testUser := &domain.User{
		ID:           userID,
		Email:        "apikey_test_" + userID.String() + "@example.com",
		PasswordHash: "hash",
		Name:         "API Key Test User",
		Role:         "user",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	err := userRepo.Create(context.Background(), testUser)
	require.NoError(t, err)

	var keyID uuid.UUID
	keyString := "test-api-key-12345"

	t.Run("CreateAPIKey", func(t *testing.T) {
		keyID = uuid.New()
		apiKey := &domain.APIKey{
			ID:        keyID,
			UserID:    userID,
			Key:       keyString,
			Name:      "test-key",
			CreatedAt: time.Now(),
		}

		err := repo.CreateAPIKey(ctx, apiKey)
		require.NoError(t, err)
	})

	t.Run("GetAPIKeyByKey", func(t *testing.T) {
		apiKey, err := repo.GetAPIKeyByKey(ctx, keyString)
		require.NoError(t, err)
		assert.Equal(t, keyID, apiKey.ID)
		assert.Equal(t, userID, apiKey.UserID)
		assert.Equal(t, "test-key", apiKey.Name)
	})

	t.Run("GetAPIKeyByID", func(t *testing.T) {
		apiKey, err := repo.GetAPIKeyByID(ctx, keyID)
		require.NoError(t, err)
		assert.Equal(t, keyString, apiKey.Key)
		assert.Equal(t, "test-key", apiKey.Name)
	})

	t.Run("ListAPIKeysByUserID", func(t *testing.T) {
		// Create another API key
		key2 := &domain.APIKey{
			ID:        uuid.New(),
			UserID:    userID,
			Key:       "another-api-key-67890",
			Name:      "another-key",
			CreatedAt: time.Now(),
		}
		err := repo.CreateAPIKey(ctx, key2)
		require.NoError(t, err)

		keys, err := repo.ListAPIKeysByUserID(ctx, userID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(keys), 2)

		// Verify we can find our keys
		foundTestKey := false
		foundAnotherKey := false
		for _, k := range keys {
			if k.Name == "test-key" {
				foundTestKey = true
			}
			if k.Name == "another-key" {
				foundAnotherKey = true
			}
		}
		assert.True(t, foundTestKey)
		assert.True(t, foundAnotherKey)
	})

	t.Run("DeleteAPIKey", func(t *testing.T) {
		err := repo.DeleteAPIKey(ctx, keyID)
		require.NoError(t, err)

		_, err = repo.GetAPIKeyByID(ctx, keyID)
		assert.Error(t, err)
	})

	t.Run("GetAPIKeyByKey_NotFound", func(t *testing.T) {
		_, err := repo.GetAPIKeyByKey(ctx, "non-existent-key")
		assert.Error(t, err)
	})

	t.Run("GetAPIKeyByID_NotFound", func(t *testing.T) {
		_, err := repo.GetAPIKeyByID(ctx, uuid.New())
		assert.Error(t, err)
	})
}
