package services_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCachedRBACService_Unit(t *testing.T) {
	t.Run("ListRoles", testCachedRBACServiceListRoles)
	t.Run("AuthorizeDelegates", testCachedRBACServiceAuthorizeDelegates)
	t.Run("HasPermissionCacheHit", testCachedRBACServiceHasPermissionCacheHit)
	t.Run("HasPermissionCacheMiss", testCachedRBACServiceHasPermissionCacheMiss)
	t.Run("HasPermissionError", testCachedRBACServiceHasPermissionError)
	t.Run("CreateRoleDelegates", testCachedRBACServiceCreateRoleDelegates)
	t.Run("GetRoleByIDCaches", testCachedRBACServiceGetRoleByIDCaches)
	t.Run("GetRoleByNameCaches", testCachedRBACServiceGetRoleByNameCaches)
	t.Run("UpdateRoleInvalidatesCache", testCachedRBACServiceUpdateRoleInvalidatesCache)
	t.Run("DeleteRoleInvalidatesCache", testCachedRBACServiceDeleteRoleInvalidatesCache)
	t.Run("AddPermissionInvalidatesCache", testCachedRBACServiceAddPermissionInvalidatesCache)
	t.Run("RemovePermissionInvalidatesCache", testCachedRBACServiceRemovePermissionInvalidatesCache)
	t.Run("BindRoleDelegates", testCachedRBACServiceBindRoleDelegates)
	t.Run("ListRoleBindings", testCachedRBACServiceListRoleBindings)
}

func testCachedRBACServiceListRoles(t *testing.T) {
	mockSvc, mockSARepo, cache, mr := setupCachedRBACTest(t)
	defer mr.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewCachedRBACService(mockSvc, mockSARepo, cache, logger)

	roles := []*domain.Role{{ID: uuid.New(), Name: "admin"}}
	mockSvc.On("ListRoles", mock.Anything).Return(roles, nil).Once()

	res, err := svc.ListRoles(context.Background())
	require.NoError(t, err)
	assert.Len(t, res, 1)
	mockSvc.AssertExpectations(t)
}

