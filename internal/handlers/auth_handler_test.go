package httphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockAuthService struct {
	mock.Mock
}

func (m *mockAuthService) Register(ctx context.Context, email, password, name string) (*domain.User, error) {
	args := m.Called(ctx, email, password, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *mockAuthService) Login(ctx context.Context, email, password string) (*domain.User, string, error) {
	args := m.Called(ctx, email, password)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).(*domain.User), args.String(1), args.Error(2)
}

func (m *mockAuthService) ValidateUser(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

type mockPasswordResetService struct {
	mock.Mock
}

func (m *mockPasswordResetService) RequestReset(ctx context.Context, email string) error {
	args := m.Called(ctx, email)
	return args.Error(0)
}

func (m *mockPasswordResetService) ResetPassword(ctx context.Context, token, newPassword string) error {
	args := m.Called(ctx, token, newPassword)
	return args.Error(0)
}

func setupAuthHandlerTest(t *testing.T) (*mockAuthService, *mockPasswordResetService, *AuthHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockAuthService)
	pwdSvc := new(mockPasswordResetService)
	handler := NewAuthHandler(svc, pwdSvc)
	r := gin.New()
	return svc, pwdSvc, handler, r
}

func TestAuthHandler_Register(t *testing.T) {
	svc, _, handler, r := setupAuthHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST("/auth/register", handler.Register)

	user := &domain.User{ID: uuid.New(), Email: "test@example.com"}
	svc.On("Register", mock.Anything, "test@example.com", "password123", "Test User").Return(user, nil)

	body, err := json.Marshal(map[string]string{
		"email":    "test@example.com",
		"password": "password123",
		"name":     "Test User",
	})
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(body))
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAuthHandler_Login(t *testing.T) {
	svc, _, handler, r := setupAuthHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST("/auth/login", handler.Login)

	user := &domain.User{ID: uuid.New(), Email: "test@example.com"}
	svc.On("Login", mock.Anything, "test@example.com", "password123").Return(user, "key123", nil)

	body, err := json.Marshal(map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	})
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(body))
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "key123")
}
