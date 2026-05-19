package domain

import (
	"errors"
	"net"
	"time"

	"github.com/google/uuid"
)

// RouteTargetType defines what kind of target a route points to.
type RouteTargetType string

const (
	RouteTargetLocal   RouteTargetType = "local"
	RouteTargetIGW     RouteTargetType = "igw"
	RouteTargetNAT     RouteTargetType = "nat"
	RouteTargetPeering RouteTargetType = "peering"
	RouteTargetTransit RouteTargetType = "transit"
)

// RouteTable represents a collection of routes associated with a VPC.
// It controls where network traffic is directed.
type RouteTable struct {
	ID           uuid.UUID               `json:"id"`
	VPCID        uuid.UUID               `json:"vpc_id"`
	Name         string                  `json:"name"`
	IsMain       bool                    `json:"is_main"`
	Routes       []Route                 `json:"routes,omitempty"`
	Associations []RouteTableAssociation `json:"associations,omitempty"`
	CreatedAt    time.Time               `json:"created_at"`
}

// Validate checks if the route table fields are valid.
func (rt *RouteTable) Validate() error {
	if rt.Name == "" {
		return errors.New("route table name cannot be empty")
	}
	if rt.VPCID == uuid.Nil {
		return errors.New("route table must be associated with a VPC")
	}
	return nil
}

// Route represents a single routing rule within a route table.
type Route struct {
	ID              uuid.UUID       `json:"id"`
	RouteTableID    uuid.UUID       `json:"route_table_id"`
	DestinationCIDR string          `json:"destination_cidr"`
	TargetType      RouteTargetType `json:"target_type"`
	TargetID        *uuid.UUID      `json:"target_id,omitempty"`
	TargetName      string          `json:"target_name,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

// Validate checks if the route fields are valid.
func (r *Route) Validate() error {
	if r.RouteTableID == uuid.Nil {
		return errors.New("route must be associated with a route table")
	}
	if r.DestinationCIDR == "" {
		return errors.New("destination CIDR is required")
	}
	_, _, err := net.ParseCIDR(r.DestinationCIDR)
	if err != nil {
		return errors.New("invalid destination CIDR")
	}
	if !isValidRouteTargetType(r.TargetType) {
		return errors.New("invalid target type")
	}
	return nil
}

func isValidRouteTargetType(t RouteTargetType) bool {
	switch t {
	case RouteTargetLocal, RouteTargetIGW, RouteTargetNAT, RouteTargetPeering, RouteTargetTransit:
		return true
	}
	return false
}

// RouteTableAssociation links a subnet to a route table.
// A subnet can only be associated with one route table, but a route table
// can have multiple subnet associations.
type RouteTableAssociation struct {
	ID           uuid.UUID `json:"id"`
	RouteTableID uuid.UUID `json:"route_table_id"`
	SubnetID     uuid.UUID `json:"subnet_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// Validate checks if the association fields are valid.
func (a *RouteTableAssociation) Validate() error {
	if a.RouteTableID == uuid.Nil {
		return errors.New("association must have a route table")
	}
	if a.SubnetID == uuid.Nil {
		return errors.New("association must have a subnet")
	}
	return nil
}
