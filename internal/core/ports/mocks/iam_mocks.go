package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/mock"
)

// IAMRepository is a mock for ports.IAMRepository
type IAMRepository struct {
	mock.Mock
}

func (m *IAMRepository) CreatePolicy(ctx context.Context, tenantID uuid.UUID, policy *domain.Policy) error {
	args := m.Called(ctx, tenantID, policy)
	return args.Error(0)
}

func (m *IAMRepository) GetPolicyByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*domain.Policy, error) {
	args := m.Called(ctx, tenantID, id)
	r0, _ := args.Get(0).(*domain.Policy)
	return r0, args.Error(1)
}

func (m *IAMRepository) ListPolicies(ctx context.Context, tenantID uuid.UUID) ([]*domain.Policy, error) {
	args := m.Called(ctx, tenantID)
	r0, _ := args.Get(0).([]*domain.Policy)
	return r0, args.Error(1)
}

func (m *IAMRepository) UpdatePolicy(ctx context.Context, tenantID uuid.UUID, policy *domain.Policy) error {
	args := m.Called(ctx, tenantID, policy)
	return args.Error(0)
}

func (m *IAMRepository) DeletePolicy(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error {
	args := m.Called(ctx, tenantID, id)
	return args.Error(0)
}

func (m *IAMRepository) AttachPolicyToUser(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, policyID uuid.UUID) error {
	args := m.Called(ctx, tenantID, userID, policyID)
	return args.Error(0)
}

func (m *IAMRepository) DetachPolicyFromUser(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, policyID uuid.UUID) error {
	args := m.Called(ctx, tenantID, userID, policyID)
	return args.Error(0)
}

func (m *IAMRepository) GetPoliciesForUser(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID) ([]*domain.Policy, error) {
	args := m.Called(ctx, tenantID, userID)
	r0, _ := args.Get(0).([]*domain.Policy)
	return r0, args.Error(1)
}

func (m *IAMRepository) AttachPolicyToRole(ctx context.Context, tenantID uuid.UUID, roleName string, policyID uuid.UUID) error {
	args := m.Called(ctx, tenantID, roleName, policyID)
	return args.Error(0)
}

func (m *IAMRepository) DetachPolicyFromRole(ctx context.Context, tenantID uuid.UUID, roleName string, policyID uuid.UUID) error {
	args := m.Called(ctx, tenantID, roleName, policyID)
	return args.Error(0)
}

func (m *IAMRepository) GetPoliciesForRole(ctx context.Context, tenantID uuid.UUID, roleName string) ([]*domain.Policy, error) {
	args := m.Called(ctx, tenantID, roleName)
	r0, _ := args.Get(0).([]*domain.Policy)
	return r0, args.Error(1)
}

func (m *IAMRepository) AttachPolicyToServiceAccount(ctx context.Context, tenantID uuid.UUID, saID uuid.UUID, policyID uuid.UUID) error {
	args := m.Called(ctx, tenantID, saID, policyID)
	return args.Error(0)
}

func (m *IAMRepository) DetachPolicyFromServiceAccount(ctx context.Context, tenantID uuid.UUID, saID uuid.UUID, policyID uuid.UUID) error {
	args := m.Called(ctx, tenantID, saID, policyID)
	return args.Error(0)
}

func (m *IAMRepository) GetPoliciesForServiceAccount(ctx context.Context, tenantID uuid.UUID, saID uuid.UUID) ([]*domain.Policy, error) {
	args := m.Called(ctx, tenantID, saID)
	r0, _ := args.Get(0).([]*domain.Policy)
	return r0, args.Error(1)
}

// IAMService is a mock for ports.IAMService
type IAMService struct {
	mock.Mock
}

func (m *IAMService) CreatePolicy(ctx context.Context, policy *domain.Policy) error {
	args := m.Called(ctx, policy)
	return args.Error(0)
}

func (m *IAMService) GetPolicyByID(ctx context.Context, id uuid.UUID) (*domain.Policy, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.Policy)
	return r0, args.Error(1)
}

func (m *IAMService) ListPolicies(ctx context.Context) ([]*domain.Policy, error) {
	args := m.Called(ctx)
	r0, _ := args.Get(0).([]*domain.Policy)
	return r0, args.Error(1)
}

