package domain

import (
	"time"

	"github.com/google/uuid"
)

// TransitGatewayStatus represents the operational status of a Transit Gateway.
type TransitGatewayStatus string

const (
	TransitGatewayStatusPending   TransitGatewayStatus = "pending"
	TransitGatewayStatusAvailable TransitGatewayStatus = "available"
	TransitGatewayStatusDeleting  TransitGatewayStatus = "deleting"
)

// TransitGateway represents a central hub for hub-and-spoke VPC connectivity.
type TransitGateway struct {
	ID            uuid.UUID  `json:"id"`
	Name          string     `json:"name"`
	OwnerTenantID uuid.UUID  `json:"owner_tenant_id"`
	Status        TransitGatewayStatus `json:"status"`
	ARN           string     `json:"arn,omitempty"`
	RouteTables   []*TransitGatewayRouteTable `json:"route_tables,omitempty"`
	Attachments   []*TransitGatewayAttachment   `json:"attachments,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at,omitempty"`
}

// TransitGatewayAttachment represents a VPC attached to a Transit Gateway.
type TransitGatewayAttachment struct {
	ID               uuid.UUID `json:"id"`
	TransitGatewayID uuid.UUID `json:"transit_gateway_id"`
	VPCID            uuid.UUID `json:"vpc_id"`
	TenantID         uuid.UUID `json:"tenant_id"`
	Status           string    `json:"status,omitempty"`
	AttachmentType   string    `json:"attachment_type,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// TransitGatewayRouteTable represents a route table within a Transit Gateway.
type TransitGatewayRouteTable struct {
	ID                 uuid.UUID  `json:"id"`
	TransitGatewayID   uuid.UUID  `json:"transit_gateway_id"`
	Name               string     `json:"name"`
	DefaultRouteTable  bool       `json:"default_route_table"`
	PropagationEnabled bool       `json:"propagation_enabled"`
	Associations       []uuid.UUID `json:"associations,omitempty"`
	Routes             []*TransitGatewayRoute `json:"routes,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

// TransitGatewayRoute represents a route within a Transit Gateway route table.
type TransitGatewayRoute struct {
	ID                 uuid.UUID          `json:"id"`
	TransitGatewayRTID uuid.UUID          `json:"transit_gateway_rt_id"`
	DestinationCIDR    string             `json:"destination_cidr"`
	TargetType         TransitGatewayTargetType `json:"target_type"`
	TargetID           *uuid.UUID         `json:"target_id,omitempty"`
	TargetName         string             `json:"target_name,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
}

// TransitGatewayTargetType defines what a TGW route points to.
type TransitGatewayTargetType string

const (
	TransitGatewayTargetAttachment TransitGatewayTargetType = "attachment"
	TransitGatewayTargetLocal      TransitGatewayTargetType = "local"
)
