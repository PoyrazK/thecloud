// Package sdk provides the official Go SDK for the platform.
package sdk

import (
	"fmt"
	"time"
)

// RouteTargetType defines what kind of target a route points to.
type RouteTargetType string

const (
	RouteTargetLocal   RouteTargetType = "local"
	RouteTargetIGW     RouteTargetType = "igw"
	RouteTargetNAT     RouteTargetType = "nat"
	RouteTargetPeering RouteTargetType = "peering"
)

// RouteTable describes a route table resource.
type RouteTable struct {
	ID        string    `json:"id"`
	VPCID     string    `json:"vpc_id"`
	Name      string    `json:"name"`
	IsMain    bool      `json:"is_main"`
	CreatedAt time.Time `json:"created_at"`
}

// Route describes a routing rule within a route table.
type Route struct {
	ID              string          `json:"id"`
	RouteTableID    string          `json:"route_table_id"`
	DestinationCIDR string          `json:"destination_cidr"`
	TargetType      RouteTargetType `json:"target_type"`
	TargetID        *string         `json:"target_id,omitempty"`
	TargetName      string          `json:"target_name,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

// RouteTableAssociation links a subnet to a route table.
type RouteTableAssociation struct {
	ID           string    `json:"id"`
	RouteTableID string    `json:"route_table_id"`
	SubnetID     string    `json:"subnet_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// ListRouteTables returns all route tables for a VPC.
func (c *Client) ListRouteTables(vpcID string) ([]RouteTable, error) {
	var res Response[[]RouteTable]
	path := fmt.Sprintf("/route-tables?vpc_id=%s", vpcID)
	if err := c.get(path, &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// CreateRouteTable creates a new route table for a VPC.
func (c *Client) CreateRouteTable(vpcID, name string, isMain bool) (*RouteTable, error) {
	body := map[string]interface{}{
		"name":    name,
		"vpc_id":  vpcID,
		"is_main": isMain,
	}
	var res Response[RouteTable]
	if err := c.post("/route-tables", body, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// GetRouteTable retrieves details of a specific route table.
func (c *Client) GetRouteTable(id string) (*RouteTable, error) {
	var res Response[RouteTable]
	if err := c.get(fmt.Sprintf("/route-tables/%s", id), &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// DeleteRouteTable permanently removes a route table.
func (c *Client) DeleteRouteTable(id string) error {
	return c.delete(fmt.Sprintf("/route-tables/%s", id), nil)
}

// AddRoute adds a route to a route table.
func (c *Client) AddRoute(rtID, destCIDR string, targetType RouteTargetType, targetID string) (*Route, error) {
	body := map[string]interface{}{
		"destination_cidr": destCIDR,
		"target_type":      targetType,
	}
	if targetID != "" {
		body["target_id"] = targetID
	}
	var res Response[Route]
	if err := c.post(fmt.Sprintf("/route-tables/%s/routes", rtID), body, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// RemoveRoute removes a route from a route table.
func (c *Client) RemoveRoute(rtID, routeID string) error {
	return c.delete(fmt.Sprintf("/route-tables/%s/routes/%s", rtID, routeID), nil)
}

// AssociateSubnet associates a subnet with a route table.
func (c *Client) AssociateSubnet(rtID, subnetID string) error {
	body := map[string]string{
		"subnet_id": subnetID,
	}
	return c.post(fmt.Sprintf("/route-tables/%s/associations", rtID), body, nil)
}

// DisassociateSubnet disassociates a subnet from a route table.
func (c *Client) DisassociateSubnet(rtID, subnetID string) error {
	return c.delete(fmt.Sprintf("/route-tables/%s/associations/%s", rtID, subnetID), nil)
}
