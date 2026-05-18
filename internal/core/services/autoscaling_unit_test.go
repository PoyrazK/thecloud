package services_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAutoScalingService_Unit(t *testing.T) {
	t.Run("CRUD", testAutoScalingServiceUnitCRUD)
	t.Run("RBACErrors", testAutoScalingServiceUnitRbacErrors)
	t.Run("RepoErrors", testAutoScalingServiceUnitRepoErrors)
	t.Run("ValidationErrors", testAutoScalingServiceUnitValidationErrors)
}

func testAutoScalingServiceUnitCRUD(t *testing.T) {
	repo := new(MockAutoScalingRepo)
	rbacSvc := new(MockRBACService)
	vpcRepo := new(MockVpcRepo)
	auditSvc := new(MockAuditService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	defer mock.AssertExpectationsForObjects(t, repo, rbacSvc, vpcRepo, auditSvc)

	svc := services.NewAutoScalingService(repo, rbacSvc, vpcRepo, auditSvc, slog.Default())
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()
	ctx = appcontext.WithTenantID(ctx, tenantID)
	ctx = appcontext.WithUserID(ctx, userID)

	vpcID := uuid.New()
	groupID := uuid.New()
	policyID := uuid.New()

	t.Run("CreateGroup", func(t *testing.T) {
		vpcRepo.On("GetByID", mock.Anything, vpcID).Return(&domain.VPC{ID: vpcID}, nil).Once()
		repo.On("CountGroupsByVPC", mock.Anything, vpcID).Return(0, nil).Once()
		repo.On("CreateGroup", mock.Anything, mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "asg.group_create", "scaling_group", mock.Anything, mock.Anything).Return(nil).Once()

		group, err := svc.CreateGroup(ctx, ports.CreateScalingGroupParams{
			Name:         "test-group",
			VpcID:        vpcID,
			Image:        "ami-123",
			MinInstances: 1,
			MaxInstances: 5,
			DesiredCount: 2,
		})
		require.NoError(t, err)
		assert.NotNil(t, group)
		assert.Equal(t, "test-group", group.Name)
	})

	t.Run("GetGroup", func(t *testing.T) {
		repo.On("GetGroupByID", mock.Anything, groupID).Return(&domain.ScalingGroup{ID: groupID, TenantID: tenantID}, nil).Once()

		group, err := svc.GetGroup(ctx, groupID)
		require.NoError(t, err)
		assert.Equal(t, groupID, group.ID)
	})

	t.Run("ListGroups", func(t *testing.T) {
		repo.On("ListGroups", mock.Anything).Return([]*domain.ScalingGroup{{ID: groupID}}, nil).Once()

		groups, err := svc.ListGroups(ctx)
		require.NoError(t, err)
		assert.Len(t, groups, 1)
	})

	t.Run("DeleteGroup", func(t *testing.T) {
		repo.On("GetGroupByID", mock.Anything, groupID).Return(&domain.ScalingGroup{ID: groupID, UserID: userID, TenantID: tenantID}, nil).Once()
		repo.On("UpdateGroup", mock.Anything, mock.MatchedBy(func(g *domain.ScalingGroup) bool {
			return g.ID == groupID && g.Status == "DELETING"
		})).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "asg.group_delete", "scaling_group", groupID.String(), mock.Anything).Return(nil).Once()

		err := svc.DeleteGroup(ctx, groupID)
		require.NoError(t, err)
	})

	t.Run("SetDesiredCapacity", func(t *testing.T) {
		repo.On("GetGroupByID", mock.Anything, groupID).Return(&domain.ScalingGroup{ID: groupID, MinInstances: 1, MaxInstances: 10, TenantID: tenantID}, nil).Once()
		repo.On("UpdateGroup", mock.Anything, mock.MatchedBy(func(g *domain.ScalingGroup) bool {
			return g.DesiredCount == 5
		})).Return(nil).Once()

		err := svc.SetDesiredCapacity(ctx, groupID, 5)
		require.NoError(t, err)
	})

	t.Run("CreatePolicy", func(t *testing.T) {
		repo.On("GetGroupByID", mock.Anything, groupID).Return(&domain.ScalingGroup{ID: groupID, TenantID: tenantID}, nil).Once()
		repo.On("CreatePolicy", mock.Anything, mock.Anything).Return(nil).Once()

		policy, err := svc.CreatePolicy(ctx, ports.CreateScalingPolicyParams{
			GroupID:     groupID,
			Name:        "cpu-high",
			MetricType:  "cpu",
			TargetValue: 70.0,
			ScaleOut:    1,
			ScaleIn:     1,
			CooldownSec: 60,
		})
		require.NoError(t, err)
		assert.NotNil(t, policy)
	})

	t.Run("DeletePolicy", func(t *testing.T) {
		repo.On("DeletePolicy", mock.Anything, policyID).Return(nil).Once()

		err := svc.DeletePolicy(ctx, policyID)
		require.NoError(t, err)
	})

}

