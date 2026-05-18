package httputil

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	apiKeyHeader  = "X-API-Key"
	protectedPath = "/protected"
	tenantPath    = "/tenant"
	validAPIKey   = "valid-key"
	invalidAPIKey = "invalid-key"
)

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
	r0, _ := args.Get(0).([]*domain.APIKey)
	return r0, args.Error(1)
}
func (m *mockIdentityService) RevokeKey(ctx context.Context, userID, id uuid.UUID) error {
	args := m.Called(ctx, userID, id)
	return args.Error(0)
}
func (m *mockIdentityService) RotateKey(ctx context.Context, userID, id uuid.UUID) (*domain.APIKey, error) {
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

type mockTenantService struct {
	mock.Mock
}

type mockRBACService struct {
	mock.Mock
}

func (m *mockTenantService) CreateTenant(ctx context.Context, name, slug string, ownerID uuid.UUID) (*domain.Tenant, error) {
	return nil, nil
}
func (m *mockTenantService) GetTenant(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return nil, nil
}
func (m *mockTenantService) InviteMember(ctx context.Context, tenantID uuid.UUID, email, role string) error {
	return nil
}
func (m *mockTenantService) RemoveMember(ctx context.Context, tenantID, userID uuid.UUID) error {
	return nil
}
func (m *mockTenantService) SwitchTenant(ctx context.Context, userID, tenantID uuid.UUID) error {
	return nil
}
func (m *mockTenantService) CheckQuota(ctx context.Context, tenantID uuid.UUID, resource string, requested int) error {
	return nil
}
func (m *mockTenantService) GetMembership(ctx context.Context, tenantID, userID uuid.UUID) (*domain.TenantMember, error) {
	args := m.Called(ctx, tenantID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.TenantMember)
	return r0, args.Error(1)
}
func (m *mockTenantService) ListUserTenants(ctx context.Context, userID uuid.UUID) ([]domain.Tenant, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]domain.Tenant)
	return r0, args.Error(1)
}
func (m *mockTenantService) IncrementUsage(ctx context.Context, tenantID uuid.UUID, resource string, amount int) error {
	return nil
}
func (m *mockTenantService) DecrementUsage(ctx context.Context, tenantID uuid.UUID, resource string, amount int) error {
	return nil
}

func (m *mockRBACService) Authorize(ctx context.Context, userID, tenantID uuid.UUID, permission domain.Permission, resource string) error {
	args := m.Called(ctx, userID, tenantID, permission, resource)
	return args.Error(0)
}

func (m *mockRBACService) HasPermission(ctx context.Context, userID, tenantID uuid.UUID, permission domain.Permission, resource string) (bool, error) {
	args := m.Called(ctx, userID, tenantID, permission, resource)
	return args.Bool(0), args.Error(1)
}

func (m *mockRBACService) CreateRole(ctx context.Context, role *domain.Role) error { return nil }
func (m *mockRBACService) GetRoleByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	return nil, nil
}
func (m *mockRBACService) GetRoleByName(ctx context.Context, name string) (*domain.Role, error) {
	return nil, nil
}
func (m *mockRBACService) ListRoles(ctx context.Context) ([]*domain.Role, error)   { return nil, nil }
func (m *mockRBACService) UpdateRole(ctx context.Context, role *domain.Role) error { return nil }
func (m *mockRBACService) DeleteRole(ctx context.Context, id uuid.UUID) error      { return nil }
func (m *mockRBACService) AddPermissionToRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	return nil
}
func (m *mockRBACService) RemovePermissionFromRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	return nil
}
func (m *mockRBACService) BindRole(ctx context.Context, userIdentifier string, roleName string) error {
	return nil
}
func (m *mockRBACService) ListRoleBindings(ctx context.Context) ([]*domain.User, error) {
	return nil, nil
}

