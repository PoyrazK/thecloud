// Package ports defines service and repository interfaces.
package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

// VPCPeeringRepository manages the persistent state of VPC peering connections.
type VPCPeeringRepository interface {
	// Create saves a new peering connection request.
	Create(ctx context.Context, peering *domain.VPCPeering) error
	// GetByID retrieves a peering connection by its unique UUID.
	GetByID(ctx context.Context, id uuid.UUID) (*domain.VPCPeering, error)
	// List returns all peering connections for a given tenant.
	List(ctx context.Context, tenantID uuid.UUID) ([]*domain.VPCPeering, error)
	// ListByVPC returns all peering connections involving a specific VPC.
	ListByVPC(ctx context.Context, vpcID uuid.UUID) ([]*domain.VPCPeering, error)
	// UpdateStatus changes the status of a peering connection.
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	// Delete removes a peering connection record.
	Delete(ctx context.Context, id uuid.UUID) error
	// GetActiveByVPCPair returns an existing active or pending peering between two VPCs.
	GetActiveByVPCPair(ctx context.Context, vpc1, vpc2 uuid.UUID) (*domain.VPCPeering, error)
}

// VPCPeeringService provides business logic for managing VPC peering connections.
type VPCPeeringService interface {
	// CreatePeering initiates a peering connection request between two VPCs.
	// For cross-tenant peering, accepterTenantID specifies the tenant of the accepter VPC.
	CreatePeering(ctx context.Context, requesterVPCID, accepterVPCID, accepterTenantID uuid.UUID) (*domain.VPCPeering, error)
	// AcceptPeering accepts a pending peering connection and establishes network routes.
	AcceptPeering(ctx context.Context, peeringID uuid.UUID) (*domain.VPCPeering, error)
	// RejectPeering rejects a pending peering connection request.
	RejectPeering(ctx context.Context, peeringID uuid.UUID) error
	// DeletePeering tears down a peering connection and removes network routes.
	DeletePeering(ctx context.Context, peeringID uuid.UUID) error
	// GetPeering retrieves details of a specific peering connection.
	GetPeering(ctx context.Context, peeringID uuid.UUID) (*domain.VPCPeering, error)
	// ListPeerings returns all peering connections for the current tenant.
	ListPeerings(ctx context.Context) ([]*domain.VPCPeering, error)
}
