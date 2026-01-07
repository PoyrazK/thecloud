package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
)

type rbacService struct {
	userRepo ports.UserRepository
	roleRepo ports.RoleRepository
	logger   *slog.Logger
}

func NewRBACService(userRepo ports.UserRepository, roleRepo ports.RoleRepository, logger *slog.Logger) *rbacService {
	return &rbacService{
		userRepo: userRepo,
		roleRepo: roleRepo,
		logger:   logger,
	}
}

func (s *rbacService) Authorize(ctx context.Context, userID uuid.UUID, permission domain.Permission) error {
	allowed, err := s.HasPermission(ctx, userID, permission)
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New(errors.Forbidden, fmt.Sprintf("permission denied: %s", permission))
	}
	return nil
}

func (s *rbacService) HasPermission(ctx context.Context, userID uuid.UUID, permission domain.Permission) (bool, error) {
	// 1. Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get user: %w", err)
	}

	// 2. Get role
	role, err := s.roleRepo.GetRoleByName(ctx, user.Role)
	if err != nil {
		// If role not found in DB, default to viewer if it was just a string
		s.logger.Warn("role not found in DB, checking if it is a default role", "role", user.Role)
		return false, nil // For now, strict check
	}

	// 3. Check permissions
	for _, p := range role.Permissions {
		if p == domain.PermissionFullAccess {
			return true, nil
		}
		if p == permission {
			return true, nil
		}
	}

	return false, nil
}

func (s *rbacService) CreateRole(ctx context.Context, role *domain.Role) error {
	if role.ID == uuid.Nil {
		role.ID = uuid.New()
	}
	return s.roleRepo.CreateRole(ctx, role)
}

func (s *rbacService) GetRoleByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	return s.roleRepo.GetRoleByID(ctx, id)
}

func (s *rbacService) GetRoleByName(ctx context.Context, name string) (*domain.Role, error) {
	return s.roleRepo.GetRoleByName(ctx, name)
}

func (s *rbacService) ListRoles(ctx context.Context) ([]*domain.Role, error) {
	return s.roleRepo.ListRoles(ctx)
}

func (s *rbacService) UpdateRole(ctx context.Context, role *domain.Role) error {
	return s.roleRepo.UpdateRole(ctx, role)
}

func (s *rbacService) DeleteRole(ctx context.Context, id uuid.UUID) error {
	return s.roleRepo.DeleteRole(ctx, id)
}

func (s *rbacService) AddPermissionToRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	return s.roleRepo.AddPermissionToRole(ctx, roleID, permission)
}

func (s *rbacService) RemovePermissionFromRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	return s.roleRepo.RemovePermissionFromRole(ctx, roleID, permission)
}

func (s *rbacService) BindRole(ctx context.Context, userID uuid.UUID, roleName string) error {
	// 1. Verify role exists
	_, err := s.roleRepo.GetRoleByName(ctx, roleName)
	if err != nil {
		return fmt.Errorf("role %s does not exist: %w", roleName, err)
	}

	// 2. Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// 3. Update role
	user.Role = roleName
	return s.userRepo.Update(ctx, user)
}

func (s *rbacService) ListRoleBindings(ctx context.Context) ([]*domain.User, error) {
	// In this implementation, bindings are just users with their roles
	return s.userRepo.List(ctx)
}
