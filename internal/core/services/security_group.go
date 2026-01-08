package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
)

type SecurityGroupService struct {
	repo     ports.SecurityGroupRepository
	vpcRepo  ports.VpcRepository
	network  ports.NetworkBackend
	auditSvc ports.AuditService
	logger   *slog.Logger
}

func NewSecurityGroupService(
	repo ports.SecurityGroupRepository,
	vpcRepo ports.VpcRepository,
	network ports.NetworkBackend,
	auditSvc ports.AuditService,
	logger *slog.Logger,
) *SecurityGroupService {
	return &SecurityGroupService{
		repo:     repo,
		vpcRepo:  vpcRepo,
		network:  network,
		auditSvc: auditSvc,
		logger:   logger,
	}
}

func (s *SecurityGroupService) CreateGroup(ctx context.Context, vpcID uuid.UUID, name, description string) (*domain.SecurityGroup, error) {
	userID := appcontext.UserIDFromContext(ctx)
	sgID := uuid.New()

	arn := fmt.Sprintf("arn:thecloud:vpc:local:%s:security-group/%s", userID.String(), sgID.String())

	sg := &domain.SecurityGroup{
		ID:          sgID,
		UserID:      userID,
		VPCID:       vpcID,
		Name:        name,
		Description: description,
		ARN:         arn,
		CreatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, sg); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to create security group", err)
	}

	_ = s.auditSvc.Log(ctx, userID, "security_group.create", "security_group", sgID.String(), map[string]interface{}{
		"name":   name,
		"vpc_id": vpcID,
	})

	return sg, nil
}

func (s *SecurityGroupService) GetGroup(ctx context.Context, idOrName string, vpcID uuid.UUID) (*domain.SecurityGroup, error) {
	id, err := uuid.Parse(idOrName)
	if err == nil {
		return s.repo.GetByID(ctx, id)
	}
	return s.repo.GetByName(ctx, vpcID, idOrName)
}

func (s *SecurityGroupService) ListGroups(ctx context.Context, vpcID uuid.UUID) ([]*domain.SecurityGroup, error) {
	return s.repo.ListByVPC(ctx, vpcID)
}

func (s *SecurityGroupService) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)

	// In a real implementation, we should check if any instances are still attached
	// For now, we just delete.
	if err := s.repo.Delete(ctx, id); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete security group", err)
	}

	_ = s.auditSvc.Log(ctx, userID, "security_group.delete", "security_group", id.String(), nil)

	return nil
}

func (s *SecurityGroupService) AddRule(ctx context.Context, groupID uuid.UUID, rule domain.SecurityRule) (*domain.SecurityRule, error) {
	sg, err := s.repo.GetByID(ctx, groupID)
	if err != nil {
		return nil, err
	}

	rule.ID = uuid.New()
	rule.GroupID = groupID
	rule.CreatedAt = time.Now()

	if err := s.repo.AddRule(ctx, &rule); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to add security rule", err)
	}

	// Update OVS flows
	if err := s.syncGroupFlows(ctx, sg); err != nil {
		s.logger.Error("failed to sync OVS flows", "group_id", groupID, "error", err)
		// We don't rollback DB here in this simple pass, but in real life we should.
	}

	_ = s.auditSvc.Log(ctx, sg.UserID, "security_group.add_rule", "security_group", groupID.String(), map[string]interface{}{
		"rule_id": rule.ID.String(),
	})

	return &rule, nil
}

func (s *SecurityGroupService) RemoveRule(ctx context.Context, ruleID uuid.UUID) error {
	// 1. Get rule to know groupID
	// Implementation omitted: would need a GetRuleByID in repo
	return nil
}

func (s *SecurityGroupService) AttachToInstance(ctx context.Context, instanceID, groupID uuid.UUID) error {
	if err := s.repo.AddInstanceToGroup(ctx, instanceID, groupID); err != nil {
		return err
	}

	sg, err := s.repo.GetByID(ctx, groupID)
	if err != nil {
		return err
	}

	// Update OVS flows
	return s.syncGroupFlows(ctx, sg)
}

func (s *SecurityGroupService) DetachFromInstance(ctx context.Context, instanceID, groupID uuid.UUID) error {
	return s.repo.RemoveInstanceFromGroup(ctx, instanceID, groupID)
}

func (s *SecurityGroupService) syncGroupFlows(ctx context.Context, sg *domain.SecurityGroup) error {
	vpc, err := s.vpcRepo.GetByID(ctx, sg.VPCID)
	if err != nil {
		return err
	}

	// For each rule, generate an OVS flow
	for _, rule := range sg.Rules {
		flow := s.translateToFlow(rule)
		if err := s.network.AddFlowRule(ctx, vpc.NetworkID, flow); err != nil {
			return err
		}
	}

	return nil
}

func (s *SecurityGroupService) translateToFlow(rule domain.SecurityRule) ports.FlowRule {
	matchParts := []string{}

	if rule.Protocol == "tcp" {
		matchParts = append(matchParts, "tcp")
	} else if rule.Protocol == "udp" {
		matchParts = append(matchParts, "udp")
	} else if rule.Protocol == "icmp" {
		matchParts = append(matchParts, "icmp")
	}

	if rule.CIDR != "" && rule.CIDR != "0.0.0.0/0" {
		if rule.Direction == domain.RuleIngress {
			matchParts = append(matchParts, fmt.Sprintf("nw_src=%s", rule.CIDR))
		} else {
			matchParts = append(matchParts, fmt.Sprintf("nw_dst=%s", rule.CIDR))
		}
	}

	if rule.PortMin > 0 {
		if rule.PortMin == rule.PortMax {
			if rule.Protocol == "tcp" {
				matchParts = append(matchParts, fmt.Sprintf("tp_dst=%d", rule.PortMin))
			} else if rule.Protocol == "udp" {
				matchParts = append(matchParts, fmt.Sprintf("tp_dst=%d", rule.PortMin))
			}
		} else if rule.PortMax > rule.PortMin {
			// OVS doesn't support easy ranges, would need multiple flows or mask
			// Simplified for now
			matchParts = append(matchParts, fmt.Sprintf("tp_dst=%d", rule.PortMin))
		}
	}

	return ports.FlowRule{
		Priority: rule.Priority,
		Match:    strings.Join(matchParts, ","),
		Actions:  "allow", // Simplified
	}
}
