package services

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
)

type iamService struct {
	repo      ports.IAMRepository
	auditSvc  ports.AuditService
	eventSvc  ports.EventService
	logger    *slog.Logger
	evaluator *iamEvaluator
}

// NewIAMService creates a new IAM service.
func NewIAMService(repo ports.IAMRepository, auditSvc ports.AuditService, eventSvc ports.EventService, logger *slog.Logger) *iamService {
	if logger == nil {
		logger = slog.Default()
	}
	return &iamService{
		repo:      repo,
		auditSvc:  auditSvc,
		eventSvc:  eventSvc,
		logger:    logger,
		evaluator: NewIAMEvaluator(),
	}
}

func (s *iamService) CreatePolicy(ctx context.Context, policy *domain.Policy) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	if policy.ID == uuid.Nil {
		policy.ID = uuid.New()
	}
	policy.TenantID = tenantID

	if err := s.repo.CreatePolicy(ctx, tenantID, policy); err != nil {
		return err
	}

	if err := s.eventSvc.RecordEvent(ctx, "IAM_POLICY_CREATE", policy.ID.String(), "POLICY", map[string]interface{}{"name": policy.Name}); err != nil {
		s.logger.Warn("failed to record event", "action", "IAM_POLICY_CREATE", "policy_id", policy.ID, "error", err)
	}
	return nil
}

func (s *iamService) GetPolicyByID(ctx context.Context, id uuid.UUID) (*domain.Policy, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	return s.repo.GetPolicyByID(ctx, tenantID, id)
}

func (s *iamService) ListPolicies(ctx context.Context) ([]*domain.Policy, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	return s.repo.ListPolicies(ctx, tenantID)
}

func (s *iamService) UpdatePolicy(ctx context.Context, policy *domain.Policy) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.repo.UpdatePolicy(ctx, tenantID, policy); err != nil {
		return err
	}
	if err := s.eventSvc.RecordEvent(ctx, "IAM_POLICY_UPDATE", policy.ID.String(), "POLICY", map[string]interface{}{"name": policy.Name}); err != nil {
		s.logger.Warn("failed to record event", "action", "IAM_POLICY_UPDATE", "policy_id", policy.ID, "error", err)
	}
	return nil
}

func (s *iamService) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.repo.DeletePolicy(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.eventSvc.RecordEvent(ctx, "IAM_POLICY_DELETE", id.String(), "POLICY", nil); err != nil {
		s.logger.Warn("failed to record event", "action", "IAM_POLICY_DELETE", "policy_id", id, "error", err)
	}
	return nil
}

func (s *iamService) AttachPolicyToUser(ctx context.Context, userID uuid.UUID, policyID uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.repo.AttachPolicyToUser(ctx, tenantID, userID, policyID); err != nil {
		return err
	}
	if err := s.auditSvc.Log(ctx, userID, "iam.policy_attach", "user", userID.String(), map[string]interface{}{"policy_id": policyID}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "iam.policy_attach", "user_id", userID, "error", err)
	}
	return nil
}

func (s *iamService) DetachPolicyFromUser(ctx context.Context, userID uuid.UUID, policyID uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.repo.DetachPolicyFromUser(ctx, tenantID, userID, policyID); err != nil {
		return err
	}
	if err := s.auditSvc.Log(ctx, userID, "iam.policy_detach", "user", userID.String(), map[string]interface{}{"policy_id": policyID}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "iam.policy_detach", "user_id", userID, "error", err)
	}
	return nil
}

func (s *iamService) GetPoliciesForUser(ctx context.Context, userID uuid.UUID) ([]*domain.Policy, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	return s.repo.GetPoliciesForUser(ctx, tenantID, userID)
}

func (s *iamService) AttachPolicyToRole(ctx context.Context, roleName string, policyID uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.repo.AttachPolicyToRole(ctx, tenantID, roleName, policyID); err != nil {
		return err
	}
	if err := s.eventSvc.RecordEvent(ctx, "IAM_POLICY_ATTACH_ROLE", policyID.String(), "POLICY", map[string]interface{}{"role_name": roleName}); err != nil {
		s.logger.Warn("failed to record event", "action", "IAM_POLICY_ATTACH_ROLE", "policy_id", policyID, "error", err)
	}
	return nil
}

func (s *iamService) DetachPolicyFromRole(ctx context.Context, roleName string, policyID uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.repo.DetachPolicyFromRole(ctx, tenantID, roleName, policyID); err != nil {
		return err
	}
	if err := s.eventSvc.RecordEvent(ctx, "IAM_POLICY_DETACH_ROLE", policyID.String(), "POLICY", map[string]interface{}{"role_name": roleName}); err != nil {
		s.logger.Warn("failed to record event", "action", "IAM_POLICY_DETACH_ROLE", "policy_id", policyID, "error", err)
	}
	return nil
}

func (s *iamService) GetPoliciesForRole(ctx context.Context, roleName string) ([]*domain.Policy, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	return s.repo.GetPoliciesForRole(ctx, tenantID, roleName)
}

