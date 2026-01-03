package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

type QueueService struct {
	repo     ports.QueueRepository
	eventSvc ports.EventService
}

func NewQueueService(repo ports.QueueRepository, eventSvc ports.EventService) ports.QueueService {
	return &QueueService{repo: repo, eventSvc: eventSvc}
}

func (s *QueueService) CreateQueue(ctx context.Context, name string, opts *ports.CreateQueueOptions) (*domain.Queue, error) {
	userID, err := appcontext.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Check if already exists
	existing, err := s.repo.GetByName(ctx, name, userID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("queue with name %s already exists", name)
	}

	qID := uuid.New()
	q := &domain.Queue{
		ID:                qID,
		UserID:            userID,
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

	_ = s.eventSvc.RecordEvent(ctx, "QUEUE_CREATED", q.ID.String(), "QUEUE", nil)

	return q, nil
}

func (s *QueueService) GetQueue(ctx context.Context, id uuid.UUID) (*domain.Queue, error) {
	userID, err := appcontext.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	q, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, fmt.Errorf("queue not found")
	}

	return q, nil
}

func (s *QueueService) ListQueues(ctx context.Context) ([]*domain.Queue, error) {
	userID, err := appcontext.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.List(ctx, userID)
}

func (s *QueueService) DeleteQueue(ctx context.Context, id uuid.UUID) error {
	q, err := s.GetQueue(ctx, id)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, q.ID); err != nil {
		return err
	}

	_ = s.eventSvc.RecordEvent(ctx, "QUEUE_DELETED", q.ID.String(), "QUEUE", nil)

	return nil
}

func (s *QueueService) SendMessage(ctx context.Context, queueID uuid.UUID, body string) (*domain.Message, error) {
	q, err := s.GetQueue(ctx, queueID)
	if err != nil {
		return nil, err
	}

	if len(body) > q.MaxMessageSize {
		return nil, fmt.Errorf("message size exceeds limit of %d bytes", q.MaxMessageSize)
	}

	m, err := s.repo.SendMessage(ctx, q.ID, body)
	if err != nil {
		return nil, err
	}

	_ = s.eventSvc.RecordEvent(ctx, "MESSAGE_SENT", m.ID.String(), "MESSAGE", map[string]interface{}{"queue_id": q.ID})

	return m, nil
}

func (s *QueueService) ReceiveMessages(ctx context.Context, queueID uuid.UUID, maxMessages int) ([]*domain.Message, error) {
	q, err := s.GetQueue(ctx, queueID)
	if err != nil {
		return nil, err
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
		_ = s.eventSvc.RecordEvent(ctx, "MESSAGE_RECEIVED", m.ID.String(), "MESSAGE", map[string]interface{}{"queue_id": q.ID})
	}

	return msgs, nil
}

func (s *QueueService) DeleteMessage(ctx context.Context, queueID uuid.UUID, receiptHandle string) error {
	q, err := s.GetQueue(ctx, queueID)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteMessage(ctx, q.ID, receiptHandle); err != nil {
		return err
	}

	_ = s.eventSvc.RecordEvent(ctx, "MESSAGE_DELETED", receiptHandle, "MESSAGE", map[string]interface{}{"queue_id": q.ID})

	return nil
}

func (s *QueueService) PurgeQueue(ctx context.Context, queueID uuid.UUID) error {
	q, err := s.GetQueue(ctx, queueID)
	if err != nil {
		return err
	}

	_, err = s.repo.PurgeMessages(ctx, q.ID)
	if err != nil {
		return err
	}

	_ = s.eventSvc.RecordEvent(ctx, "QUEUE_PURGED", q.ID.String(), "QUEUE", nil)

	return nil
}
