package services_test

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/mock"
)

// MockRBACService
type MockRBACService struct {
	mock.Mock
}

func (m *MockRBACService) Authorize(ctx context.Context, userID, tenantID uuid.UUID, permission domain.Permission, resource string) error {
	return m.Called(ctx, userID, tenantID, permission, resource).Error(0)
}
func (m *MockRBACService) HasPermission(ctx context.Context, userID, tenantID uuid.UUID, permission domain.Permission, resource string) (bool, error) {
	args := m.Called(ctx, userID, tenantID, permission, resource)
	return args.Bool(0), args.Error(1)
}
func (m *MockRBACService) CreateRole(ctx context.Context, role *domain.Role) error {
	return m.Called(ctx, role).Error(0)
}
func (m *MockRBACService) GetRoleByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}
func (m *MockRBACService) GetRoleByName(ctx context.Context, name string) (*domain.Role, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}
func (m *MockRBACService) ListRoles(ctx context.Context) ([]*domain.Role, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Role), args.Error(1)
}
func (m *MockRBACService) UpdateRole(ctx context.Context, role *domain.Role) error {
	return m.Called(ctx, role).Error(0)
}
func (m *MockRBACService) DeleteRole(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockRBACService) AddPermissionToRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	return m.Called(ctx, roleID, permission).Error(0)
}
func (m *MockRBACService) RemovePermissionFromRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	return m.Called(ctx, roleID, permission).Error(0)
}
func (m *MockRBACService) BindRole(ctx context.Context, userIdentifier string, roleName string) error {
	return m.Called(ctx, userIdentifier, roleName).Error(0)
}
func (m *MockRBACService) ListRoleBindings(ctx context.Context) ([]*domain.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.User), args.Error(1)
}
func (m *MockRBACService) EvaluatePolicy(ctx context.Context, userID uuid.UUID, action string, resource string, context map[string]interface{}) (bool, error) {
	args := m.Called(ctx, userID, action, resource, context)
	return args.Bool(0), args.Error(1)
}
func (m *MockRBACService) AuthorizeServiceAccount(ctx context.Context, saID, tenantID uuid.UUID, permission domain.Permission, resource string) error {
	return m.Called(ctx, saID, tenantID, permission, resource).Error(0)
}

// MockIAMRepository
type MockIAMRepository struct{ mock.Mock }

func (m *MockIAMRepository) CreatePolicy(ctx context.Context, tenantID uuid.UUID, policy *domain.Policy) error {
	return m.Called(ctx, tenantID, policy).Error(0)
}
func (m *MockIAMRepository) GetPolicyByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Policy, error) {
	args := m.Called(ctx, tenantID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Policy), args.Error(1)
}
func (m *MockIAMRepository) ListPolicies(ctx context.Context, tenantID uuid.UUID) ([]*domain.Policy, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Policy), args.Error(1)
}
func (m *MockIAMRepository) UpdatePolicy(ctx context.Context, tenantID uuid.UUID, policy *domain.Policy) error {
	return m.Called(ctx, tenantID, policy).Error(0)
}
func (m *MockIAMRepository) DeletePolicy(ctx context.Context, tenantID, id uuid.UUID) error {
	return m.Called(ctx, tenantID, id).Error(0)
}
func (m *MockIAMRepository) AttachPolicyToUser(ctx context.Context, tenantID, userID, policyID uuid.UUID) error {
	return m.Called(ctx, tenantID, userID, policyID).Error(0)
}
func (m *MockIAMRepository) DetachPolicyFromUser(ctx context.Context, tenantID, userID, policyID uuid.UUID) error {
	return m.Called(ctx, tenantID, userID, policyID).Error(0)
}
func (m *MockIAMRepository) GetPoliciesForUser(ctx context.Context, tenantID, userID uuid.UUID) ([]*domain.Policy, error) {
	args := m.Called(ctx, tenantID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Policy), args.Error(1)
}
func (m *MockIAMRepository) AttachPolicyToRole(ctx context.Context, tenantID uuid.UUID, roleName string, policyID uuid.UUID) error {
	return m.Called(ctx, tenantID, roleName, policyID).Error(0)
}
func (m *MockIAMRepository) DetachPolicyFromRole(ctx context.Context, tenantID uuid.UUID, roleName string, policyID uuid.UUID) error {
	return m.Called(ctx, tenantID, roleName, policyID).Error(0)
}
func (m *MockIAMRepository) GetPoliciesForRole(ctx context.Context, tenantID uuid.UUID, roleName string) ([]*domain.Policy, error) {
	args := m.Called(ctx, tenantID, roleName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Policy), args.Error(1)
}
func (m *MockIAMRepository) AttachPolicyToServiceAccount(ctx context.Context, tenantID, saID, policyID uuid.UUID) error {
	return m.Called(ctx, tenantID, saID, policyID).Error(0)
}
func (m *MockIAMRepository) DetachPolicyFromServiceAccount(ctx context.Context, tenantID, saID, policyID uuid.UUID) error {
	return m.Called(ctx, tenantID, saID, policyID).Error(0)
}
func (m *MockIAMRepository) GetPoliciesForServiceAccount(ctx context.Context, tenantID, saID uuid.UUID) ([]*domain.Policy, error) {
	args := m.Called(ctx, tenantID, saID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Policy), args.Error(1)
}

