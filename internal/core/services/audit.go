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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// AuditServiceParams defines the dependencies for AuditService.
type AuditServiceParams struct {
	Repo    ports.AuditRepository
	RBACSvc ports.RBACService
	Logger  *slog.Logger
}

// AuditService records user actions for compliance and tracing.
type AuditService struct {
	repo    ports.AuditRepository
	rbacSvc ports.RBACService
	logger  *slog.Logger
}

// NewAuditService constructs an audit service for persisting audit logs.
func NewAuditService(params AuditServiceParams) *AuditService {
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &AuditService{
		repo:    params.Repo,
		rbacSvc: params.RBACSvc,
		logger:  logger,
	}
}

func (s *AuditService) Log(ctx context.Context, userID uuid.UUID, action, resourceType, resourceID string, details map[string]interface{}) error {
	ctx, span := otel.Tracer("audit-service").Start(ctx, "Log")
	defer span.End()

	tenantID := appcontext.TenantIDFromContext(ctx)

	span.SetAttributes(
		attribute.String("audit.action", action),
		attribute.String("audit.resource_type", resourceType),
		attribute.String("audit.resource_id", resourceID),
	)
	log := &domain.AuditLog{
		ID:           uuid.New(),
		UserID:       userID,
		TenantID:     tenantID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Details:      details,
		CreatedAt:    time.Now().UTC(),
	}

	err := s.repo.Create(ctx, log)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("failed to create audit log", "error", err, "action", action, "user_id", userID)
		}
	}
	return err
}

func (s *AuditService) ListLogs(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.AuditLog, error) {
	uID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, uID, tenantID, domain.PermissionAuditRead, "*"); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 50
	}
	return s.repo.ListByUserID(ctx, userID, limit)
}
