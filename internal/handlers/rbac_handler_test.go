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

type mockRBACService struct {
	mock.Mock
}

func (m *mockRBACService) Authorize(ctx context.Context, userID uuid.UUID, permission domain.Permission) error {
	args := m.Called(ctx, userID, permission)
	return args.Error(0)
}
func (m *mockRBACService) HasPermission(ctx context.Context, userID uuid.UUID, permission domain.Permission) (bool, error) {
	args := m.Called(ctx, userID, permission)
	return args.Bool(0), args.Error(1)
}
func (m *mockRBACService) CreateRole(ctx context.Context, role *domain.Role) error {
	args := m.Called(ctx, role)
	return args.Error(0)
}
func (m *mockRBACService) GetRoleByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}
func (m *mockRBACService) GetRoleByName(ctx context.Context, name string) (*domain.Role, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}
func (m *mockRBACService) ListRoles(ctx context.Context) ([]*domain.Role, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Role), args.Error(1)
}
func (m *mockRBACService) UpdateRole(ctx context.Context, role *domain.Role) error {
	args := m.Called(ctx, role)
	return args.Error(0)
}
func (m *mockRBACService) DeleteRole(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *mockRBACService) AddPermissionToRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	args := m.Called(ctx, roleID, permission)
	return args.Error(0)
}
func (m *mockRBACService) RemovePermissionFromRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	args := m.Called(ctx, roleID, permission)
	return args.Error(0)
}
func (m *mockRBACService) BindRole(ctx context.Context, userIdentifier string, roleName string) error {
	args := m.Called(ctx, userIdentifier, roleName)
	return args.Error(0)
}
func (m *mockRBACService) ListRoleBindings(ctx context.Context) ([]*domain.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.User), args.Error(1)
}

func TestRBACHandler_CreateRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockRBACService)
	handler := NewRBACHandler(svc)

	r := gin.New()
	r.POST("/rbac/roles", handler.CreateRole)

	req := CreateRoleRequest{
		Name:        "test-role",
		Description: "A test role",
		Permissions: []domain.Permission{domain.PermissionInstanceRead},
	}
	body, _ := json.Marshal(req)

	svc.On("CreateRole", mock.Anything, mock.MatchedBy(func(r *domain.Role) bool {
		return r.Name == req.Name && len(r.Permissions) == 1
	})).Return(nil)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/rbac/roles", bytes.NewBuffer(body))
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestRBACHandler_ListRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockRBACService)
	handler := NewRBACHandler(svc)

	r := gin.New()
	r.GET("/rbac/roles", handler.ListRoles)

	roles := []*domain.Role{
		{ID: uuid.New(), Name: "admin"},
		{ID: uuid.New(), Name: "viewer"},
	}

	svc.On("ListRoles", mock.Anything).Return(roles, nil)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("GET", "/rbac/roles", nil)
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "admin")
	assert.Contains(t, w.Body.String(), "viewer")
	svc.AssertExpectations(t)
}

func TestRBACHandler_GetRoleByID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockRBACService)
	handler := NewRBACHandler(svc)

	r := gin.New()
	r.GET("/rbac/roles/:id", handler.GetRole)

	roleID := uuid.New()
	role := &domain.Role{ID: roleID, Name: "admin"}

	svc.On("GetRoleByID", mock.Anything, roleID).Return(role, nil)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("GET", "/rbac/roles/"+roleID.String(), nil)
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "admin")
	svc.AssertExpectations(t)
}

func TestRBACHandler_GetRoleByName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockRBACService)
	handler := NewRBACHandler(svc)

	r := gin.New()
	r.GET("/rbac/roles/:id", handler.GetRole)

	roleName := "developer"
	role := &domain.Role{ID: uuid.New(), Name: roleName}

	svc.On("GetRoleByName", mock.Anything, roleName).Return(role, nil)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("GET", "/rbac/roles/"+roleName, nil)
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), roleName)
	svc.AssertExpectations(t)
}

func TestRBACHandler_DeleteRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockRBACService)
	handler := NewRBACHandler(svc)

	r := gin.New()
	r.DELETE("/rbac/roles/:id", handler.DeleteRole)

	roleID := uuid.New()

	svc.On("DeleteRole", mock.Anything, roleID).Return(nil)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("DELETE", "/rbac/roles/"+roleID.String(), nil)
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestRBACHandler_AddPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockRBACService)
	handler := NewRBACHandler(svc)

	r := gin.New()
	r.POST("/rbac/roles/:id/permissions", handler.AddPermission)

	roleID := uuid.New()

	// Create a minimal request body with just the Permission field
	body, _ := json.Marshal(map[string]interface{}{
		"permission": domain.PermissionInstanceLaunch,
	})

	svc.On("AddPermissionToRole", mock.Anything, roleID, domain.PermissionInstanceLaunch).Return(nil)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/rbac/roles/"+roleID.String()+"/permissions", bytes.NewBuffer(body))
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestRBACHandler_BindRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockRBACService)
	handler := NewRBACHandler(svc)

	r := gin.New()
	r.POST("/rbac/bind", handler.BindRole)

	userEmail := "user@example.com"
	roleName := "admin"

	body, _ := json.Marshal(map[string]string{
		"user_identifier": userEmail,
		"role_name":       roleName,
	})

	svc.On("BindRole", mock.Anything, userEmail, roleName).Return(nil)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/rbac/bind", bytes.NewBuffer(body))
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}