// MockIdentityService
type MockIdentityService struct {
	mock.Mock
}

func (m *MockIdentityService) CreateKey(ctx context.Context, userID uuid.UUID, name string) (*domain.APIKey, error) {
	args := m.Called(ctx, userID, name)
	r0, _ := args.Get(0).(*domain.APIKey)
	return r0, args.Error(1)
}
func (m *MockIdentityService) ValidateAPIKey(ctx context.Context, key string) (*domain.APIKey, error) {
	args := m.Called(ctx, key)
	r0, _ := args.Get(0).(*domain.APIKey)
	return r0, args.Error(1)
}
func (m *MockIdentityService) GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.APIKey)
	return r0, args.Error(1)
}
func (m *MockIdentityService) ListKeys(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(ctx, userID)
	r0, _ := args.Get(0).([]*domain.APIKey)
	return r0, args.Error(1)
}
func (m *MockIdentityService) RevokeKey(ctx context.Context, userID, id uuid.UUID) error {
	args := m.Called(ctx, userID, id)
	return args.Error(0)
}
func (m *MockIdentityService) RotateKey(ctx context.Context, userID, id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(ctx, userID, id)
	r0, _ := args.Get(0).(*domain.APIKey)
	return r0, args.Error(1)
}
func (m *MockIdentityService) CreateServiceAccount(ctx context.Context, tenantID uuid.UUID, name, role string) (*domain.ServiceAccountWithSecret, error) {
	args := m.Called(ctx, tenantID, name, role)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.ServiceAccountWithSecret)
	return r0, args.Error(1)
}
func (m *MockIdentityService) GetServiceAccount(ctx context.Context, id uuid.UUID) (*domain.ServiceAccount, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.ServiceAccount)
	return r0, args.Error(1)
}
func (m *MockIdentityService) ListServiceAccounts(ctx context.Context, tenantID uuid.UUID) ([]*domain.ServiceAccount, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.ServiceAccount)
	return r0, args.Error(1)
}
func (m *MockIdentityService) UpdateServiceAccount(ctx context.Context, sa *domain.ServiceAccount) error {
	args := m.Called(ctx, sa)
	return args.Error(0)
}
func (m *MockIdentityService) DeleteServiceAccount(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockIdentityService) ValidateClientCredentials(ctx context.Context, clientID, clientSecret string) (string, error) {
	args := m.Called(ctx, clientID, clientSecret)
	return args.String(0), args.Error(1)
}
func (m *MockIdentityService) ValidateAccessToken(ctx context.Context, token string) (*domain.ServiceAccountClaims, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.ServiceAccountClaims)
	return r0, args.Error(1)
}
func (m *MockIdentityService) RotateServiceAccountSecret(ctx context.Context, saID uuid.UUID) (string, error) {
	args := m.Called(ctx, saID)
	return args.String(0), args.Error(1)
}
func (m *MockIdentityService) RevokeServiceAccountSecret(ctx context.Context, saID, secretID uuid.UUID) error {
	args := m.Called(ctx, saID, secretID)
	return args.Error(0)
}
func (m *MockIdentityService) ListServiceAccountSecrets(ctx context.Context, saID uuid.UUID) ([]*domain.ServiceAccountSecret, error) {
	args := m.Called(ctx, saID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.ServiceAccountSecret)
	return r0, args.Error(1)
}
func (m *MockIdentityService) ValidateKey(ctx context.Context, key string) (*domain.User, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockIdentityService) TokenTTL() time.Duration {
	return time.Hour
}

