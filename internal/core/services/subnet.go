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

type SubnetService struct {
	repo     ports.SubnetRepository
	vpcRepo  ports.VpcRepository
	auditSvc ports.AuditService
	logger   *slog.Logger
}

// NewSubnetService creates a SubnetService configured with the given subnet and VPC repositories, audit service, and logger.
func NewSubnetService(repo ports.SubnetRepository, vpcRepo ports.VpcRepository, auditSvc ports.AuditService, logger *slog.Logger) *SubnetService {
	return &SubnetService{
		repo:     repo,
		vpcRepo:  vpcRepo,
		auditSvc: auditSvc,
		logger:   logger,
	}
}

func (s *SubnetService) CreateSubnet(ctx context.Context, vpcID uuid.UUID, name, cidrBlock, az string) (*domain.Subnet, error) {
	userID := appcontext.UserIDFromContext(ctx)

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
		VPCID:            vpcID,
		Name:             name,
		CIDRBlock:        cidrBlock,
		AvailabilityZone: az,
		GatewayIP:        gatewayIP,
		ARN:              arn,
		Status:           "available",
		CreatedAt:        time.Now(),
	}

	if err := s.repo.Create(ctx, subnet); err != nil {
		return nil, err
	}

	_ = s.auditSvc.Log(ctx, userID, "subnet.create", "subnet", subnetID.String(), map[string]interface{}{
		"vpc_id":     vpcID.String(),
		"name":       name,
		"cidr_block": cidrBlock,
	})

	return subnet, nil
}

func (s *SubnetService) GetSubnet(ctx context.Context, idOrName string, vpcID uuid.UUID) (*domain.Subnet, error) {
	id, err := uuid.Parse(idOrName)
	if err == nil {
		return s.repo.GetByID(ctx, id)
	}
	return s.repo.GetByName(ctx, vpcID, idOrName)
}

func (s *SubnetService) ListSubnets(ctx context.Context, vpcID uuid.UUID) ([]*domain.Subnet, error) {
	return s.repo.ListByVPC(ctx, vpcID)
}

func (s *SubnetService) DeleteSubnet(ctx context.Context, id uuid.UUID) error {
	subnet, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	_ = s.auditSvc.Log(ctx, subnet.UserID, "subnet.delete", "subnet", id.String(), nil)
	return nil
}

func (s *SubnetService) calculateGatewayIP(ip net.IP, n *net.IPNet) string {
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