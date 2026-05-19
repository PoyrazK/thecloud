// Package ports defines service and repository interfaces.
package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

// RoleRepository manages the persistence of roles and their associated permission sets.
type RoleRepository interface {
	// CreateRole saves a new security role.
	CreateRole(ctx context.Context, role *domain.Role) error
	// GetRoleByID retrieves a role definition by its UUID.
	GetRoleByID(ctx context.Context, id uuid.UUID) (*domain.Role, error)
	// GetRoleByName retrieves a role definition by its unique name string.
	GetRoleByName(ctx context.Context, name string) (*domain.Role, error)
	// ListRoles returns all defined roles in the system.
	ListRoles(ctx context.Context) ([]*domain.Role, error)
	// UpdateRole modifies an existing role's metadata or permissions.
	UpdateRole(ctx context.Context, role *domain.Role) error
	// DeleteRole removes a security role from storage.
	DeleteRole(ctx context.Context, id uuid.UUID) error

	// Role-Permission mapping

	// AddPermissionToRole grants a specific capability to a role.
	AddPermissionToRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error
	// RemovePermissionFromRole revokes a specific capability from a role.
	RemovePermissionFromRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error
	// GetPermissionsForRole lists all capabilities granted to a specific role.
	GetPermissionsForRole(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error)
}

// RBACService provides business logic for enforcing security policies and managing access control.
type RBACService interface {
	// Authorize checks if a user has a specific permission on a resource, returning an error if not.
	Authorize(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, permission domain.Permission, resource string) error
	// HasPermission checks if a user has a specific permission on a resource, returning a boolean flag.
	HasPermission(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, permission domain.Permission, resource string) (bool, error)

	// Role management

	// CreateRole registers a new user role with specific permissions.
	CreateRole(ctx context.Context, role *domain.Role) error
	// GetRoleByID fetches a role definition by UUID.
	GetRoleByID(ctx context.Context, id uuid.UUID) (*domain.Role, error)
	// GetRoleByName fetches a role definition by its unique name.
	GetRoleByName(ctx context.Context, name string) (*domain.Role, error)
	// ListRoles retrieves all roles available in the system.
	ListRoles(ctx context.Context) ([]*domain.Role, error)
	// UpdateRole modifies role properties or permissions.
	UpdateRole(ctx context.Context, role *domain.Role) error
	// DeleteRole decommission a security role.
	DeleteRole(ctx context.Context, id uuid.UUID) error

	// Permission management

	// AddPermissionToRole attaches a permission capability to an existing role.
	AddPermissionToRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error
	// RemovePermissionFromRole detaches a permission capability from a role.
	RemovePermissionFromRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error

	// Role binding (User-Role assignment)

	// BindRole assigns a security role to a user.
	BindRole(ctx context.Context, userIdentifier string, roleName string) error
	// ListRoleBindings returns users along with their assigned role information.
	ListRoleBindings(ctx context.Context) ([]*domain.User, error)

	// IAM Policy Support
	EvaluatePolicy(ctx context.Context, userID uuid.UUID, action string, resource string, evalCtx map[string]interface{}) (bool, error)

	// AuthorizeServiceAccount checks if a service account has a specific permission on a resource.
	AuthorizeServiceAccount(ctx context.Context, saID uuid.UUID, tenantID uuid.UUID, permission domain.Permission, resource string) error
}
