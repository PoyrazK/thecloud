package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockIdentityService struct {
	mock.Mock
}

func (m *mockIdentityService) CreateKey(ctx context.Context, userID uuid.UUID, name string) (*domain.APIKey, error) {
	args := m.Called(ctx, userID, name)
	r0, _ := args.Get(0).(*domain.APIKey)
	return r0, args.Error(1)
}

func (m *mockIdentityService) ValidateAPIKey(ctx context.Context, key string) (*domain.APIKey, error) {
	args := m.Called(ctx, key)
	r0, _ := args.Get(0).(*domain.APIKey)
	return r0, args.Error(1)
}

func (m *mockIdentityService) GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.APIKey)
	return r0, args.Error(1)
}

func (m *mockIdentityService) ListKeys(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(ctx, userID)
	r0, _ := args.Get(0).([]*domain.APIKey)
	return r0, args.Error(1)
}

func (m *mockIdentityService) RevokeKey(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	return m.Called(ctx, userID, id).Error(0)
}

func (m *mockIdentityService) RotateKey(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(ctx, userID, id)
	r0, _ := args.Get(0).(*domain.APIKey)
	return r0, args.Error(1)
}

func (m *mockIdentityService) CreateServiceAccount(ctx context.Context, tenantID uuid.UUID, name, role string) (*domain.ServiceAccountWithSecret, error) {
	args := m.Called(ctx, tenantID, name, role)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.ServiceAccountWithSecret)
	return r0, args.Error(1)
}

func (m *mockIdentityService) GetServiceAccount(ctx context.Context, id uuid.UUID) (*domain.ServiceAccount, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.ServiceAccount)
	return r0, args.Error(1)
}

func (m *mockIdentityService) ListServiceAccounts(ctx context.Context, tenantID uuid.UUID) ([]*domain.ServiceAccount, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.ServiceAccount)
	return r0, args.Error(1)
}

func (m *mockIdentityService) UpdateServiceAccount(ctx context.Context, sa *domain.ServiceAccount) error {
	args := m.Called(ctx, sa)
	return args.Error(0)
}

func (m *mockIdentityService) DeleteServiceAccount(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockIdentityService) ValidateClientCredentials(ctx context.Context, clientID, clientSecret string) (string, error) {
	args := m.Called(ctx, clientID, clientSecret)
	return args.String(0), args.Error(1)
}

func (m *mockIdentityService) ValidateAccessToken(ctx context.Context, token string) (*domain.ServiceAccountClaims, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.ServiceAccountClaims)
	return r0, args.Error(1)
}

func (m *mockIdentityService) RotateServiceAccountSecret(ctx context.Context, saID uuid.UUID) (string, error) {
	args := m.Called(ctx, saID)
	return args.String(0), args.Error(1)
}

func (m *mockIdentityService) RevokeServiceAccountSecret(ctx context.Context, saID, secretID uuid.UUID) error {
	args := m.Called(ctx, saID, secretID)
	return args.Error(0)
}

func (m *mockIdentityService) ListServiceAccountSecrets(ctx context.Context, saID uuid.UUID) ([]*domain.ServiceAccountSecret, error) {
	args := m.Called(ctx, saID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.ServiceAccountSecret)
	return r0, args.Error(1)
}

func (m *mockIdentityService) TokenTTL() time.Duration {
	return time.Hour
}

func setupCachedIdentityTest(t *testing.T) (*mockIdentityService, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	base := new(mockIdentityService)
	return base, client
}

func TestCachedIdentityServiceValidateAPIKey(t *testing.T) {
	t.Parallel()
	base, client := setupCachedIdentityTest(t)
	svc := NewCachedIdentityService(base, client, slog.Default())
	ctx := context.Background()
	key := "test-key"
	apiKey := &domain.APIKey{ID: uuid.New(), Key: key, Name: "token1"}

	t.Run("cache miss", func(t *testing.T) {
		base.On("ValidateAPIKey", mock.Anything, key).Return(apiKey, nil).Once()

		res, err := svc.ValidateAPIKey(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, apiKey.ID, res.ID)
		base.AssertExpectations(t)
	})

	t.Run("cache hit", func(t *testing.T) {
		// Should not call base service
		res, err := svc.ValidateAPIKey(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, apiKey.ID, res.ID)
		base.AssertExpectations(t)
	})

	t.Run("invalid cache data", func(t *testing.T) {
		// Corrupt cache
		client.Set(ctx, "apikey:invalid", "corrupt", 0)
		base.On("ValidateAPIKey", mock.Anything, "invalid").Return(apiKey, nil).Once()

		res, err := svc.ValidateAPIKey(ctx, "invalid")
		require.NoError(t, err)
		assert.NotNil(t, res)
		base.AssertExpectations(t)
	})
}

