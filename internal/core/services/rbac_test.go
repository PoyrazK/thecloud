package services_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAuthorize(t *testing.T) {
	userRepo := new(MockUserRepo)
	roleRepo := new(MockRoleRepo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewRBACService(userRepo, roleRepo, logger)

	ctx := context.Background()
	userID := uuid.New()

	t.Run("Success_ExactPermission", func(t *testing.T) {
		user := &domain.User{ID: userID, Role: "developer"}
		role := &domain.Role{
			Name: "developer",
			Permissions: []domain.Permission{
				domain.PermissionInstanceLaunch,
				domain.PermissionInstanceRead,
			},
		}

		userRepo.On("GetByID", ctx, userID).Return(user, nil).Once()
		roleRepo.On("GetRoleByName", ctx, "developer").Return(role, nil).Once()

		err := svc.Authorize(ctx, userID, domain.PermissionInstanceLaunch)
		assert.NoError(t, err)

		userRepo.AssertExpectations(t)
		roleRepo.AssertExpectations(t)
	})

	t.Run("Success_FullAccess", func(t *testing.T) {
		user := &domain.User{ID: userID, Role: "admin"}
		role := &domain.Role{
			Name: "admin",
			Permissions: []domain.Permission{
				domain.PermissionFullAccess,
			},
		}

		userRepo.On("GetByID", ctx, userID).Return(user, nil).Once()
		roleRepo.On("GetRoleByName", ctx, "admin").Return(role, nil).Once()

		err := svc.Authorize(ctx, userID, domain.PermissionVpcCreate)
		assert.NoError(t, err)

		userRepo.AssertExpectations(t)
		roleRepo.AssertExpectations(t)
	})

	t.Run("Failure_Denied", func(t *testing.T) {
		user := &domain.User{ID: userID, Role: "viewer"}
		role := &domain.Role{
			Name: "viewer",
			Permissions: []domain.Permission{
				domain.PermissionInstanceRead,
			},
		}

		userRepo.On("GetByID", ctx, userID).Return(user, nil).Once()
		roleRepo.On("GetRoleByName", ctx, "viewer").Return(role, nil).Once()

		err := svc.Authorize(ctx, userID, domain.PermissionInstanceLaunch)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")

		userRepo.AssertExpectations(t)
		roleRepo.AssertExpectations(t)
	})

	t.Run("Fallback_Admin", func(t *testing.T) {
		user := &domain.User{ID: userID, Role: domain.RoleAdmin}
		// Role not found in DB
		userRepo.On("GetByID", ctx, userID).Return(user, nil).Once()
		roleRepo.On("GetRoleByName", ctx, string(domain.RoleAdmin)).Return(nil, errors.New("not found")).Once()

		err := svc.Authorize(ctx, userID, domain.PermissionInstanceLaunch)
		assert.NoError(t, err)

		userRepo.AssertExpectations(t)
		roleRepo.AssertExpectations(t)
	})

	t.Run("Fallback_Viewer_Allowed", func(t *testing.T) {
		user := &domain.User{ID: userID, Role: domain.RoleViewer}
		userRepo.On("GetByID", ctx, userID).Return(user, nil).Once()
		roleRepo.On("GetRoleByName", ctx, string(domain.RoleViewer)).Return(nil, errors.New("not found")).Once()

		err := svc.Authorize(ctx, userID, domain.PermissionInstanceRead)
		assert.NoError(t, err)

		userRepo.AssertExpectations(t)
		roleRepo.AssertExpectations(t)
	})

	t.Run("Fallback_Viewer_Denied", func(t *testing.T) {
		user := &domain.User{ID: userID, Role: domain.RoleViewer}
		userRepo.On("GetByID", ctx, userID).Return(user, nil).Once()
		roleRepo.On("GetRoleByName", ctx, string(domain.RoleViewer)).Return(nil, errors.New("not found")).Once()

		err := svc.Authorize(ctx, userID, domain.PermissionInstanceLaunch)
		assert.Error(t, err)

		userRepo.AssertExpectations(t)
		roleRepo.AssertExpectations(t)
	})

	t.Run("Fallback_UnknownRole", func(t *testing.T) {
		user := &domain.User{ID: userID, Role: "unknown"}
		userRepo.On("GetByID", ctx, userID).Return(user, nil).Once()
		roleRepo.On("GetRoleByName", ctx, "unknown").Return(nil, errors.New("not found")).Once()

		err := svc.Authorize(ctx, userID, domain.PermissionInstanceLaunch)
		assert.Error(t, err)

		userRepo.AssertExpectations(t)
		roleRepo.AssertExpectations(t)
	})
}

func TestRBAC_CRUD(t *testing.T) {
	userRepo := new(MockUserRepo)
	roleRepo := new(MockRoleRepo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewRBACService(userRepo, roleRepo, logger)

	ctx := context.Background()

	t.Run("CreateRole", func(t *testing.T) {
		role := &domain.Role{Name: "test-role"}
		roleRepo.On("CreateRole", ctx, role).Return(nil).Once()
		err := svc.CreateRole(ctx, role)
		assert.NoError(t, err)
		roleRepo.AssertExpectations(t)
	})

	t.Run("ListRoles", func(t *testing.T) {
		roles := []*domain.Role{{Name: "r1"}}
		roleRepo.On("ListRoles", ctx).Return(roles, nil).Once()
		res, err := svc.ListRoles(ctx)
		assert.NoError(t, err)
		assert.Equal(t, roles, res)
		roleRepo.AssertExpectations(t)
	})

	t.Run("GetRoleByID", func(t *testing.T) {
		id := uuid.New()
		role := &domain.Role{ID: id}
		roleRepo.On("GetRoleByID", ctx, id).Return(role, nil).Once()
		res, err := svc.GetRoleByID(ctx, id)
		assert.NoError(t, err)
		assert.Equal(t, role, res)
		roleRepo.AssertExpectations(t)
	})

	t.Run("UpdateRole", func(t *testing.T) {
		role := &domain.Role{Name: "updated"}
		roleRepo.On("UpdateRole", ctx, role).Return(nil).Once()
		err := svc.UpdateRole(ctx, role)
		assert.NoError(t, err)
		roleRepo.AssertExpectations(t)
	})

	t.Run("DeleteRole", func(t *testing.T) {
		id := uuid.New()
		roleRepo.On("DeleteRole", ctx, id).Return(nil).Once()
		err := svc.DeleteRole(ctx, id)
		assert.NoError(t, err)
		roleRepo.AssertExpectations(t)
	})
}

func TestRBAC_BindRole(t *testing.T) {
	userRepo := new(MockUserRepo)
	roleRepo := new(MockRoleRepo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewRBACService(userRepo, roleRepo, logger)

	ctx := context.Background()
	userEmail := "test@example.com"
	userID := uuid.New()
	roleName := "developer"

	t.Run("Success_ByEmail", func(t *testing.T) {
		role := &domain.Role{Name: roleName}
		user := &domain.User{ID: userID, Email: userEmail, Role: "old"}

		roleRepo.On("GetRoleByName", ctx, roleName).Return(role, nil).Once()
		// GetByEmail fails if UUID parsing fails for email, so checking logic
		// services/rbac.go:127 tries uuid.Parse, if error, MockUserRepo.GetByEmail
		userRepo.On("GetByEmail", ctx, userEmail).Return(user, nil).Once()
		userRepo.On("Update", ctx, mock.MatchedBy(func(u *domain.User) bool {
			return u.Role == roleName
		})).Return(nil).Once()

		err := svc.BindRole(ctx, userEmail, roleName)
		assert.NoError(t, err)

		roleRepo.AssertExpectations(t)
		userRepo.AssertExpectations(t)
	})

	t.Run("Success_ByID", func(t *testing.T) {
		role := &domain.Role{Name: roleName}
		user := &domain.User{ID: userID, Role: "old"}
		idStr := userID.String()

		roleRepo.On("GetRoleByName", ctx, roleName).Return(role, nil).Once()
		userRepo.On("GetByID", ctx, userID).Return(user, nil).Once()
		userRepo.On("Update", ctx, mock.MatchedBy(func(u *domain.User) bool {
			return u.Role == roleName
		})).Return(nil).Once()

		err := svc.BindRole(ctx, idStr, roleName)
		assert.NoError(t, err)

		roleRepo.AssertExpectations(t)
		userRepo.AssertExpectations(t)
	})

	t.Run("RoleNotFound", func(t *testing.T) {
		roleRepo.On("GetRoleByName", ctx, "missing").Return(nil, errors.New("not found")).Once()
		err := svc.BindRole(ctx, userEmail, "missing")
		assert.Error(t, err)
		roleRepo.AssertExpectations(t)
	})
}

func TestRBAC_Permissions(t *testing.T) {
	userRepo := new(MockUserRepo)
	roleRepo := new(MockRoleRepo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewRBACService(userRepo, roleRepo, logger)
	ctx := context.Background()
	roleID := uuid.New()

	t.Run("AddPermission", func(t *testing.T) {
		roleRepo.On("AddPermissionToRole", ctx, roleID, domain.PermissionInstanceLaunch).Return(nil).Once()
		err := svc.AddPermissionToRole(ctx, roleID, domain.PermissionInstanceLaunch)
		assert.NoError(t, err)
		roleRepo.AssertExpectations(t)
	})

	t.Run("RemovePermission", func(t *testing.T) {
		roleRepo.On("RemovePermissionFromRole", ctx, roleID, domain.PermissionInstanceLaunch).Return(nil).Once()
		err := svc.RemovePermissionFromRole(ctx, roleID, domain.PermissionInstanceLaunch)
		assert.NoError(t, err)
		roleRepo.AssertExpectations(t)
	})
}
