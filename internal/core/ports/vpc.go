// Package ports defines service and repository interfaces.
package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

// VpcRepository manages the persistent state of Virtual Private Clouds (VPCs).
type VpcRepository interface {
	// Create saves a new VPC configuration record.
	Create(ctx context.Context, vpc *domain.VPC) error
	// GetByID retrieves a VPC definition by its unique UUID.
	GetByID(ctx context.Context, id uuid.UUID) (*domain.VPC, error)
	// GetByName retrieves a VPC definition by its friendly name (scoped to authorized user).
	GetByName(ctx context.Context, name string) (*domain.VPC, error)
	// GetByIdempotencyKey retrieves a VPC by its idempotency key (scoped to user).
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.VPC, error)
	// List returns all VPCs accessible to the current user context.
	List(ctx context.Context) ([]*domain.VPC, error)
	// Update modifies an existing VPC record.
	Update(ctx context.Context, vpc *domain.VPC) error
	// Delete removes a VPC definition from storage.
	Delete(ctx context.Context, id uuid.UUID) error
}

// VpcService provides business logic for managing isolated virtual networks.
type VpcService interface {
	// CreateVPC establishes a new isolated virtual network with a specific address space.
	CreateVPC(ctx context.Context, name, cidrBlock, idempotencyKey string) (*domain.VPC, error)
	// GetVPC retrieves detailed information for a specific virtual network.
	GetVPC(ctx context.Context, idOrName string) (*domain.VPC, error)
	// ListVPCs returns all virtual networks registered to the current authorized user.
	ListVPCs(ctx context.Context) ([]*domain.VPC, error)
	// UpdateVPC modifies an existing VPC's name.
	UpdateVPC(ctx context.Context, idOrName, name string) (*domain.VPC, error)
	// DeleteVPC decommissioning an existing virtual network.
	// If force is true, dependency checks are skipped (for async cleanup scenarios).
	DeleteVPC(ctx context.Context, idOrName string, force bool) error
}