func TestCachedIdentityServicePassthrough(t *testing.T) {
	t.Parallel()
	base, client := setupCachedIdentityTest(t)
	svc := NewCachedIdentityService(base, client, slog.Default())
	ctx := context.Background()
	userID := uuid.New()
	keyID := uuid.New()

	t.Run("CreateKey", func(t *testing.T) {
		base.On("CreateKey", mock.Anything, userID, "name").Return(&domain.APIKey{}, nil)
		_, err := svc.CreateKey(ctx, userID, "name")
		require.NoError(t, err)
		base.AssertExpectations(t)
	})

	t.Run("ListKeys", func(t *testing.T) {
		base.On("ListKeys", mock.Anything, userID).Return([]*domain.APIKey{}, nil)
		_, err := svc.ListKeys(ctx, userID)
		require.NoError(t, err)
		base.AssertExpectations(t)
	})

	t.Run("RevokeKey", func(t *testing.T) {
		apiKey := &domain.APIKey{ID: keyID, Key: "test-key"}
		base.On("GetAPIKeyByID", mock.Anything, keyID).Return(apiKey, nil)
		base.On("RevokeKey", mock.Anything, userID, keyID).Return(nil)
		err := svc.RevokeKey(ctx, userID, keyID)
		require.NoError(t, err)
		base.AssertExpectations(t)
	})

	t.Run("RotateKey", func(t *testing.T) {
		oldKey := &domain.APIKey{ID: keyID, Key: "old-key"}
		newKey := &domain.APIKey{ID: uuid.New(), Key: "new-key"}
		base.On("GetAPIKeyByID", mock.Anything, keyID).Return(oldKey, nil)
		base.On("RotateKey", mock.Anything, userID, keyID).Return(newKey, nil)
		_, err := svc.RotateKey(ctx, userID, keyID)
		require.NoError(t, err)
		base.AssertExpectations(t)
	})
}

func mustMarshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestCachedIdentityServiceCacheInvalidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("RevokeKey invalidates cache", func(t *testing.T) {
		base, client := setupCachedIdentityTest(t)
		svc := NewCachedIdentityService(base, client, slog.Default())

		keyID := uuid.New()
		rawKey := "thecloud_testkey123"
		apiKey := &domain.APIKey{ID: keyID, Key: rawKey, Name: "test"}

		// Pre-populate cache (simulating a prior ValidateAPIKey call)
		keyHash := computeKeyHash(rawKey)
		cacheKey := fmt.Sprintf("apikey:hash:%s", keyHash)
		client.Set(ctx, cacheKey, mustMarshal(apiKey), 5*time.Minute)

		// Confirm it's cached
		exists, err := client.Exists(ctx, cacheKey).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), exists)

		// Revoke — should delete from cache
		base.On("GetAPIKeyByID", mock.Anything, keyID).Return(apiKey, nil)
		base.On("RevokeKey", mock.Anything, mock.Anything, keyID).Return(nil)

		err = svc.RevokeKey(ctx, uuid.New(), keyID)
		require.NoError(t, err)

		// Verify cache was deleted
		exists, err = client.Exists(ctx, cacheKey).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(0), exists)
		base.AssertExpectations(t)
	})

	t.Run("RotateKey invalidates old cache entry", func(t *testing.T) {
		base, client := setupCachedIdentityTest(t)
		svc := NewCachedIdentityService(base, client, slog.Default())

		keyID := uuid.New()
		oldKey := "thecloud_oldkey456"
		newKey := &domain.APIKey{ID: uuid.New(), Key: "thecloud_newkey789", Name: "rotated"}

		oldHash := computeKeyHash(oldKey)
		oldCacheKey := fmt.Sprintf("apikey:hash:%s", oldHash)

		// Pre-populate cache with old key
		client.Set(ctx, oldCacheKey, mustMarshal(&domain.APIKey{ID: keyID, Key: oldKey, Name: "test"}), 5*time.Minute)
		exists, err := client.Exists(ctx, oldCacheKey).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), exists)

		base.On("GetAPIKeyByID", mock.Anything, keyID).Return(&domain.APIKey{ID: keyID, Key: oldKey, Name: "test"}, nil)
		base.On("RotateKey", mock.Anything, mock.Anything, keyID).Return(newKey, nil)

		result, err := svc.RotateKey(ctx, uuid.New(), keyID)
		require.NoError(t, err)
		assert.Equal(t, newKey.ID, result.ID)

		// Old cache key should be gone
		exists, err = client.Exists(ctx, oldCacheKey).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(0), exists)
		base.AssertExpectations(t)
	})
}
