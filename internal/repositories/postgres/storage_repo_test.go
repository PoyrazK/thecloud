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

func TestStorageRepository_Integration(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	repo := NewStorageRepository(db)
	ctx := setupTestUser(t, db)
	userID := appcontext.UserIDFromContext(ctx)

	// Cleanup
	_, _ = db.Exec(context.Background(), "DELETE FROM objects")

	bucket := "test-bucket"
	key := "test-file.txt"

	t.Run("SaveMeta", func(t *testing.T) {
		obj := &domain.Object{
			ID:          uuid.New(),
			UserID:      userID,
			ARN:         "arn:cloud:s3:::test-bucket/test-file.txt",
			Bucket:      bucket,
			Key:         key,
			SizeBytes:   1024,
			ContentType: "text/plain",
			CreatedAt:   time.Now(),
		}

		err := repo.SaveMeta(ctx, obj)
		require.NoError(t, err)
	})

	t.Run("GetMeta", func(t *testing.T) {
		obj, err := repo.GetMeta(ctx, bucket, key)
		require.NoError(t, err)
		assert.Equal(t, bucket, obj.Bucket)
		assert.Equal(t, key, obj.Key)
		assert.Equal(t, int64(1024), obj.SizeBytes)
		assert.Equal(t, "text/plain", obj.ContentType)
	})

	t.Run("List", func(t *testing.T) {
		// Add another object
		obj2 := &domain.Object{
			ID:          uuid.New(),
			UserID:      userID,
			ARN:         "arn:cloud:s3:::test-bucket/test-file2.txt",
			Bucket:      bucket,
			Key:         "test-file2.txt",
			SizeBytes:   2048,
			ContentType: "text/plain",
			CreatedAt:   time.Now(),
		}
		err := repo.SaveMeta(ctx, obj2)
		require.NoError(t, err)

		objects, err := repo.List(ctx, bucket)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(objects), 2)
	})

	t.Run("SaveMeta_UpdateExisting", func(t *testing.T) {
		// Update existing object
		obj := &domain.Object{
			ID:          uuid.New(),
			UserID:      userID,
			ARN:         "arn:cloud:s3:::test-bucket/test-file.txt",
			Bucket:      bucket,
			Key:         key,
			SizeBytes:   2048, // Updated size
			ContentType: "application/octet-stream",
			CreatedAt:   time.Now(),
		}

		err := repo.SaveMeta(ctx, obj)
		require.NoError(t, err)

		fetched, err := repo.GetMeta(ctx, bucket, key)
		require.NoError(t, err)
		assert.Equal(t, int64(2048), fetched.SizeBytes)
	})

	t.Run("SoftDelete", func(t *testing.T) {
		err := repo.SoftDelete(ctx, bucket, key)
		require.NoError(t, err)

		// Should not be found after soft delete
		_, err = repo.GetMeta(ctx, bucket, key)
		assert.Error(t, err)
	})

	t.Run("SoftDelete_NotFound", func(t *testing.T) {
		err := repo.SoftDelete(ctx, bucket, "non-existent-key")
		assert.Error(t, err)
	})

	t.Run("GetMeta_NotFound", func(t *testing.T) {
		_, err := repo.GetMeta(ctx, "non-existent-bucket", "non-existent-key")
		assert.Error(t, err)
	})
}
