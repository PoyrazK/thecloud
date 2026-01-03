package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

type CreateQueueOptions struct {
	VisibilityTimeout *int
	RetentionDays     *int
	MaxMessageSize    *int
}

type QueueRepository interface {
	Create(ctx context.Context, queue *domain.Queue) error
	GetByID(ctx context.Context, id, userID uuid.UUID) (*domain.Queue, error)
	GetByName(ctx context.Context, name string, userID uuid.UUID) (*domain.Queue, error)
	List(ctx context.Context, userID uuid.UUID) ([]*domain.Queue, error)
	Delete(ctx context.Context, id uuid.UUID) error

	// Messages
	SendMessage(ctx context.Context, queueID uuid.UUID, body string) (*domain.Message, error)
	ReceiveMessages(ctx context.Context, queueID uuid.UUID, maxMessages, visibilityTimeout int) ([]*domain.Message, error)
	DeleteMessage(ctx context.Context, queueID uuid.UUID, receiptHandle string) error
	PurgeMessages(ctx context.Context, queueID uuid.UUID) (int64, error)
}

type QueueService interface {
	CreateQueue(ctx context.Context, name string, opts *CreateQueueOptions) (*domain.Queue, error)
	GetQueue(ctx context.Context, id uuid.UUID) (*domain.Queue, error)
	ListQueues(ctx context.Context) ([]*domain.Queue, error)
	DeleteQueue(ctx context.Context, id uuid.UUID) error

	// Messages
	SendMessage(ctx context.Context, queueID uuid.UUID, body string) (*domain.Message, error)
	ReceiveMessages(ctx context.Context, queueID uuid.UUID, maxMessages int) ([]*domain.Message, error)
	DeleteMessage(ctx context.Context, queueID uuid.UUID, receiptHandle string) error
	PurgeQueue(ctx context.Context, queueID uuid.UUID) error
}
