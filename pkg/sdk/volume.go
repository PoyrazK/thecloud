// Package sdk provides the official Go SDK for the platform.
package sdk

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Volume describes a storage volume.
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

func (c *Client) ListVolumesWithContext(ctx context.Context) ([]Volume, error) {
	var res Response[[]Volume]
	if err := c.getWithContext(ctx, "/volumes", &res); err != nil {
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
	id, err := c.resolveID("volume", func() ([]interface{}, error) {
		vols, err := c.ListVolumes()
		return interfaceSlice(vols), err
	}, func(v interface{}) string { return v.(Volume).ID.String() }, func(v interface{}) string { return v.(Volume).Name }, idOrName)
	if err != nil {
		return nil, err
	}
	var res Response[Volume]
	if err := c.get(fmt.Sprintf("/volumes/%s", id), &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

func (c *Client) DeleteVolume(idOrName string) error {
	id, err := c.resolveID("volume", func() ([]interface{}, error) {
		vols, err := c.ListVolumes()
		return interfaceSlice(vols), err
	}, func(v interface{}) string { return v.(Volume).ID.String() }, func(v interface{}) string { return v.(Volume).Name }, idOrName)
	if err != nil {
		return err
	}
	return c.delete(fmt.Sprintf("/volumes/%s", id), nil)
}

func (c *Client) AttachVolume(volumeID, instanceID, mountPath string) (string, error) {
	body := map[string]string{
		"instance_id": instanceID,
		"mount_path":  mountPath,
	}
	var res Response[map[string]string]
	if err := c.post(fmt.Sprintf("/volumes/%s/attach", volumeID), body, &res); err != nil {
		return "", err
	}
	return res.Data["device_path"], nil
}

func (c *Client) DetachVolume(volumeID string) error {
	return c.post(fmt.Sprintf("/volumes/%s/detach", volumeID), nil, nil)
}

// ResizeVolume increases the capacity of a volume.
func (c *Client) ResizeVolume(idOrName string, newSizeGB int) error {
	id, err := c.resolveID("volume", func() ([]interface{}, error) {
		vols, err := c.ListVolumes()
		return interfaceSlice(vols), err
	}, func(v interface{}) string { return v.(Volume).ID.String() }, func(v interface{}) string { return v.(Volume).Name }, idOrName)
	if err != nil {
		return err
	}
	body := map[string]int{"new_size_gb": newSizeGB}
	return c.post(fmt.Sprintf("/volumes/%s/resize", id), body, nil)
}
