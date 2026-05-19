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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTransitGatewayService_Unit(t *testing.T) {
	mockRepo := new(MockTransitGatewayRepo)
	mockVpcRepo := new(MockVpcRepo)
	mockSubnetRepo := new(MockSubnetRepo)
	mockRTRepo := new(MockRTRepo)
	mockNetwork := new(MockNetworkBackend)
	mockRBACSvc := new(MockRBACService)
	mockAuditSvc := new(MockAuditService)
	svc := services.NewTransitGatewayService(services.TransitGatewayServiceParams{
		Repo:       mockRepo,
		VpcRepo:    mockVpcRepo,
		SubnetRepo: mockSubnetRepo,
		RTRepo:     mockRTRepo,
		Network:    mockNetwork,
		RBACSvc:    mockRBACSvc,
		AuditSvc:   mockAuditSvc,
		Logger:     slog.Default(),
	})

	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	ctx = appcontext.WithTenantID(ctx, tenantID)

	t.Run("CreateTransitGateway_Success", func(t *testing.T) {
		name := "my-tgw"

		mockRBACSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionVpcCreate, "*").Return(nil).Once()
		mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(tgArg *domain.TransitGateway) bool {
			return tgArg.Name == name && tgArg.OwnerTenantID == tenantID
		})).Return(nil).Once()
		mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(tgArg *domain.TransitGateway) bool {
			return tgArg.Status == domain.TransitGatewayStatusAvailable
		})).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "transit_gateway.create", "transit_gateway", mock.Anything, mock.Anything).Return(nil).Once()

		result, err := svc.CreateTransitGateway(ctx, name)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, name, result.Name)
		assert.Equal(t, tenantID, result.OwnerTenantID)
		assert.Equal(t, domain.TransitGatewayStatusAvailable, result.Status)
		mockRepo.AssertExpectations(t)
	})

	t.Run("CreateTransitGateway_NameRequired", func(t *testing.T) {
		mockRBACSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionVpcCreate, "*").Return(nil).Once()

		result, err := svc.CreateTransitGateway(ctx, "")
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("CreateTransitGateway_Unauthorized", func(t *testing.T) {
		mockRBACSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionVpcCreate, "*").Return(fmt.Errorf("unauthorized")).Once()

		result, err := svc.CreateTransitGateway(ctx, "my-tgw")
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("GetTransitGateway_Success", func(t *testing.T) {
		tgID := uuid.New()
		tg := &domain.TransitGateway{ID: tgID, Name: "my-tgw", OwnerTenantID: tenantID}
		mockRBACSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionVpcRead, tgID.String()).Return(nil).Once()
		mockRepo.On("GetByID", mock.Anything, tgID).Return(tg, nil).Once()

		result, err := svc.GetTransitGateway(ctx, tgID)
		require.NoError(t, err)
		assert.Equal(t, tgID, result.ID)
	})

	t.Run("GetTransitGateway_NotFound", func(t *testing.T) {
		tgID := uuid.New()
		mockRBACSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionVpcRead, tgID.String()).Return(nil).Once()
		mockRepo.On("GetByID", mock.Anything, tgID).Return(nil, fmt.Errorf("not found")).Once()

		_, err := svc.GetTransitGateway(ctx, tgID)
		require.Error(t, err)
	})

	t.Run("ListTransitGateways_Success", func(t *testing.T) {
		tgs := []*domain.TransitGateway{
			{ID: uuid.New(), Name: "tgw-1", OwnerTenantID: tenantID},
			{ID: uuid.New(), Name: "tgw-2", OwnerTenantID: tenantID},
		}
		mockRepo.On("List", mock.Anything, tenantID).Return(tgs, nil).Once()

		result, err := svc.ListTransitGateways(ctx)
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("DeleteTransitGateway_Success", func(t *testing.T) {
		tgID := uuid.New()
		tg := &domain.TransitGateway{ID: tgID, OwnerTenantID: tenantID}
		mockRBACSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionVpcDelete, tgID.String()).Return(nil).Once()
		mockRepo.On("GetByID", mock.Anything, tgID).Return(tg, nil).Once()
		mockRepo.On("ListAttachments", mock.Anything, tgID).Return([]*domain.TransitGatewayAttachment{}, nil).Once()
		mockRepo.On("ListRouteTables", mock.Anything, tgID).Return([]*domain.TransitGatewayRouteTable{}, nil).Once()
		mockRepo.On("Delete", mock.Anything, tgID).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "transit_gateway.delete", "transit_gateway", tgID.String(), mock.Anything).Return(nil).Once()

		err := svc.DeleteTransitGateway(ctx, tgID)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("DeleteTransitGateway_NotFound", func(t *testing.T) {
		tgID := uuid.New()
		mockRBACSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionVpcDelete, tgID.String()).Return(nil).Once()
		mockRepo.On("GetByID", mock.Anything, tgID).Return(nil, fmt.Errorf("not found")).Once()

		err := svc.DeleteTransitGateway(ctx, tgID)
		require.Error(t, err)
	})

	t.Run("AttachVPC_Success", func(t *testing.T) {
		tgID := uuid.New()
		vpcID := uuid.New()
		vpc := &domain.VPC{ID: vpcID, Name: "my-vpc", CIDRBlock: "10.0.0.0/16"}
		tg := &domain.TransitGateway{ID: tgID, OwnerTenantID: tenantID}
		subnets := []*domain.Subnet{{ID: uuid.New(), CIDRBlock: "10.0.0.0/24"}}
		defaultRT := &domain.TransitGatewayRouteTable{ID: uuid.New(), DefaultRouteTable: true}

		mockRBACSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionVpcCreate, "*").Return(nil).Once()
		mockRepo.On("GetByID", mock.Anything, tgID).Return(tg, nil).Once()
		mockVpcRepo.On("GetByID", mock.Anything, vpcID).Return(vpc, nil).Once()
		mockRepo.On("ListAttachments", mock.Anything, tgID).Return([]*domain.TransitGatewayAttachment{}, nil).Once()
		mockRepo.On("AddAttachment", mock.Anything, mock.Anything).Return(nil).Once()
		mockSubnetRepo.On("ListByVPC", mock.Anything, vpcID).Return(subnets, nil).Once()
		mockRepo.On("ListRouteTables", mock.Anything, tgID).Return([]*domain.TransitGatewayRouteTable{defaultRT}, nil).Once()
		mockRepo.On("AddRoute", mock.Anything, defaultRT.ID, mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "transit_gateway.attach_vpc", "transit_gateway", tgID.String(), mock.Anything).Return(nil).Once()

		att, err := svc.AttachVPC(ctx, tgID, vpcID)
		require.NoError(t, err)
		assert.NotNil(t, att)
		assert.Equal(t, vpcID, att.VPCID)
	})

	t.Run("AttachVPC_Duplicate", func(t *testing.T) {
		tgID := uuid.New()
		vpcID := uuid.New()
		vpc := &domain.VPC{ID: vpcID, Name: "my-vpc", CIDRBlock: "10.0.0.0/16"}
		tg := &domain.TransitGateway{ID: tgID, OwnerTenantID: tenantID}
		existingAtt := &domain.TransitGatewayAttachment{ID: uuid.New(), VPCID: vpcID}

		mockRBACSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionVpcCreate, "*").Return(nil).Once()
		mockRepo.On("GetByID", mock.Anything, tgID).Return(tg, nil).Once()
		mockVpcRepo.On("GetByID", mock.Anything, vpcID).Return(vpc, nil).Once()
		mockRepo.On("ListAttachments", mock.Anything, tgID).Return([]*domain.TransitGatewayAttachment{existingAtt}, nil).Once()

		_, err := svc.AttachVPC(ctx, tgID, vpcID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already attached")
	})

	t.Run("AttachVPC_TGWNotFound", func(t *testing.T) {
		tgID := uuid.New()
		vpcID := uuid.New()
		mockRBACSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionVpcCreate, "*").Return(nil).Once()
		mockRepo.On("GetByID", mock.Anything, tgID).Return(nil, fmt.Errorf("not found")).Once()

		_, err := svc.AttachVPC(ctx, tgID, vpcID)
		require.Error(t, err)
	})

	t.Run("DetachVPC_Success", func(t *testing.T) {
		attID := uuid.New()
		tgID := uuid.New()
		vpcID := uuid.New()
		att := &domain.TransitGatewayAttachment{ID: attID, TransitGatewayID: tgID, VPCID: vpcID}

		mockRBACSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionVpcDelete, attID.String()).Return(nil).Once()
		mockRepo.On("GetAttachment", mock.Anything, attID).Return(att, nil).Once()
		mockRepo.On("ListRouteTables", mock.Anything, tgID).Return([]*domain.TransitGatewayRouteTable{}, nil).Once()
		mockRepo.On("RemoveAttachment", mock.Anything, attID).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "transit_gateway.detach_vpc", "transit_gateway", tgID.String(), mock.Anything).Return(nil).Once()

		err := svc.DetachVPC(ctx, attID)
		require.NoError(t, err)
	})

	t.Run("DetachVPC_NotFound", func(t *testing.T) {
		attID := uuid.New()
		mockRBACSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionVpcDelete, attID.String()).Return(nil).Once()
		mockRepo.On("GetAttachment", mock.Anything, attID).Return(nil, fmt.Errorf("not found")).Once()

		err := svc.DetachVPC(ctx, attID)
		require.Error(t, err)
	})

	t.Run("AssociateRouteTable_Success", func(t *testing.T) {
		rtID := uuid.New()
		attID := uuid.New()
		mockRBACSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionVpcUpdate, rtID.String()).Return(nil).Once()
		mockRepo.On("AssociateAttachment", mock.Anything, rtID, attID).Return(nil).Once()

		err := svc.AssociateRouteTable(ctx, rtID, attID)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("EnableRoutePropagation_Success", func(t *testing.T) {
		rtID := uuid.New()
		attID := uuid.New()
		mockRBACSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionVpcUpdate, rtID.String()).Return(nil).Once()
		mockRepo.On("EnablePropagation", mock.Anything, rtID, attID).Return(nil).Once()

		err := svc.EnableRoutePropagation(ctx, rtID, attID)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})
}