// MockIdentityRepo
type MockIdentityRepo struct{ mock.Mock }

func (m *MockIdentityRepo) CreateAPIKey(ctx context.Context, apiKey *domain.APIKey) error {
	return m.Called(ctx, apiKey).Error(0)
}
func (m *MockIdentityRepo) GetAPIKeyByHash(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	args := m.Called(ctx, keyHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}
func (m *MockIdentityRepo) GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}
func (m *MockIdentityRepo) ListAPIKeysByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.APIKey), args.Error(1)
}
func (m *MockIdentityRepo) DeleteAPIKey(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// MockUserRepo
type MockUserRepo struct{ mock.Mock }

func (m *MockUserRepo) Create(ctx context.Context, u *domain.User) error {
	return m.Called(ctx, u).Error(0)
}
func (m *MockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepo) Update(ctx context.Context, u *domain.User) error {
	return m.Called(ctx, u).Error(0)
}
func (m *MockUserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockUserRepo) List(ctx context.Context) ([]*domain.User, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.User), args.Error(1)
}

// MockTenantRepo
type MockTenantRepo struct{ mock.Mock }

func (m *MockTenantRepo) Create(ctx context.Context, tenant *domain.Tenant) error {
	return m.Called(ctx, tenant).Error(0)
}
func (m *MockTenantRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tenant), args.Error(1)
}
func (m *MockTenantRepo) GetBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	args := m.Called(ctx, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tenant), args.Error(1)
}
func (m *MockTenantRepo) Update(ctx context.Context, tenant *domain.Tenant) error {
	return m.Called(ctx, tenant).Error(0)
}
func (m *MockTenantRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockTenantRepo) AddMember(ctx context.Context, tenantID, userID uuid.UUID, role string) error {
	return m.Called(ctx, tenantID, userID, role).Error(0)
}
func (m *MockTenantRepo) RemoveMember(ctx context.Context, tenantID, userID uuid.UUID) error {
	return m.Called(ctx, tenantID, userID).Error(0)
}
func (m *MockTenantRepo) ListMembers(ctx context.Context, tenantID uuid.UUID) ([]domain.TenantMember, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]domain.TenantMember), args.Error(1)
}
func (m *MockTenantRepo) GetMembership(ctx context.Context, tenantID, userID uuid.UUID) (*domain.TenantMember, error) {
	args := m.Called(ctx, tenantID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TenantMember), args.Error(1)
}
func (m *MockTenantRepo) ListUserTenants(ctx context.Context, userID uuid.UUID) ([]domain.Tenant, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]domain.Tenant), args.Error(1)
}
func (m *MockTenantRepo) GetQuota(ctx context.Context, tenantID uuid.UUID) (*domain.TenantQuota, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TenantQuota), args.Error(1)
}
func (m *MockTenantRepo) UpdateQuota(ctx context.Context, quota *domain.TenantQuota) error {
	return m.Called(ctx, quota).Error(0)
}
func (m *MockTenantRepo) IncrementUsage(ctx context.Context, tenantID uuid.UUID, resource string, amount int) error {
	return m.Called(ctx, tenantID, resource, amount).Error(0)
}
func (m *MockTenantRepo) DecrementUsage(ctx context.Context, tenantID uuid.UUID, resource string, amount int) error {
	return m.Called(ctx, tenantID, resource, amount).Error(0)
}

