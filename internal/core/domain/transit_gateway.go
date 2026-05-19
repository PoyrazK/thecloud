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
	ID            uuid.UUID
	Name          string
	OwnerTenantID uuid.UUID
	Status        TransitGatewayStatus
	ARN           string
	RouteTables   []*TransitGatewayRouteTable
	Attachments   []*TransitGatewayAttachment
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TransitGatewayAttachment represents a VPC attached to a Transit Gateway.
type TransitGatewayAttachment struct {
	ID              uuid.UUID
	TransitGatewayID uuid.UUID
	VPCID           uuid.UUID
	TenantID        uuid.UUID
	Status          string // "pending" | "attached" | "detaching"
	AttachmentType string // "vpc"
	CreatedAt       time.Time
}

// TransitGatewayRouteTable represents a route table within a Transit Gateway.
type TransitGatewayRouteTable struct {
	ID                 uuid.UUID
	TransitGatewayID   uuid.UUID
	Name               string
	DefaultRouteTable  bool
	PropagationEnabled bool
	Associations       []uuid.UUID // attachment IDs associated
	Routes             []*TransitGatewayRoute
	CreatedAt          time.Time
}

// TransitGatewayRoute represents a route within a Transit Gateway route table.
type TransitGatewayRoute struct {
	ID                 uuid.UUID
	TransitGatewayRTID uuid.UUID
	DestinationCIDR    string
	TargetType         TransitGatewayTargetType
	TargetID           *uuid.UUID // attachment ID for propagated routes
	TargetName         string
	CreatedAt          time.Time
}

// TransitGatewayTargetType defines what a TGW route points to.
type TransitGatewayTargetType string

const (
	TransitGatewayTargetAttachment TransitGatewayTargetType = "attachment"
	TransitGatewayTargetLocal      TransitGatewayTargetType = "local"
)
