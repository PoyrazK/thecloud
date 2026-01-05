package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

type AuditService struct {
	repo ports.AuditRepository
}

func NewAuditService(repo ports.AuditRepository) *AuditService {
	return &AuditService{repo: repo}
}

func (s *AuditService) Log(ctx context.Context, userID uuid.UUID, action, resourceType, resourceID string, details map[string]interface{}) error {
	log := &domain.AuditLog{
		ID:           uuid.New(),
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Details:      details,
		CreatedAt:    time.Now(),
	}

	// In a real app, we might also get IP and UserAgent from the context/middleware
	return s.repo.Create(ctx, log)
}

func (s *AuditService) ListLogs(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.AuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.repo.ListByUserID(ctx, userID, limit)
}
