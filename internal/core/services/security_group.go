// Package services implements core business workflows.
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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const securityGroupTracer = "security-group-service"

// SecurityGroupService manages security group lifecycle and rules.
type SecurityGroupService struct {
	repo     ports.SecurityGroupRepository
	rbacSvc  ports.RBACService
	vpcRepo  ports.VpcRepository
	network  ports.NetworkBackend
	auditSvc ports.AuditService
	logger   *slog.Logger
}

// NewSecurityGroupService constructs a SecurityGroupService with its dependencies.
func NewSecurityGroupService(
	repo ports.SecurityGroupRepository,
	rbacSvc ports.RBACService,
	vpcRepo ports.VpcRepository,
	network ports.NetworkBackend,
	auditSvc ports.AuditService,
	logger *slog.Logger,
) *SecurityGroupService {
	return &SecurityGroupService{
		repo:     repo,
		rbacSvc:  rbacSvc,
		vpcRepo:  vpcRepo,
		network:  network,
		auditSvc: auditSvc,
		logger:   logger,
	}
}

func (s *SecurityGroupService) CreateGroup(ctx context.Context, vpcID uuid.UUID, name, description string) (*domain.SecurityGroup, error) {
	ctx, span := otel.Tracer(securityGroupTracer).Start(ctx, "CreateGroup")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSgCreate, "*"); err != nil {
		return nil, err
	}

	span.SetAttributes(
		attribute.String("vpc_id", vpcID.String()),
		attribute.String("name", name),
	)

	sgID := uuid.New()

	arn := fmt.Sprintf("arn:thecloud:vpc:local:%s:security-group/%s", userID.String(), sgID.String())

	sg := &domain.SecurityGroup{
		ID:          sgID,
		UserID:      userID,
		TenantID:    tenantID,
		VPCID:       vpcID,
		Name:        name,
		Description: description,
		ARN:         arn,
		CreatedAt:   time.Now(),
		Rules:       []domain.SecurityRule{},
	}

	// Default allow ARP
	sg.Rules = append(sg.Rules, domain.SecurityRule{
		ID:        uuid.New(),
		GroupID:   sgID,
		Protocol:  "arp",
		Direction: domain.RuleIngress,
		Priority:  1000,
		CreatedAt: time.Now(),
	})
	sg.Rules = append(sg.Rules, domain.SecurityRule{
		ID:        uuid.New(),
		GroupID:   sgID,
		Protocol:  "arp",
		Direction: domain.RuleEgress,
		Priority:  1000,
		CreatedAt: time.Now(),
	})

	if err := s.repo.Create(ctx, sg); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to create security group", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "security_group.create", "security_group", sgID.String(), map[string]interface{}{
		"name":   name,
		"vpc_id": vpcID,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "security_group.create", "resource_id", sgID.String(), "error", err)
	}

	return sg, nil
}

func (s *SecurityGroupService) GetGroup(ctx context.Context, idOrName string, vpcID uuid.UUID) (*domain.SecurityGroup, error) {
	ctx, span := otel.Tracer(securityGroupTracer).Start(ctx, "GetGroup")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSgRead, idOrName); err != nil {
		return nil, err
	}

	span.SetAttributes(attribute.String("id_or_name", idOrName))

	id, err := uuid.Parse(idOrName)
	if err == nil {
		return s.repo.GetByID(ctx, id)
	}
	return s.repo.GetByName(ctx, vpcID, idOrName)
}

func (s *SecurityGroupService) ListGroups(ctx context.Context, vpcID uuid.UUID) ([]*domain.SecurityGroup, error) {
	ctx, span := otel.Tracer(securityGroupTracer).Start(ctx, "ListGroups")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSgRead, "*"); err != nil {
		return nil, err
	}

	span.SetAttributes(attribute.String("vpc_id", vpcID.String()))

	return s.repo.ListByVPC(ctx, vpcID)
}

func (s *SecurityGroupService) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	ctx, span := otel.Tracer(securityGroupTracer).Start(ctx, "DeleteGroup")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSgDelete, id.String()); err != nil {
		return err
	}

	span.SetAttributes(attribute.String("group_id", id.String()))

	// In a real implementation, we should check if any instances are still attached
	// For now, we just delete.
	if err := s.repo.Delete(ctx, id); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete security group", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "security_group.delete", "security_group", id.String(), nil); err != nil {
		s.logger.Warn("failed to log audit event", "action", "security_group.delete", "resource_id", id.String(), "error", err)
	}

	return nil
}

func (s *SecurityGroupService) AddRule(ctx context.Context, idOrName string, rule domain.SecurityRule) (*domain.SecurityRule, error) {
	ctx, span := otel.Tracer(securityGroupTracer).Start(ctx, "AddRule")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	groupID, err := s.resolveGroupID(ctx, idOrName)
	if err != nil {
		return nil, err
	}

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSgUpdate, groupID.String()); err != nil {
		return nil, err
	}

	span.SetAttributes(
		attribute.String("group_id", groupID.String()),
		attribute.String("protocol", rule.Protocol),
		attribute.Int("port_min", rule.PortMin),
		attribute.String("cidr", rule.CIDR),
	)

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

	if err := s.auditSvc.Log(ctx, sg.UserID, "security_group.add_rule", "security_group", groupID.String(), map[string]interface{}{
		"rule_id": rule.ID.String(),
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "security_group.add_rule", "resource_id", groupID.String(), "error", err)
	}

	return &rule, nil
}

