// Package sdk provides the official Go SDK for the platform.
package sdk

import (
	"context"
	"fmt"

	"github.com/poyrazk/thecloud/internal/core/domain"
)

func (c *Client) CreateSnapshot(ctx context.Context, volumeIDOrName string, description string) (*domain.Snapshot, error) {
	id, err := c.resolveIDWithContext(ctx, "volume", func(ctx context.Context) ([]interface{}, error) {
		vols, err := c.ListVolumesWithContext(ctx)
		return interfaceSlice(vols), err
	}, func(v interface{}) string { return v.(Volume).ID.String() }, func(v interface{}) string { return v.(Volume).Name }, volumeIDOrName)
	if err != nil {
		return nil, err
	}
	req := map[string]interface{}{
		"volume_id":   id,
		"description": description,
	}

	var res Response[domain.Snapshot]
	err = c.postWithContext(ctx, "/snapshots", req, &res)
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

func (c *Client) ListSnapshots() ([]*domain.Snapshot, error) {
	var res Response[[]*domain.Snapshot]
	err := c.get("/snapshots", &res)
	if err != nil {
		return nil, err
	}
	return res.Data, nil
}

func (c *Client) GetSnapshot(idOrName string) (*domain.Snapshot, error) {
	id, err := c.resolveID("snapshot", func() ([]interface{}, error) {
		snaps, err := c.ListSnapshots()
		return interfaceSlice(snaps), err
	}, func(v interface{}) string { return v.(*domain.Snapshot).ID.String() }, func(v interface{}) string { return v.(*domain.Snapshot).VolumeName }, idOrName)
	if err != nil {
		return nil, err
	}
	var res Response[domain.Snapshot]
	err = c.get(fmt.Sprintf("/snapshots/%s", id), &res)
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

func (c *Client) DeleteSnapshot(idOrName string) error {
	id, err := c.resolveID("snapshot", func() ([]interface{}, error) {
		snaps, err := c.ListSnapshots()
		return interfaceSlice(snaps), err
	}, func(v interface{}) string { return v.(*domain.Snapshot).ID.String() }, func(v interface{}) string { return v.(*domain.Snapshot).VolumeName }, idOrName)
	if err != nil {
		return err
	}
	return c.delete(fmt.Sprintf("/snapshots/%s", id), nil)
}

func (c *Client) RestoreSnapshot(idOrName string, newVolumeName string) (*domain.Volume, error) {
	id, err := c.resolveID("snapshot", func() ([]interface{}, error) {
		snaps, err := c.ListSnapshots()
		return interfaceSlice(snaps), err
	}, func(v interface{}) string { return v.(*domain.Snapshot).ID.String() }, func(v interface{}) string { return v.(*domain.Snapshot).VolumeName }, idOrName)
	if err != nil {
		return nil, err
	}
	req := map[string]interface{}{
		"new_volume_name": newVolumeName,
	}

	var res Response[domain.Volume]
	err = c.post(fmt.Sprintf("/snapshots/%s/restore", id), req, &res)
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}
