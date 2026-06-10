package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

// TransitGatewayRepository manages the persistent state of Transit Gateways.
type TransitGatewayRepository interface {
	Create(ctx context.Context, tg *domain.TransitGateway) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.TransitGateway, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]*domain.TransitGateway, error)
	Update(ctx context.Context, tg *domain.TransitGateway) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Attachment operations
	AddAttachment(ctx context.Context, att *domain.TransitGatewayAttachment) error
	RemoveAttachment(ctx context.Context, id uuid.UUID) error
	RemoveAttachmentAssociations(ctx context.Context, attID uuid.UUID) error
	ListAttachments(ctx context.Context, tgID uuid.UUID) ([]*domain.TransitGatewayAttachment, error)
	GetAttachment(ctx context.Context, id uuid.UUID) (*domain.TransitGatewayAttachment, error)

	// Route table operations
	CreateRouteTable(ctx context.Context, rt *domain.TransitGatewayRouteTable) error
	GetRouteTable(ctx context.Context, id uuid.UUID) (*domain.TransitGatewayRouteTable, error)
	ListRouteTables(ctx context.Context, tgID uuid.UUID) ([]*domain.TransitGatewayRouteTable, error)
	DeleteRouteTable(ctx context.Context, id uuid.UUID) error

	// Route operations
	AddRoute(ctx context.Context, rtID uuid.UUID, route *domain.TransitGatewayRoute) error
	RemoveRoute(ctx context.Context, rtID, routeID uuid.UUID) error
	ListRoutes(ctx context.Context, rtID uuid.UUID) ([]*domain.TransitGatewayRoute, error)

	// Association/propagation
	AssociateAttachment(ctx context.Context, rtID, attID uuid.UUID) error
	EnablePropagation(ctx context.Context, rtID, attID uuid.UUID) error
}

// TransitGatewayService provides business logic for Transit Gateway management.
type TransitGatewayService interface {
	// CreateTransitGateway creates a new Transit Gateway.
	CreateTransitGateway(ctx context.Context, name string) (*domain.TransitGateway, error)

	// GetTransitGateway retrieves a Transit Gateway by ID.
	GetTransitGateway(ctx context.Context, id uuid.UUID) (*domain.TransitGateway, error)

	// ListTransitGateways returns all Transit Gateways for the current tenant.
	ListTransitGateways(ctx context.Context) ([]*domain.TransitGateway, error)

	// DeleteTransitGateway removes a Transit Gateway and all its attachments.
	DeleteTransitGateway(ctx context.Context, id uuid.UUID) error

	// AttachVPC attaches a VPC to the Transit Gateway.
	AttachVPC(ctx context.Context, tgID, vpcID uuid.UUID) (*domain.TransitGatewayAttachment, error)

	// DetachVPC detaches a VPC from the Transit Gateway.
	DetachVPC(ctx context.Context, attID uuid.UUID) error

	// AssociateRouteTable associates an attachment with a TGW route table.
	AssociateRouteTable(ctx context.Context, rtID, attID uuid.UUID) error

	// EnableRoutePropagation enables automatic route propagation from an attachment to a RT.
	EnableRoutePropagation(ctx context.Context, rtID, attID uuid.UUID) error
}
