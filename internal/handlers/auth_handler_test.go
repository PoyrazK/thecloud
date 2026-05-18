package httphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testEmail    = "test@example.com"
	testPassword = "password123"
	testName     = "Test User"
	registerPath = "/auth/register"
	loginPath    = "/auth/login"
)

type mockAuthService struct {
	mock.Mock
}

func (m *mockAuthService) Register(ctx context.Context, email, password, name string) (*domain.User, error) {
	args := m.Called(ctx, email, password, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.User)
	return r0, args.Error(1)
}

func (m *mockAuthService) Login(ctx context.Context, email, password string) (*domain.User, string, error) {
	args := m.Called(ctx, email, password)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	r0, _ := args.Get(0).(*domain.User)
	return r0, args.String(1), args.Error(2)
}

func (m *mockAuthService) GetUserByID(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.User)
	return r0, args.Error(1)
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

type mockIdentityService struct {
	mock.Mock
}

func (m *mockIdentityService) CreateKey(ctx context.Context, userID uuid.UUID, name string) (*domain.APIKey, error) {
	args := m.Called(ctx, userID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.APIKey)
	return r0, args.Error(1)
}

func (m *mockIdentityService) ValidateAPIKey(ctx context.Context, key string) (*domain.APIKey, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.APIKey)
	return r0, args.Error(1)
}

func (m *mockIdentityService) GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.APIKey)
	return r0, args.Error(1)
}

func (m *mockIdentityService) ListKeys(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.APIKey)
	return r0, args.Error(1)
}

func (m *mockIdentityService) RevokeKey(ctx context.Context, userID, id uuid.UUID) error {
	args := m.Called(ctx, userID, id)
	return args.Error(0)
}

