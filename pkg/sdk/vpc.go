package sdk

import (
	"fmt"
	"time"
)

type VPC struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	NetworkID string    `json:"network_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (c *Client) ListVPCs() ([]VPC, error) {
	var res Response[[]VPC]
	if err := c.get("/vpcs", &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

func (c *Client) CreateVPC(name string) (*VPC, error) {
	body := map[string]string{"name": name}
	var res Response[VPC]
	if err := c.post("/vpcs", body, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

func (c *Client) GetVPC(id string) (*VPC, error) {
	var res Response[VPC]
	if err := c.get(fmt.Sprintf("/vpcs/%s", id), &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

func (c *Client) DeleteVPC(id string) error {
	return c.delete(fmt.Sprintf("/vpcs/%s", id), nil)
}
