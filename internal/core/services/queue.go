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
	"github.com/poyrazk/thecloud/internal/platform"
)

// QueueService manages queue resources and message operations.
type QueueService struct {
	repo     ports.QueueRepository
	rbacSvc  ports.RBACService
	eventSvc ports.EventService
	auditSvc ports.AuditService
	logger   *slog.Logger
}

// NewQueueService constructs a QueueService with its dependencies.
func NewQueueService(repo ports.QueueRepository, rbacSvc ports.RBACService, eventSvc ports.EventService, auditSvc ports.AuditService, logger *slog.Logger) ports.QueueService {
	return &QueueService{
		repo:     repo,
		rbacSvc:  rbacSvc,
		eventSvc: eventSvc,
		auditSvc: auditSvc,
		logger:   logger,
	}
}

func (s *QueueService) CreateQueue(ctx context.Context, name string, opts *ports.CreateQueueOptions) (*domain.Queue, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionQueueCreate, "*"); err != nil {
		return nil, err
	}

	// Check if already exists
	existing, err := s.repo.GetByName(ctx, name, tenantID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New(errors.Conflict, fmt.Sprintf("queue with name %s already exists", name))
	}

	qID := uuid.New()
	q := &domain.Queue{
		ID:                qID,
		UserID:            userID,
		TenantID:          tenantID,
		Name:              name,
		ARN:               fmt.Sprintf("arn:thecloud:queue:local:%s:queue/%s", userID, qID),
		VisibilityTimeout: 30,
		RetentionDays:     4,
		MaxMessageSize:    262144, // 256KB
		Status:            domain.QueueStatusActive,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if opts != nil {
		if opts.VisibilityTimeout != nil {
			q.VisibilityTimeout = *opts.VisibilityTimeout
		}
		if opts.RetentionDays != nil {
			q.RetentionDays = *opts.RetentionDays
		}
		if opts.MaxMessageSize != nil {
			q.MaxMessageSize = *opts.MaxMessageSize
		}
	}

	if err := s.repo.Create(ctx, q); err != nil {
		return nil, err
	}

	if err := s.eventSvc.RecordEvent(ctx, "QUEUE_CREATED", q.ID.String(), "QUEUE", nil); err != nil {
		s.logger.Warn("failed to record event", "action", "QUEUE_CREATED", "queue_id", q.ID, "error", err)
	}

	if err := s.auditSvc.Log(ctx, q.UserID, "queue.create", "queue", q.ID.String(), map[string]interface{}{
		"name": q.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "queue.create", "queue_id", q.ID, "error", err)
	}

	return q, nil
}

func (s *QueueService) GetQueue(ctx context.Context, id uuid.UUID) (*domain.Queue, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionQueueRead, id.String()); err != nil {
		return nil, err
	}

	q, err := s.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, errors.New(errors.NotFound, "queue not found")
	}

	return q, nil
}

func (s *QueueService) ListQueues(ctx context.Context) ([]*domain.Queue, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionQueueRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.List(ctx, tenantID)
}

func (s *QueueService) DeleteQueue(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionQueueDelete, id.String()); err != nil {
		return err
	}

	q, err := s.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		return err
	}
	if q == nil {
		return errors.New(errors.NotFound, "queue not found")
	}

	if err := s.repo.Delete(ctx, q.ID, tenantID); err != nil {
		return err
	}

	if err := s.eventSvc.RecordEvent(ctx, "QUEUE_DELETED", q.ID.String(), "QUEUE", nil); err != nil {
		s.logger.Warn("failed to record event", "action", "QUEUE_DELETED", "queue_id", id, "error", err)
	}

	if err := s.auditSvc.Log(ctx, q.UserID, "queue.delete", "queue", q.ID.String(), map[string]interface{}{
		"name": q.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "queue.delete", "queue_id", id, "error", err)
	}

	return nil
}

