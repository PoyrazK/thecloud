package services_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockPolicyEvaluator struct {
	mock.Mock
}

func (m *MockPolicyEvaluator) Evaluate(ctx context.Context, policies []*domain.Policy, action string, resource string, evalCtx map[string]interface{}) (*domain.EvalResult, error) {
	args := m.Called(ctx, policies, action, resource, evalCtx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EvalResult), args.Error(1)
}

func TestRBACService_Unit(t *testing.T) {
	mockUserRepo := new(MockUserRepo)
	mockRoleRepo := new(MockRoleRepository)
	mockTenantRepo := new(MockTenantRepo)
	mockIAMRepo := new(MockIAMRepository)
	mockEval := new(MockPolicyEvaluator)
	svc := services.NewRBACService(services.RBACServiceParams{
		UserRepo:   mockUserRepo,
		RoleRepo:   mockRoleRepo,
		TenantRepo: mockTenantRepo,
		IAMRepo:    mockIAMRepo,
		Evaluator:  mockEval,
		Logger:     slog.Default(),
	})

	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()

	t.Run("HasPermission_AdminRole", func(t *testing.T) {
		mockTenantRepo.On("GetMembership", mock.Anything, tenantID, userID).Return(&domain.TenantMember{Role: domain.RoleAdmin}, nil).Once()
		mockIAMRepo.On("GetPoliciesForUser", mock.Anything, tenantID, userID).Return([]*domain.Policy{}, nil).Once()
		mockIAMRepo.On("GetPoliciesForRole", mock.Anything, tenantID, domain.RoleAdmin).Return([]*domain.Policy{}, nil).Once()
		mockRoleRepo.On("GetRoleByName", mock.Anything, domain.RoleAdmin).Return(nil, errors.New(errors.NotFound, "not found")).Once() // Fallback to hardcoded

		allowed, err := svc.HasPermission(ctx, userID, tenantID, domain.PermissionInstanceRead, "*")
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("HasPermission_IAMPolicy", func(t *testing.T) {
		policy := &domain.Policy{Name: "custom"}
		mockTenantRepo.On("GetMembership", mock.Anything, tenantID, userID).Return(&domain.TenantMember{Role: domain.RoleViewer}, nil).Once()
		mockIAMRepo.On("GetPoliciesForUser", mock.Anything, tenantID, userID).Return([]*domain.Policy{policy}, nil).Once()
		mockEval.On("Evaluate", mock.Anything, mock.Anything, string(domain.PermissionInstanceLaunch), "*", mock.Anything).
			Return(&domain.EvalResult{Effect: domain.EffectAllow}, nil).Once()

		allowed, err := svc.HasPermission(ctx, userID, tenantID, domain.PermissionInstanceLaunch, "*")
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("Authorize_Denied", func(t *testing.T) {
		mockTenantRepo.On("GetMembership", mock.Anything, tenantID, userID).Return(&domain.TenantMember{Role: domain.RoleViewer}, nil).Once()
		mockIAMRepo.On("GetPoliciesForUser", mock.Anything, tenantID, userID).Return([]*domain.Policy{}, nil).Once()
		mockIAMRepo.On("GetPoliciesForRole", mock.Anything, tenantID, domain.RoleViewer).Return([]*domain.Policy{}, nil).Once()
		mockRoleRepo.On("GetRoleByName", mock.Anything, domain.RoleViewer).Return(&domain.Role{
			Name:        domain.RoleViewer,
			Permissions: []domain.Permission{domain.PermissionInstanceRead},
		}, nil).Once()

		err := svc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceLaunch, "*")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")
	})

	t.Run("HasPermission_UserError", func(t *testing.T) {
		mockTenantRepo.On("GetMembership", mock.Anything, tenantID, userID).Return(nil, fmt.Errorf("db fail")).Once()
		_, err := svc.HasPermission(ctx, userID, tenantID, domain.PermissionInstanceRead, "*")
		require.Error(t, err)
	})

	t.Run("BindRole_Success", func(t *testing.T) {
		bindUserRepo := new(MockUserRepo)
		bindRoleRepo := new(MockRoleRepository)
		bindTenantRepo := new(MockTenantRepo)
		bindIAMRepo := new(MockIAMRepository)
		bindEval := new(MockPolicyEvaluator)
		bindSvc := services.NewRBACService(services.RBACServiceParams{
			UserRepo: bindUserRepo, RoleRepo: bindRoleRepo, TenantRepo: bindTenantRepo,
			IAMRepo: bindIAMRepo, Evaluator: bindEval, Logger: slog.Default(),
		})

		user := &domain.User{ID: userID, Email: "test@test.com", Role: domain.RoleViewer}
		bindRoleRepo.On("GetRoleByName", mock.Anything, domain.RoleAdmin).Return(&domain.Role{Name: domain.RoleAdmin}, nil).Once()
		bindUserRepo.On("GetByEmail", mock.Anything, "test@test.com").Return(user, nil).Once()
		bindUserRepo.On("Update", mock.Anything, mock.MatchedBy(func(u *domain.User) bool {
			return u.Role == domain.RoleAdmin
		})).Return(nil).Once()

		err := bindSvc.BindRole(ctx, "test@test.com", domain.RoleAdmin)
		require.NoError(t, err)
	})

	t.Run("CreateRole_Success", func(t *testing.T) {
		mockRoleRepo := new(MockRoleRepository)
		svc := services.NewRBACService(services.RBACServiceParams{
			UserRepo: mockUserRepo, RoleRepo: mockRoleRepo, TenantRepo: mockTenantRepo,
			IAMRepo: mockIAMRepo, Evaluator: mockEval, Logger: slog.Default(),
		})

		mockRoleRepo.On("GetRoleByName", mock.Anything, "new-role").Return(nil, errors.New(errors.NotFound, "not found")).Once()
		mockRoleRepo.On("CreateRole", mock.Anything, mock.Anything).Return(nil).Once()

		role := &domain.Role{Name: "new-role"}
		err := svc.CreateRole(ctx, role)
		require.NoError(t, err)
		mockRoleRepo.AssertExpectations(t)
	})

	t.Run("CreateRole_Conflict", func(t *testing.T) {
		mockRoleRepo := new(MockRoleRepository)
		svc := services.NewRBACService(services.RBACServiceParams{
			UserRepo: mockUserRepo, RoleRepo: mockRoleRepo, TenantRepo: mockTenantRepo,
			IAMRepo: mockIAMRepo, Evaluator: mockEval, Logger: slog.Default(),
		})

		mockRoleRepo.On("GetRoleByName", mock.Anything, "existing-role").Return(&domain.Role{Name: "existing-role"}, nil).Once()

		role := &domain.Role{Name: "existing-role"}
		err := svc.CreateRole(ctx, role)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("GetRoleByName_Success", func(t *testing.T) {
		mockRoleRepo := new(MockRoleRepository)
		svc := services.NewRBACService(services.RBACServiceParams{
			UserRepo: mockUserRepo, RoleRepo: mockRoleRepo, TenantRepo: mockTenantRepo,
			IAMRepo: mockIAMRepo, Evaluator: mockEval, Logger: slog.Default(),
		})

		expectedRole := &domain.Role{Name: domain.RoleAdmin}
		mockRoleRepo.On("GetRoleByName", mock.Anything, domain.RoleAdmin).Return(expectedRole, nil).Once()

		role, err := svc.GetRoleByName(ctx, domain.RoleAdmin)
		require.NoError(t, err)
		assert.Equal(t, domain.RoleAdmin, role.Name)
		mockRoleRepo.AssertExpectations(t)
	})

	t.Run("GetRoleByID_Success", func(t *testing.T) {
		mockRoleRepo := new(MockRoleRepository)
		svc := services.NewRBACService(services.RBACServiceParams{
			UserRepo: mockUserRepo, RoleRepo: mockRoleRepo, TenantRepo: mockTenantRepo,
			IAMRepo: mockIAMRepo, Evaluator: mockEval, Logger: slog.Default(),
		})

		roleID := uuid.New()
		expectedRole := &domain.Role{Name: domain.RoleAdmin}
		mockRoleRepo.On("GetRoleByID", mock.Anything, roleID).Return(expectedRole, nil).Once()

		role, err := svc.GetRoleByID(ctx, roleID)
		require.NoError(t, err)
		assert.Equal(t, domain.RoleAdmin, role.Name)
		mockRoleRepo.AssertExpectations(t)
	})

	t.Run("ListRoles_Success", func(t *testing.T) {
		mockRoleRepo := new(MockRoleRepository)
		svc := services.NewRBACService(services.RBACServiceParams{
			UserRepo: mockUserRepo, RoleRepo: mockRoleRepo, TenantRepo: mockTenantRepo,
			IAMRepo: mockIAMRepo, Evaluator: mockEval, Logger: slog.Default(),
		})

		roles := []*domain.Role{
			{Name: domain.RoleAdmin},
			{Name: domain.RoleViewer},
		}
		mockRoleRepo.On("ListRoles", mock.Anything).Return(roles, nil).Once()

		res, err := svc.ListRoles(ctx)
		require.NoError(t, err)
		assert.Len(t, res, 2)
		mockRoleRepo.AssertExpectations(t)
	})

	t.Run("UpdateRole_Success", func(t *testing.T) {
		mockRoleRepo := new(MockRoleRepository)
		svc := services.NewRBACService(services.RBACServiceParams{
			UserRepo: mockUserRepo, RoleRepo: mockRoleRepo, TenantRepo: mockTenantRepo,
			IAMRepo: mockIAMRepo, Evaluator: mockEval, Logger: slog.Default(),
		})

		mockRoleRepo.On("UpdateRole", mock.Anything, mock.Anything).Return(nil).Once()

		role := &domain.Role{Name: "updated-role"}
		err := svc.UpdateRole(ctx, role)
		require.NoError(t, err)
		mockRoleRepo.AssertExpectations(t)
	})

	t.Run("DeleteRole_Success", func(t *testing.T) {
		mockRoleRepo := new(MockRoleRepository)
		svc := services.NewRBACService(services.RBACServiceParams{
			UserRepo: mockUserRepo, RoleRepo: mockRoleRepo, TenantRepo: mockTenantRepo,
			IAMRepo: mockIAMRepo, Evaluator: mockEval, Logger: slog.Default(),
		})

		roleID := uuid.New()
		mockRoleRepo.On("DeleteRole", mock.Anything, roleID).Return(nil).Once()

		err := svc.DeleteRole(ctx, roleID)
		require.NoError(t, err)
		mockRoleRepo.AssertExpectations(t)
	})

	t.Run("AddPermissionToRole_Success", func(t *testing.T) {
		mockRoleRepo := new(MockRoleRepository)
		svc := services.NewRBACService(services.RBACServiceParams{
			UserRepo: mockUserRepo, RoleRepo: mockRoleRepo, TenantRepo: mockTenantRepo,
			IAMRepo: mockIAMRepo, Evaluator: mockEval, Logger: slog.Default(),
		})

		roleID := uuid.New()
		mockRoleRepo.On("AddPermissionToRole", mock.Anything, roleID, domain.PermissionInstanceRead).Return(nil).Once()

		err := svc.AddPermissionToRole(ctx, roleID, domain.PermissionInstanceRead)
		require.NoError(t, err)
		mockRoleRepo.AssertExpectations(t)
	})

	t.Run("RemovePermissionFromRole_Success", func(t *testing.T) {
		mockRoleRepo := new(MockRoleRepository)
		svc := services.NewRBACService(services.RBACServiceParams{
			UserRepo: mockUserRepo, RoleRepo: mockRoleRepo, TenantRepo: mockTenantRepo,
			IAMRepo: mockIAMRepo, Evaluator: mockEval, Logger: slog.Default(),
		})

		roleID := uuid.New()
		mockRoleRepo.On("RemovePermissionFromRole", mock.Anything, roleID, domain.PermissionInstanceRead).Return(nil).Once()

		err := svc.RemovePermissionFromRole(ctx, roleID, domain.PermissionInstanceRead)
		require.NoError(t, err)
		mockRoleRepo.AssertExpectations(t)
	})

	t.Run("ListRoleBindings_Success", func(t *testing.T) {
		mockUserRepo := new(MockUserRepo)
		svc := services.NewRBACService(services.RBACServiceParams{
			UserRepo: mockUserRepo, RoleRepo: mockRoleRepo, TenantRepo: mockTenantRepo,
			IAMRepo: mockIAMRepo, Evaluator: mockEval, Logger: slog.Default(),
		})

		users := []*domain.User{
			{ID: uuid.New(), Role: domain.RoleAdmin},
			{ID: uuid.New(), Role: domain.RoleViewer},
		}
		mockUserRepo.On("List", mock.Anything).Return(users, nil).Once()

		res, err := svc.ListRoleBindings(ctx)
		require.NoError(t, err)
		assert.Len(t, res, 2)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("EvaluatePolicy_Success", func(t *testing.T) {
		mockUserRepo := new(MockUserRepo)
		mockIAMRepo := new(MockIAMRepository)
		mockEval := new(MockPolicyEvaluator)
		svc := services.NewRBACService(services.RBACServiceParams{
			UserRepo: mockUserRepo, RoleRepo: mockRoleRepo, TenantRepo: mockTenantRepo,
			IAMRepo: mockIAMRepo, Evaluator: mockEval, Logger: slog.Default(),
		})

		uid := uuid.New()
		policy := &domain.Policy{Name: "allow-read"}
		mockUserRepo.On("GetByID", mock.Anything, uid).Return(&domain.User{ID: uid}, nil).Once()
		mockIAMRepo.On("GetPoliciesForUser", mock.Anything, uuid.Nil, uid).Return([]*domain.Policy{policy}, nil).Once()
		mockEval.On("Evaluate", mock.Anything, mock.Anything, "read", "resource1", mock.Anything).Return(&domain.EvalResult{Effect: domain.EffectAllow}, nil).Once()

		allowed, err := svc.EvaluatePolicy(ctx, uid, "read", "resource1", nil)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("EvaluatePolicy_NoPolicies", func(t *testing.T) {
		mockUserRepo := new(MockUserRepo)
		mockIAMRepo := new(MockIAMRepository)
		svc := services.NewRBACService(services.RBACServiceParams{
			UserRepo: mockUserRepo, RoleRepo: mockRoleRepo, TenantRepo: mockTenantRepo,
			IAMRepo: mockIAMRepo, Evaluator: mockEval, Logger: slog.Default(),
		})

		uid := uuid.New()
		mockUserRepo.On("GetByID", mock.Anything, uid).Return(&domain.User{ID: uid}, nil).Once()
		mockIAMRepo.On("GetPoliciesForUser", mock.Anything, uuid.Nil, uid).Return([]*domain.Policy{}, nil).Once()

		allowed, err := svc.EvaluatePolicy(ctx, uid, "read", "resource1", nil)
		require.NoError(t, err)
		assert.False(t, allowed)
	})

	t.Run("EvaluatePolicy_IAMNotInitialized", func(t *testing.T) {
		svc := services.NewRBACService(services.RBACServiceParams{
			UserRepo: mockUserRepo, RoleRepo: mockRoleRepo, TenantRepo: mockTenantRepo,
			IAMRepo: nil, Evaluator: nil, Logger: slog.Default(),
		})

		_, err := svc.EvaluatePolicy(ctx, uuid.New(), "read", "resource1", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not initialized")
	})
}
