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
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	rolesPath       = "/rbac/roles"
	bindPath        = "/rbac/bindings"
	testRoleName    = "test-role"
	adminRole       = "admin"
	permSuffix      = "/permissions"
	permFullSuffix  = "/permissions/:permission"
	rbacPathInvalid = "/invalid"
)

type mockRBACService struct {
	mock.Mock
}

func (m *mockRBACService) Authorize(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, permission domain.Permission, resource string) error {
	args := m.Called(ctx, userID, tenantID, permission, resource)
	return args.Error(0)
}
func (m *mockRBACService) HasPermission(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, permission domain.Permission, resource string) (bool, error) {
	args := m.Called(ctx, userID, tenantID, permission, resource)
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
	r0, _ := args.Get(0).(*domain.Role)
	return r0, args.Error(1)
}
func (m *mockRBACService) GetRoleByName(ctx context.Context, name string) (*domain.Role, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Role)
	return r0, args.Error(1)
}
func (m *mockRBACService) ListRoles(ctx context.Context) ([]*domain.Role, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Role)
	return r0, args.Error(1)
}
func (m *mockRBACService) UpdateRole(ctx context.Context, role *domain.Role) error {
	return m.Called(ctx, role).Error(0)
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
	return m.Called(ctx, roleID, permission).Error(0)
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
	r0, _ := args.Get(0).([]*domain.User)
	return r0, args.Error(1)
}

func (m *mockRBACService) EvaluatePolicy(ctx context.Context, userID uuid.UUID, action string, resource string, context map[string]interface{}) (bool, error) {
	args := m.Called(ctx, userID, action, resource, context)
	return args.Bool(0), args.Error(1)
}

func (m *mockRBACService) AuthorizeServiceAccount(ctx context.Context, saID uuid.UUID, tenantID uuid.UUID, permission domain.Permission, resource string) error {
	args := m.Called(ctx, saID, tenantID, permission, resource)
	return args.Error(0)
}

func setupRBACHandlerTest(_ *testing.T) (*mockRBACService, *RBACHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockRBACService)
	handler := NewRBACHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestRBACHandlerCreateRole(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRBACHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(rolesPath, handler.CreateRole)

	req := CreateRoleRequest{
		Name:        testRoleName,
		Description: "A test role",
		Permissions: []domain.Permission{domain.PermissionInstanceRead},
	}
	body, err := json.Marshal(req)
	require.NoError(t, err)

	svc.On("CreateRole", mock.Anything, mock.MatchedBy(func(r *domain.Role) bool {
		return r.Name == req.Name && len(r.Permissions) == 1
	})).Return(nil)

	w := httptest.NewRecorder()
	httpReq, err := http.NewRequest("POST", rolesPath, bytes.NewBuffer(body))
	require.NoError(t, err)
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestRBACHandlerListRoles(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRBACHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(rolesPath, handler.ListRoles)

	roles := []*domain.Role{
		{ID: uuid.New(), Name: adminRole},
		{ID: uuid.New(), Name: "viewer"},
	}

	svc.On("ListRoles", mock.Anything).Return(roles, nil)

	w := httptest.NewRecorder()
	httpReq, err := http.NewRequest("GET", rolesPath, nil)
	require.NoError(t, err)
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), adminRole)
	assert.Contains(t, w.Body.String(), "viewer")
}

func TestRBACHandlerGetRoleByID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRBACHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(rolesPath+"/:id", handler.GetRole)

	roleID := uuid.New()
	role := &domain.Role{ID: roleID, Name: adminRole}

	svc.On("GetRoleByID", mock.Anything, roleID).Return(role, nil)

	w := httptest.NewRecorder()
	httpReq, err := http.NewRequest("GET", rolesPath+"/"+roleID.String(), nil)
	require.NoError(t, err)
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), adminRole)
}

func TestRBACHandlerGetRoleByName(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRBACHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(rolesPath+"/:id", handler.GetRole)

	roleName := "developer"
	role := &domain.Role{ID: uuid.New(), Name: roleName}

	svc.On("GetRoleByName", mock.Anything, roleName).Return(role, nil)

	w := httptest.NewRecorder()
	httpReq, err := http.NewRequest("GET", rolesPath+"/"+roleName, nil)
	require.NoError(t, err)
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), roleName)
}