func (s *iamService) AttachPolicyToServiceAccount(ctx context.Context, saID uuid.UUID, policyID uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.repo.AttachPolicyToServiceAccount(ctx, tenantID, saID, policyID); err != nil {
		return err
	}
	if err := s.eventSvc.RecordEvent(ctx, "IAM_POLICY_ATTACH_SA", policyID.String(), "POLICY", map[string]interface{}{"sa_id": saID.String()}); err != nil {
		s.logger.Warn("failed to record event", "action", "IAM_POLICY_ATTACH_SA", "policy_id", policyID, "error", err)
	}
	return nil
}

func (s *iamService) DetachPolicyFromServiceAccount(ctx context.Context, saID uuid.UUID, policyID uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.repo.DetachPolicyFromServiceAccount(ctx, tenantID, saID, policyID); err != nil {
		return err
	}
	if err := s.eventSvc.RecordEvent(ctx, "IAM_POLICY_DETACH_SA", policyID.String(), "POLICY", map[string]interface{}{"sa_id": saID.String()}); err != nil {
		s.logger.Warn("failed to record event", "action", "IAM_POLICY_DETACH_SA", "policy_id", policyID, "error", err)
	}
	return nil
}

func (s *iamService) GetPoliciesForServiceAccount(ctx context.Context, saID uuid.UUID) ([]*domain.Policy, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	return s.repo.GetPoliciesForServiceAccount(ctx, tenantID, saID)
}

// Group Management

func (s *iamService) CreateGroup(ctx context.Context, group *domain.Group) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	if group.ID == uuid.Nil {
		group.ID = uuid.New()
	}
	group.TenantID = tenantID
	if err := s.repo.CreateGroup(ctx, tenantID, group); err != nil {
		return err
	}
	if err := s.eventSvc.RecordEvent(ctx, "IAM_GROUP_CREATE", group.ID.String(), "GROUP", map[string]interface{}{"name": group.Name}); err != nil {
		s.logger.Warn("failed to record event", "action", "IAM_GROUP_CREATE", "group_id", group.ID, "error", err)
	}
	return nil
}

func (s *iamService) GetGroupByID(ctx context.Context, id uuid.UUID) (*domain.Group, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	return s.repo.GetGroupByID(ctx, tenantID, id)
}

func (s *iamService) ListGroups(ctx context.Context) ([]*domain.Group, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	return s.repo.ListGroups(ctx, tenantID)
}

func (s *iamService) UpdateGroup(ctx context.Context, group *domain.Group) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.repo.UpdateGroup(ctx, tenantID, group); err != nil {
		return err
	}
	if err := s.eventSvc.RecordEvent(ctx, "IAM_GROUP_UPDATE", group.ID.String(), "GROUP", map[string]interface{}{"name": group.Name}); err != nil {
		s.logger.Warn("failed to record event", "action", "IAM_GROUP_UPDATE", "group_id", group.ID, "error", err)
	}
	return nil
}

func (s *iamService) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.repo.DeleteGroup(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.eventSvc.RecordEvent(ctx, "IAM_GROUP_DELETE", id.String(), "GROUP", nil); err != nil {
		s.logger.Warn("failed to record event", "action", "IAM_GROUP_DELETE", "group_id", id, "error", err)
	}
	return nil
}

// Group Membership

func (s *iamService) AddUserToGroup(ctx context.Context, userID uuid.UUID, groupID uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.repo.AddUserToGroup(ctx, tenantID, userID, groupID); err != nil {
		return err
	}
	if err := s.auditSvc.Log(ctx, userID, "iam.add_to_group", "user", userID.String(), map[string]interface{}{"group_id": groupID}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "iam.add_to_group", "user_id", userID, "error", err)
	}
	return nil
}

func (s *iamService) RemoveUserFromGroup(ctx context.Context, userID uuid.UUID, groupID uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.repo.RemoveUserFromGroup(ctx, tenantID, userID, groupID); err != nil {
		return err
	}
	if err := s.auditSvc.Log(ctx, userID, "iam.remove_from_group", "user", userID.String(), map[string]interface{}{"group_id": groupID}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "iam.remove_from_group", "user_id", userID, "error", err)
	}
	return nil
}

func (s *iamService) GetGroupsForUser(ctx context.Context, userID uuid.UUID) ([]*domain.Group, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	return s.repo.GetGroupsForUser(ctx, tenantID, userID)
}

func (s *iamService) GetUsersInGroup(ctx context.Context, groupID uuid.UUID) ([]uuid.UUID, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	return s.repo.GetUsersInGroup(ctx, tenantID, groupID)
}

// Group Policy Assignment

func (s *iamService) AttachPolicyToGroup(ctx context.Context, groupID uuid.UUID, policyID uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.repo.AttachPolicyToGroup(ctx, tenantID, groupID, policyID); err != nil {
		return err
	}
	if err := s.eventSvc.RecordEvent(ctx, "IAM_POLICY_ATTACH_GROUP", policyID.String(), "POLICY", map[string]interface{}{"group_id": groupID.String()}); err != nil {
		s.logger.Warn("failed to record event", "action", "IAM_POLICY_ATTACH_GROUP", "policy_id", policyID, "error", err)
	}
	return nil
}

