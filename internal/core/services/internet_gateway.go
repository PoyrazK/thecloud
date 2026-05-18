// Package services implements core business workflows.
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

const igwTracer = "internet-gateway-service"

// InternetGatewayService manages the lifecycle of Internet Gateways.
type InternetGatewayService struct {
	repo     ports.IGWRepository
	rtRepo   ports.RouteTableRepository
	vpcRepo  ports.VpcRepository
	rbacSvc  ports.RBACService
	auditSvc ports.AuditService
	logger   *slog.Logger
}

// InternetGatewayServiceParams holds dependencies for InternetGatewayService.
type InternetGatewayServiceParams struct {
	Repo     ports.IGWRepository
	RTRepo   ports.RouteTableRepository
	VpcRepo  ports.VpcRepository
	RBACSvc  ports.RBACService
	AuditSvc ports.AuditService
	Logger   *slog.Logger
}

// NewInternetGatewayService constructs an InternetGatewayService with its dependencies.
func NewInternetGatewayService(params InternetGatewayServiceParams) *InternetGatewayService {
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &InternetGatewayService{
		repo:     params.Repo,
		rtRepo:   params.RTRepo,
		vpcRepo:  params.VpcRepo,
		rbacSvc:  params.RBACSvc,
		auditSvc: params.AuditSvc,
		logger:   logger,
	}
}

// CreateIGW creates a new Internet Gateway (starts in detached state).
func (s *InternetGatewayService) CreateIGW(ctx context.Context) (*domain.InternetGateway, error) {
	ctx, span := otel.Tracer(igwTracer).Start(ctx, "CreateIGW")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcCreate, "*"); err != nil {
		return nil, err
	}

	igwID := uuid.New()
	arn := fmt.Sprintf("arn:thecloud:vpc:local:%s:internet-gateway/%s", userID.String(), igwID.String())

	igw := &domain.InternetGateway{
		ID:        igwID,
		VPCID:     nil,
		UserID:    userID,
		TenantID:  tenantID,
		Status:    domain.IGWStatusDetached,
		ARN:       arn,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, igw); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to create internet gateway", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "igw.create", "internet_gateway", igwID.String(), nil); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	s.logger.Info("internet gateway created", "id", igwID)
	return igw, nil
}

// AttachIGW attaches an IGW to a VPC.
// This also adds a default route (0.0.0.0/0) to the VPC's main route table pointing to this IGW.
func (s *InternetGatewayService) AttachIGW(ctx context.Context, igwID, vpcID uuid.UUID) error {
	ctx, span := otel.Tracer(igwTracer).Start(ctx, "AttachIGW")
	defer span.End()

	span.SetAttributes(
		attribute.String("igw_id", igwID.String()),
		attribute.String("vpc_id", vpcID.String()),
	)

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcUpdate, vpcID.String()); err != nil {
		return err
	}

	// Get IGW and verify it can be attached
	igw, err := s.repo.GetByID(ctx, igwID)
	if err != nil {
		return errors.Wrap(errors.NotFound, "internet gateway not found", err)
	}

	if !igw.CanAttach() {
		return errors.New(errors.InvalidInput, "internet gateway is already attached or cannot be attached")
	}

	// Verify VPC exists
	vpc, err := s.vpcRepo.GetByID(ctx, vpcID)
	if err != nil {
		return errors.Wrap(errors.NotFound, "VPC not found", err)
	}

	// Check if VPC already has an IGW attached
	existingIGW, _ := s.repo.GetByVPC(ctx, vpcID)
	if existingIGW != nil && existingIGW.Status == domain.IGWStatusAttached {
		return errors.New(errors.Conflict, "VPC already has an internet gateway attached")
	}

	// Update IGW to attached state
	igw.VPCID = &vpcID
	igw.Status = domain.IGWStatusAttached
	if err := s.repo.Update(ctx, igw); err != nil {
		return errors.Wrap(errors.Internal, "failed to attach internet gateway", err)
	}

	// Get main route table and add 0.0.0.0/0 route
	mainRT, err := s.rtRepo.GetMainByVPC(ctx, vpcID)
	if err != nil {
		s.logger.Warn("failed to get main route table for IGW attachment", "vpc_id", vpcID, "error", err)
		// Don't fail - IGW is attached, route can be added later
	} else {
		// Add default route pointing to IGW
		route := &domain.Route{
			ID:              uuid.New(),
			RouteTableID:    mainRT.ID,
			DestinationCIDR: "0.0.0.0/0",
			TargetType:      domain.RouteTargetIGW,
			TargetID:        &igwID,
			TargetName:      fmt.Sprintf("igw-%s", igwID.String()[:8]),
			CreatedAt:       time.Now().UTC(),
		}
		if err := s.rtRepo.AddRoute(ctx, mainRT.ID, route); err != nil {
			s.logger.Warn("failed to add default route for IGW", "rt_id", mainRT.ID, "error", err)
		}
	}

	if err := s.auditSvc.Log(ctx, userID, "igw.attach", "internet_gateway", igwID.String(), map[string]interface{}{
		"vpc_id": vpcID.String(),
	}); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	s.logger.Info("internet gateway attached", "igw_id", igwID, "vpc_id", vpcID, "vpc_name", vpc.Name)
	return nil
}

