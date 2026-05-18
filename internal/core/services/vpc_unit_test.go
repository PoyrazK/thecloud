package services_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestVpcServiceUnit(t *testing.T) {
	repo := new(MockVpcRepo)
	lbRepo := new(MockLBRepo)
	network := new(MockNetworkBackend)
	auditSvc := new(MockAuditService)
	peeringRepo := new(MockVPCPeeringRepo)
	routeTableRepo := new(MockRTRepo)
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := services.NewVpcService(services.VpcServiceParams{
		Repo:           repo,
		LBRepo:         lbRepo,
		PeeringRepo:    peeringRepo,
		RouteTableRepo: routeTableRepo,
		RBACSvc:        rbacSvc,
		Network:        network,
		AuditSvc:       auditSvc,
		Logger:         slog.Default(),
		DefaultCIDR:    "10.0.0.0/16",
	})

	ctx := context.Background()
	userID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)

	t.Run("CreateVPC_AutoCreatesMainRouteTable", func(t *testing.T) {
		network.On("CreateBridge", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		routeTableRepo.On("Create", mock.Anything, mock.MatchedBy(func(rt *domain.RouteTable) bool {
			return rt.IsMain && rt.Name == "main" && len(rt.Routes) == 1 &&
				rt.Routes[0].DestinationCIDR == "10.1.0.0/16" &&
				rt.Routes[0].TargetType == domain.RouteTargetLocal
		})).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "vpc.create", "vpc", mock.Anything, mock.Anything).Return(nil).Once()

		vpc, err := svc.CreateVPC(ctx, "test-vpc", "10.1.0.0/16", "")
		require.NoError(t, err)
		assert.NotNil(t, vpc)
		assert.Equal(t, "10.1.0.0/16", vpc.CIDRBlock)
		repo.AssertExpectations(t)
		routeTableRepo.AssertExpectations(t)
	})

	t.Run("CreateVPC_BridgeFailure", func(t *testing.T) {
		network.On("CreateBridge", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("bridge fail")).Once()

		_, err := svc.CreateVPC(ctx, "fail", "10.2.0.0/16", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bridge")
	})

	t.Run("CreateVPC_RepoFailureRollback", func(t *testing.T) {
		network.On("CreateBridge", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("Create", mock.Anything, mock.Anything).Return(fmt.Errorf("db fail")).Once()
		network.On("DeleteBridge", mock.Anything, mock.Anything).Return(nil).Once()

		_, err := svc.CreateVPC(ctx, "rollback", "10.3.0.0/16", "")
		require.Error(t, err)
	})

	t.Run("CreateVPC_InvalidCIDR", func(t *testing.T) {
		_, err := svc.CreateVPC(ctx, "test-vpc", "invalid", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid CIDR")
	})

	t.Run("CreateVPC_IdempotencyKey_ReturnsExisting", func(t *testing.T) {
		existingVPC := &domain.VPC{
			ID:        uuid.New(),
			UserID:    userID,
			Name:      "test-vpc",
			CIDRBlock: "10.1.0.0/16",
		}
		repo.On("GetByIdempotencyKey", mock.Anything, "my-idempotency-key").Return(existingVPC, nil).Once()

		vpc, err := svc.CreateVPC(ctx, "test-vpc", "10.1.0.0/16", "my-idempotency-key")
		require.NoError(t, err)
		assert.Equal(t, existingVPC.ID, vpc.ID)
		repo.AssertExpectations(t)
	})

	t.Run("CreateVPC_IdempotencyKey_EmptyKey_CreatesNew", func(t *testing.T) {
		network.On("CreateBridge", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		routeTableRepo.On("Create", mock.Anything, mock.MatchedBy(func(rt *domain.RouteTable) bool {
			return rt.IsMain && rt.Name == "main"
		})).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "vpc.create", "vpc", mock.Anything, mock.Anything).Return(nil).Once()

		vpc, err := svc.CreateVPC(ctx, "new-vpc", "10.1.0.0/16", "")
		require.NoError(t, err)
		assert.NotNil(t, vpc)
		repo.AssertExpectations(t)
	})

	t.Run("DeleteVPC_Success", func(t *testing.T) {
		vpcID := uuid.New()
		vpc := &domain.VPC{ID: vpcID, UserID: userID, NetworkID: "br-1"}

		repo.On("GetByID", mock.Anything, vpcID).Return(vpc, nil).Once()
		lbRepo.On("ListAll", mock.Anything).Return([]*domain.LoadBalancer{}, nil).Once()
		peeringRepo.On("ListByVPC", mock.Anything, vpcID).Return([]*domain.VPCPeering{}, nil).Once()
		network.On("DeleteBridge", mock.Anything, "br-1").Return(nil).Once()
		repo.On("Delete", mock.Anything, vpcID).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "vpc.delete", "vpc", vpcID.String(), mock.Anything).Return(nil).Once()

		err := svc.DeleteVPC(ctx, vpcID.String(), false)
		require.NoError(t, err)
	})

	t.Run("DeleteVPC_NotFound", func(t *testing.T) {
		vpcID := uuid.New()
		repo.On("GetByID", mock.Anything, vpcID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		err := svc.DeleteVPC(ctx, vpcID.String(), false)
		require.Error(t, err)
	})

	t.Run("DeleteVPC_WithLoadBalancers", func(t *testing.T) {
		vpcID := uuid.New()
		vpc := &domain.VPC{ID: vpcID, UserID: userID, NetworkID: "br-1"}

		repo.On("GetByID", mock.Anything, vpcID).Return(vpc, nil).Once()
		lbRepo.On("ListAll", mock.Anything).Return([]*domain.LoadBalancer{
			{ID: uuid.New(), VpcID: vpcID},
		}, nil).Once()

		err := svc.DeleteVPC(ctx, vpcID.String(), false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load balancers still exist")
	})

	t.Run("DeleteVPC_WithActivePeering", func(t *testing.T) {
		vpcID := uuid.New()
		vpc := &domain.VPC{ID: vpcID, UserID: userID, NetworkID: "br-1"}

		repo.On("GetByID", mock.Anything, vpcID).Return(vpc, nil).Once()
		lbRepo.On("ListAll", mock.Anything).Return([]*domain.LoadBalancer{}, nil).Once()
		peeringRepo.On("ListByVPC", mock.Anything, vpcID).Return([]*domain.VPCPeering{
			{ID: uuid.New(), Status: domain.PeeringStatusActive},
		}, nil).Once()

		err := svc.DeleteVPC(ctx, vpcID.String(), false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "active peering connections")
	})

	t.Run("DeleteVPC_WithLoadBalancers", func(t *testing.T) {
		vpcID := uuid.New()
		vpc := &domain.VPC{ID: vpcID, UserID: userID}

		repo.On("GetByID", mock.Anything, vpcID).Return(vpc, nil).Once()
		lbRepo.On("ListAll", mock.Anything).Return([]*domain.LoadBalancer{
			{ID: uuid.New(), VpcID: vpcID, Status: domain.LBStatusActive},
		}, nil).Once()

		err := svc.DeleteVPC(ctx, vpcID.String(), false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load balancers still exist")
	})

	t.Run("UpdateVPC_Success", func(t *testing.T) {
		vpcID := uuid.New()
		existingVPC := &domain.VPC{ID: vpcID, UserID: userID, Name: "old-name"}

		repo.On("GetByID", mock.Anything, vpcID).Return(existingVPC, nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(v *domain.VPC) bool {
			return v.ID == vpcID && v.Name == "new-name"
		})).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "vpc.update", "vpc", vpcID.String(), map[string]interface{}{
			"name": "new-name",
		}).Return(nil).Once()

		vpc, err := svc.UpdateVPC(ctx, vpcID.String(), "new-name")
		require.NoError(t, err)
		assert.Equal(t, "new-name", vpc.Name)
		repo.AssertExpectations(t)
	})

	t.Run("UpdateVPC_NotFound", func(t *testing.T) {
		vpcID := uuid.New()
		repo.On("GetByID", mock.Anything, vpcID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		_, err := svc.UpdateVPC(ctx, vpcID.String(), "new-name")
		require.Error(t, err)
	})

	t.Run("UpdateVPC_WhitespaceNameRejected", func(t *testing.T) {
		vpcID := uuid.New()
		existingVPC := &domain.VPC{ID: vpcID, UserID: userID, Name: "old-name"}

		repo.On("GetByID", mock.Anything, vpcID).Return(existingVPC, nil).Once()

		_, err := svc.UpdateVPC(ctx, vpcID.String(), "   ")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "whitespace")
	})
}
