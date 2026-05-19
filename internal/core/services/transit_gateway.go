package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const transitGatewayTracer = "transit-gateway-service"

// TransitGatewayService manages the lifecycle of Transit Gateways.
type TransitGatewayService struct {
	repo       ports.TransitGatewayRepository
	vpcRepo    ports.VpcRepository
	subnetRepo ports.SubnetRepository
	rtRepo     ports.RouteTableRepository
	network    ports.NetworkBackend
	rbacSvc    ports.RBACService
	auditSvc   ports.AuditService
	logger     *slog.Logger
}

// TransitGatewayServiceParams holds dependencies for TransitGatewayService.
type TransitGatewayServiceParams struct {
	Repo       ports.TransitGatewayRepository
	VpcRepo    ports.VpcRepository
	SubnetRepo ports.SubnetRepository
	RTRepo     ports.RouteTableRepository
	Network    ports.NetworkBackend
	RBACSvc    ports.RBACService
	AuditSvc   ports.AuditService
	Logger     *slog.Logger
}

// NewTransitGatewayService constructs a TransitGatewayService with its dependencies.
func NewTransitGatewayService(params TransitGatewayServiceParams) *TransitGatewayService {
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &TransitGatewayService{
		repo:       params.Repo,
		vpcRepo:    params.VpcRepo,
		subnetRepo: params.SubnetRepo,
		rtRepo:     params.RTRepo,
		network:    params.Network,
		rbacSvc:    params.RBACSvc,
		auditSvc:   params.AuditSvc,
		logger:     logger,
	}
}

// CreateTransitGateway creates a new Transit Gateway with a default route table.
func (s *TransitGatewayService) CreateTransitGateway(ctx context.Context, name string) (*domain.TransitGateway, error) {
	ctx, span := otel.Tracer(transitGatewayTracer).Start(ctx, "CreateTransitGateway")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcCreate, "*"); err != nil {
		return nil, err
	}

	if name == "" {
		return nil, errors.New(errors.InvalidInput, "name is required")
	}

	tgID := uuid.New()
	arn := fmt.Sprintf("arn:thecloud:transit-gateway:local:%s:transit-gateway/%s", tenantID.String(), tgID.String())

	tg := &domain.TransitGateway{
		ID:            tgID,
		Name:          name,
		OwnerTenantID: tenantID,
		Status:        domain.TransitGatewayStatusPending,
		ARN:           arn,
		CreatedAt:     time.Now().UTC(),
	}

	// Create default route table
	defaultRT := &domain.TransitGatewayRouteTable{
		ID:                 uuid.New(),
		TransitGatewayID:   tgID,
		Name:               "default",
		DefaultRouteTable:  true,
		PropagationEnabled: true,
		Routes:             []*domain.TransitGatewayRoute{},
		CreatedAt:          time.Now().UTC(),
	}
	tg.RouteTables = []*domain.TransitGatewayRouteTable{defaultRT}

	if err := s.repo.Create(ctx, tg); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to create transit gateway", err)
	}

	// Mark as available
	tg.Status = domain.TransitGatewayStatusAvailable
	if err := s.repo.Update(ctx, tg); err != nil {
		s.logger.Warn("failed to update transit gateway status to available", "error", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "transit_gateway.create", "transit_gateway", tgID.String(), map[string]interface{}{
		"name": name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	s.logger.Info("transit gateway created", "id", tgID, "name", name)
	return tg, nil
}

// GetTransitGateway retrieves a Transit Gateway by ID.
func (s *TransitGatewayService) GetTransitGateway(ctx context.Context, id uuid.UUID) (*domain.TransitGateway, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcRead, id.String()); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, id)
}

// ListTransitGateways returns all Transit Gateways for the current tenant.
func (s *TransitGatewayService) ListTransitGateways(ctx context.Context) ([]*domain.TransitGateway, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	return s.repo.List(ctx, tenantID)
}

