// Package sdk provides the official Go SDK for the platform.
package sdk

import (
	"context"
	"fmt"
)

// Deployment describes a container deployment.
type Deployment struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	Name         string `json:"name"`
	Image        string `json:"image"`
	Replicas     int    `json:"replicas"`
	CurrentCount int    `json:"current_count"`
	Ports        string `json:"ports"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func (c *Client) CreateDeployment(name, image string, replicas int, ports string) (*Deployment, error) {
	req := struct {
		Name     string `json:"name"`
		Image    string `json:"image"`
		Replicas int    `json:"replicas"`
		Ports    string `json:"ports"`
	}{
		Name:     name,
		Image:    image,
		Replicas: replicas,
		Ports:    ports,
	}

	var dep Deployment
	err := c.post("/containers/deployments", req, &dep)
	return &dep, err
}

func (c *Client) ListDeployments() ([]Deployment, error) {
	var res Response[[]Deployment]
	if err := c.get("/containers/deployments", &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

func (c *Client) ListDeploymentsWithPagination(limit, offset int) ([]Deployment, *ListResponse[Deployment], error) {
	var res Response[ListResponse[Deployment]]
	if err := c.getWithPagination("/containers/deployments", &res, limit, offset); err != nil {
		return nil, nil, err
	}
	return res.Data.Data, &res.Data, nil
}

// ListDeploymentsWithContextAndPagination returns deployments with context and pagination metadata.
func (c *Client) ListDeploymentsWithContextAndPagination(ctx context.Context, limit, offset int) ([]Deployment, *ListResponse[Deployment], error) {
	var res Response[ListResponse[Deployment]]
	if err := c.getContextWithPagination(ctx, "/containers/deployments", &res, limit, offset); err != nil {
		return nil, nil, err
	}
	return res.Data.Data, &res.Data, nil
}

func (c *Client) GetDeployment(id string) (*Deployment, error) {
	var dep Deployment
	err := c.get(fmt.Sprintf("/containers/deployments/%s", id), &dep)
	return &dep, err
}

func (c *Client) ScaleDeployment(id string, replicas int) error {
	req := struct {
		Replicas int `json:"replicas"`
	}{Replicas: replicas}
	return c.post(fmt.Sprintf("/containers/deployments/%s/scale", id), req, nil)
}

func (c *Client) DeleteDeployment(id string) error {
	return c.delete(fmt.Sprintf("/containers/deployments/%s", id), nil)
}
