// Package services implements core business workflows.
package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
)

// LBService manages load balancers and target registration.
type LBService struct {
	lbRepo       ports.LBRepository
	rbacSvc      ports.RBACService
	vpcRepo      ports.VpcRepository
	instanceRepo ports.InstanceRepository
	auditSvc     ports.AuditService
	logger       *slog.Logger
}

// NewLBService constructs an LBService with its dependencies.
func NewLBService(lbRepo ports.LBRepository, rbacSvc ports.RBACService, vpcRepo ports.VpcRepository, instanceRepo ports.InstanceRepository, auditSvc ports.AuditService, logger *slog.Logger) *LBService {
	if logger == nil {
		logger = slog.Default()
	}
	return &LBService{
		lbRepo:       lbRepo,
		rbacSvc:      rbacSvc,
		vpcRepo:      vpcRepo,
		instanceRepo: instanceRepo,
		auditSvc:     auditSvc,
		logger:       logger,
	}
}

func (s *LBService) Create(ctx context.Context, name string, vpcID uuid.UUID, port int, algo string, idempotencyKey string) (*domain.LoadBalancer, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	// Defence-in-depth: the HTTP handler validates min=1,max=65535, but other
	// callers (CLI, tests, future internal services) can hit Create directly.
	if port < 1 || port > 65535 {
		return nil, errors.New(errors.InvalidInput, "port must be in 1..65535")
	}

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionLbCreate, "*"); err != nil {
		return nil, err
	}

	// Check if already created via idempotency key
	if idempotencyKey != "" {
		existing, err := s.lbRepo.GetByIdempotencyKey(ctx, idempotencyKey)
		if err == nil {
			return existing, nil
		}
	}

	// Validate VPC exists
	_, err := s.vpcRepo.GetByID(ctx, vpcID)
	if err != nil {
		return nil, errors.Wrap(errors.NotFound, "VPC not found", err)
	}

	// Set default algorithm
	if algo == "" {
		algo = "round-robin"
	}

	lb := &domain.LoadBalancer{
		ID:             uuid.New(),
		UserID:         userID,
		TenantID:       tenantID,
		IdempotencyKey: idempotencyKey,
		Name:           name,
		VpcID:          vpcID,
		Port:           port,
		Algorithm:      algo,
		Status:         domain.LBStatusCreating,
		Version:        1,
		CreatedAt:      time.Now(),
	}

	if err := s.lbRepo.Create(ctx, lb); err != nil {
		return nil, err
	}

	if err := s.auditSvc.Log(ctx, lb.UserID, "lb.create", "loadbalancer", lb.ID.String(), map[string]interface{}{
		"name": lb.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "lb.create", "lb_id", lb.ID, "error", err)
	}

	return lb, nil
}

func (s *LBService) Get(ctx context.Context, idOrName string) (*domain.LoadBalancer, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionLbRead, idOrName); err != nil {
		return nil, err
	}

	// 1. Try UUID lookup
	if id, err := uuid.Parse(idOrName); err == nil {
		return s.lbRepo.GetByID(ctx, id)
	}
	// 2. Fallback to name lookup
	return s.lbRepo.GetByName(ctx, idOrName)
}

func (s *LBService) List(ctx context.Context) ([]*domain.LoadBalancer, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionLbRead, "*"); err != nil {
		return nil, err
	}

	return s.lbRepo.List(ctx)
}

func (s *LBService) Delete(ctx context.Context, idOrName string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionLbDelete, idOrName); err != nil {
		return err
	}

	// 1. Get from DB (handles both Name and UUID)
	lb, err := s.Get(ctx, idOrName)
	if err != nil {
		return err
	}

	lb.Status = domain.LBStatusDeleted
	if err := s.lbRepo.Update(ctx, lb); err != nil {
		return err
	}

	if err := s.auditSvc.Log(ctx, lb.UserID, "lb.delete", "loadbalancer", lb.ID.String(), map[string]interface{}{
		"name": lb.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "lb.delete", "lb_id", lb.ID, "error", err)
	}

	return nil
}

func (s *LBService) AddTarget(ctx context.Context, lbID, instanceID uuid.UUID, port int, weight int) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if port < 1 || port > 65535 {
		return errors.New(errors.InvalidInput, "port must be in 1..65535")
	}

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionLbUpdate, lbID.String()); err != nil {
		return err
	}

	// Get LB
	lb, err := s.lbRepo.GetByID(ctx, lbID)
	if err != nil {
		return err
	}

	// Get Instance
	inst, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return errors.Wrap(errors.NotFound, "target instance not found", err)
	}

	// Validate cross-VPC
	if inst.VpcID == nil || *inst.VpcID != lb.VpcID {
		return errors.ErrLBCrossVPC
	}

	if weight == 0 {
		weight = 1
	}

	target := &domain.LBTarget{
		ID:         uuid.New(),
		LBID:       lbID,
		InstanceID: instanceID,
		Port:       port,
		Weight:     weight,
		Health:     "unknown",
	}

	if err := s.lbRepo.AddTarget(ctx, target); err != nil {
		return err
	}

	if err := s.auditSvc.Log(ctx, lb.UserID, "lb.target_add", "loadbalancer", lb.ID.String(), map[string]interface{}{
		"instance_id": instanceID.String(),
		"port":        port,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "lb.target_add", "lb_id", lb.ID, "error", err)
	}

	return nil
}

func (s *LBService) RemoveTarget(ctx context.Context, lbID, instanceID uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionLbUpdate, lbID.String()); err != nil {
		return err
	}

	lb, err := s.lbRepo.GetByID(ctx, lbID)
	if err != nil {
		return err
	}

	if err := s.lbRepo.RemoveTarget(ctx, lbID, instanceID); err != nil {
		return err
	}

	if err := s.auditSvc.Log(ctx, lb.UserID, "lb.target_remove", "loadbalancer", lb.ID.String(), map[string]interface{}{
		"instance_id": instanceID.String(),
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "lb.target_remove", "lb_id", lb.ID, "error", err)
	}

	return nil
}

func (s *LBService) ListTargets(ctx context.Context, lbID uuid.UUID) ([]*domain.LBTarget, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionLbRead, lbID.String()); err != nil {
		return nil, err
	}

	return s.lbRepo.ListTargets(ctx, lbID)
}
