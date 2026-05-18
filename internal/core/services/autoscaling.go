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
)

// AutoScalingService manages scaling groups and policies.
type AutoScalingService struct {
	repo     ports.AutoScalingRepository
	rbacSvc  ports.RBACService
	vpcRepo  ports.VpcRepository
	auditSvc ports.AuditService
	logger   *slog.Logger
}

// NewAutoScalingService constructs an AutoScalingService with its dependencies.
func NewAutoScalingService(repo ports.AutoScalingRepository, rbacSvc ports.RBACService, vpcRepo ports.VpcRepository, auditSvc ports.AuditService, logger *slog.Logger) *AutoScalingService {
	return &AutoScalingService{
		repo:     repo,
		rbacSvc:  rbacSvc,
		vpcRepo:  vpcRepo,
		auditSvc: auditSvc,
		logger:   logger,
	}
}

func (s *AutoScalingService) CreateGroup(ctx context.Context, params ports.CreateScalingGroupParams) (*domain.ScalingGroup, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionAsgCreate, "*"); err != nil {
		return nil, err
	}

	// Idempotency check. The existing record must match the current request,
	// otherwise we'd silently return a different group than the caller asked
	// for — which can happen if a caller reuses an idempotency key across
	// distinct create requests.
	if params.IdempotencyKey != "" {
		if existing, err := s.repo.GetGroupByIdempotencyKey(ctx, params.IdempotencyKey); err == nil && existing != nil {
			if err := scalingGroupMatchesCreate(existing, params); err != nil {
				return nil, err
			}
			return existing, nil
		}
	}

	// Validation
	if params.MaxInstances > domain.MaxInstancesHardLimit {
		return nil, errors.New(errors.InvalidInput, fmt.Sprintf("max_instances cannot exceed %d", domain.MaxInstancesHardLimit))
	}
	if params.MinInstances < 0 {
		return nil, errors.New(errors.InvalidInput, "min_instances cannot be negative")
	}
	if params.MinInstances > params.MaxInstances {
		return nil, errors.New(errors.InvalidInput, "min_instances cannot be greater than max_instances")
	}
	if params.DesiredCount < params.MinInstances || params.DesiredCount > params.MaxInstances {
		return nil, errors.New(errors.InvalidInput, "desired_count must be between min and max instances")
	}

	// Check VPC exists
	if _, err := s.vpcRepo.GetByID(ctx, params.VpcID); err != nil {
		return nil, err
	}

	// Security: Check VPC group limit
	count, err := s.repo.CountGroupsByVPC(ctx, params.VpcID)
	if err != nil {
		return nil, err
	}
	if count >= domain.MaxScalingGroupsPerVPC {
		return nil, errors.New(errors.ResourceLimitExceeded, fmt.Sprintf("VPC already has %d scaling groups (max: %d)", count, domain.MaxScalingGroupsPerVPC))
	}

	group := &domain.ScalingGroup{
		ID:             uuid.New(),
		UserID:         userID,
		TenantID:       tenantID,
		IdempotencyKey: params.IdempotencyKey,
		Name:           params.Name,
		VpcID:          params.VpcID,
		LoadBalancerID: params.LoadBalancerID,
		Image:          params.Image,
		Ports:          params.Ports,
		MinInstances:   params.MinInstances,
		MaxInstances:   params.MaxInstances,
		DesiredCount:   params.DesiredCount,
		CurrentCount:   0, // Worker will spawn these
		Status:         domain.ScalingGroupStatusActive,
		Version:        1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.repo.CreateGroup(ctx, group); err != nil {
		return nil, err
	}

	if err := s.auditSvc.Log(ctx, group.UserID, "asg.group_create", "scaling_group", group.ID.String(), map[string]interface{}{
		"name": group.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "asg.group_create", "group_id", group.ID, "error", err)
	}

	return group, nil
}

func (s *AutoScalingService) GetGroup(ctx context.Context, id uuid.UUID) (*domain.ScalingGroup, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionAsgRead, id.String()); err != nil {
		return nil, err
	}

	return s.repo.GetGroupByID(ctx, id)
}

func (s *AutoScalingService) ListGroups(ctx context.Context) ([]*domain.ScalingGroup, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionAsgRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.ListGroups(ctx)
}