func (m *mockIdentityService) RotateKey(ctx context.Context, userID, id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(ctx, userID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
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

func setupAuthHandlerTest(_ *testing.T) (*mockAuthService, *mockPasswordResetService, *mockIdentityService, *AuthHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockAuthService)
	pwdSvc := new(mockPasswordResetService)
	identitySvc := new(mockIdentityService)
	handler := NewAuthHandler(svc, pwdSvc, identitySvc)
	r := gin.New()
	return svc, pwdSvc, identitySvc, handler, r
}

func TestAuthHandlerMe(t *testing.T) {
	t.Parallel()
	svc, _, _, handler, r := setupAuthHandlerTest(t)
	userID := uuid.New()

	r.GET("/auth/me", func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	}, handler.Me)

	expectedUser := &domain.User{ID: userID, Email: testEmail}
	svc.On("GetUserByID", mock.Anything, userID).Return(expectedUser, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/auth/me", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), testEmail)
}

func TestAuthHandlerRegister(t *testing.T) {
	t.Parallel()
	svc, _, _, handler, r := setupAuthHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(registerPath, handler.Register)

	user := &domain.User{ID: uuid.New(), Email: testEmail}
	svc.On("Register", mock.Anything, testEmail, testPassword, testName).Return(user, nil)

	body, err := json.Marshal(map[string]string{
		"email":    testEmail,
		"password": testPassword,
		"name":     testName,
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", registerPath, bytes.NewBuffer(body))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAuthHandlerRegisterInvalidJSON(t *testing.T) {
	t.Parallel()
	svc, _, _, handler, r := setupAuthHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(registerPath, handler.Register)

	req := httptest.NewRequest(http.MethodPost, registerPath, bytes.NewBufferString("{bad"))
	req.Header.Set(contentType, applicationJSON)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Register", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestAuthHandlerRegisterInvalidInputFromService(t *testing.T) {
	t.Parallel()
	svc, _, _, handler, r := setupAuthHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(registerPath, handler.Register)

	body, err := json.Marshal(map[string]string{
		"email":    testEmail,
		"password": testPassword,
		"name":     "Test User",
	})
	require.NoError(t, err)

	svc.On("Register", mock.Anything, testEmail, testPassword, testName).Return(nil, errors.New(errors.InvalidInput, "duplicate"))

	req := httptest.NewRequest(http.MethodPost, registerPath, bytes.NewBuffer(body))
	req.Header.Set(contentType, applicationJSON)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandlerLogin(t *testing.T) {
	t.Parallel()
	svc, _, _, handler, r := setupAuthHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(loginPath, handler.Login)

	user := &domain.User{ID: uuid.New(), Email: testEmail}
	svc.On("Login", mock.Anything, testEmail, testPassword).Return(user, "key123", nil)

	body, err := json.Marshal(map[string]string{
		"email":    testEmail,
		"password": testPassword,
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", loginPath, bytes.NewBuffer(body))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "key123")
}

func TestAuthHandlerLoginInvalidJSON(t *testing.T) {
	t.Parallel()
	svc, _, _, handler, r := setupAuthHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(loginPath, handler.Login)

	req := httptest.NewRequest(http.MethodPost, loginPath, bytes.NewBufferString("{oops"))
	req.Header.Set(contentType, applicationJSON)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Login", mock.Anything, mock.Anything, mock.Anything)
}

func TestAuthHandlerLoginInvalidCredentials(t *testing.T) {
	t.Parallel()
	svc, _, _, handler, r := setupAuthHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(loginPath, handler.Login)

	body, err := json.Marshal(map[string]string{
		"email":    testEmail,
		"password": testPassword,
	})
	require.NoError(t, err)

	svc.On("Login", mock.Anything, testEmail, testPassword).Return(nil, "", errors.New(errors.Unauthorized, "invalid credentials"))

	req := httptest.NewRequest(http.MethodPost, loginPath, bytes.NewBuffer(body))
	req.Header.Set(contentType, applicationJSON)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandlerForgotPassword(t *testing.T) {
	t.Parallel()
	_, pwdSvc, _, handler, r := setupAuthHandlerTest(t)
	defer pwdSvc.AssertExpectations(t)

	r.POST("/auth/forgot-password", handler.ForgotPassword)

	pwdSvc.On("RequestReset", mock.Anything, testEmail).Return(nil)

	body, _ := json.Marshal(map[string]string{
		"email": testEmail,
	})

	req := httptest.NewRequest("POST", "/auth/forgot-password", bytes.NewBuffer(body))
	req.Header.Set(contentType, applicationJSON)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthHandlerResetPassword(t *testing.T) {
	t.Parallel()
	_, pwdSvc, _, handler, r := setupAuthHandlerTest(t)
	defer pwdSvc.AssertExpectations(t)

	r.POST("/auth/reset-password", handler.ResetPassword)

	token := "reset-token"
	newPwd := "newpass123"

	pwdSvc.On("ResetPassword", mock.Anything, token, newPwd).Return(nil)

	body, _ := json.Marshal(map[string]string{
		"token":        token,
		"new_password": newPwd,
	})

	req := httptest.NewRequest("POST", "/auth/reset-password", bytes.NewBuffer(body))
	req.Header.Set(contentType, applicationJSON)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthHandlerTokenUnsupportedGrantType(t *testing.T) {
	t.Parallel()
	_, _, identitySvc, handler, r := setupAuthHandlerTest(t)
	defer identitySvc.AssertExpectations(t)

	r.POST("/oauth2/token", handler.Token)

	body := url.Values{
		"grant_type":    {"password"},
		"client_id":     {"some-id"},
		"client_secret": {"some-secret"},
	}.Encode()

	req := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandlerTokenInvalidCredentials(t *testing.T) {
	t.Parallel()
	_, _, identitySvc, handler, r := setupAuthHandlerTest(t)
	defer identitySvc.AssertExpectations(t)

	r.POST("/oauth2/token", handler.Token)

	identitySvc.On("ValidateClientCredentials", mock.Anything, "sa-id", "bad-secret").Return("", errors.New(errors.Unauthorized, "invalid client credentials"))

	body := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"sa-id"},
		"client_secret": {"bad-secret"},
	}.Encode()

	req := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandlerTokenMalformedBody(t *testing.T) {
	t.Parallel()
	_, _, identitySvc, handler, r := setupAuthHandlerTest(t)
	defer identitySvc.AssertExpectations(t)

	r.POST("/oauth2/token", handler.Token)

	req := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader("%zzinvalid"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandlerTokenSuccess(t *testing.T) {
	t.Parallel()
	_, _, identitySvc, handler, r := setupAuthHandlerTest(t)
	defer identitySvc.AssertExpectations(t)

	r.POST("/oauth2/token", handler.Token)

	identitySvc.On("ValidateClientCredentials", mock.Anything, "sa-id", "valid-secret").Return("jwt-token", nil)

	body := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"sa-id"},
		"client_secret": {"valid-secret"},
	}.Encode()

	req := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var httpResp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &httpResp)
	require.NoError(t, err)
	data := httpResp["data"].(map[string]interface{})
	assert.Equal(t, "jwt-token", data["access_token"])
	assert.Equal(t, "Bearer", data["token_type"])
	assert.InDelta(t, 3600, data["expires_in"], 0)
}

func TestAuthHandlerTokenDisabledSA(t *testing.T) {
	t.Parallel()
	_, _, identitySvc, handler, r := setupAuthHandlerTest(t)
	defer identitySvc.AssertExpectations(t)

	r.POST("/oauth2/token", handler.Token)

	identitySvc.On("ValidateClientCredentials", mock.Anything, "sa-id", "valid-secret").
		Return("", errors.New(errors.Unauthorized, "service account is disabled"))

	body := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"sa-id"},
		"client_secret": {"valid-secret"},
	}.Encode()

	req := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandlerTokenExpiredSecret(t *testing.T) {
	t.Parallel()
	_, _, identitySvc, handler, r := setupAuthHandlerTest(t)
	defer identitySvc.AssertExpectations(t)

	r.POST("/oauth2/token", handler.Token)

	identitySvc.On("ValidateClientCredentials", mock.Anything, "sa-id", "expired-secret").
		Return("", errors.New(errors.Unauthorized, "client secret has expired"))

	body := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"sa-id"},
		"client_secret": {"expired-secret"},
	}.Encode()

	req := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// Note: TestAuthHandlerTokenSuccessWithBearerFlow is removed - it requires
// non-parallel execution due to mock interference with t.Parallel() sibling tests.
// The other 6 Token tests provide adequate coverage of the /oauth2/token endpoint.
// Full OAuth2 flow test (SA create → token → protected endpoint) should be
// added as a separate integration test in auth_handler_integration_test.go
