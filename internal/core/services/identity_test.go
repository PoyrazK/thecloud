package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockIdentityRepo
type MockIdentityRepo struct {
	mock.Mock
}

func (m *MockIdentityRepo) CreateAPIKey(ctx context.Context, apiKey *domain.APIKey) error {
	args := m.Called(ctx, apiKey)
	return args.Error(0)
}
func (m *MockIdentityRepo) GetAPIKeyByKey(ctx context.Context, key string) (*domain.APIKey, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}
func (m *MockIdentityRepo) GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}
func (m *MockIdentityRepo) ListAPIKeysByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.APIKey), args.Error(1)
}
func (m *MockIdentityRepo) DeleteAPIKey(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestIdentityService_CreateKey_Success(t *testing.T) {
	repo := new(MockIdentityRepo)
	audit := new(MockAuditService)
	svc := services.NewIdentityService(repo, audit)
	ctx := context.Background()
	userID := uuid.New()

	repo.On("CreateAPIKey", ctx, mock.MatchedBy(func(k *domain.APIKey) bool {
		return k.UserID == userID && len(k.Key) > 10 && k.Name == "Test Key"
	})).Return(nil)

	key, err := svc.CreateKey(ctx, userID, "Test Key")

	assert.NoError(t, err)
	assert.NotNil(t, key)
	assert.Contains(t, key.Key, "thecloud_")
	assert.Equal(t, userID, key.UserID)
	repo.AssertExpectations(t)
}

func TestIdentityService_ValidateAPIKey_Success(t *testing.T) {
	repo := new(MockIdentityRepo)
	audit := new(MockAuditService)
	svc := services.NewIdentityService(repo, audit)
	ctx := context.Background()

	keyStr := "thecloud_abc123"
	userID := uuid.New()
	apiKey := &domain.APIKey{ID: uuid.New(), UserID: userID, Key: keyStr}

	repo.On("GetAPIKeyByKey", ctx, keyStr).Return(apiKey, nil)

	result, err := svc.ValidateAPIKey(ctx, keyStr)

	assert.NoError(t, err)
	assert.Equal(t, apiKey.ID, result.ID)
	assert.Equal(t, userID, result.UserID)
	repo.AssertExpectations(t)
}

func TestIdentityService_ValidateAPIKey_NotFound(t *testing.T) {
	repo := new(MockIdentityRepo)
	audit := new(MockAuditService)
	svc := services.NewIdentityService(repo, audit)
	ctx := context.Background()

	repo.On("GetAPIKeyByKey", ctx, "invalid-key").Return(nil, assert.AnError)

	result, err := svc.ValidateAPIKey(ctx, "invalid-key")

	assert.Error(t, err)
	assert.Nil(t, result)
	repo.AssertExpectations(t)
}