func TestRBACHandlerDeleteRole(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRBACHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(rolesPath+"/:id", handler.DeleteRole)

	roleID := uuid.New()

	svc.On("DeleteRole", mock.Anything, roleID).Return(nil)

	w := httptest.NewRecorder()
	httpReq, err := http.NewRequest("DELETE", rolesPath+"/"+roleID.String(), nil)
	require.NoError(t, err)
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestRBACHandlerAddPermission(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRBACHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(rolesPath+"/:id"+permSuffix, handler.AddPermission)

	roleID := uuid.New()

	// Create a minimal request body with just the Permission field
	body, err := json.Marshal(map[string]interface{}{
		"permission": domain.PermissionInstanceLaunch,
	})
	require.NoError(t, err)

	svc.On("AddPermissionToRole", mock.Anything, roleID, domain.PermissionInstanceLaunch).Return(nil)

	w := httptest.NewRecorder()
	httpReq, err := http.NewRequest("POST", rolesPath+"/"+roleID.String()+permSuffix, bytes.NewBuffer(body))
	require.NoError(t, err)
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestRBACHandlerBindRole(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRBACHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(bindPath, handler.BindRole)

	userEmail := "user@example.com"
	roleName := adminRole

	body, err := json.Marshal(map[string]string{
		"user_identifier": userEmail,
		"role_name":       roleName,
	})
	require.NoError(t, err)

	svc.On("BindRole", mock.Anything, userEmail, roleName).Return(nil)

	w := httptest.NewRecorder()
	httpReq, err := http.NewRequest("POST", bindPath, bytes.NewBuffer(body))
	require.NoError(t, err)
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestRBACHandlerUpdateRole(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRBACHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.PUT(rolesPath+"/:id", handler.UpdateRole)

	roleID := uuid.New()
	req := CreateRoleRequest{
		Name:        "Updated Role",
		Description: "Updated desc",
		Permissions: []domain.Permission{domain.PermissionInstanceRead},
	}
	body, _ := json.Marshal(req)

	svc.On("UpdateRole", mock.Anything, mock.MatchedBy(func(r *domain.Role) bool {
		return r.ID == roleID && r.Name == req.Name
	})).Return(nil)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("PUT", rolesPath+"/"+roleID.String(), bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRBACHandlerRemovePermission(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRBACHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(rolesPath+"/:id"+permFullSuffix, handler.RemovePermission)

	roleID := uuid.New()
	perm := domain.PermissionInstanceRead

	svc.On("RemovePermissionFromRole", mock.Anything, roleID, perm).Return(nil)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("DELETE", rolesPath+"/"+roleID.String()+"/permissions/"+string(perm), nil)
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestRBACHandlerListRoleBindings(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRBACHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(bindPath, handler.ListRoleBindings)

	bindings := []*domain.User{}
	svc.On("ListRoleBindings", mock.Anything).Return(bindings, nil)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("GET", bindPath, nil)
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)
}
func TestRBACHandlerErrorPaths(t *testing.T) {
	t.Parallel()
	t.Run("CreateRoleInvalidJSON", func(t *testing.T) {
		_, handler, r := setupRBACHandlerTest(t)
		r.POST(rolesPath, handler.CreateRole)
		req, _ := http.NewRequest("POST", rolesPath, bytes.NewBufferString("invalid"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("CreateRoleServiceError", func(t *testing.T) {
		svc, handler, r := setupRBACHandlerTest(t)
		r.POST(rolesPath, handler.CreateRole)
		svc.On("CreateRole", mock.Anything, mock.Anything).Return(errors.New(errors.Internal, "error"))
		body, _ := json.Marshal(map[string]interface{}{"name": "n"})
		req, _ := http.NewRequest("POST", rolesPath, bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("ListRolesServiceError", func(t *testing.T) {
		svc, handler, r := setupRBACHandlerTest(t)
		r.GET(rolesPath, handler.ListRoles)
		svc.On("ListRoles", mock.Anything).Return(nil, errors.New(errors.Internal, "error"))
		req, _ := http.NewRequest("GET", rolesPath, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("GetRoleByIDError", func(t *testing.T) {
		svc, handler, r := setupRBACHandlerTest(t)
		r.GET(rolesPath+"/:id", handler.GetRole)
		id := uuid.New()
		svc.On("GetRoleByID", mock.Anything, id).Return(nil, errors.New(errors.Internal, "error"))
		req, _ := http.NewRequest("GET", rolesPath+"/"+id.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("GetRoleByNameError", func(t *testing.T) {
		svc, handler, r := setupRBACHandlerTest(t)
		r.GET(rolesPath+"/:id", handler.GetRole)
		svc.On("GetRoleByName", mock.Anything, "n").Return(nil, errors.New(errors.Internal, "error"))
		req, _ := http.NewRequest("GET", rolesPath+"/n", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("UpdateRoleInvalidID", func(t *testing.T) {
		_, handler, r := setupRBACHandlerTest(t)
		r.PUT(rolesPath+"/:id", handler.UpdateRole)
		req, _ := http.NewRequest("PUT", rolesPath+rbacPathInvalid, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("UpdateRoleInvalidJSON", func(t *testing.T) {
		_, handler, r := setupRBACHandlerTest(t)
		r.PUT(rolesPath+"/:id", handler.UpdateRole)
		id := uuid.New()
		req, _ := http.NewRequest("PUT", rolesPath+"/"+id.String(), bytes.NewBufferString("invalid"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("UpdateRoleServiceError", func(t *testing.T) {
		svc, handler, r := setupRBACHandlerTest(t)
		r.PUT(rolesPath+"/:id", handler.UpdateRole)
		id := uuid.New()
		svc.On("UpdateRole", mock.Anything, mock.Anything).Return(errors.New(errors.Internal, "error"))
		body, _ := json.Marshal(map[string]interface{}{"name": "n"})
		req, _ := http.NewRequest("PUT", rolesPath+"/"+id.String(), bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("DeleteRoleInvalidID", func(t *testing.T) {
		_, handler, r := setupRBACHandlerTest(t)
		r.DELETE(rolesPath+"/:id", handler.DeleteRole)
		req, _ := http.NewRequest("DELETE", rolesPath+rbacPathInvalid, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("DeleteRoleServiceError", func(t *testing.T) {
		svc, handler, r := setupRBACHandlerTest(t)
		r.DELETE(rolesPath+"/:id", handler.DeleteRole)
		id := uuid.New()
		svc.On("DeleteRole", mock.Anything, id).Return(errors.New(errors.Internal, "error"))
		req, _ := http.NewRequest("DELETE", rolesPath+"/"+id.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("AddPermissionInvalidID", func(t *testing.T) {
		_, handler, r := setupRBACHandlerTest(t)
		r.POST(rolesPath+"/:id"+permSuffix, handler.AddPermission)
		req, _ := http.NewRequest("POST", rolesPath+rbacPathInvalid+permSuffix, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("AddPermissionInvalidJSON", func(t *testing.T) {
		_, handler, r := setupRBACHandlerTest(t)
		r.POST(rolesPath+"/:id"+permSuffix, handler.AddPermission)
		id := uuid.New()
		req, _ := http.NewRequest("POST", rolesPath+"/"+id.String()+permSuffix, bytes.NewBufferString("invalid"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("AddPermissionServiceError", func(t *testing.T) {
		svc, handler, r := setupRBACHandlerTest(t)
		r.POST(rolesPath+"/:id"+permSuffix, handler.AddPermission)
		id := uuid.New()
		svc.On("AddPermissionToRole", mock.Anything, id, mock.Anything).Return(errors.New(errors.Internal, "error"))
		body, _ := json.Marshal(map[string]interface{}{"permission": "p"})
		req, _ := http.NewRequest("POST", rolesPath+"/"+id.String()+permSuffix, bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("RemovePermissionInvalidID", func(t *testing.T) {
		_, handler, r := setupRBACHandlerTest(t)
		r.DELETE(rolesPath+"/:id"+permFullSuffix, handler.RemovePermission)
		req, _ := http.NewRequest("DELETE", rolesPath+rbacPathInvalid+permSuffix+"/p", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("RemovePermissionServiceError", func(t *testing.T) {
		svc, handler, r := setupRBACHandlerTest(t)
		r.DELETE(rolesPath+"/:id"+permFullSuffix, handler.RemovePermission)
		id := uuid.New()
		svc.On("RemovePermissionFromRole", mock.Anything, id, mock.Anything).Return(errors.New(errors.Internal, "error"))
		req, _ := http.NewRequest("DELETE", rolesPath+"/"+id.String()+permSuffix+"/p", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("BindRoleInvalidJSON", func(t *testing.T) {
		_, handler, r := setupRBACHandlerTest(t)
		r.POST(bindPath, handler.BindRole)
		req, _ := http.NewRequest("POST", bindPath, bytes.NewBufferString("invalid"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("BindRoleServiceError", func(t *testing.T) {
		svc, handler, r := setupRBACHandlerTest(t)
		r.POST(bindPath, handler.BindRole)
		svc.On("BindRole", mock.Anything, mock.Anything, mock.Anything).Return(errors.New(errors.Internal, "error"))
		body, _ := json.Marshal(map[string]interface{}{"user_identifier": "u", "role_name": "r"})
		req, _ := http.NewRequest("POST", bindPath, bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("ListBindingsServiceError", func(t *testing.T) {
		svc, handler, r := setupRBACHandlerTest(t)
		r.GET(bindPath, handler.ListRoleBindings)
		svc.On("ListRoleBindings", mock.Anything).Return(nil, errors.New(errors.Internal, "error"))
		req, _ := http.NewRequest("GET", bindPath, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