func (m *mockRBACService) EvaluatePolicy(ctx context.Context, userID uuid.UUID, action string, resource string, context map[string]interface{}) (bool, error) {
	args := m.Called(ctx, userID, action, resource, context)
	return args.Bool(0), args.Error(1)
}

func (m *mockRBACService) AuthorizeServiceAccount(ctx context.Context, saID, tenantID uuid.UUID, permission domain.Permission, resource string) error {
	args := m.Called(ctx, saID, tenantID, permission, resource)
	return args.Error(0)
}

func TestAuthSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockIdentityService)
	tenantSvc := new(mockTenantService)
	userID := uuid.New()
	svc.On("ValidateAPIKey", mock.Anything, validAPIKey).Return(&domain.APIKey{UserID: userID}, nil)

	r := gin.New()
	r.Use(Auth(svc, tenantSvc))
	r.GET(protectedPath, func(c *gin.Context) {
		val, _ := c.Get("userID")
		assert.Equal(t, userID, val)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", protectedPath, nil)
	req.Header.Set(apiKeyHeader, validAPIKey)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMissingKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockIdentityService)
	tenantSvc := new(mockTenantService)

	r := gin.New()
	r.Use(Auth(svc, tenantSvc))
	r.GET(protectedPath, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", protectedPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthInvalidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockIdentityService)
	tenantSvc := new(mockTenantService)
	svc.On("ValidateAPIKey", mock.Anything, invalidAPIKey).Return(nil, fmt.Errorf("invalid"))

	r := gin.New()
	r.Use(Auth(svc, tenantSvc))
	r.GET(protectedPath, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", protectedPath, nil)
	req.Header.Set(apiKeyHeader, invalidAPIKey)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPermissionUnauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rbacSvc := new(mockRBACService)

	r := gin.New()
	r.Use(Permission(rbacSvc, domain.PermissionInstanceRead))
	r.GET(protectedPath, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, protectedPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPermissionForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rbacSvc := new(mockRBACService)
	userID := uuid.New()
	tenantID := uuid.New()
	rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceRead, "*").Return(fmt.Errorf("nope"))

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("tenantID", tenantID)
		c.Next()
	})
	r.Use(Permission(rbacSvc, domain.PermissionInstanceRead))
	r.GET(protectedPath, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, protectedPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestPermissionAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rbacSvc := new(mockRBACService)
	userID := uuid.New()
	tenantID := uuid.New()
	rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceRead, "*").Return(nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("tenantID", tenantID)
		c.Next()
	})
	r.Use(Permission(rbacSvc, domain.PermissionInstanceRead))
	r.GET(protectedPath, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, protectedPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetTenantID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	tenantID := uuid.New()

	c.Set("tenantID", tenantID)
	assert.Equal(t, tenantID, GetTenantID(c))

	c.Set("tenantID", "invalid")
	assert.Equal(t, uuid.Nil, GetTenantID(c))
}

func TestGetUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	userID := uuid.New()

	c.Set("userID", userID)
	assert.Equal(t, userID, GetUserID(c))

	c.Set("userID", 123)
	assert.Equal(t, uuid.Nil, GetUserID(c))
}

func TestAuthInvalidTenantHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockIdentityService)
	tenantSvc := new(mockTenantService)
	userID := uuid.New()
	svc.On("ValidateAPIKey", mock.Anything, validAPIKey).Return(&domain.APIKey{UserID: userID}, nil)

	r := gin.New()
	r.Use(Auth(svc, tenantSvc))
	r.GET(protectedPath, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", protectedPath, nil)
	req.Header.Set(apiKeyHeader, validAPIKey)
	req.Header.Set("X-Tenant-ID", "not-a-uuid")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthTenantNotMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockIdentityService)
	tenantSvc := new(mockTenantService)
	userID := uuid.New()
	tenantID := uuid.New()

	svc.On("ValidateAPIKey", mock.Anything, validAPIKey).Return(&domain.APIKey{UserID: userID}, nil)
	tenantSvc.On("GetMembership", mock.Anything, tenantID, userID).Return(nil, nil)

	r := gin.New()
	r.Use(Auth(svc, tenantSvc))
	r.GET(protectedPath, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", protectedPath, nil)
	req.Header.Set(apiKeyHeader, validAPIKey)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAuthDefaultTenantMembership(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockIdentityService)
	tenantSvc := new(mockTenantService)
	userID := uuid.New()
	tenantID := uuid.New()

	svc.On("ValidateAPIKey", mock.Anything, validAPIKey).Return(&domain.APIKey{UserID: userID, DefaultTenantID: &tenantID}, nil)
	tenantSvc.On("GetMembership", mock.Anything, tenantID, userID).Return(&domain.TenantMember{TenantID: tenantID, UserID: userID}, nil)

	r := gin.New()
	r.Use(Auth(svc, tenantSvc))
	r.GET(protectedPath, func(c *gin.Context) {
		val, _ := c.Get("tenantID")
		assert.Equal(t, tenantID, val)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", protectedPath, nil)
	req.Header.Set(apiKeyHeader, validAPIKey)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestResponseSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("requestID", "abc")

	data := map[string]string{"foo": "bar"}
	Success(c, http.StatusOK, data)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "foo")
	assert.Contains(t, w.Body.String(), "abc")
}

func TestResponseError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		err        error
		expectCode int
	}{
		{"NotFound", errors.New(errors.NotFound, "not found"), http.StatusNotFound},
		{"InvalidInput", errors.New(errors.InvalidInput, "invalid"), http.StatusBadRequest},
		{"Unauthorized", errors.New(errors.Unauthorized, "unauthorized"), http.StatusUnauthorized},
		{"UnknownError", fmt.Errorf("weird error"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			Error(c, tt.err)
			assert.Equal(t, tt.expectCode, w.Code)
		})
	}
}

func TestRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/id", func(c *gin.Context) {
		id, _ := c.Get("requestID")
		assert.NotEmpty(t, id)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/id", nil)
	r.ServeHTTP(w, req)
	assert.NotEmpty(t, w.Header().Get(HeaderXRequestID))
}

func TestLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	r := gin.New()
	r.Use(Logger(logger))
	r.GET("/log", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/log", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS())
	r.GET("/cors", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	t.Run("GET request", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/cors", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("OPTIONS request", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("OPTIONS", "/cors", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeadersMiddleware())
	r.GET("/secure", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/secure", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
}

func TestMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Metrics())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequireTenant())
	r.GET(tenantPath, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", tenantPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRequireTenantWithTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("tenantID", uuid.New())
		c.Next()
	})
	r.Use(RequireTenant())
	r.GET(tenantPath, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", tenantPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTenantMemberMissingContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tenantSvc := new(mockTenantService)

	r := gin.New()
	r.Use(TenantMember(tenantSvc))
	r.GET(tenantPath, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", tenantPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestTenantMemberNotMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tenantSvc := new(mockTenantService)
	userID := uuid.New()
	tenantID := uuid.New()

	tenantSvc.On("GetMembership", mock.Anything, tenantID, userID).Return(nil, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("tenantID", tenantID)
		c.Next()
	})
	r.Use(TenantMember(tenantSvc))
	r.GET(tenantPath, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", tenantPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTenantMemberSetsRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tenantSvc := new(mockTenantService)
	userID := uuid.New()
	tenantID := uuid.New()

	tenantSvc.On("GetMembership", mock.Anything, tenantID, userID).Return(&domain.TenantMember{TenantID: tenantID, UserID: userID, Role: "admin"}, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("tenantID", tenantID)
		c.Next()
	})
	r.Use(TenantMember(tenantSvc))
	r.GET(tenantPath, func(c *gin.Context) {
		role, _ := c.Get("tenantRole")
		assert.Equal(t, "admin", role)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", tenantPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
