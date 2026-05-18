// Package services implements core business workflows.
package services

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const natGatewayTracer = "nat-gateway-service"

// NATGatewayService manages the lifecycle of NAT Gateways.
type NATGatewayService struct {
	repo       ports.NATGatewayRepository
	eipRepo    ports.ElasticIPRepository
	subnetRepo ports.SubnetRepository
	vpcRepo    ports.VpcRepository
	rbacSvc    ports.RBACService
	network    ports.NetworkBackend
	auditSvc   ports.AuditService
	logger     *slog.Logger
}

// NATGatewayServiceParams holds dependencies for NATGatewayService.
type NATGatewayServiceParams struct {
	Repo       ports.NATGatewayRepository
	EIPRepo    ports.ElasticIPRepository
	SubnetRepo ports.SubnetRepository
	VpcRepo    ports.VpcRepository
	RBACSvc    ports.RBACService
	Network    ports.NetworkBackend
	AuditSvc   ports.AuditService
	Logger     *slog.Logger
}

// NewNATGatewayService constructs a NATGatewayService with its dependencies.
func NewNATGatewayService(params NATGatewayServiceParams) *NATGatewayService {
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &NATGatewayService{
		repo:       params.Repo,
		eipRepo:    params.EIPRepo,
		subnetRepo: params.SubnetRepo,
		vpcRepo:    params.VpcRepo,
		rbacSvc:    params.RBACSvc,
		network:    params.Network,
		auditSvc:   params.AuditSvc,
		logger:     logger,
	}
}

// CreateNATGateway creates a NAT Gateway in a subnet with an allocated Elastic IP.
func (s *NATGatewayService) CreateNATGateway(ctx context.Context, subnetID, eipID uuid.UUID) (*domain.NATGateway, error) {
	ctx, span := otel.Tracer(natGatewayTracer).Start(ctx, "CreateNATGateway")
	defer span.End()

	span.SetAttributes(
		attribute.String("subnet_id", subnetID.String()),
		attribute.String("eip_id", eipID.String()),
	)

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcCreate, "*"); err != nil {
		return nil, err
	}

	// Get subnet to find VPC and gateway IP
	subnet, err := s.subnetRepo.GetByID(ctx, subnetID)
	if err != nil {
		return nil, errors.Wrap(errors.NotFound, "subnet not found", err)
	}

	// Get VPC for validation
	vpc, err := s.vpcRepo.GetByID(ctx, subnet.VPCID)
	if err != nil {
		return nil, errors.Wrap(errors.NotFound, "VPC not found", err)
	}

	// Get Elastic IP
	eip, err := s.eipRepo.GetByID(ctx, eipID)
	if err != nil {
		return nil, errors.Wrap(errors.NotFound, "elastic IP not found", err)
	}

	// Verify EIP is allocated
	if eip.Status != domain.EIPStatusAllocated {
		return nil, errors.New(errors.InvalidInput, "elastic IP is not in allocated state")
	}

	// Validate egress IP format
	if net.ParseIP(eip.PublicIP) == nil {
		return nil, errors.New(errors.InvalidInput, "invalid EIP address format")
	}

	// Check if another NAT gateway already exists for this subnet
	existing, err := s.repo.ListBySubnet(ctx, subnetID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to check existing NAT gateways", err)
	}
	for _, nat := range existing {
		if nat.PrivateIP == subnet.GatewayIP {
			return nil, errors.New(errors.InvalidInput, "NAT gateway already exists for this subnet with the same private IP")
		}
	}

	natID := uuid.New()
	arn := fmt.Sprintf("arn:thecloud:vpc:local:%s:nat-gateway/%s", userID.String(), natID.String())

	// Use gateway IP from subnet as the NAT's private IP
	nat := &domain.NATGateway{
		ID:          natID,
		VPCID:       vpc.ID,
		SubnetID:    subnetID,
		ElasticIPID: eipID,
		UserID:      userID,
		TenantID:    tenantID,
		Status:      domain.NATGatewayStatusPending,
		PrivateIP:   subnet.GatewayIP,
		ARN:         arn,
		CreatedAt:   time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, nat); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to create NAT gateway", err)
	}

	// Setup NAT via NetworkBackend
	natVethEnd := fmt.Sprintf("nat-%s", natID.String()[:8])
	if err := s.network.SetupNATForSubnet(ctx, vpc.NetworkID, natVethEnd, subnet.CIDRBlock, eip.PublicIP); err != nil {
		s.logger.Error("failed to setup NAT", "nat_id", natID, "error", err)
		// Update NAT status to failed
		nat.Status = domain.NATGatewayStatusFailed
		_ = s.repo.Update(ctx, nat)
		return nil, errors.Wrap(errors.Internal, "failed to setup NAT", err)
	}

	// Mark as active
	nat.Status = domain.NATGatewayStatusActive
	if err := s.repo.Update(ctx, nat); err != nil {
		s.logger.Warn("failed to update NAT gateway status to active", "error", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "nat_gateway.create", "nat_gateway", natID.String(), map[string]interface{}{
		"subnet_id":  subnetID.String(),
		"eip_id":     eipID.String(),
		"private_ip": nat.PrivateIP,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	s.logger.Info("NAT gateway created", "id", natID, "subnet_id", subnetID, "eip", eip.PublicIP, "private_ip", nat.PrivateIP)
	return nat, nil
}

// GetNATGateway retrieves a NAT Gateway by ID.
func (s *NATGatewayService) GetNATGateway(ctx context.Context, natID uuid.UUID) (*domain.NATGateway, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcRead, natID.String()); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, natID)
}

