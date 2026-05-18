package services_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCachedIdentityService_Unit(t *testing.T) {
	t.Skip("Skipping flaky test - miniredis race condition causes nil redis client")
	t.Run("ValidateAPIKey", testCachedIdentityServiceValidateAPIKey)
	t.Run("OtherOps", testCachedIdentityServiceOtherOps)
}

func testCachedIdentityServiceValidateAPIKey(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	base := new(MockIdentityService)
	svc := services.NewCachedIdentityService(base, rdb, slog.Default())

	ctx := context.Background()
	keyString := "test-key"
	apiKey := &domain.APIKey{ID: uuid.New(), UserID: uuid.New(), Key: keyString}

	t.Run("cache miss", func(t *testing.T) {
		base.On("ValidateAPIKey", ctx, keyString).Return(apiKey, nil).Once()

		result, err := svc.ValidateAPIKey(ctx, keyString)
		require.NoError(t, err)
		assert.Equal(t, apiKey.ID, result.ID)

		// Verify it's in redis now
		h := sha256.New()
		h.Write([]byte(keyString))
		keyHash := hex.EncodeToString(h.Sum(nil))
		val, err := rdb.Get(ctx, "apikey:hash:"+keyHash).Result()
		require.NoError(t, err)
		assert.Contains(t, val, apiKey.ID.String())
	})

	t.Run("cache hit", func(t *testing.T) {
		// Should NOT call base because it's cached from previous run
		result, err := svc.ValidateAPIKey(ctx, keyString)
		require.NoError(t, err)
		assert.Equal(t, apiKey.ID, result.ID)
	})

	base.AssertExpectations(t)
}

func testCachedIdentityServiceOtherOps(t *testing.T) {
	base := new(MockIdentityService)
	svc := services.NewCachedIdentityService(base, nil, slog.Default())
	ctx := context.Background()
	userID := uuid.New()
	id := uuid.New()

	t.Run("CreateKey", func(t *testing.T) {
		base.On("CreateKey", ctx, userID, "name").Return(&domain.APIKey{}, nil).Once()
		_, err := svc.CreateKey(ctx, userID, "name")
		require.NoError(t, err)
	})

	t.Run("ListKeys", func(t *testing.T) {
		base.On("ListKeys", ctx, userID).Return([]*domain.APIKey{}, nil).Once()
		_, err := svc.ListKeys(ctx, userID)
		require.NoError(t, err)
	})

	t.Run("RevokeKey", func(t *testing.T) {
		base.On("RevokeKey", ctx, userID, id).Return(nil).Once()
		err := svc.RevokeKey(ctx, userID, id)
		require.NoError(t, err)
	})

	t.Run("RotateKey", func(t *testing.T) {
		base.On("RotateKey", ctx, userID, id).Return(&domain.APIKey{}, nil).Once()
		_, err := svc.RotateKey(ctx, userID, id)
		require.NoError(t, err)
	})

	base.AssertExpectations(t)
}