func testAutoScalingServiceUnitRbacErrors(t *testing.T) {
	repo := new(MockAutoScalingRepo)
	rbacSvc := new(MockRBACService)
	vpcRepo := new(MockVpcRepo)
	auditSvc := new(MockAuditService)

	svc := services.NewAutoScalingService(repo, rbacSvc, vpcRepo, auditSvc, slog.Default())
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()
	ctx = appcontext.WithTenantID(ctx, tenantID)
	ctx = appcontext.WithUserID(ctx, userID)

	vpcID := uuid.New()
	groupID := uuid.New()
	policyID := uuid.New()

	type rbacCase struct {
		name       string
		permission domain.Permission
		resourceID string
		invoke     func() error
	}

	cases := []rbacCase{
		{
			name:       "CreateGroup_Unauthorized",
			permission: domain.PermissionAsgCreate,
			resourceID: "*",
			invoke: func() error {
				_, err := svc.CreateGroup(ctx, ports.CreateScalingGroupParams{Name: "g", VpcID: vpcID})
				return err
			},
		},
		{
			name:       "GetGroup_Unauthorized",
			permission: domain.PermissionAsgRead,
			resourceID: groupID.String(),
			invoke: func() error {
				_, err := svc.GetGroup(ctx, groupID)
				return err
			},
		},
		{
			name:       "ListGroups_Unauthorized",
			permission: domain.PermissionAsgRead,
			resourceID: "*",
			invoke: func() error {
				_, err := svc.ListGroups(ctx)
				return err
			},
		},
		{
			name:       "DeleteGroup_Unauthorized",
			permission: domain.PermissionAsgDelete,
			resourceID: groupID.String(),
			invoke: func() error {
				return svc.DeleteGroup(ctx, groupID)
			},
		},
		{
			name:       "SetDesiredCapacity_Unauthorized",
			permission: domain.PermissionAsgUpdate,
			resourceID: groupID.String(),
			invoke: func() error {
				return svc.SetDesiredCapacity(ctx, groupID, 3)
			},
		},
		{
			name:       "CreatePolicy_Unauthorized",
			permission: domain.PermissionAsgUpdate,
			resourceID: "*",
			invoke: func() error {
				_, err := svc.CreatePolicy(ctx, ports.CreateScalingPolicyParams{GroupID: groupID, Name: "p", CooldownSec: 60})
				return err
			},
		},
		{
			name:       "DeletePolicy_Unauthorized",
			permission: domain.PermissionAsgUpdate,
			resourceID: policyID.String(),
			invoke: func() error {
				return svc.DeletePolicy(ctx, policyID)
			},
		},
	}

	authErr := errors.New(errors.Forbidden, "permission denied")
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rbacSvc.On("Authorize", mock.Anything, userID, tenantID, c.permission, c.resourceID).Return(authErr).Once()
			err := c.invoke()
			require.Error(t, err)
			assert.True(t, errors.Is(err, errors.Forbidden))
		})
	}
}