// DetachIGW detaches an IGW from its VPC.
// This removes the default route from the main route table.
func (s *InternetGatewayService) DetachIGW(ctx context.Context, igwID uuid.UUID) error {
	ctx, span := otel.Tracer(igwTracer).Start(ctx, "DetachIGW")
	defer span.End()

	span.SetAttributes(attribute.String("igw_id", igwID.String()))

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcUpdate, igwID.String()); err != nil {
		return err
	}

	// Get IGW and verify it can be detached
	igw, err := s.repo.GetByID(ctx, igwID)
	if err != nil {
		return errors.Wrap(errors.NotFound, "internet gateway not found", err)
	}

	if !igw.CanDetach() {
		return errors.New(errors.InvalidInput, "internet gateway is not attached")
	}

	vpcID := igw.VPCID
	if vpcID == nil {
		return errors.New(errors.Internal, "IGW has nil VPC ID but is marked attached")
	}

	// Get main route table and remove 0.0.0.0/0 route pointing to this IGW
	mainRT, err := s.rtRepo.GetMainByVPC(ctx, *vpcID)
	if err == nil {
		routes, _ := s.rtRepo.ListRoutes(ctx, mainRT.ID)
		for _, r := range routes {
			if r.DestinationCIDR == "0.0.0.0/0" && r.TargetType == domain.RouteTargetIGW && r.TargetID != nil && *r.TargetID == igwID {
				if removeErr := s.rtRepo.RemoveRoute(ctx, mainRT.ID, r.ID); removeErr != nil {
					s.logger.Warn("failed to remove default route during IGW detach", "route_id", r.ID, "error", removeErr)
				}
				break
			}
		}
	}

	// Update IGW to detached state
	igw.VPCID = nil
	igw.Status = domain.IGWStatusDetached
	if err := s.repo.Update(ctx, igw); err != nil {
		return errors.Wrap(errors.Internal, "failed to detach internet gateway", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "igw.detach", "internet_gateway", igwID.String(), map[string]interface{}{
		"vpc_id": vpcID.String(),
	}); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	s.logger.Info("internet gateway detached", "igw_id", igwID)
	return nil
}

// GetIGW retrieves an IGW by ID.
func (s *InternetGatewayService) GetIGW(ctx context.Context, igwID uuid.UUID) (*domain.InternetGateway, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcRead, igwID.String()); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, igwID)
}

// ListIGWs returns all IGWs for the current tenant.
func (s *InternetGatewayService) ListIGWs(ctx context.Context) ([]*domain.InternetGateway, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.ListAll(ctx)
}

// DeleteIGW permanently removes an IGW (must be detached first).
func (s *InternetGatewayService) DeleteIGW(ctx context.Context, igwID uuid.UUID) error {
	ctx, span := otel.Tracer(igwTracer).Start(ctx, "DeleteIGW")
	defer span.End()

	span.SetAttributes(attribute.String("igw_id", igwID.String()))

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcDelete, igwID.String()); err != nil {
		return err
	}

	// Get IGW and verify it's detached
	igw, err := s.repo.GetByID(ctx, igwID)
	if err != nil {
		return errors.Wrap(errors.NotFound, "internet gateway not found", err)
	}

	if igw.IsAttached() {
		return errors.New(errors.InvalidInput, "cannot delete an attached internet gateway; detach it first")
	}

	if err := s.repo.Delete(ctx, igwID); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete internet gateway", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "igw.delete", "internet_gateway", igwID.String(), nil); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	s.logger.Info("internet gateway deleted", "id", igwID)
	return nil
}