// DeleteTransitGateway removes a Transit Gateway and all its attachments.
func (s *TransitGatewayService) DeleteTransitGateway(ctx context.Context, id uuid.UUID) error {
	ctx, span := otel.Tracer(transitGatewayTracer).Start(ctx, "DeleteTransitGateway")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcDelete, id.String()); err != nil {
		return err
	}

	tg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return errors.Wrap(errors.NotFound, "transit gateway not found", err)
	}
	_ = tg

	// Get attachments to clean up OVS flows
	attachments, _ := s.repo.ListAttachments(ctx, id)
	for _, att := range attachments {
		if err := s.detachVPC(ctx, att); err != nil {
			s.logger.Error("failed to detach VPC during TGW deletion", "attachment_id", att.ID, "error", err)
		}
	}

	// Delete route tables
	rts, _ := s.repo.ListRouteTables(ctx, id)
	for _, rt := range rts {
		if err := s.repo.DeleteRouteTable(ctx, rt.ID); err != nil {
			s.logger.Warn("failed to delete route table during TGW deletion", "rt_id", rt.ID, "error", err)
		}
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete transit gateway", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "transit_gateway.delete", "transit_gateway", id.String(), nil); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	s.logger.Info("transit gateway deleted", "id", id)
	return nil
}

// AttachVPC attaches a VPC to the Transit Gateway.
func (s *TransitGatewayService) AttachVPC(ctx context.Context, tgID, vpcID uuid.UUID) (*domain.TransitGatewayAttachment, error) {
	ctx, span := otel.Tracer(transitGatewayTracer).Start(ctx, "AttachVPC")
	defer span.End()

	span.SetAttributes(
		attribute.String("tg_id", tgID.String()),
		attribute.String("vpc_id", vpcID.String()),
	)

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcCreate, "*"); err != nil {
		return nil, err
	}

	// Get TGW
	tg, err := s.repo.GetByID(ctx, tgID)
	if err != nil {
		return nil, errors.Wrap(errors.NotFound, "transit gateway not found", err)
	}

	// Get VPC
	vpc, err := s.vpcRepo.GetByID(ctx, vpcID)
	if err != nil {
		return nil, errors.Wrap(errors.NotFound, "VPC not found", err)
	}

	// Check for existing attachment
	existing, _ := s.repo.ListAttachments(ctx, tgID)
	for _, att := range existing {
		if att.VPCID == vpcID {
			return nil, errors.New(errors.Conflict, "VPC is already attached to this transit gateway")
		}
	}

	attID := uuid.New()
	att := &domain.TransitGatewayAttachment{
		ID:               attID,
		TransitGatewayID: tgID,
		VPCID:            vpcID,
		TenantID:         tenantID,
		Status:           "attached",
		AttachmentType:   "vpc",
	}

	if err := s.repo.AddAttachment(ctx, att); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to create attachment", err)
	}

	// Propagate VPC subnet routes to TGW route tables
	if err := s.propagateSubnetRoutes(ctx, tg, vpc, attID); err != nil {
		s.logger.Error("failed to propagate subnet routes for attachment", "att_id", attID, "error", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "transit_gateway.attach_vpc", "transit_gateway", tgID.String(), map[string]interface{}{
		"vpc_id": vpcID.String(),
	}); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	s.logger.Info("VPC attached to transit gateway", "tg_id", tgID, "vpc_id", vpcID, "att_id", attID)
	return att, nil
}

// DetachVPC detaches a VPC from the Transit Gateway.
func (s *TransitGatewayService) DetachVPC(ctx context.Context, attID uuid.UUID) error {
	ctx, span := otel.Tracer(transitGatewayTracer).Start(ctx, "DetachVPC")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcDelete, attID.String()); err != nil {
		return err
	}

	att, err := s.repo.GetAttachment(ctx, attID)
	if err != nil {
		return errors.Wrap(errors.NotFound, "attachment not found", err)
	}

	if err := s.detachVPC(ctx, att); err != nil {
		return err
	}

	if err := s.auditSvc.Log(ctx, userID, "transit_gateway.detach_vpc", "transit_gateway", att.TransitGatewayID.String(), map[string]interface{}{
		"vpc_id": att.VPCID.String(),
	}); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	s.logger.Info("VPC detached from transit gateway", "att_id", attID)
	return nil
}

