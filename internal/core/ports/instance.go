package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyraz/cloud/internal/core/domain"
)

// InstanceRepository defines the interface for data persistence.
type InstanceRepository interface {
	Create(ctx context.Context, instance *domain.Instance) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Instance, error)
	GetByName(ctx context.Context, name string) (*domain.Instance, error)
	List(ctx context.Context) ([]*domain.Instance, error)
	Update(ctx context.Context, instance *domain.Instance) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// InstanceService defines the business logic interface.
type InstanceService interface {
	LaunchInstance(ctx context.Context, name, image, ports string, vpcID *uuid.UUID) (*domain.Instance, error)
	StopInstance(ctx context.Context, idOrName string) error
	ListInstances(ctx context.Context) ([]*domain.Instance, error)
	GetInstance(ctx context.Context, idOrName string) (*domain.Instance, error)
	GetInstanceLogs(ctx context.Context, idOrName string) (string, error)
	GetInstanceStats(ctx context.Context, idOrName string) (*domain.InstanceStats, error)
	TerminateInstance(ctx context.Context, idOrName string) error
}
