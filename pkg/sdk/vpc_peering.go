// Package sdk provides the official Go SDK for the platform.
package sdk

import (
	"fmt"
	"time"
)

// VPCPeering describes a network peering connection resource.
type VPCPeering struct {
	ID             string    `json:"id"`
	RequesterVPCID string    `json:"requester_vpc_id"`
	AccepterVPCID  string    `json:"accepter_vpc_id"`
	TenantID       string    `json:"tenant_id"`
	Status         string    `json:"status"`
	ARN            string    `json:"arn"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CreateVPCPeering initiates a new VPC peering connection request.
func (c *Client) CreateVPCPeering(requesterVPCID, accepterVPCID string) (*VPCPeering, error) {
	body := map[string]string{
		"requester_vpc_id": requesterVPCID,
		"accepter_vpc_id":  accepterVPCID,
	}
	var res Response[VPCPeering]
	if err := c.post("/vpc-peerings", body, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// ListVPCPeerings returns all VPC peering connections for the tenant.
func (c *Client) ListVPCPeerings() ([]VPCPeering, error) {
	var res Response[[]VPCPeering]
	if err := c.get("/vpc-peerings", &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// GetVPCPeering retrieves details of a specific VPC peering connection.
// Note: VPC Peering resources don't have a Name field, so getName returns ID (matching by ID prefix only).
func (c *Client) GetVPCPeering(idOrName string) (*VPCPeering, error) {
	id, err := c.resolveID("vpc-peering", func() ([]interface{}, error) {
		peerings, err := c.ListVPCPeerings()
		return interfaceSlice(peerings), err
	}, func(v interface{}) string { return v.(VPCPeering).ID }, func(v interface{}) string { return v.(VPCPeering).ID }, idOrName)
	if err != nil {
		return nil, err
	}
	var res Response[VPCPeering]
	if err := c.get(fmt.Sprintf("/vpc-peerings/%s", id), &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// AcceptVPCPeering accepts a pending VPC peering connection.
func (c *Client) AcceptVPCPeering(id string) (*VPCPeering, error) {
	var res Response[VPCPeering]
	if err := c.post(fmt.Sprintf("/vpc-peerings/%s/accept", id), nil, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// RejectVPCPeering rejects a pending VPC peering connection.
func (c *Client) RejectVPCPeering(id string) error {
	return c.post(fmt.Sprintf("/vpc-peerings/%s/reject", id), nil, nil)
}

// DeleteVPCPeering tears down and removes a VPC peering connection.
func (c *Client) DeleteVPCPeering(id string) error {
	return c.delete(fmt.Sprintf("/vpc-peerings/%s", id), nil)
}
