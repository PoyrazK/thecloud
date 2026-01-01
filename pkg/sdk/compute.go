package sdk

import (
	"fmt"
	"time"
)

type Instance struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Image       string    `json:"image"`
	Status      string    `json:"status"`
	Ports       string    `json:"ports"`
	VpcID       string    `json:"vpc_id,omitempty"`
	ContainerID string    `json:"container_id"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (c *Client) ListInstances() ([]Instance, error) {
	var res Response[[]Instance]
	if err := c.get("/instances", &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

func (c *Client) GetInstance(idOrName string) (*Instance, error) {
	var res Response[Instance]
	if err := c.get(fmt.Sprintf("/instances/%s", idOrName), &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

func (c *Client) LaunchInstance(name, image, ports string, vpcID string) (*Instance, error) {
	body := map[string]string{
		"name":   name,
		"image":  image,
		"ports":  ports,
		"vpc_id": vpcID,
	}
	var res Response[Instance]
	if err := c.post("/instances", body, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

func (c *Client) StopInstance(idOrName string) error {
	return c.post(fmt.Sprintf("/instances/%s/stop", idOrName), nil, nil)
}

func (c *Client) TerminateInstance(idOrName string) error {
	return c.delete(fmt.Sprintf("/instances/%s", idOrName), nil)
}

func (c *Client) GetInstanceLogs(idOrName string) (string, error) {
	resp, err := c.resty.R().Get(c.apiURL + fmt.Sprintf("/instances/%s/logs", idOrName))
	if err != nil {
		return "", err
	}
	if resp.IsError() {
		return "", fmt.Errorf("api error: %s", resp.String())
	}
	return string(resp.Body()), nil
}
