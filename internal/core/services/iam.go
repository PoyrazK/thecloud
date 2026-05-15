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

func (s *iamService) SimulatePolicy(ctx context.Context, principal ports.Principal, actions []string, resources []string, evalCtx map[string]interface{}) (*ports.SimulateResult, error) {
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

	const maxSimulatePairs = 100
	if len(actions)*len(resources) > maxSimulatePairs {
		return nil, errors.New(errors.InvalidInput, "too many action-resource pairs (max 100)")
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
					Action:      action,
					Resource:    resource,
					PolicyID:    evalResult.PolicyID,
					PolicyName:  evalResult.PolicyName,
					StatementSid: evalResult.StatementSid,
					Effect:      domain.EffectDeny,
					Reason:      evalResult.Reason,
				}
				return result, nil
			}
			if evalResult.Effect == domain.EffectAllow {
				result.Decision = domain.EffectAllow
				result.Matched = &ports.StatementMatch{
					Action:      action,
					Resource:    resource,
					PolicyID:    evalResult.PolicyID,
					PolicyName:  evalResult.PolicyName,
					StatementSid: evalResult.StatementSid,
					Effect:      domain.EffectAllow,
					Reason:      evalResult.Reason,
				}
			}
		}
	}

	return result, nil
}
