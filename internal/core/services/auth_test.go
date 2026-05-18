//go:build integration
// +build integration

package services_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	internalerrors "github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/repositories/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testPassword = "password123ABC!@#123"

func setupAuthServiceTest(t *testing.T) (*pgxpool.Pool, *services.AuthService, *postgres.UserRepo, *services.IdentityService) {
	t.Helper()
	db := setupDB(t)
	cleanDB(t, db)

	userRepo := postgres.NewUserRepo(db)
	auditRepo := postgres.NewAuditRepository(db)
	identityRepo := postgres.NewIdentityRepository(db)
	tenantRepo := postgres.NewTenantRepo(db)

	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	auditSvc := services.NewAuditService(services.AuditServiceParams{
		Repo:    auditRepo,
		RBACSvc: rbacSvc,
	})
	identitySvc := services.NewIdentityService(services.IdentityServiceParams{
		Repo:     identityRepo,
		RbacSvc:  rbacSvc,
		AuditSvc: auditSvc,
	})
	tenantSvc := services.NewTenantService(services.TenantServiceParams{
		Repo:     tenantRepo,
		UserRepo: userRepo,
		RBACSvc:  rbacSvc,
		Logger:   slog.Default(),
	})
	svc := services.NewAuthService(userRepo, identitySvc, auditSvc, tenantSvc, db, slog.Default())

	return db, svc, userRepo, identitySvc
}

func TestAuthService_GetUserByID(t *testing.T) {
	ctx := context.Background()
	mockUserRepo := new(MockUserRepo)
	svc := services.NewAuthService(mockUserRepo, nil, nil, nil, slog.Default())

	tests := []struct {
		name          string
		userID        uuid.UUID
		setupMock     func()
		expectedUser  *domain.User
		expectedError string
	}{
		{
			name:   "Success",
			userID: uuid.New(),
			setupMock: func() {
				uid := uuid.New()
				mockUserRepo.On("GetByID", mock.Anything, mock.Anything).Return(&domain.User{ID: uid}, nil).Once()
			},
			expectedUser: &domain.User{}, // ID will match mock return
		},
		{
			name:   "Not Found",
			userID: uuid.New(),
			setupMock: func() {
				mockUserRepo.On("GetByID", mock.Anything, mock.Anything).Return(nil, internalerrors.New(internalerrors.NotFound, "not found")).Once()
			},
			expectedError: "user not found",
		},
		{
			name:   "Internal Error",
			userID: uuid.New(),
			setupMock: func() {
				mockUserRepo.On("GetByID", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("db error")).Once()
			},
			expectedError: "failed to fetch user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			user, err := svc.GetUserByID(ctx, tt.userID)
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, user)
			}
		})
	}
}

func TestAuthServiceRegister(t *testing.T) {
	_, svc, userRepo, _ := setupAuthServiceTest(t)
	ctx := context.Background()

	email := "new_" + uuid.NewString() + "@example.com"
	pass := testPassword
	name := "New User"

	user, err := svc.Register(ctx, email, pass, name)
	require.NoError(t, err)
	assert.Equal(t, email, user.Email)
	assert.Equal(t, name, user.Name)

	// Verify persistence
	dbUser, err := userRepo.GetByEmail(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, user.ID, dbUser.ID)
}

func TestAuthServiceLogin(t *testing.T) {
	_, svc, _, _ := setupAuthServiceTest(t)
	ctx := context.Background()

	email := "login_" + uuid.NewString() + "@example.com"
	pass := testPassword
	name := "Login User"

	_, err := svc.Register(ctx, email, pass, name)
	require.NoError(t, err)

	user, token, err := svc.Login(ctx, email, pass)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, email, user.Email)
}

func TestAuthServiceLoginInvalidCredentials(t *testing.T) {
	_, svc, _, _ := setupAuthServiceTest(t)
	ctx := context.Background()

	email := "wrong_" + uuid.NewString() + "@example.com"
	_, err := svc.Register(ctx, email, testPassword, "User")
	require.NoError(t, err)

	_, _, err = svc.Login(ctx, email, "wrongpass")
	require.Error(t, err)
}

func TestAuthServiceLoginUserNotFound(t *testing.T) {
	_, svc, _, _ := setupAuthServiceTest(t)

	_, _, err := svc.Login(context.Background(), "notfound_"+uuid.NewString()+"@example.com", "wrong")
	require.Error(t, err)
}

func TestAuthServiceValidateToken(t *testing.T) {
	_, svc, _, identitySvc := setupAuthServiceTest(t)
	ctx := context.Background()

	email := "session_" + uuid.NewString() + "@example.com"
	user, err := svc.Register(ctx, email, testPassword, "User")
	if user != nil && user.DefaultTenantID != nil {
		ctx = appcontext.WithTenantID(ctx, *user.DefaultTenantID)
	}
	require.NoError(t, err)

	apiKey, err := identitySvc.CreateKey(ctx, user.ID, "session")
	require.NoError(t, err)

	validatedKey, err := identitySvc.ValidateAPIKey(ctx, apiKey.Key)
	require.NoError(t, err)
	assert.Equal(t, user.ID, validatedKey.UserID)
}