func (s *SecurityGroupService) resolveGroupID(ctx context.Context, idOrName string) (uuid.UUID, error) {
	id, err := uuid.Parse(idOrName)
	if err == nil {
		return id, nil
	}
	sg, err := s.repo.GetByNameAcrossVPCs(ctx, idOrName)
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			return uuid.Nil, errors.Wrap(errors.NotFound, "security group not found", err)
		}
		return uuid.Nil, err
	}
	return sg.ID, nil
}

func (s *SecurityGroupService) RemoveRule(ctx context.Context, ruleID uuid.UUID) error {
	ctx, span := otel.Tracer(securityGroupTracer).Start(ctx, "RemoveRule")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSgUpdate, ruleID.String()); err != nil {
		return err
	}

	span.SetAttributes(attribute.String("rule_id", ruleID.String()))

	// 1. Get Rule to find GroupID
	rule, err := s.repo.GetRuleByID(ctx, ruleID)
	if err != nil {
		return err
	}

	// 2. Get Group (needed for audit logs and flow context)
	sg, err := s.repo.GetByID(ctx, rule.GroupID)
	if err != nil {
		return err
	}

	// 3. Remove from OVS
	// We attempt this before DB deletion. If it fails, we assume consistency check later or manual fix.
	// In production, this should likely be a localized transaction or workflow.
	vpc, err := s.vpcRepo.GetByID(ctx, sg.VPCID)
	if err == nil {
		flow := s.translateToFlow(*rule)
		if err := s.network.DeleteFlowRule(ctx, vpc.NetworkID, flow.Match); err != nil {
			// Log but proceed to ensure DB consistency
			s.logger.Error("failed to delete OVS flow rule", "rule_id", ruleID, "error", err)
		}
	} else {
		// VPC might be gone?
		s.logger.Warn("vpc not found during rule deletion", "vpc_id", sg.VPCID)
	}

	// 4. Delete from DB
	if err := s.repo.DeleteRule(ctx, ruleID); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete security rule", err)
	}

	if err := s.auditSvc.Log(ctx, sg.UserID, "security_group.remove_rule", "security_group", sg.ID.String(), map[string]interface{}{
		"rule_id": ruleID.String(),
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "security_group.remove_rule", "resource_id", sg.ID.String(), "error", err)
	}

	return nil
}

func (s *SecurityGroupService) AttachToInstance(ctx context.Context, instanceID, groupID uuid.UUID) error {
	ctx, span := otel.Tracer(securityGroupTracer).Start(ctx, "AttachToInstance")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSgUpdate, groupID.String()); err != nil {
		return err
	}

	span.SetAttributes(
		attribute.String("instance_id", instanceID.String()),
		attribute.String("group_id", groupID.String()),
	)

	if err := s.repo.AddInstanceToGroup(ctx, instanceID, groupID); err != nil {
		return err
	}

	sg, err := s.repo.GetByID(ctx, groupID)
	if err != nil {
		return err
	}

	// Update OVS flows
	if err := s.syncGroupFlows(ctx, sg); err != nil {
		return err
	}

	if err := s.auditSvc.Log(ctx, sg.UserID, "security_group.attach", "instance", instanceID.String(), map[string]interface{}{
		"group_id": groupID.String(),
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "security_group.attach", "resource_id", instanceID.String(), "error", err)
	}

	return nil
}

func (s *SecurityGroupService) DetachFromInstance(ctx context.Context, instanceID, groupID uuid.UUID) error {
	ctx, span := otel.Tracer(securityGroupTracer).Start(ctx, "DetachFromInstance")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSgUpdate, groupID.String()); err != nil {
		return err
	}

	span.SetAttributes(
		attribute.String("instance_id", instanceID.String()),
		attribute.String("group_id", groupID.String()),
	)

	if err := s.repo.RemoveInstanceFromGroup(ctx, instanceID, groupID); err != nil {
		return err
	}

	sg, err := s.repo.GetByID(ctx, groupID)
	if err == nil {
		// Cleanup OVS flows
		if err := s.removeGroupFlows(ctx, sg); err != nil {
			s.logger.Error("failed to remove OVS flows", "group_id", groupID, "error", err)
		}
	}

	if err := s.auditSvc.Log(ctx, userID, "security_group.detach", "instance", instanceID.String(), map[string]interface{}{
		"group_id": groupID.String(),
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "security_group.detach", "resource_id", instanceID.String(), "error", err)
	}

	return nil
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

func (s *SecurityGroupService) removeGroupFlows(ctx context.Context, sg *domain.SecurityGroup) error {
	vpc, err := s.vpcRepo.GetByID(ctx, sg.VPCID)
	if err != nil {
		return err
	}

	for _, rule := range sg.Rules {
		flow := s.translateToFlow(rule)
		if err := s.network.DeleteFlowRule(ctx, vpc.NetworkID, flow.Match); err != nil {
			s.logger.Error("failed to delete flow rule", "rule", rule.ID, "error", err)
		}
	}

	return nil
}

func (s *SecurityGroupService) translateToFlow(rule domain.SecurityRule) ports.FlowRule {
	matchParts := []string{}

	switch rule.Protocol {
	case "tcp":
		matchParts = append(matchParts, "tcp")
	case "udp":
		matchParts = append(matchParts, "udp")
	case "icmp":
		matchParts = append(matchParts, "icmp")
	case "arp":
		matchParts = append(matchParts, "arp")
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
			switch rule.Protocol {
			case "tcp", "udp":
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
		Actions:  "NORMAL",
	}
}