func (s *QueueService) SendMessage(ctx context.Context, queueID uuid.UUID, body string) (*domain.Message, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionQueueWrite, queueID.String()); err != nil {
		return nil, err
	}

	q, err := s.repo.GetByID(ctx, queueID, tenantID)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, errors.New(errors.NotFound, "queue not found")
	}

	if len(body) > q.MaxMessageSize {
		return nil, errors.New(errors.InvalidInput, fmt.Sprintf("message size exceeds limit of %d bytes", q.MaxMessageSize))
	}

	m, err := s.repo.SendMessage(ctx, q.ID, body)
	if err != nil {
		return nil, err
	}

	if err := s.eventSvc.RecordEvent(ctx, "MESSAGE_SENT", m.ID.String(), "MESSAGE", map[string]interface{}{"queue_id": q.ID}); err != nil {
		s.logger.Warn("failed to record event", "action", "MESSAGE_SENT", "message_id", m.ID, "error", err)
	}

	platform.QueueMessagesTotal.WithLabelValues(q.ID.String(), "send").Inc()

	return m, nil
}

func (s *QueueService) ReceiveMessages(ctx context.Context, queueID uuid.UUID, maxMessages int) ([]*domain.Message, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionQueueRead, queueID.String()); err != nil {
		return nil, err
	}

	q, err := s.repo.GetByID(ctx, queueID, tenantID)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, errors.New(errors.NotFound, "queue not found")
	}

	if maxMessages <= 0 {
		maxMessages = 1
	}
	if maxMessages > 10 {
		maxMessages = 10
	}

	msgs, err := s.repo.ReceiveMessages(ctx, q.ID, maxMessages, q.VisibilityTimeout)
	if err != nil {
		return nil, err
	}

	for _, m := range msgs {
		if err := s.eventSvc.RecordEvent(ctx, "MESSAGE_RECEIVED", m.ID.String(), "MESSAGE", map[string]interface{}{"queue_id": q.ID}); err != nil {
			s.logger.Warn("failed to record event", "action", "MESSAGE_RECEIVED", "message_id", m.ID, "error", err)
		}
		platform.QueueMessagesTotal.WithLabelValues(q.ID.String(), "receive").Inc()
	}

	return msgs, nil
}

func (s *QueueService) DeleteMessage(ctx context.Context, queueID uuid.UUID, receiptHandle string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionQueueWrite, queueID.String()); err != nil {
		return err
	}

	q, err := s.repo.GetByID(ctx, queueID, tenantID)
	if err != nil {
		return err
	}
	if q == nil {
		return errors.New(errors.NotFound, "queue not found")
	}

	if err := s.repo.DeleteMessage(ctx, q.ID, receiptHandle); err != nil {
		return err
	}

	if err := s.eventSvc.RecordEvent(ctx, "MESSAGE_DELETED", receiptHandle, "MESSAGE", map[string]interface{}{"queue_id": q.ID}); err != nil {
		s.logger.Warn("failed to record event", "action", "MESSAGE_DELETED", "receipt_handle", receiptHandle, "error", err)
	}

	platform.QueueMessagesTotal.WithLabelValues(q.ID.String(), "delete").Inc()

	return nil
}

func (s *QueueService) PurgeQueue(ctx context.Context, queueID uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionQueueWrite, queueID.String()); err != nil {
		return err
	}

	q, err := s.repo.GetByID(ctx, queueID, tenantID)
	if err != nil {
		return err
	}
	if q == nil {
		return errors.New(errors.NotFound, "queue not found")
	}

	_, err = s.repo.PurgeMessages(ctx, q.ID)
	if err != nil {
		return err
	}

	if err := s.eventSvc.RecordEvent(ctx, "QUEUE_PURGED", q.ID.String(), "QUEUE", nil); err != nil {
		s.logger.Warn("failed to record event", "action", "QUEUE_PURGED", "queue_id", q.ID, "error", err)
	}

	if err := s.auditSvc.Log(ctx, q.UserID, "queue.purge", "queue", q.ID.String(), map[string]interface{}{}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "queue.purge", "queue_id", q.ID, "error", err)
	}

	return nil
}