// detachVPC removes the attachment and cleans up OVS flows.
func (s *TransitGatewayService) detachVPC(ctx context.Context, att *domain.TransitGatewayAttachment) error {
	// Remove propagated routes from TGW route tables
	rts, _ := s.repo.ListRouteTables(ctx, att.TransitGatewayID)
	for _, rt := range rts {
		routes, _ := s.repo.ListRoutes(ctx, rt.ID)
		for _, r := range routes {
			if r.TargetType == domain.TransitGatewayTargetAttachment && r.TargetID != nil && *r.TargetID == att.ID {
				if err := s.repo.RemoveRoute(ctx, rt.ID, r.ID); err != nil {
					s.logger.Warn("failed to remove propagated route during detachment", "route_id", r.ID, "error", err)
				}
			}
		}
	}

	if err := s.repo.RemoveAttachment(ctx, att.ID); err != nil {
		return errors.Wrap(errors.Internal, "failed to remove attachment", err)
	}

	s.logger.Info("attachment cleaned up", "att_id", att.ID)
	return nil
}

// propagateSubnetRoutes propagates a VPC's subnet CIDRs as routes in the TGW route tables.
func (s *TransitGatewayService) propagateSubnetRoutes(ctx context.Context, tg *domain.TransitGateway, vpc *domain.VPC, attID uuid.UUID) error {
	subnets, err := s.subnetRepo.ListByVPC(ctx, vpc.ID)
	if err != nil {
		return fmt.Errorf("failed to list subnets for route propagation: %w", err)
	}

	rts, err := s.repo.ListRouteTables(ctx, tg.ID)
	if err != nil {
		return fmt.Errorf("failed to list TGW route tables: %w", err)
	}

	for _, rt := range rts {
		for _, sn := range subnets {
			route := &domain.TransitGatewayRoute{
				ID:                 uuid.New(),
				TransitGatewayRTID: rt.ID,
				DestinationCIDR:    sn.CIDRBlock,
				TargetType:         domain.TransitGatewayTargetAttachment,
				TargetID:           &attID,
				TargetName:         fmt.Sprintf("vpc-%s", vpc.ID.String()[:8]),
			}
			if err := s.repo.AddRoute(ctx, rt.ID, route); err != nil {
				s.logger.Warn("failed to propagate subnet route", "rt_id", rt.ID, "subnet", sn.CIDRBlock, "error", err)
			}
		}
	}

	return nil
}

// AssociateRouteTable associates an attachment with a TGW route table.
func (s *TransitGatewayService) AssociateRouteTable(ctx context.Context, rtID, attID uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcUpdate, rtID.String()); err != nil {
		return err
	}

	if err := s.repo.AssociateAttachment(ctx, rtID, attID); err != nil {
		return errors.Wrap(errors.Internal, "failed to associate attachment", err)
	}

	s.logger.Info("attachment associated with route table", "rt_id", rtID, "att_id", attID)
	return nil
}

// EnableRoutePropagation enables automatic route propagation from an attachment to a RT.
func (s *TransitGatewayService) EnableRoutePropagation(ctx context.Context, rtID, attID uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcUpdate, rtID.String()); err != nil {
		return err
	}

	if err := s.repo.EnablePropagation(ctx, rtID, attID); err != nil {
		return errors.Wrap(errors.Internal, "failed to enable route propagation", err)
	}

	s.logger.Info("route propagation enabled", "rt_id", rtID, "att_id", attID)
	return nil
}
