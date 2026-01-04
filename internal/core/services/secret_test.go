package services_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockSecretRepo struct{ mock.Mock }

func (m *MockSecretRepo) Create(ctx context.Context, s *domain.Secret) error {
	args := m.Called(ctx, s)
	return args.Error(0)
}
func (m *MockSecretRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Secret, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Secret), args.Error(1)
}
func (m *MockSecretRepo) GetByName(ctx context.Context, name string) (*domain.Secret, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Secret), args.Error(1)
}
func (m *MockSecretRepo) List(ctx context.Context) ([]*domain.Secret, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Secret), args.Error(1)
}
func (m *MockSecretRepo) Update(ctx context.Context, s *domain.Secret) error {
	args := m.Called(ctx, s)
	return args.Error(0)
}
func (m *MockSecretRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestSecretService_CreateAndGet(t *testing.T) {
	repo := new(MockSecretRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	_ = os.Setenv("SECRETS_ENCRYPTION_KEY", "test-key-must-be-32-bytes-long---")
	svc := services.NewSecretService(repo, eventSvc, logger)
	userID := uuid.New()
	ctxWithUser := setupTestUserCtx(userID) // Helper needed or inline

	name := "API_KEY"
	value := "super-secret-value"

	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Secret")).Return(nil)
	eventSvc.On("RecordEvent", mock.Anything, "SECRET_CREATE", mock.Anything, "SECRET", mock.Anything).Return(nil)

	// Test Create
	secret, err := svc.CreateSecret(ctxWithUser, name, value, "test desc")
	assert.NoError(t, err)
	assert.NotEqual(t, value, secret.EncryptedValue) // Should be encrypted

	// Test Get
	repo.On("GetByID", mock.Anything, secret.ID).Return(secret, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(s *domain.Secret) bool {
		return s.LastAccessedAt != nil
	})).Return(nil)
	eventSvc.On("RecordEvent", mock.Anything, "SECRET_ACCESS", secret.ID.String(), "SECRET", mock.Anything).Return(nil)

	fetched, err := svc.GetSecret(ctxWithUser, secret.ID)
	assert.NoError(t, err)
	assert.Equal(t, value, fetched.EncryptedValue) // Should be decrypted in response

	repo.AssertExpectations(t)
	eventSvc.AssertExpectations(t)
}

func setupTestUserCtx(userID uuid.UUID) context.Context {
	return appcontext.WithUserID(context.Background(), userID)
}

func TestSecretService_Delete(t *testing.T) {
	repo := new(MockSecretRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	_ = os.Setenv("SECRETS_ENCRYPTION_KEY", "test-key-must-be-32-bytes-long---")
	svc := services.NewSecretService(repo, eventSvc, logger)
	ctx := context.Background()
	secretID := uuid.New()

	repo.On("GetByID", ctx, secretID).Return(&domain.Secret{ID: secretID, Name: "TEST"}, nil)
	repo.On("Delete", ctx, secretID).Return(nil)
	eventSvc.On("RecordEvent", ctx, "SECRET_DELETE", secretID.String(), "SECRET", mock.Anything).Return(nil)

	err := svc.DeleteSecret(ctx, secretID)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
	eventSvc.AssertExpectations(t)
}

func TestSecretService_List(t *testing.T) {
	repo := new(MockSecretRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	_ = os.Setenv("SECRETS_ENCRYPTION_KEY", "test-key-must-be-32-bytes-long---")
	svc := services.NewSecretService(repo, eventSvc, logger)
	ctx := context.Background()

	secrets := []*domain.Secret{
		{ID: uuid.New(), Name: "secret1"},
		{ID: uuid.New(), Name: "secret2"},
	}
	repo.On("List", ctx).Return(secrets, nil)

	result, err := svc.ListSecrets(ctx)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	repo.AssertExpectations(t)
}
