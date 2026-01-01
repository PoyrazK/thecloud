package sdk

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Volume struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	SizeGB     int        `json:"size_gb"`
	Status     string     `json:"status"`
	InstanceID *uuid.UUID `json:"instance_id,omitempty"`
	MountPath  string     `json:"mount_path,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (c *Client) ListVolumes() ([]Volume, error) {
	var res Response[[]Volume]
	if err := c.get("/volumes", &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

func (c *Client) CreateVolume(name string, sizeGB int) (*Volume, error) {
	body := map[string]interface{}{
		"name":    name,
		"size_gb": sizeGB,
	}
	var res Response[Volume]
	if err := c.post("/volumes", body, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

func (c *Client) GetVolume(idOrName string) (*Volume, error) {
	var res Response[Volume]
	if err := c.get(fmt.Sprintf("/volumes/%s", idOrName), &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

func (c *Client) DeleteVolume(idOrName string) error {
	return c.delete(fmt.Sprintf("/volumes/%s", idOrName), nil)
}