func (s *iamService) DetachPolicyFromGroup(ctx context.Context, groupID uuid.UUID, policyID uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.repo.DetachPolicyFromGroup(ctx, tenantID, groupID, policyID); err != nil {
		return err
	}
	if err := s.eventSvc.RecordEvent(ctx, "IAM_POLICY_DETACH_GROUP", policyID.String(), "POLICY", map[string]interface{}{"group_id": groupID.String()}); err != nil {
		s.logger.Warn("failed to record event", "action", "IAM_POLICY_DETACH_GROUP", "policy_id", policyID, "error", err)
	}
	return nil
}

func (s *iamService) GetPoliciesForGroup(ctx context.Context, groupID uuid.UUID) ([]*domain.Policy, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	return s.repo.GetPoliciesForGroup(ctx, tenantID, groupID)
}

func (s *iamService) SimulatePolicy(ctx context.Context, principal ports.Principal, actions []string, resources []string, evalCtx map[string]interface{}) (*ports.SimulateResult, error) {
	const maxSimulatePairs = 100
	if len(actions)*len(resources) > maxSimulatePairs {
		return nil, errors.New(errors.InvalidInput, "too many action-resource pairs (max 100)")
	}

	tenantID := appcontext.TenantIDFromContext(ctx)
	var policies []*domain.Policy
	var err error

	switch {
	case principal.UserID != nil:
		policies, err = s.repo.GetPoliciesForUser(ctx, tenantID, *principal.UserID)
	case principal.ServiceAccountID != nil:
		policies, err = s.repo.GetPoliciesForServiceAccount(ctx, tenantID, *principal.ServiceAccountID)
	default:
		return nil, errors.New(errors.InvalidInput, "no principal specified")
	}
	if err != nil {
		return nil, err
	}

	result := &ports.SimulateResult{Evaluated: 0}

	for _, action := range actions {
		for _, resource := range resources {
			evalResult, err := s.evaluator.Evaluate(ctx, policies, action, resource, evalCtx)
			if err != nil {
				return nil, err
			}
			result.Evaluated++

			if evalResult.Effect == domain.EffectDeny {
				result.Decision = domain.EffectDeny
				result.Matched = &ports.StatementMatch{
					Action:       action,
					Resource:     resource,
					PolicyID:     evalResult.PolicyID,
					PolicyName:   evalResult.PolicyName,
					StatementSid: evalResult.StatementSid,
					Effect:       domain.EffectDeny,
					Reason:       evalResult.Reason,
				}
				return result, nil
			}
			if evalResult.Effect == domain.EffectAllow {
				result.Decision = domain.EffectAllow
				result.Matched = &ports.StatementMatch{
					Action:       action,
					Resource:     resource,
					PolicyID:     evalResult.PolicyID,
					PolicyName:   evalResult.PolicyName,
					StatementSid: evalResult.StatementSid,
					Effect:       domain.EffectAllow,
					Reason:       evalResult.Reason,
				}
			}
		}
	}

	return result, nil
}

func (s *iamService) ListPolicyVersions(ctx context.Context, policyID uuid.UUID) ([]*domain.PolicyVersion, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	return s.repo.ListPolicyVersions(ctx, tenantID, policyID)
}

func (s *iamService) GetPolicyVersion(ctx context.Context, policyID uuid.UUID, versionNumber int) (*domain.PolicyVersion, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	return s.repo.GetPolicyVersion(ctx, tenantID, policyID, versionNumber)
}

func (s *iamService) RollbackPolicyVersion(ctx context.Context, policyID uuid.UUID, versionNumber int) error {
	tenantID := appcontext.TenantIDFromContext(ctx)

	// Fetch the version to restore
	pv, err := s.repo.GetPolicyVersion(ctx, tenantID, policyID, versionNumber)
	if err != nil {
		return err
	}

	// Get max version to determine new version number
	versions, err := s.repo.ListPolicyVersions(ctx, tenantID, policyID)
	if err != nil {
		return err
	}
	maxVersion := 0
	if len(versions) > 0 {
		maxVersion = versions[0].VersionNumber
	}

	// Create a new version with the same content as the rollback target
	newVersion := &domain.PolicyVersion{
		ID:            uuid.New(),
		PolicyID:      policyID,
		VersionNumber: maxVersion + 1,
		Name:          pv.Name,
		Description:   pv.Description,
		Statements:    pv.Statements,
	}

	// SyncPolicyCurrentState handles both inserting the version row
	// and updating the policies table for fast lookups.
	if err := s.repo.SyncPolicyCurrentState(ctx, tenantID, newVersion); err != nil {
		return err
	}

	if err := s.eventSvc.RecordEvent(ctx, "IAM_POLICY_ROLLBACK", policyID.String(), "POLICY", map[string]interface{}{
		"rolled_back_to_version": versionNumber,
		"new_version":            newVersion.VersionNumber,
	}); err != nil {
		s.logger.Warn("failed to record event", "action", "IAM_POLICY_ROLLBACK", "policy_id", policyID, "error", err)
	}
	return nil
}