func (m *IAMService) UpdatePolicy(ctx context.Context, policy *domain.Policy) error {
	args := m.Called(ctx, policy)
	return args.Error(0)
}

func (m *IAMService) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *IAMService) AttachPolicyToUser(ctx context.Context, userID uuid.UUID, policyID uuid.UUID) error {
	args := m.Called(ctx, userID, policyID)
	return args.Error(0)
}

func (m *IAMService) DetachPolicyFromUser(ctx context.Context, userID uuid.UUID, policyID uuid.UUID) error {
	args := m.Called(ctx, userID, policyID)
	return args.Error(0)
}

func (m *IAMService) GetPoliciesForUser(ctx context.Context, userID uuid.UUID) ([]*domain.Policy, error) {
	args := m.Called(ctx, userID)
	r0, _ := args.Get(0).([]*domain.Policy)
	return r0, args.Error(1)
}

func (m *IAMService) AttachPolicyToRole(ctx context.Context, roleName string, policyID uuid.UUID) error {
	args := m.Called(ctx, roleName, policyID)
	return args.Error(0)
}

func (m *IAMService) DetachPolicyFromRole(ctx context.Context, roleName string, policyID uuid.UUID) error {
	args := m.Called(ctx, roleName, policyID)
	return args.Error(0)
}

func (m *IAMService) GetPoliciesForRole(ctx context.Context, roleName string) ([]*domain.Policy, error) {
	args := m.Called(ctx, roleName)
	r0, _ := args.Get(0).([]*domain.Policy)
	return r0, args.Error(1)
}

func (m *IAMService) AttachPolicyToServiceAccount(ctx context.Context, saID uuid.UUID, policyID uuid.UUID) error {
	args := m.Called(ctx, saID, policyID)
	return args.Error(0)
}

func (m *IAMService) DetachPolicyFromServiceAccount(ctx context.Context, saID uuid.UUID, policyID uuid.UUID) error {
	args := m.Called(ctx, saID, policyID)
	return args.Error(0)
}

func (m *IAMService) GetPoliciesForServiceAccount(ctx context.Context, saID uuid.UUID) ([]*domain.Policy, error) {
	args := m.Called(ctx, saID)
	r0, _ := args.Get(0).([]*domain.Policy)
	return r0, args.Error(1)
}

// UserRepository is a mock for ports.UserRepository
type UserRepository struct {
	mock.Mock
}

func (m *UserRepository) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.User)
	return r0, args.Error(1)
}

func (m *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	r0, _ := args.Get(0).(*domain.User)
	return r0, args.Error(1)
}

func (m *UserRepository) Update(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *UserRepository) List(ctx context.Context) ([]*domain.User, error) {
	args := m.Called(ctx)
	r0, _ := args.Get(0).([]*domain.User)
	return r0, args.Error(1)
}

// RoleRepository is a mock for ports.RoleRepository
type RoleRepository struct {
	mock.Mock
}

func (m *RoleRepository) CreateRole(ctx context.Context, role *domain.Role) error {
	args := m.Called(ctx, role)
	return args.Error(0)
}

func (m *RoleRepository) GetRoleByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.Role)
	return r0, args.Error(1)
}

func (m *RoleRepository) GetRoleByName(ctx context.Context, name string) (*domain.Role, error) {
	args := m.Called(ctx, name)
	r0, _ := args.Get(0).(*domain.Role)
	return r0, args.Error(1)
}

func (m *RoleRepository) ListRoles(ctx context.Context) ([]*domain.Role, error) {
	args := m.Called(ctx)
	r0, _ := args.Get(0).([]*domain.Role)
	return r0, args.Error(1)
}

func (m *RoleRepository) UpdateRole(ctx context.Context, role *domain.Role) error {
	args := m.Called(ctx, role)
	return args.Error(0)
}

func (m *RoleRepository) DeleteRole(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *RoleRepository) AddPermissionToRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	args := m.Called(ctx, roleID, permission)
	return args.Error(0)
}

func (m *RoleRepository) RemovePermissionFromRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	args := m.Called(ctx, roleID, permission)
	return args.Error(0)
}

func (m *RoleRepository) GetPermissionsForRole(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
	args := m.Called(ctx, roleID)
	r0, _ := args.Get(0).([]domain.Permission)
	return r0, args.Error(1)
}
