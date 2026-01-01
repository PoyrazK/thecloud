package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyraz/cloud/internal/core/domain"
)

type VolumeRepository interface {
	Create(ctx context.Context, v *domain.Volume) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Volume, error)
	GetByName(ctx context.Context, name string) (*domain.Volume, error)
	List(ctx context.Context) ([]*domain.Volume, error)
	Update(ctx context.Context, v *domain.Volume) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type VolumeService interface {
	CreateVolume(ctx context.Context, name string, sizeGB int) (*domain.Volume, error)
	ListVolumes(ctx context.Context) ([]*domain.Volume, error)
	GetVolume(ctx context.Context, idOrName string) (*domain.Volume, error)
	DeleteVolume(ctx context.Context, idOrName string) error
}
