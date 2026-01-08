// Package services contains the core business logic of the application.
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
)

// VpcService handles the lifecycle of Virtual Private Clouds (VPCs),
// including network isolation through OVS bridges and backend persistence.
type VpcService struct {
	repo        ports.VpcRepository
	network     ports.NetworkBackend
	auditSvc    ports.AuditService
	logger      *slog.Logger
	defaultCIDR string
}

// NewVpcService creates a new instance of VpcService.
// If defaultCIDR is empty, it defaults to "10.0.0.0/16".
func NewVpcService(repo ports.VpcRepository, network ports.NetworkBackend, auditSvc ports.AuditService, logger *slog.Logger, defaultCIDR string) *VpcService {
	if defaultCIDR == "" {
		defaultCIDR = "10.0.0.0/16" // Fallback if not provided
	}
	return &VpcService{
		repo:        repo,
		network:     network,
		auditSvc:    auditSvc,
		logger:      logger,
		defaultCIDR: defaultCIDR,
	}
}

// CreateVPC provisions a new VPC with an associated OVS bridge for network isolation.
// It generates a unique VXLAN ID and persists the VPC metadata to the database.
func (s *VpcService) CreateVPC(ctx context.Context, name, cidrBlock string) (*domain.VPC, error) {
	if cidrBlock == "" {
		cidrBlock = s.defaultCIDR
	}

	userID := appcontext.UserIDFromContext(ctx)
	vpcID := uuid.New()

	// 1. Generate unique VNI (for demo purposes we use a hash based int)
	vxlanID := int(vpcID[0]) + 100

	// 2. Create OVS bridge
	bridgeName := fmt.Sprintf("br-vpc-%s", vpcID.String()[:8])
	if err := s.network.CreateBridge(ctx, bridgeName, vxlanID); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to create OVS bridge", err)
	}

	// 3. Construct ARN
	arn := fmt.Sprintf("arn:thecloud:vpc:local:%s:vpc/%s", userID.String(), vpcID.String())

	// 4. Persist to DB
	vpc := &domain.VPC{
		ID:        vpcID,
		UserID:    userID,
		Name:      name,
		CIDRBlock: cidrBlock,
		NetworkID: bridgeName,
		VXLANID:   vxlanID,
		Status:    "active",
		ARN:       arn,
		CreatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, vpc); err != nil {
		// Cleanup OVS bridge if DB fails
		s.logger.Error("failed to create VPC in DB, rolling back bridge", "name", name, "error", err)
		if rbErr := s.network.DeleteBridge(ctx, bridgeName); rbErr != nil {
			s.logger.Error("failed to rollback bridge", "bridge", bridgeName, "error", rbErr)
		}
		return nil, errors.Wrap(errors.Internal, "failed to create VPC in database", err)
	}

	_ = s.auditSvc.Log(ctx, vpc.UserID, "vpc.create", "vpc", vpc.ID.String(), map[string]interface{}{
		"name":       vpc.Name,
		"cidr_block": vpc.CIDRBlock,
		"arn":        vpc.ARN,
	})

	return vpc, nil
}

// GetVPC retrieves a VPC by its unique identifier (UUID) or its name.
func (s *VpcService) GetVPC(ctx context.Context, idOrName string) (*domain.VPC, error) {
	id, err := uuid.Parse(idOrName)
	if err == nil {
		return s.repo.GetByID(ctx, id)
	}
	return s.repo.GetByName(ctx, idOrName)
}

// ListVPCs returns a list of all VPCs accessible by the current user.
func (s *VpcService) ListVPCs(ctx context.Context) ([]*domain.VPC, error) {
	return s.repo.List(ctx)
}

// DeleteVPC removes a VPC, its associated OVS bridge, and all related database records.
func (s *VpcService) DeleteVPC(ctx context.Context, idOrName string) error {
	vpc, err := s.GetVPC(ctx, idOrName)
	if err != nil {
		return err
	}

	// 1. Remove OVS bridge
	if err := s.network.DeleteBridge(ctx, vpc.NetworkID); err != nil {
		s.logger.Error("failed to remove OVS bridge", "bridge", vpc.NetworkID, "error", err)
		return errors.Wrap(errors.Internal, "failed to remove OVS bridge", err)
	}
	s.logger.Info("vpc bridge removed", "bridge", vpc.NetworkID)

	// 2. Delete from DB
	if err := s.repo.Delete(ctx, vpc.ID); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete VPC from database", err)
	}

	_ = s.auditSvc.Log(ctx, vpc.UserID, "vpc.delete", "vpc", vpc.ID.String(), map[string]interface{}{
		"name": vpc.Name,
	})

	return nil
}
