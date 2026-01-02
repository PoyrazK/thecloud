package ports

import (
	"context"

	"github.com/poyrazk/thecloud/internal/core/domain"
)

// AuditLogger interface for logging security events
type AuditLogger interface {
	Log(ctx context.Context, entry *domain.AuditLog) error
}
