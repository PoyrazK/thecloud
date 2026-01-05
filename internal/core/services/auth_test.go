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

// Mocks are now in shared_test.go

// Helper to get a strong password for tests
const strongTestPassword = "CorrectHorseBatteryStaple123!"

func TestAuthService_Register_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	identitySvc := new(MockIdentityService)
	auditSvc := new(MockAuditService)

	svc := services.NewAuthService(userRepo, identitySvc, auditSvc)
	ctx := context.Background()

	email := "test@example.com"
	password := strongTestPassword
	name := "Test User"

	userRepo.On("GetByEmail", ctx, email).Return(nil, nil) // Not existing
	userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).Return(nil)
	auditSvc.On("Log", ctx, mock.Anything, "user.register", "user", mock.Anything, mock.Anything).Return(nil)

	user, err := svc.Register(ctx, email, password, name)

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, email, user.Email)
	assert.Equal(t, name, user.Name)
	assert.NotEmpty(t, user.PasswordHash)
	assert.NotEqual(t, password, user.PasswordHash) // Hashed

	userRepo.AssertExpectations(t)
}

func TestAuthService_Register_WeakPassword(t *testing.T) {
	userRepo := new(MockUserRepo)
	identitySvc := new(MockIdentityService)
	auditSvc := new(MockAuditService)

	svc := services.NewAuthService(userRepo, identitySvc, auditSvc)
	ctx := context.Background()

	// "123" is definitely too weak
	user, err := svc.Register(ctx, "test@example.com", "123", "User")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "password is too weak")
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	userRepo := new(MockUserRepo)
	identitySvc := new(MockIdentityService)
	auditSvc := new(MockAuditService)

	svc := services.NewAuthService(userRepo, identitySvc, auditSvc)
	ctx := context.Background()

	email := "existing@example.com"
	existing := &domain.User{ID: uuid.New(), Email: email}

	userRepo.On("GetByEmail", ctx, email).Return(existing, nil)

	user, err := svc.Register(ctx, email, strongTestPassword, "name")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "already exists")
}

func TestAuthService_Login_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	identitySvc := new(MockIdentityService)
	auditSvc := new(MockAuditService)

	svc := services.NewAuthService(userRepo, identitySvc, auditSvc)
	ctx := context.Background()

	email := "login@example.com"
	password := "correct-password-is-long-enough"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	userID := uuid.New()
	user := &domain.User{ID: userID, Email: email, PasswordHash: string(hashedPassword)}

	userRepo.On("GetByEmail", ctx, email).Return(user, nil)
	identitySvc.On("CreateKey", ctx, userID, "Default Key").Return(&domain.APIKey{
		Key:       "sk_test_123",
		UserID:    userID,
		CreatedAt: time.Now(),
	}, nil)
	auditSvc.On("Log", ctx, userID, "user.login", "user", userID.String(), mock.Anything).Return(nil)

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
	auditSvc := new(MockAuditService)

	svc := services.NewAuthService(userRepo, identitySvc, auditSvc)
	ctx := context.Background()

	email := "wrong@example.com"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("real-password"), bcrypt.DefaultCost)
	user := &domain.User{ID: uuid.New(), Email: email, PasswordHash: string(hashedPassword)}

	userRepo.On("GetByEmail", ctx, email).Return(user, nil)

	// Since we are mocking the repo, the service will find the user but fail password check.
	// This counts as a failed login attempt.
	// However, GetByEmail is called.

	resultUser, apiKey, err := svc.Login(ctx, email, "wrong-password")

	assert.Error(t, err)
	assert.Nil(t, resultUser)
	assert.Empty(t, apiKey)
	assert.Contains(t, err.Error(), "invalid")
}

func TestAuthService_ValidateUser(t *testing.T) {
	userRepo := new(MockUserRepo)
	identitySvc := new(MockIdentityService)
	auditSvc := new(MockAuditService)

	svc := services.NewAuthService(userRepo, identitySvc, auditSvc)
	ctx := context.Background()
	userID := uuid.New()
	user := &domain.User{ID: userID, Email: "validate@example.com"}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)

	result, err := svc.ValidateUser(ctx, userID)

	assert.NoError(t, err)
	assert.Equal(t, userID, result.ID)
	userRepo.AssertExpectations(t)
}
