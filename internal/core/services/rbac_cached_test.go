package services_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
)

type mockRBACService struct {
	mock.Mock
}

const (
	rbacPermPrefix     = "rbac:perm:"
	rbacRoleIDPrefix   = "rbac:role:id:"
	rbacRoleNamePrefix = "rbac:role:name:"
)

func rbacPermKey(tenantID, userID uuid.UUID, permission domain.Permission, resource string) string {
	return fmt.Sprintf("%s%s:%s:%s:%s", rbacPermPrefix, tenantID, userID, permission, resource)
}

func rbacRoleIDKey(roleID uuid.UUID) string {
	return rbacRoleIDPrefix + roleID.String()
}

func rbacRoleNameKey(name string) string {
	return rbacRoleNamePrefix + name
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

type mockServiceAccountRepository struct {
	mock.Mock
}

func (m *mockServiceAccountRepository) Create(ctx context.Context, sa *domain.ServiceAccount) error {
	args := m.Called(ctx, sa)
	return args.Error(0)
}

func (m *mockServiceAccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.ServiceAccount, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.ServiceAccount)
	return r0, args.Error(1)
}

func (m *mockServiceAccountRepository) GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*domain.ServiceAccount, error) {
	args := m.Called(ctx, tenantID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.ServiceAccount)
	return r0, args.Error(1)
}

func (m *mockServiceAccountRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]*domain.ServiceAccount, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.ServiceAccount)
	return r0, args.Error(1)
}

func (m *mockServiceAccountRepository) Update(ctx context.Context, sa *domain.ServiceAccount) error {
	args := m.Called(ctx, sa)
	return args.Error(0)
}

func (m *mockServiceAccountRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockServiceAccountRepository) CreateSecret(ctx context.Context, secret *domain.ServiceAccountSecret) error {
	args := m.Called(ctx, secret)
	return args.Error(0)
}

func (m *mockServiceAccountRepository) GetSecretByHash(ctx context.Context, secretHash string) (*domain.ServiceAccountSecret, error) {
	args := m.Called(ctx, secretHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.ServiceAccountSecret)
	return r0, args.Error(1)
}

func (m *mockServiceAccountRepository) ListSecretsByServiceAccount(ctx context.Context, saID uuid.UUID) ([]*domain.ServiceAccountSecret, error) {
	args := m.Called(ctx, saID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.ServiceAccountSecret)
	return r0, args.Error(1)
}

func (m *mockServiceAccountRepository) UpdateSecretLastUsed(ctx context.Context, secretID uuid.UUID) error {
	args := m.Called(ctx, secretID)
	return args.Error(0)
}

func (m *mockServiceAccountRepository) DeleteSecret(ctx context.Context, secretID uuid.UUID) error {
	args := m.Called(ctx, secretID)
	return args.Error(0)
}

func (m *mockServiceAccountRepository) DeleteAllSecrets(ctx context.Context, saID uuid.UUID) error {
	args := m.Called(ctx, saID)
	return args.Error(0)
}

func setupCachedRBACTest(t *testing.T) (*mockRBACService, *mockServiceAccountRepository, *redis.Client, *miniredis.Miniredis) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return new(mockRBACService), new(mockServiceAccountRepository), client, mr
}
