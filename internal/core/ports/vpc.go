package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyraz/cloud/internal/core/domain"
)

type VpcRepository interface {
	Create(ctx context.Context, vpc *domain.VPC) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.VPC, error)
	GetByName(ctx context.Context, name string) (*domain.VPC, error)
	List(ctx context.Context) ([]*domain.VPC, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type VpcService interface {
	CreateVPC(ctx context.Context, name string) (*domain.VPC, error)
	GetVPC(ctx context.Context, idOrName string) (*domain.VPC, error)
	ListVPCs(ctx context.Context) ([]*domain.VPC, error)
	DeleteVPC(ctx context.Context, idOrName string) error
}
