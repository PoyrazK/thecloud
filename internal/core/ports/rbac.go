package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

type RoleRepository interface {
	CreateRole(ctx context.Context, role *domain.Role) error
	GetRoleByID(ctx context.Context, id uuid.UUID) (*domain.Role, error)
	GetRoleByName(ctx context.Context, name string) (*domain.Role, error)
	ListRoles(ctx context.Context) ([]*domain.Role, error)
	UpdateRole(ctx context.Context, role *domain.Role) error
	DeleteRole(ctx context.Context, id uuid.UUID) error

	// Role-Permission mapping
	AddPermissionToRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error
	RemovePermissionFromRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error
	GetPermissionsForRole(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error)
}

// RBACService handles role-based access control checks and management.
type RBACService interface {
	Authorize(ctx context.Context, userID uuid.UUID, permission domain.Permission) error
	HasPermission(ctx context.Context, userID uuid.UUID, permission domain.Permission) (bool, error)

	// Role management
	CreateRole(ctx context.Context, role *domain.Role) error
	GetRoleByID(ctx context.Context, id uuid.UUID) (*domain.Role, error)
	GetRoleByName(ctx context.Context, name string) (*domain.Role, error)
	ListRoles(ctx context.Context) ([]*domain.Role, error)
	UpdateRole(ctx context.Context, role *domain.Role) error
	DeleteRole(ctx context.Context, id uuid.UUID) error

	// Permission management
	AddPermissionToRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error
	RemovePermissionFromRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error
}
