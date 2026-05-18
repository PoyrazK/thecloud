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
)

// SubnetServiceParams defines dependencies for SubnetService.
type SubnetServiceParams struct {
	Repo     ports.SubnetRepository
	RBACSvc  ports.RBACService
	VpcRepo  ports.VpcRepository
	AuditSvc ports.AuditService
	Logger   *slog.Logger
}

// SubnetService manages subnet lifecycle within VPCs.
type SubnetService struct {
	repo     ports.SubnetRepository
	rbacSvc  ports.RBACService
	vpcRepo  ports.VpcRepository
	auditSvc ports.AuditService
	logger   *slog.Logger
}

// NewSubnetService constructs a SubnetService with its dependencies.
func NewSubnetService(params SubnetServiceParams) *SubnetService {
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &SubnetService{
		repo:     params.Repo,
		rbacSvc:  params.RBACSvc,
		vpcRepo:  params.VpcRepo,
		auditSvc: params.AuditSvc,
		logger:   logger,
	}
}

func (s *SubnetService) CreateSubnet(ctx context.Context, vpcID uuid.UUID, name, cidrBlock, az string) (*domain.Subnet, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	// Creating a subnet is considered a VPC Update
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcUpdate, vpcID.String()); err != nil {
		return nil, err
	}

	// 1. Get VPC and validate CIDR range
	vpc, err := s.vpcRepo.GetByID(ctx, vpcID)
	if err != nil {
		return nil, err
	}

	_, vpcNet, err := net.ParseCIDR(vpc.CIDRBlock)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "invalid VPC CIDR", err)
	}

	ip, subnetNet, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return nil, errors.New(errors.InvalidInput, "invalid subnet CIDR block")
	}

	if !vpcNet.Contains(ip) {
		return nil, errors.New(errors.InvalidInput, "subnet CIDR must be within VPC CIDR range")
	}

	// 2. Gateway IP (first usable IP in subnet)
	gatewayIP := s.calculateGatewayIP(ip, subnetNet)

	// 3. Construct entity
	subnetID := uuid.New()
	arn := fmt.Sprintf("arn:thecloud:subnet:local:%s:subnet/%s", userID.String(), subnetID.String())

	subnet := &domain.Subnet{
		ID:               subnetID,
		UserID:           userID,
		TenantID:         tenantID,
		VPCID:            vpcID,
		Name:             name,
		CIDRBlock:        cidrBlock,
		AvailabilityZone: az,
		GatewayIP:        gatewayIP,
		ARN:              arn,
		Status:           "available",
		CreatedAt:        time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, subnet); err != nil {
		return nil, err
	}

	if err := s.auditSvc.Log(ctx, userID, "subnet.create", "subnet", subnetID.String(), map[string]interface{}{
		"vpc_id":     vpcID.String(),
		"name":       name,
		"cidr_block": cidrBlock,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "subnet.create", "subnet_id", subnetID, "error", err)
	}

	return subnet, nil
}

func (s *SubnetService) GetSubnet(ctx context.Context, idOrName string, vpcID uuid.UUID) (*domain.Subnet, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcRead, idOrName); err != nil {
		return nil, err
	}

	id, err := uuid.Parse(idOrName)
	if err == nil {
		return s.repo.GetByID(ctx, id)
	}
	return s.repo.GetByName(ctx, vpcID, idOrName)
}

func (s *SubnetService) ListSubnets(ctx context.Context, vpcID uuid.UUID) ([]*domain.Subnet, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcRead, vpcID.String()); err != nil {
		return nil, err
	}

	return s.repo.ListByVPC(ctx, vpcID)
}

func (s *SubnetService) DeleteSubnet(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcDelete, id.String()); err != nil {
		return err
	}

	subnet, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	if err := s.auditSvc.Log(ctx, subnet.UserID, "subnet.delete", "subnet", id.String(), nil); err != nil {
		s.logger.Warn("failed to log audit event", "action", "subnet.delete", "subnet_id", id, "error", err)
	}
	return nil
}

func (s *SubnetService) calculateGatewayIP(ip net.IP, _ *net.IPNet) string {
	// Simple implementation: IP + 1
	gw := make(net.IP, len(ip))
	copy(gw, ip)
	for i := len(gw) - 1; i >= 0; i-- {
		gw[i]++
		if gw[i] > 0 {
			break
		}
	}
	return gw.String()
}
