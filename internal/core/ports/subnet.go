package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

type SubnetRepository interface {
	Create(ctx context.Context, subnet *domain.Subnet) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Subnet, error)
	GetByName(ctx context.Context, vpcID uuid.UUID, name string) (*domain.Subnet, error)
	ListByVPC(ctx context.Context, vpcID uuid.UUID) ([]*domain.Subnet, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type SubnetService interface {
	CreateSubnet(ctx context.Context, vpcID uuid.UUID, name, cidrBlock, az string) (*domain.Subnet, error)
	GetSubnet(ctx context.Context, idOrName string, vpcID uuid.UUID) (*domain.Subnet, error)
	ListSubnets(ctx context.Context, vpcID uuid.UUID) ([]*domain.Subnet, error)
	DeleteSubnet(ctx context.Context, id uuid.UUID) error
}