func (s *AutoScalingService) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionAsgDelete, id.String()); err != nil {
		return err
	}

	group, err := s.repo.GetGroupByID(ctx, id)
	if err != nil {
		return err
	}

	// Mark as DELETING to let worker handle cleanup asynchronously.
	//
	// MinInstances and DesiredCount are forced to 0 here as a deletion signal,
	// NOT to represent "this group is unscaled". Callers must always check
	// group.Status before interpreting these counts: a group in
	// ScalingGroupStatusDeleting with zero counts is being torn down by the
	// scaling worker, not idling. GetGroup returns the Status field so
	// downstream code can distinguish.
	group.Status = domain.ScalingGroupStatusDeleting
	group.MinInstances = 0
	group.DesiredCount = 0
	if err := s.repo.UpdateGroup(ctx, group); err != nil {
		return err
	}

	if err := s.auditSvc.Log(ctx, group.UserID, "asg.group_delete", "scaling_group", group.ID.String(), map[string]interface{}{
		"name": group.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "asg.group_delete", "group_id", group.ID, "error", err)
	}

	return nil
}

// SetDesiredCapacity just updates the DB. Worker reconciles.
func (s *AutoScalingService) SetDesiredCapacity(ctx context.Context, groupID uuid.UUID, desired int) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionAsgUpdate, groupID.String()); err != nil {
		return err
	}

	group, err := s.repo.GetGroupByID(ctx, groupID)
	if err != nil {
		return err
	}

	if group.Status == domain.ScalingGroupStatusDeleting {
		return errors.New(errors.InvalidInput, "cannot set desired capacity: group is being deleted")
	}

	if desired < group.MinInstances || desired > group.MaxInstances {
		return errors.New(errors.InvalidInput, fmt.Sprintf("desired must be between %d and %d", group.MinInstances, group.MaxInstances))
	}

	group.DesiredCount = desired
	return s.repo.UpdateGroup(ctx, group)
}

func (s *AutoScalingService) CreatePolicy(ctx context.Context, params ports.CreateScalingPolicyParams) (*domain.ScalingPolicy, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionAsgUpdate, "*"); err != nil {
		return nil, err
	}

	if _, err := s.repo.GetGroupByID(ctx, params.GroupID); err != nil {
		return nil, err
	}

	if params.CooldownSec < domain.MinCooldownSeconds {
		return nil, errors.New(errors.InvalidInput, fmt.Sprintf("cooldown must be at least %d seconds", domain.MinCooldownSeconds))
	}

	policy := &domain.ScalingPolicy{
		ID:             uuid.New(),
		ScalingGroupID: params.GroupID,
		Name:           params.Name,
		MetricType:     params.MetricType,
		TargetValue:    params.TargetValue,
		ScaleOutStep:   params.ScaleOut,
		ScaleInStep:    params.ScaleIn,
		CooldownSec:    params.CooldownSec,
	}

	if err := s.repo.CreatePolicy(ctx, policy); err != nil {
		return nil, err
	}
	return policy, nil
}

func (s *AutoScalingService) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionAsgUpdate, id.String()); err != nil {
		return err
	}

	return s.repo.DeletePolicy(ctx, id)
}

// scalingGroupMatchesCreate verifies that the parameters of a new CreateGroup
// request match the previously-stored group keyed by the same idempotency key.
// Returns Conflict if they differ, so an honest caller never gets back a group
// they wouldn't recognise and a buggy caller is told about their key reuse.
func scalingGroupMatchesCreate(existing *domain.ScalingGroup, params ports.CreateScalingGroupParams) error {
	switch {
	case existing.Name != params.Name:
		return errors.New(errors.Conflict, "idempotency_key already used with different name")
	case existing.VpcID != params.VpcID:
		return errors.New(errors.Conflict, "idempotency_key already used with different vpc_id")
	case existing.LoadBalancerID != params.LoadBalancerID:
		return errors.New(errors.Conflict, "idempotency_key already used with different load_balancer_id")
	case existing.Image != params.Image:
		return errors.New(errors.Conflict, "idempotency_key already used with different image")
	case existing.MinInstances != params.MinInstances:
		return errors.New(errors.Conflict, "idempotency_key already used with different min_instances")
	case existing.MaxInstances != params.MaxInstances:
		return errors.New(errors.Conflict, "idempotency_key already used with different max_instances")
	case existing.DesiredCount != params.DesiredCount:
		return errors.New(errors.Conflict, "idempotency_key already used with different desired_count")
	}
	return nil
}