type MockTenantRepository = MockTenantRepo

// MockTenantService
type MockTenantService struct{ mock.Mock }

func (m *MockTenantService) CreateTenant(ctx context.Context, name, slug string, ownerID uuid.UUID) (*domain.Tenant, error) {
	args := m.Called(ctx, name, slug, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tenant), args.Error(1)
}
func (m *MockTenantService) GetTenant(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tenant), args.Error(1)
}
func (m *MockTenantService) ListUserTenants(ctx context.Context, userID uuid.UUID) ([]domain.Tenant, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Tenant), args.Error(1)
}
func (m *MockTenantService) InviteMember(ctx context.Context, tenantID uuid.UUID, email, role string) error {
	return m.Called(ctx, tenantID, email, role).Error(0)
}
func (m *MockTenantService) RemoveMember(ctx context.Context, tenantID, userID uuid.UUID) error {
	return m.Called(ctx, tenantID, userID).Error(0)
}
func (m *MockTenantService) SwitchTenant(ctx context.Context, userID, tenantID uuid.UUID) error {
	return m.Called(ctx, userID, tenantID).Error(0)
}
func (m *MockTenantService) CheckQuota(ctx context.Context, tenantID uuid.UUID, resource string, requested int) error {
	return m.Called(ctx, tenantID, resource, requested).Error(0)
}
func (m *MockTenantService) GetMembership(ctx context.Context, tenantID, userID uuid.UUID) (*domain.TenantMember, error) {
	args := m.Called(ctx, tenantID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TenantMember), args.Error(1)
}
func (m *MockTenantService) IncrementUsage(ctx context.Context, tenantID uuid.UUID, resource string, amount int) error {
	return m.Called(ctx, tenantID, resource, amount).Error(0)
}
func (m *MockTenantService) DecrementUsage(ctx context.Context, tenantID uuid.UUID, resource string, amount int) error {
	return m.Called(ctx, tenantID, resource, amount).Error(0)
}

// MockRoleRepository
type MockRoleRepository struct{ mock.Mock }

func (m *MockRoleRepository) CreateRole(ctx context.Context, role *domain.Role) error {
	return m.Called(ctx, role).Error(0)
}
func (m *MockRoleRepository) GetRoleByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}
func (m *MockRoleRepository) GetRoleByName(ctx context.Context, name string) (*domain.Role, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}
func (m *MockRoleRepository) ListRoles(ctx context.Context) ([]*domain.Role, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Role), args.Error(1)
}
func (m *MockRoleRepository) UpdateRole(ctx context.Context, role *domain.Role) error {
	return m.Called(ctx, role).Error(0)
}
func (m *MockRoleRepository) DeleteRole(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockRoleRepository) AddPermissionToRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	return m.Called(ctx, roleID, permission).Error(0)
}
func (m *MockRoleRepository) RemovePermissionFromRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	return m.Called(ctx, roleID, permission).Error(0)
}
func (m *MockRoleRepository) GetPermissionsForRole(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
	args := m.Called(ctx, roleID)
	return args.Get(0).([]domain.Permission), args.Error(1)
}

// MockImageRepo
type MockImageRepo struct {
	mock.Mock
}

func (m *MockImageRepo) Create(ctx context.Context, img *domain.Image) error {
	return m.Called(ctx, img).Error(0)
}
func (m *MockImageRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Image, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Image), args.Error(1)
}
func (m *MockImageRepo) List(ctx context.Context, userID uuid.UUID, includePublic bool) ([]*domain.Image, error) {
	args := m.Called(ctx, userID, includePublic)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Image), args.Error(1)
}
func (m *MockImageRepo) Update(ctx context.Context, img *domain.Image) error {
	return m.Called(ctx, img).Error(0)
}
func (m *MockImageRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockImageRepo) GetByName(ctx context.Context, name string, userID uuid.UUID) (*domain.Image, error) {
	args := m.Called(ctx, name, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Image), args.Error(1)
}