func TestAuthServiceRevokeToken(t *testing.T) {
	_, svc, _, identitySvc := setupAuthServiceTest(t)
	ctx := context.Background()

	email := "revoke_" + uuid.NewString() + "@example.com"
	user, err := svc.Register(ctx, email, testPassword, "User")
	if user != nil && user.DefaultTenantID != nil {
		ctx = appcontext.WithTenantID(ctx, *user.DefaultTenantID)
	}
	require.NoError(t, err)

	apiKey, err := identitySvc.CreateKey(ctx, user.ID, "session")
	require.NoError(t, err)

	err = identitySvc.RevokeKey(ctx, user.ID, apiKey.ID)
	require.NoError(t, err)

	_, err = identitySvc.ValidateAPIKey(ctx, apiKey.Key)
	require.Error(t, err)
}

func TestAuthServiceRotateToken(t *testing.T) {
	_, svc, _, identitySvc := setupAuthServiceTest(t)
	ctx := context.Background()

	email := "rotate_" + uuid.NewString() + "@example.com"
	user, err := svc.Register(ctx, email, testPassword, "User")
	if user != nil && user.DefaultTenantID != nil {
		ctx = appcontext.WithTenantID(ctx, *user.DefaultTenantID)
	}
	require.NoError(t, err)

	apiKey, err := identitySvc.CreateKey(ctx, user.ID, "session")
	require.NoError(t, err)

	newToken, err := identitySvc.RotateKey(ctx, user.ID, apiKey.ID)
	require.NoError(t, err)
	assert.NotEqual(t, apiKey.Key, newToken.Key)

	// Old token should be invalid
	_, err = identitySvc.ValidateAPIKey(ctx, apiKey.Key)
	require.Error(t, err)

	// New token should be valid
	validatedKey, err := identitySvc.ValidateAPIKey(ctx, newToken.Key)
	require.NoError(t, err)
	assert.Equal(t, user.ID, validatedKey.UserID)
}

func TestAuthServiceLogout(t *testing.T) {
	_, svc, _, identitySvc := setupAuthServiceTest(t)
	ctx := context.Background()

	email := "logout_" + uuid.NewString() + "@example.com"
	pass := testPassword
	user, err := svc.Register(ctx, email, pass, "User")
	require.NoError(t, err)

	_, token, err := svc.Login(ctx, email, pass)
	require.NoError(t, err)

	// In current implementation, login creates a key. We need to find it to revoke it.
	keys, err := identitySvc.ListKeys(ctx, user.ID)
	require.NoError(t, err)
	require.NotEmpty(t, keys)

	err = identitySvc.RevokeKey(ctx, user.ID, keys[0].ID)
	require.NoError(t, err)

	_, err = identitySvc.ValidateAPIKey(ctx, token)
	require.Error(t, err)
}

func TestAuthServiceTokenRotationIntegration(t *testing.T) {
	db, svc, _, identitySvc := setupAuthServiceTest(t)
	defer db.Close()

	ctx := context.Background()
	email := "rotate_int_" + uuid.NewString() + "@example.com"
	user, err := svc.Register(ctx, email, testPassword, "User")
	if user != nil && user.DefaultTenantID != nil {
		ctx = appcontext.WithTenantID(ctx, *user.DefaultTenantID)
	}
	require.NoError(t, err)

	// Initial token
	token1, err := identitySvc.CreateKey(ctx, user.ID, "session")
	require.NoError(t, err)

	// Rotate
	token2, err := identitySvc.RotateKey(ctx, user.ID, token1.ID)
	require.NoError(t, err)

	// Verify
	_, err = identitySvc.ValidateAPIKey(ctx, token1.Key)
	require.Error(t, err)

	vKey, err := identitySvc.ValidateAPIKey(ctx, token2.Key)
	require.NoError(t, err)
	assert.Equal(t, user.ID, vKey.UserID)
}

func TestAuthService_LockoutLogicReal(t *testing.T) {
	_, svc, _, _ := setupAuthServiceTest(t)
	ctx := context.Background()
	email := "locked_" + uuid.NewString() + "@example.com"
	pass := testPassword
	_, err := svc.Register(ctx, email, pass, "Locked User")
	require.NoError(t, err)

	t.Run("Lockout after 5 attempts", func(t *testing.T) {
		// 5 failed attempts
		for i := 0; i < 5; i++ {
			_, _, err := svc.Login(ctx, email, "wrong-password")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid email or password")
		}

		// 6th attempt should be locked out
		_, _, err = svc.Login(ctx, email, pass)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "account is locked")
	})

	t.Run("Success clears failures", func(t *testing.T) {
		email2 := "clean_" + uuid.NewString() + "@example.com"
		_, err := svc.Register(ctx, email2, pass, "Clean User")
		require.NoError(t, err)

		// 2 failed attempts
		for i := 0; i < 2; i++ {
			_, _, err := svc.Login(ctx, email2, "wrong")
			require.Error(t, err)
		}

		// Success
		_, _, err = svc.Login(ctx, email2, pass)
		require.NoError(t, err)
	})

	t.Run("Lockout Expiration", func(t *testing.T) {
		email3 := "expire_" + uuid.NewString() + "@example.com"
		_, err := svc.Register(ctx, email3, pass, "Expire User")
		require.NoError(t, err)

		// Set a very short lockout
		svc.SetLockoutDuration(100 * time.Millisecond)

		// Trigger lockout
		for i := 0; i < 5; i++ {
			_, _, _ = svc.Login(ctx, email3, "wrong")
		}

		// Verify locked
		_, _, err = svc.Login(ctx, email3, pass)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "locked")

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)

		// Login should now work
		_, _, err = svc.Login(ctx, email3, pass)
		require.NoError(t, err)
	})
}
