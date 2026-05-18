// Package services provides the implementation for global load balancing.
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

// GlobalLBService coordinates multi-region traffic distribution via GeoDNS.
type GlobalLBService struct {
	repo     ports.GlobalLBRepository
	rbacSvc  ports.RBACService
	lbRepo   ports.LBRepository
	geoDNS   ports.GeoDNSBackend
	auditSvc ports.AuditService
	logger   *slog.Logger
}

// GlobalLBServiceParams holds dependencies for GlobalLBService.
type GlobalLBServiceParams struct {
	Repo     ports.GlobalLBRepository
	RBAC     ports.RBACService
	LBRepo   ports.LBRepository
	GeoDNS   ports.GeoDNSBackend
	AuditSvc ports.AuditService
	Logger   *slog.Logger
}

// NewGlobalLBService creates a new instance of the global load balancer service.
func NewGlobalLBService(params GlobalLBServiceParams) *GlobalLBService {
	return &GlobalLBService{
		repo:     params.Repo,
		rbacSvc:  params.RBAC,
		lbRepo:   params.LBRepo,
		geoDNS:   params.GeoDNS,
		auditSvc: params.AuditSvc,
		logger:   params.Logger,
	}
}

func (s *GlobalLBService) Create(ctx context.Context, name, hostname string, policy domain.RoutingPolicy, healthCheck domain.GlobalHealthCheckConfig) (*domain.GlobalLoadBalancer, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionLbCreate, "*"); err != nil {
		return nil, err
	}

	// Validate inputs
	if name == "" || hostname == "" {
		return nil, errors.New(errors.InvalidInput, "name and hostname are required")
	}

	// Check for hostname uniqueness
	existing, err := s.repo.GetByHostname(ctx, hostname)
	if err != nil && !errors.Is(err, errors.NotFound) {
		return nil, errors.Wrap(errors.Internal, "failed to check hostname uniqueness", err)
	}
	if existing != nil {
		return nil, errors.New(errors.Conflict, "hostname already in use")
	}

	glb := &domain.GlobalLoadBalancer{
		ID:          uuid.New(),
		UserID:      userID,
		TenantID:    tenantID,
		Name:        name,
		Hostname:    hostname,
		Policy:      policy,
		HealthCheck: healthCheck,
		Status:      "ACTIVE",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now(),
		Endpoints:   []*domain.GlobalEndpoint{},
	}

	if err := s.repo.Create(ctx, glb); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to create global load balancer", err)
	}

	// Initialize the associated GeoDNS record set.
	// Current behavior initializes an empty record set to ensure immediate resolution readiness.
	if err := s.geoDNS.CreateGeoRecord(ctx, hostname, nil); err != nil {
		s.logger.Error("geo-dns initialization failed", "hostname", hostname, "error", err)
		// Non-blocking: failures in the DNS synchronization layer are logged for asynchronous remediation.
	}

	if err := s.auditSvc.Log(ctx, glb.UserID, "global_lb.create", "global_lb", glb.ID.String(), map[string]interface{}{
		"hostname": hostname,
		"policy":   policy,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "global_lb.create", "resource_id", glb.ID.String(), "error", err)
	}

	return glb, nil
}

func (s *GlobalLBService) Get(ctx context.Context, id uuid.UUID) (*domain.GlobalLoadBalancer, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionLbRead, id.String()); err != nil {
		return nil, err
	}

	glb, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Verify ownership
	if glb.UserID != userID || glb.TenantID != tenantID {
		return nil, errors.New(errors.Unauthorized, "unauthorized access to global load balancer")
	}

	// Load endpoints
	endpoints, err := s.repo.ListEndpoints(ctx, id)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list endpoints", err)
	}
	glb.Endpoints = endpoints

	return glb, nil
}

func (s *GlobalLBService) List(ctx context.Context, userID uuid.UUID) ([]*domain.GlobalLoadBalancer, error) {
	uID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, uID, tenantID, domain.PermissionLbRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.List(ctx, userID)
}

func (s *GlobalLBService) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	uID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, uID, tenantID, domain.PermissionLbDelete, id.String()); err != nil {
		return err
	}

	glb, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	// Ownership check is already in Get(), but Delete signature has userID.
	// Assuming Get() uses context userID, which should match the passed userID if from same context.

	// Synchronously remove the associated GeoDNS record set.
	if err := s.geoDNS.DeleteGeoRecord(ctx, glb.Hostname); err != nil {
		s.logger.Error("geo-dns record deletion failed", "hostname", glb.Hostname, "error", err)
		// Proceed with database deletion to maintain system consistency and prevent orphaned resources.
	}

	if err := s.repo.Delete(ctx, id, userID); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete global load balancer", err)
	}

	if err := s.auditSvc.Log(ctx, glb.UserID, "global_lb.delete", "global_lb", id.String(), nil); err != nil {
		s.logger.Warn("failed to log audit event", "action", "global_lb.delete", "resource_id", id.String(), "error", err)
	}

	return nil
}