func testCachedRBACServiceAuthorizeDelegates(t *testing.T) {
	mockSvc, mockSARepo, cache, mr := setupCachedRBACTest(t)
	defer mr.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewCachedRBACService(mockSvc, mockSARepo, cache, logger)

	userID := uuid.New()
	tenantID := uuid.New()
	resource := "*"
	mockSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceRead, resource).Return(nil).Once()

	err := svc.Authorize(context.Background(), userID, tenantID, domain.PermissionInstanceRead, resource)
	require.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func testCachedRBACServiceHasPermissionCacheHit(t *testing.T) {
	mockSvc, mockSARepo, cache, mr := setupCachedRBACTest(t)
	defer mr.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewCachedRBACService(mockSvc, mockSARepo, cache, logger)

	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()
	resource := "*"
	key := rbacPermKey(tenantID, userID, domain.PermissionInstanceRead, resource)
	cache.Set(ctx, key, "1", time.Minute)

	allowed, err := svc.HasPermission(ctx, userID, tenantID, domain.PermissionInstanceRead, resource)
	require.NoError(t, err)
	assert.True(t, allowed)
	mockSvc.AssertNotCalled(t, "HasPermission", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func testCachedRBACServiceHasPermissionCacheMiss(t *testing.T) {
	mockSvc, mockSARepo, cache, mr := setupCachedRBACTest(t)
	defer mr.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewCachedRBACService(mockSvc, mockSARepo, cache, logger)

	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()
	permission := domain.PermissionInstanceRead
	resource := "*"

	mockSvc.On("HasPermission", mock.Anything, userID, tenantID, permission, resource).Return(true, nil).Once()

	allowed, err := svc.HasPermission(ctx, userID, tenantID, permission, resource)
	require.NoError(t, err)
	assert.True(t, allowed)

	key := rbacPermKey(tenantID, userID, permission, resource)
	assert.Equal(t, int64(1), cache.Exists(ctx, key).Val())
}

func testCachedRBACServiceHasPermissionError(t *testing.T) {
	mockSvc, mockSARepo, cache, mr := setupCachedRBACTest(t)
	defer mr.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewCachedRBACService(mockSvc, mockSARepo, cache, logger)

	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()
	permission := domain.PermissionInstanceRead
	resource := "*"

	mockSvc.On("HasPermission", mock.Anything, userID, tenantID, permission, resource).Return(false, assert.AnError).Once()

	allowed, err := svc.HasPermission(ctx, userID, tenantID, permission, resource)
	require.Error(t, err)
	assert.False(t, allowed)

	key := rbacPermKey(tenantID, userID, permission, resource)
	assert.LessOrEqual(t, cache.Exists(ctx, key).Val(), int64(0))
}

func testCachedRBACServiceCreateRoleDelegates(t *testing.T) {
	mockSvc, mockSARepo, cache, mr := setupCachedRBACTest(t)
	defer mr.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewCachedRBACService(mockSvc, mockSARepo, cache, logger)

	role := &domain.Role{ID: uuid.New(), Name: "dev"}
	mockSvc.On("CreateRole", mock.Anything, role).Return(nil).Once()

	err := svc.CreateRole(context.Background(), role)
	require.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func testCachedRBACServiceGetRoleByIDCaches(t *testing.T) {
	mockSvc, mockSARepo, cache, mr := setupCachedRBACTest(t)
	defer mr.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewCachedRBACService(mockSvc, mockSARepo, cache, logger)

	roleID := uuid.New()
	role := &domain.Role{ID: roleID, Name: "dev"}
	mockSvc.On("GetRoleByID", mock.Anything, roleID).Return(role, nil).Once()

	res, err := svc.GetRoleByID(context.Background(), roleID)
	require.NoError(t, err)
	assert.Equal(t, roleID, res.ID)

	key := rbacRoleIDKey(roleID)
	assert.Positive(t, cache.Exists(context.Background(), key).Val())
}

func testCachedRBACServiceGetRoleByNameCaches(t *testing.T) {
	mockSvc, mockSARepo, cache, mr := setupCachedRBACTest(t)
	defer mr.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewCachedRBACService(mockSvc, mockSARepo, cache, logger)

	role := &domain.Role{ID: uuid.New(), Name: "ops"}
	mockSvc.On("GetRoleByName", mock.Anything, "ops").Return(role, nil).Once()

	res, err := svc.GetRoleByName(context.Background(), "ops")
	require.NoError(t, err)
	assert.Equal(t, "ops", res.Name)

	key := rbacRoleNameKey("ops")
	assert.Positive(t, cache.Exists(context.Background(), key).Val())
}

func testCachedRBACServiceUpdateRoleInvalidatesCache(t *testing.T) {
	mockSvc, mockSARepo, cache, mr := setupCachedRBACTest(t)
	defer mr.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewCachedRBACService(mockSvc, mockSARepo, cache, logger)

	roleID := uuid.New()
	role := &domain.Role{ID: roleID, Name: "dev"}

	ctx := context.Background()
	cache.Set(ctx, rbacRoleIDKey(roleID), "cached", time.Minute)
	cache.Set(ctx, rbacRoleNameKey(role.Name), "cached", time.Minute)

	mockSvc.On("UpdateRole", mock.Anything, role).Return(nil).Once()

	err := svc.UpdateRole(ctx, role)
	require.NoError(t, err)

	existsID := cache.Exists(ctx, rbacRoleIDKey(roleID)).Val()
	existsName := cache.Exists(ctx, rbacRoleNameKey(role.Name)).Val()
	assert.Equal(t, int64(0), existsID)
	assert.Equal(t, int64(0), existsName)
}

func testCachedRBACServiceDeleteRoleInvalidatesCache(t *testing.T) {
	mockSvc, mockSARepo, cache, mr := setupCachedRBACTest(t)
	defer mr.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewCachedRBACService(mockSvc, mockSARepo, cache, logger)

	roleID := uuid.New()
	role := &domain.Role{ID: roleID, Name: "ops"}

	ctx := context.Background()
	cache.Set(ctx, rbacRoleIDKey(roleID), "cached", time.Minute)
	cache.Set(ctx, rbacRoleNameKey(role.Name), "cached", time.Minute)

	mockSvc.On("GetRoleByID", mock.Anything, roleID).Return(role, nil).Once()
	mockSvc.On("DeleteRole", mock.Anything, roleID).Return(nil).Once()

	err := svc.DeleteRole(ctx, roleID)
	require.NoError(t, err)

	existsID := cache.Exists(ctx, rbacRoleIDKey(roleID)).Val()
	existsName := cache.Exists(ctx, rbacRoleNameKey(role.Name)).Val()
	assert.Equal(t, int64(0), existsID)
	assert.Equal(t, int64(0), existsName)
}

func testCachedRBACServiceAddPermissionInvalidatesCache(t *testing.T) {
	mockSvc, mockSARepo, cache, mr := setupCachedRBACTest(t)
	defer mr.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewCachedRBACService(mockSvc, mockSARepo, cache, logger)

	roleID := uuid.New()
	role := &domain.Role{ID: roleID, Name: "ops"}

	ctx := context.Background()
	cache.Set(ctx, rbacRoleIDKey(roleID), "cached", time.Minute)
	cache.Set(ctx, rbacRoleNameKey(role.Name), "cached", time.Minute)

	mockSvc.On("AddPermissionToRole", mock.Anything, roleID, domain.PermissionInstanceRead).Return(nil).Once()
	mockSvc.On("GetRoleByID", mock.Anything, roleID).Return(role, nil).Once()

	err := svc.AddPermissionToRole(ctx, roleID, domain.PermissionInstanceRead)
	require.NoError(t, err)

	existsID := cache.Exists(ctx, rbacRoleIDKey(roleID)).Val()
	existsName := cache.Exists(ctx, rbacRoleNameKey(role.Name)).Val()
	assert.Equal(t, int64(0), existsID)
	assert.Equal(t, int64(0), existsName)
}

func testCachedRBACServiceRemovePermissionInvalidatesCache(t *testing.T) {
	mockSvc, mockSARepo, cache, mr := setupCachedRBACTest(t)
	defer mr.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewCachedRBACService(mockSvc, mockSARepo, cache, logger)

	roleID := uuid.New()
	role := &domain.Role{ID: roleID, Name: "ops"}

	ctx := context.Background()
	cache.Set(ctx, rbacRoleIDKey(roleID), "cached", time.Minute)
	cache.Set(ctx, rbacRoleNameKey(role.Name), "cached", time.Minute)

	mockSvc.On("RemovePermissionFromRole", mock.Anything, roleID, domain.PermissionInstanceRead).Return(nil).Once()
	mockSvc.On("GetRoleByID", mock.Anything, roleID).Return(role, nil).Once()

	err := svc.RemovePermissionFromRole(ctx, roleID, domain.PermissionInstanceRead)
	require.NoError(t, err)

	existsID := cache.Exists(ctx, rbacRoleIDKey(roleID)).Val()
	existsName := cache.Exists(ctx, rbacRoleNameKey(role.Name)).Val()
	assert.Equal(t, int64(0), existsID)
	assert.Equal(t, int64(0), existsName)
}

func testCachedRBACServiceBindRoleDelegates(t *testing.T) {
	mockSvc, mockSARepo, cache, mr := setupCachedRBACTest(t)
	defer mr.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewCachedRBACService(mockSvc, mockSARepo, cache, logger)

	mockSvc.On("BindRole", mock.Anything, "user@example.com", "admin").Return(nil).Once()

	err := svc.BindRole(context.Background(), "user@example.com", "admin")
	require.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func testCachedRBACServiceListRoleBindings(t *testing.T) {
	mockSvc, mockSARepo, cache, mr := setupCachedRBACTest(t)
	defer mr.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewCachedRBACService(mockSvc, mockSARepo, cache, logger)

	mockSvc.On("ListRoleBindings", mock.Anything).Return([]*domain.User{{ID: uuid.New()}}, nil).Once()

	users, err := svc.ListRoleBindings(context.Background())
	require.NoError(t, err)
	assert.Len(t, users, 1)
	mockSvc.AssertExpectations(t)
}
