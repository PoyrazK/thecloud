package services_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSecurityGroupService_Unit(t *testing.T) {
	mockRepo := new(MockSecurityGroupRepo)
	mockVpcRepo := new(MockVpcRepo)
	mockNetwork := new(MockNetworkBackend)
	mockAuditSvc := new(MockAuditService)
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := services.NewSecurityGroupService(mockRepo, rbacSvc, mockVpcRepo, mockNetwork, mockAuditSvc, slog.Default())

	ctx := context.Background()
	userID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	vpcID := uuid.New()

	t.Run("CreateGroup", func(t *testing.T) {
		mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "security_group.create", "security_group", mock.Anything, mock.Anything).Return(nil).Once()

		sg, err := svc.CreateGroup(ctx, vpcID, "web-sg", "allow http")
		require.NoError(t, err)
		assert.NotNil(t, sg)
		assert.Equal(t, "web-sg", sg.Name)
		assert.Len(t, sg.Rules, 2) // Default ARP rules
		mockRepo.AssertExpectations(t)
	})

	t.Run("AddRule", func(t *testing.T) {
		sgID := uuid.New()
		sg := &domain.SecurityGroup{ID: sgID, UserID: userID, VPCID: vpcID}
		rule := domain.SecurityRule{Protocol: "tcp", PortMin: 80, PortMax: 80, Direction: domain.RuleIngress}

		mockRepo.On("GetByID", mock.Anything, sgID).Return(sg, nil).Once()
		mockRepo.On("AddRule", mock.Anything, mock.Anything).Return(nil).Once()
		mockVpcRepo.On("GetByID", mock.Anything, vpcID).Return(&domain.VPC{ID: vpcID, NetworkID: "net-1"}, nil).Maybe()
		mockNetwork.On("AddFlowRule", mock.Anything, "net-1", mock.Anything).Return(nil).Maybe()
		mockAuditSvc.On("Log", mock.Anything, userID, "security_group.add_rule", "security_group", sgID.String(), mock.Anything).Return(nil).Maybe()

		res, err := svc.AddRule(ctx, sgID.String(), rule)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("AddRule_ByName", func(t *testing.T) {
		sgID := uuid.New()
		sg := &domain.SecurityGroup{ID: sgID, UserID: userID, VPCID: vpcID, Name: "my-sg"}
		rule := domain.SecurityRule{Protocol: "tcp", PortMin: 80, PortMax: 80, Direction: domain.RuleIngress}

		mockRepo.On("GetByNameAcrossVPCs", mock.Anything, "my-sg").Return(sg, nil).Once()
		mockRepo.On("GetByID", mock.Anything, sgID).Return(sg, nil).Once()
		mockRepo.On("AddRule", mock.Anything, mock.Anything).Return(nil).Once()
		mockVpcRepo.On("GetByID", mock.Anything, vpcID).Return(&domain.VPC{ID: vpcID, NetworkID: "net-1"}, nil).Maybe()
		mockNetwork.On("AddFlowRule", mock.Anything, "net-1", mock.Anything).Return(nil).Maybe()
		mockAuditSvc.On("Log", mock.Anything, userID, "security_group.add_rule", "security_group", sgID.String(), mock.Anything).Return(nil).Maybe()

		res, err := svc.AddRule(ctx, "my-sg", rule)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("GetGroup_ByID", func(t *testing.T) {
		sgID := uuid.New()
		sg := &domain.SecurityGroup{ID: sgID, UserID: userID, VPCID: vpcID, Name: "test-sg"}

		mockRepo.On("GetByID", mock.Anything, sgID).Return(sg, nil).Once()

		res, err := svc.GetGroup(ctx, sgID.String(), vpcID)
		require.NoError(t, err)
		assert.Equal(t, sgID, res.ID)
		mockRepo.AssertExpectations(t)
	})

	t.Run("GetGroup_ByName", func(t *testing.T) {
		sg := &domain.SecurityGroup{ID: uuid.New(), UserID: userID, VPCID: vpcID, Name: "test-sg"}

		mockRepo.On("GetByName", mock.Anything, vpcID, "test-sg").Return(sg, nil).Once()

		res, err := svc.GetGroup(ctx, "test-sg", vpcID)
		require.NoError(t, err)
		assert.Equal(t, "test-sg", res.Name)
		mockRepo.AssertExpectations(t)
	})

	t.Run("ListGroups", func(t *testing.T) {
		groups := []*domain.SecurityGroup{
			{ID: uuid.New(), Name: "sg-1"},
			{ID: uuid.New(), Name: "sg-2"},
		}

		mockRepo.On("ListByVPC", mock.Anything, vpcID).Return(groups, nil).Once()

		res, err := svc.ListGroups(ctx, vpcID)
		require.NoError(t, err)
		assert.Len(t, res, 2)
		mockRepo.AssertExpectations(t)
	})

	t.Run("DeleteGroup", func(t *testing.T) {
		sgID := uuid.New()

		mockRepo.On("Delete", mock.Anything, sgID).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "security_group.delete", "security_group", sgID.String(), mock.Anything).Return(nil).Once()

		err := svc.DeleteGroup(ctx, sgID)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
		mockAuditSvc.AssertExpectations(t)
	})

	t.Run("DeleteGroup_RepoError", func(t *testing.T) {
		sgID := uuid.New()

		mockRepo.On("Delete", mock.Anything, sgID).Return(assert.AnError).Once()

		err := svc.DeleteGroup(ctx, sgID)
		require.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("RemoveRule", func(t *testing.T) {
		sgID := uuid.New()
		ruleID := uuid.New()
		sg := &domain.SecurityGroup{ID: sgID, UserID: userID, VPCID: vpcID}
		rule := &domain.SecurityRule{ID: ruleID, GroupID: sgID, Protocol: "tcp"}

		mockRepo.On("GetRuleByID", mock.Anything, ruleID).Return(rule, nil).Once()
		mockRepo.On("GetByID", mock.Anything, sgID).Return(sg, nil).Once()
		mockVpcRepo.On("GetByID", mock.Anything, vpcID).Return(&domain.VPC{ID: vpcID, NetworkID: "net-1"}, nil).Maybe()
		mockNetwork.On("DeleteFlowRule", mock.Anything, "net-1", mock.Anything).Return(nil).Maybe()
		mockRepo.On("DeleteRule", mock.Anything, ruleID).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "security_group.remove_rule", "security_group", sgID.String(), mock.Anything).Return(nil).Maybe()

		err := svc.RemoveRule(ctx, ruleID)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("RemoveRule_NotFound", func(t *testing.T) {
		ruleID := uuid.New()

		mockRepo.On("GetRuleByID", mock.Anything, ruleID).Return(nil, assert.AnError).Once()

		err := svc.RemoveRule(ctx, ruleID)
		require.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("AttachToInstance", func(t *testing.T) {
		sgID := uuid.New()
		instanceID := uuid.New()
		sg := &domain.SecurityGroup{
			ID:     sgID,
			UserID: userID,
			VPCID:  vpcID,
			Rules: []domain.SecurityRule{
				{ID: uuid.New(), GroupID: sgID, Protocol: "tcp", PortMin: 80, PortMax: 80},
			},
		}

		mockRepo.On("AddInstanceToGroup", mock.Anything, instanceID, sgID).Return(nil).Once()
		mockRepo.On("GetByID", mock.Anything, sgID).Return(sg, nil).Once()
		mockVpcRepo.On("GetByID", mock.Anything, vpcID).Return(&domain.VPC{ID: vpcID, NetworkID: "net-1"}, nil).Maybe()
		mockNetwork.On("AddFlowRule", mock.Anything, "net-1", mock.Anything).Return(nil).Maybe()
		mockAuditSvc.On("Log", mock.Anything, userID, "security_group.attach", "instance", instanceID.String(), mock.Anything).Return(nil).Maybe()

		err := svc.AttachToInstance(ctx, instanceID, sgID)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("AttachToInstance_RepoError", func(t *testing.T) {
		sgID := uuid.New()
		instanceID := uuid.New()

		mockRepo.On("AddInstanceToGroup", mock.Anything, instanceID, sgID).Return(assert.AnError).Once()

		err := svc.AttachToInstance(ctx, instanceID, sgID)
		require.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("DetachFromInstance", func(t *testing.T) {
		sgID := uuid.New()
		instanceID := uuid.New()
		sg := &domain.SecurityGroup{
			ID:     sgID,
			UserID: userID,
			VPCID:  vpcID,
			Rules: []domain.SecurityRule{
				{ID: uuid.New(), GroupID: sgID, Protocol: "tcp", PortMin: 80, PortMax: 80},
			},
		}

		mockRepo.On("RemoveInstanceFromGroup", mock.Anything, instanceID, sgID).Return(nil).Once()
		mockRepo.On("GetByID", mock.Anything, sgID).Return(sg, nil).Once()
		mockVpcRepo.On("GetByID", mock.Anything, vpcID).Return(&domain.VPC{ID: vpcID, NetworkID: "net-1"}, nil).Maybe()
		mockNetwork.On("DeleteFlowRule", mock.Anything, "net-1", mock.Anything).Return(nil).Maybe()
		mockAuditSvc.On("Log", mock.Anything, userID, "security_group.detach", "instance", instanceID.String(), mock.Anything).Return(nil).Maybe()

		err := svc.DetachFromInstance(ctx, instanceID, sgID)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("DetachFromInstance_RepoError", func(t *testing.T) {
		sgID := uuid.New()
		instanceID := uuid.New()

		mockRepo.On("RemoveInstanceFromGroup", mock.Anything, instanceID, sgID).Return(assert.AnError).Once()

		err := svc.DetachFromInstance(ctx, instanceID, sgID)
		require.Error(t, err)
		mockRepo.AssertExpectations(t)
	})
}