// ListNATGateways returns all NAT Gateways for a VPC.
func (s *NATGatewayService) ListNATGateways(ctx context.Context, vpcID uuid.UUID) ([]*domain.NATGateway, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcRead, vpcID.String()); err != nil {
		return nil, err
	}

	return s.repo.ListByVPC(ctx, vpcID)
}

// DeleteNATGateway removes a NAT Gateway and releases the associated EIP.
func (s *NATGatewayService) DeleteNATGateway(ctx context.Context, natID uuid.UUID) error {
	ctx, span := otel.Tracer(natGatewayTracer).Start(ctx, "DeleteNATGateway")
	defer span.End()

	span.SetAttributes(attribute.String("nat_id", natID.String()))

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcDelete, natID.String()); err != nil {
		return err
	}

	// Get NAT Gateway
	nat, err := s.repo.GetByID(ctx, natID)
	if err != nil {
		return errors.Wrap(errors.NotFound, "NAT gateway not found", err)
	}

	// Get subnet and VPC for cleanup - these are required for proper NAT removal
	subnet, err := s.subnetRepo.GetByID(ctx, nat.SubnetID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to get subnet during NAT cleanup", err)
	}
	vpc, err := s.vpcRepo.GetByID(ctx, nat.VPCID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to get VPC during NAT cleanup", err)
	}

	// Get EIP for public IP - required to properly remove NAT rules
	eip, err := s.eipRepo.GetByID(ctx, nat.ElasticIPID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to get EIP during NAT cleanup", err)
	}

	// Remove NAT setup
	natVethEnd := fmt.Sprintf("nat-%s", natID.String()[:8])
	if err := s.network.RemoveNATForSubnet(ctx, vpc.NetworkID, natVethEnd, subnet.CIDRBlock, eip.PublicIP); err != nil {
		return errors.Wrap(errors.Internal, "failed to remove NAT setup", err)
	}

	// Release the EIP back to allocated state
	if eip != nil && eip.Status == domain.EIPStatusAssociated {
		eip.InstanceID = nil
		eip.VpcID = nil
		eip.Status = domain.EIPStatusAllocated
		eip.UpdatedAt = time.Now()
		if err := s.eipRepo.Update(ctx, eip); err != nil {
			s.logger.Warn("failed to release EIP during NAT gateway deletion",
				"eip_id", eip.ID, "nat_id", natID, "error", err)
		}
	}

	// Delete NAT record
	if err := s.repo.Delete(ctx, natID); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete NAT gateway", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "nat_gateway.delete", "nat_gateway", natID.String(), nil); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	s.logger.Info("NAT gateway deleted", "id", natID)
	return nil
}
