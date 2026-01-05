package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

type AuditRepository interface {
	Create(ctx context.Context, log *domain.AuditLog) error
	ListByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.AuditLog, error)
}

type AuditService interface {
	Log(ctx context.Context, userID uuid.UUID, action, resourceType, resourceID string, details map[string]interface{}) error
	ListLogs(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.AuditLog, error)
}
