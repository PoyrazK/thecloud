// Package sdk provides the official Go SDK for the platform.
package sdk

import (
	"fmt"
	"time"
)

// IGWStatus represents the attachment state of an Internet Gateway.
type IGWStatus string

const (
	IGWStatusDetached IGWStatus = "detached"
	IGWStatusAttached IGWStatus = "attached"
)

// InternetGateway describes an internet gateway resource.
type InternetGateway struct {
	ID        string    `json:"id"`
	VPCID     *string   `json:"vpc_id,omitempty"`
	UserID    string    `json:"user_id"`
	TenantID  string    `json:"tenant_id"`
	Status    IGWStatus `json:"status"`
	ARN       string    `json:"arn"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateIGW creates a new internet gateway in detached state.
func (c *Client) CreateIGW() (*InternetGateway, error) {
	var res Response[InternetGateway]
	if err := c.post("/internet-gateways", nil, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// AttachIGW attaches an internet gateway to a VPC.
func (c *Client) AttachIGW(igwID, vpcID string) error {
	body := map[string]string{
		"vpc_id": vpcID,
	}
	return c.post(fmt.Sprintf("/internet-gateways/%s/attach", igwID), body, nil)
}

// DetachIGW detaches an internet gateway from its VPC.
func (c *Client) DetachIGW(igwID string) error {
	return c.post(fmt.Sprintf("/internet-gateways/%s/detach", igwID), nil, nil)
}

// ListIGWs returns all internet gateways for the tenant.
func (c *Client) ListIGWs() ([]InternetGateway, error) {
	var res Response[[]InternetGateway]
	if err := c.get("/internet-gateways", &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// GetIGW retrieves details of a specific internet gateway.
func (c *Client) GetIGW(id string) (*InternetGateway, error) {
	var res Response[InternetGateway]
	if err := c.get(fmt.Sprintf("/internet-gateways/%s", id), &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// DeleteIGW permanently removes an internet gateway (must be detached first).
func (c *Client) DeleteIGW(id string) error {
	return c.delete(fmt.Sprintf("/internet-gateways/%s", id), nil)
}
