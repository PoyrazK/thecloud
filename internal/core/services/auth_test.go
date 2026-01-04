package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

// MockUserRepo
type MockUserRepo struct {
	mock.Mock
}

func (m *MockUserRepo) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}
func (m *MockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepo) Update(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

// MockIdentityService
type MockIdentityService struct {
	mock.Mock
}

func (m *MockIdentityService) CreateKey(ctx context.Context, userID uuid.UUID, name string) (*domain.APIKey, error) {
	args := m.Called(ctx, userID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}
func (m *MockIdentityService) ValidateAPIKey(ctx context.Context, key string) (*domain.APIKey, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func TestAuthService_Register_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	identitySvc := new(MockIdentityService)

	svc := services.NewAuthService(userRepo, identitySvc)
	ctx := context.Background()

	email := "test@example.com"
	password := "password123"
	name := "Test User"

	userRepo.On("GetByEmail", ctx, email).Return(nil, nil) // Not existing
	userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).Return(nil)

	user, err := svc.Register(ctx, email, password, name)

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, email, user.Email)
	assert.Equal(t, name, user.Name)
	assert.NotEmpty(t, user.PasswordHash)
	assert.NotEqual(t, password, user.PasswordHash) // Hashed

	userRepo.AssertExpectations(t)
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	userRepo := new(MockUserRepo)
	identitySvc := new(MockIdentityService)

	svc := services.NewAuthService(userRepo, identitySvc)
	ctx := context.Background()

	email := "existing@example.com"
	existing := &domain.User{ID: uuid.New(), Email: email}

	userRepo.On("GetByEmail", ctx, email).Return(existing, nil)

	user, err := svc.Register(ctx, email, "pass", "name")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "already exists")
}

func TestAuthService_Login_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	identitySvc := new(MockIdentityService)

	svc := services.NewAuthService(userRepo, identitySvc)
	ctx := context.Background()

	email := "login@example.com"
	password := "correct-password"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	userID := uuid.New()
	user := &domain.User{ID: userID, Email: email, PasswordHash: string(hashedPassword)}

	userRepo.On("GetByEmail", ctx, email).Return(user, nil)
	identitySvc.On("CreateKey", ctx, userID, "Default Key").Return(&domain.APIKey{
		Key:       "sk_test_123",
		UserID:    userID,
		CreatedAt: time.Now(),
	}, nil)

	resultUser, apiKey, err := svc.Login(ctx, email, password)

	assert.NoError(t, err)
	assert.NotNil(t, resultUser)
	assert.Equal(t, "sk_test_123", apiKey)
	userRepo.AssertExpectations(t)
	identitySvc.AssertExpectations(t)
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	userRepo := new(MockUserRepo)
	identitySvc := new(MockIdentityService)

	svc := services.NewAuthService(userRepo, identitySvc)
	ctx := context.Background()

	email := "wrong@example.com"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("real-password"), bcrypt.DefaultCost)
	user := &domain.User{ID: uuid.New(), Email: email, PasswordHash: string(hashedPassword)}

	userRepo.On("GetByEmail", ctx, email).Return(user, nil)

	resultUser, apiKey, err := svc.Login(ctx, email, "wrong-password")

	assert.Error(t, err)
	assert.Nil(t, resultUser)
	assert.Empty(t, apiKey)
	assert.Contains(t, err.Error(), "invalid")
}

func TestAuthService_ValidateUser(t *testing.T) {
	userRepo := new(MockUserRepo)
	identitySvc := new(MockIdentityService)

	svc := services.NewAuthService(userRepo, identitySvc)
	ctx := context.Background()
	userID := uuid.New()
	user := &domain.User{ID: userID, Email: "validate@example.com"}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)

	result, err := svc.ValidateUser(ctx, userID)

	assert.NoError(t, err)
	assert.Equal(t, userID, result.ID)
	userRepo.AssertExpectations(t)
}