func testAutoScalingServiceUnitRepoErrors(t *testing.T) {
	repo := new(MockAutoScalingRepo)
	rbacSvc := new(MockRBACService)
	vpcRepo := new(MockVpcRepo)
	auditSvc := new(MockAuditService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	svc := services.NewAutoScalingService(repo, rbacSvc, vpcRepo, auditSvc, slog.Default())
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()
	ctx = appcontext.WithTenantID(ctx, tenantID)
	ctx = appcontext.WithUserID(ctx, userID)

	vpcID := uuid.New()
	groupID := uuid.New()
	policyID := uuid.New()

	t.Run("CreateGroup_VPCNotFound", func(t *testing.T) {
		vpcRepo.On("GetByID", mock.Anything, vpcID).Return(nil, errors.New(errors.NotFound, "vpc not found")).Once()

		_, err := svc.CreateGroup(ctx, ports.CreateScalingGroupParams{Name: "g", VpcID: vpcID})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("CreateGroup_MaxGroupsExceeded", func(t *testing.T) {
		vpcRepo.On("GetByID", mock.Anything, vpcID).Return(&domain.VPC{ID: vpcID}, nil).Once()
		repo.On("CountGroupsByVPC", mock.Anything, vpcID).Return(5, nil).Once()

		_, err := svc.CreateGroup(ctx, ports.CreateScalingGroupParams{Name: "g", VpcID: vpcID})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "scaling groups")
	})

	t.Run("CreateGroup_RepoError", func(t *testing.T) {
		vpcRepo.On("GetByID", mock.Anything, vpcID).Return(&domain.VPC{ID: vpcID}, nil).Once()
		repo.On("CountGroupsByVPC", mock.Anything, vpcID).Return(0, nil).Once()
		repo.On("CreateGroup", mock.Anything, mock.Anything).Return(fmt.Errorf("db error")).Once()

		_, err := svc.CreateGroup(ctx, ports.CreateScalingGroupParams{Name: "g", VpcID: vpcID})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("CreateGroup_IdempotencyKeyReturnsExisting", func(t *testing.T) {
		// Honest retry: same key + identical params → returns the existing record.
		existing := &domain.ScalingGroup{
			ID:           uuid.New(),
			Name:         "existing-group",
			VpcID:        vpcID,
			Image:        "nginx",
			MinInstances: 1,
			MaxInstances: 3,
			DesiredCount: 2,
		}
		repo.On("GetGroupByIdempotencyKey", mock.Anything, "idem-key").Return(existing, nil).Once()

		group, err := svc.CreateGroup(ctx, ports.CreateScalingGroupParams{
			Name:           existing.Name,
			VpcID:          existing.VpcID,
			Image:          existing.Image,
			MinInstances:   existing.MinInstances,
			MaxInstances:   existing.MaxInstances,
			DesiredCount:   existing.DesiredCount,
			IdempotencyKey: "idem-key",
		})
		require.NoError(t, err)
		assert.Equal(t, existing.ID, group.ID)
		assert.Equal(t, "existing-group", group.Name)
	})

	t.Run("CreateGroup_IdempotencyKeyReusedWithDifferentParams", func(t *testing.T) {
		// Key reuse with different params now fails with Conflict so the caller
		// learns about the bug instead of silently getting a different group back.
		existing := &domain.ScalingGroup{ID: uuid.New(), Name: "existing-group", VpcID: vpcID}
		repo.On("GetGroupByIdempotencyKey", mock.Anything, "idem-key-2").Return(existing, nil).Once()

		_, err := svc.CreateGroup(ctx, ports.CreateScalingGroupParams{
			Name:           "different-name",
			VpcID:          vpcID,
			IdempotencyKey: "idem-key-2",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "idempotency_key already used")
	})

	t.Run("GetGroup_NotFound", func(t *testing.T) {
		repo.On("GetGroupByID", mock.Anything, groupID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		_, err := svc.GetGroup(ctx, groupID)
		require.Error(t, err)
	})

	t.Run("GetGroup_RepoError", func(t *testing.T) {
		repo.On("GetGroupByID", mock.Anything, groupID).Return(nil, fmt.Errorf("db error")).Once()

		_, err := svc.GetGroup(ctx, groupID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("ListGroups_RepoError", func(t *testing.T) {
		repo.On("ListGroups", mock.Anything).Return(nil, fmt.Errorf("db error")).Once()

		_, err := svc.ListGroups(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("DeleteGroup_NotFound", func(t *testing.T) {
		repo.On("GetGroupByID", mock.Anything, groupID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		err := svc.DeleteGroup(ctx, groupID)
		require.Error(t, err)
	})

	t.Run("DeleteGroup_UpdateError", func(t *testing.T) {
		repo.On("GetGroupByID", mock.Anything, groupID).Return(&domain.ScalingGroup{ID: groupID, UserID: userID, TenantID: tenantID}, nil).Once()
		repo.On("UpdateGroup", mock.Anything, mock.Anything).Return(fmt.Errorf("db error")).Once()

		err := svc.DeleteGroup(ctx, groupID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("SetDesiredCapacity_GroupNotFound", func(t *testing.T) {
		repo.On("GetGroupByID", mock.Anything, groupID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		err := svc.SetDesiredCapacity(ctx, groupID, 3)
		require.Error(t, err)
	})

	t.Run("SetDesiredCapacity_UpdateError", func(t *testing.T) {
		repo.On("GetGroupByID", mock.Anything, groupID).Return(&domain.ScalingGroup{ID: groupID, MinInstances: 1, MaxInstances: 10, TenantID: tenantID}, nil).Once()
		repo.On("UpdateGroup", mock.Anything, mock.Anything).Return(fmt.Errorf("db error")).Once()

		err := svc.SetDesiredCapacity(ctx, groupID, 5)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("CreatePolicy_GroupNotFound", func(t *testing.T) {
		repo.On("GetGroupByID", mock.Anything, groupID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		_, err := svc.CreatePolicy(ctx, ports.CreateScalingPolicyParams{GroupID: groupID, Name: "p", CooldownSec: 60})
		require.Error(t, err)
	})

	t.Run("CreatePolicy_RepoError", func(t *testing.T) {
		repo.On("GetGroupByID", mock.Anything, groupID).Return(&domain.ScalingGroup{ID: groupID, TenantID: tenantID}, nil).Once()
		repo.On("CreatePolicy", mock.Anything, mock.Anything).Return(fmt.Errorf("db error")).Once()

		_, err := svc.CreatePolicy(ctx, ports.CreateScalingPolicyParams{GroupID: groupID, Name: "p", CooldownSec: 60})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("DeletePolicy_NotFound", func(t *testing.T) {
		repo.On("DeletePolicy", mock.Anything, policyID).Return(errors.New(errors.NotFound, "not found")).Once()

		err := svc.DeletePolicy(ctx, policyID)
		require.Error(t, err)
	})

	t.Run("DeletePolicy_RepoError", func(t *testing.T) {
		repo.On("DeletePolicy", mock.Anything, policyID).Return(fmt.Errorf("db error")).Once()

		err := svc.DeletePolicy(ctx, policyID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})
}

func testAutoScalingServiceUnitValidationErrors(t *testing.T) {
	repo := new(MockAutoScalingRepo)
	rbacSvc := new(MockRBACService)
	vpcRepo := new(MockVpcRepo)
	auditSvc := new(MockAuditService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	svc := services.NewAutoScalingService(repo, rbacSvc, vpcRepo, auditSvc, slog.Default())
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()
	ctx = appcontext.WithTenantID(ctx, tenantID)
	ctx = appcontext.WithUserID(ctx, userID)

	vpcID := uuid.New()

	t.Run("CreateGroup_MaxInstancesHardLimit", func(t *testing.T) {
		_, err := svc.CreateGroup(ctx, ports.CreateScalingGroupParams{
			Name:         "g",
			VpcID:        vpcID,
			MinInstances: 1,
			MaxInstances: domain.MaxInstancesHardLimit + 1,
			DesiredCount: 1,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max_instances cannot exceed")
	})

	t.Run("CreateGroup_MinInstancesNegative", func(t *testing.T) {
		_, err := svc.CreateGroup(ctx, ports.CreateScalingGroupParams{
			Name:         "g",
			VpcID:        vpcID,
			MinInstances: -1,
			MaxInstances: 5,
			DesiredCount: 1,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be negative")
	})

	t.Run("CreateGroup_MinGreaterThanMax", func(t *testing.T) {
		_, err := svc.CreateGroup(ctx, ports.CreateScalingGroupParams{
			Name:         "g",
			VpcID:        vpcID,
			MinInstances: 10,
			MaxInstances: 5,
			DesiredCount: 7,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be greater than max_instances")
	})

	t.Run("CreateGroup_DesiredBelowMin", func(t *testing.T) {
		_, err := svc.CreateGroup(ctx, ports.CreateScalingGroupParams{
			Name:         "g",
			VpcID:        vpcID,
			MinInstances: 3,
			MaxInstances: 10,
			DesiredCount: 1,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "desired_count must be between")
	})

	t.Run("CreateGroup_DesiredAboveMax", func(t *testing.T) {
		_, err := svc.CreateGroup(ctx, ports.CreateScalingGroupParams{
			Name:         "g",
			VpcID:        vpcID,
			MinInstances: 1,
			MaxInstances: 5,
			DesiredCount: 10,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "desired_count must be between")
	})

	t.Run("SetDesiredCapacity_OutOfRange", func(t *testing.T) {
		groupID := uuid.New()
		repo.On("GetGroupByID", mock.Anything, groupID).Return(&domain.ScalingGroup{ID: groupID, MinInstances: 2, MaxInstances: 5, TenantID: tenantID}, nil).Once()

		err := svc.SetDesiredCapacity(ctx, groupID, 10)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "desired must be between 2 and 5")
	})

	t.Run("CreatePolicy_CooldownBelowMinimum", func(t *testing.T) {
		groupID := uuid.New()
		repo.On("GetGroupByID", mock.Anything, groupID).Return(&domain.ScalingGroup{ID: groupID, TenantID: tenantID}, nil).Once()

		_, err := svc.CreatePolicy(ctx, ports.CreateScalingPolicyParams{
			GroupID:     groupID,
			Name:        "cpu-high",
			MetricType:  "cpu",
			CooldownSec: domain.MinCooldownSeconds - 1,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cooldown must be at least")
	})
}