func (s *GlobalLBService) AddEndpoint(ctx context.Context, glbID uuid.UUID, region string, targetType string, targetID *uuid.UUID, targetIP *string, weight, priority int) (*domain.GlobalEndpoint, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionLbUpdate, glbID.String()); err != nil {
		return nil, err
	}

	// 1. Load Global Load Balancer to verify ownership and existence
	glb, err := s.Get(ctx, glbID)
	if err != nil {
		return nil, errors.Wrap(errors.NotFound, "global load balancer not found", err)
	}

	// Ownership check is implicitly done in Get() now, but explicit check doesn't hurt if Get contract changes.
	// However, Get() above strictly enforces ownership.

	// Validate target
	switch targetType {
	case "LB":
		if targetID == nil {
			return nil, errors.New(errors.InvalidInput, "target_id required for LB endpoint")
		}
		// Verify LB exists and belongs to user
		lb, err := s.lbRepo.GetByID(ctx, *targetID)
		if err != nil {
			return nil, errors.Wrap(errors.NotFound, "target load balancer not found", err)
		}
		if lb.UserID != userID || lb.TenantID != tenantID {
			return nil, errors.New(errors.Unauthorized, "unauthorized access to regional load balancer")
		}
	case "IP":
		if targetIP == nil || *targetIP == "" {
			return nil, errors.New(errors.InvalidInput, "target_ip required for IP endpoint")
		}
	default:
		return nil, errors.New(errors.InvalidInput, "invalid target type")
	}

	ep := &domain.GlobalEndpoint{
		ID:         uuid.New(),
		GlobalLBID: glbID,
		Region:     region,
		TargetType: targetType,
		TargetID:   targetID,
		TargetIP:   targetIP,
		Weight:     weight,
		Priority:   priority,
		Healthy:    true, // Assume healthy initially
		CreatedAt:  time.Now().UTC(),
	}

	if err := s.repo.AddEndpoint(ctx, ep); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to add endpoint", err)
	}

	// Update in-memory endpoints for DNS sync
	glb.Endpoints = append(glb.Endpoints, ep)

	// Refresh DNS using the already loaded (and updated) GLB
	// We convert []*domain.GlobalEndpoint to []domain.GlobalEndpoint as expected by CreateGeoRecord
	eps := make([]domain.GlobalEndpoint, len(glb.Endpoints))
	for i, e := range glb.Endpoints {
		eps[i] = *e
	}

	if err := s.geoDNS.CreateGeoRecord(ctx, glb.Hostname, eps); err != nil {
		s.logger.Error("failed to update geo dns", "hostname", glb.Hostname, "error", err)
	}

	if err := s.auditSvc.Log(ctx, glb.UserID, "global_lb.endpoint_add", "global_lb", glbID.String(), map[string]interface{}{
		"region": region,
		"type":   targetType,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "global_lb.endpoint_add", "resource_id", glbID.String(), "error", err)
	}

	return ep, nil
}

func (s *GlobalLBService) RemoveEndpoint(ctx context.Context, glbID, endpointID uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionLbUpdate, glbID.String()); err != nil {
		return err
	}

	// 1. Get endpoint to find parent GLB and verify ownership
	ep, err := s.repo.GetEndpointByID(ctx, endpointID)
	if err != nil {
		return err
	}

	// Verify consistent GLB ID
	if ep.GlobalLBID != glbID {
		return errors.New(errors.InvalidInput, "endpoint does not belong to the specified global load balancer")
	}

	glb, err := s.Get(ctx, glbID)
	if err != nil {
		return err
	}

	if glb.UserID != userID || glb.TenantID != tenantID {
		return errors.New(errors.Unauthorized, "unauthorized access to global load balancer endpoint")
	}

	// 2. Remove from repo
	if err := s.repo.RemoveEndpoint(ctx, endpointID); err != nil {
		return err
	}

	// 3. Sync DNS
	updatedGLB, err := s.Get(ctx, glb.ID)
	if err == nil {
		eps := make([]domain.GlobalEndpoint, len(updatedGLB.Endpoints))
		for i, e := range updatedGLB.Endpoints {
			eps[i] = *e
		}

		if err := s.geoDNS.CreateGeoRecord(ctx, updatedGLB.Hostname, eps); err != nil {
			s.logger.Error("failed to update geo dns after endpoint removal", "hostname", updatedGLB.Hostname, "error", err)
		}
	}

	if err := s.auditSvc.Log(ctx, userID, "global_lb.endpoint_remove", "global_lb", glb.ID.String(), map[string]interface{}{
		"endpoint_id": endpointID.String(),
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "global_lb.endpoint_remove", "resource_id", glb.ID.String(), "error", err)
	}

	return nil
}

func (s *GlobalLBService) ListEndpoints(ctx context.Context, glbID uuid.UUID) ([]*domain.GlobalEndpoint, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionLbRead, glbID.String()); err != nil {
		return nil, err
	}

	// Verify ownership and existence
	if _, err := s.Get(ctx, glbID); err != nil {
		return nil, err
	}
	return s.repo.ListEndpoints(ctx, glbID)
}
