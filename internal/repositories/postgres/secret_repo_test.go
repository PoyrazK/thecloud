//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretRepository_Integration(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	repo := NewSecretRepository(db)
	ctx := setupTestUser(t, db)
	userID := appcontext.UserIDFromContext(ctx)

	// Cleanup
	_, _ = db.Exec(context.Background(), "DELETE FROM secrets")

	var secretID uuid.UUID

	t.Run("CreateSecret", func(t *testing.T) {
		secretID = uuid.New()
		secret := &domain.Secret{
			ID:             secretID,
			UserID:         userID,
			Name:           "test-secret",
			EncryptedValue: "encrypted-data-here",
			Description:    "Test secret for integration testing",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		err := repo.Create(ctx, secret)
		require.NoError(t, err)
	})

	t.Run("GetByID", func(t *testing.T) {
		secret, err := repo.GetByID(ctx, secretID)
		require.NoError(t, err)
		assert.Equal(t, "test-secret", secret.Name)
		assert.Equal(t, "encrypted-data-here", secret.EncryptedValue)
		assert.Equal(t, "Test secret for integration testing", secret.Description)
	})

	t.Run("GetByName", func(t *testing.T) {
		secret, err := repo.GetByName(ctx, "test-secret")
		require.NoError(t, err)
		assert.Equal(t, secretID, secret.ID)
		assert.Equal(t, "encrypted-data-here", secret.EncryptedValue)
	})

	t.Run("List", func(t *testing.T) {
		// Create another secret
		secret2 := &domain.Secret{
			ID:             uuid.New(),
			UserID:         userID,
			Name:           "another-secret",
			EncryptedValue: "more-encrypted-data",
			Description:    "Another test secret",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		err := repo.Create(ctx, secret2)
		require.NoError(t, err)

		secrets, err := repo.List(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(secrets), 2)

		// Verify secrets are sorted by name
		if len(secrets) >= 2 {
			assert.LessOrEqual(t, secrets[0].Name, secrets[1].Name)
		}
	})

	t.Run("Update", func(t *testing.T) {
		secret, err := repo.GetByID(ctx, secretID)
		require.NoError(t, err)

		secret.EncryptedValue = "updated-encrypted-data"
		secret.Description = "Updated description"
		accessTime := time.Now()
		secret.LastAccessedAt = &accessTime

		err = repo.Update(ctx, secret)
		require.NoError(t, err)

		updated, err := repo.GetByID(ctx, secretID)
		require.NoError(t, err)
		assert.Equal(t, "updated-encrypted-data", updated.EncryptedValue)
		assert.Equal(t, "Updated description", updated.Description)
		assert.NotNil(t, updated.LastAccessedAt)
	})

	t.Run("Delete", func(t *testing.T) {
		err := repo.Delete(ctx, secretID)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, secretID)
		assert.Error(t, err)
	})

	t.Run("GetByID_NotFound", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.New())
		assert.Error(t, err)
	})

	t.Run("GetByName_NotFound", func(t *testing.T) {
		_, err := repo.GetByName(ctx, "non-existent-secret")
		assert.Error(t, err)
	})
}
