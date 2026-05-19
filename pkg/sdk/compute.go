// Package sdk provides the official Go SDK for the platform.
package sdk

import (
	"context"
	"fmt"
	"time"
)

// Instance describes a compute instance.
type Instance struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	InstanceType string            `json:"instance_type,omitempty"`
	Status       string            `json:"status"`
	Ports        string            `json:"ports"`
	VpcID        string            `json:"vpc_id,omitempty"`
	SubnetID     string            `json:"subnet_id,omitempty"`
	PrivateIP    string            `json:"private_ip,omitempty"`
	ContainerID  string            `json:"container_id"`
	Version      int               `json:"version"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	SSHKeyID     string            `json:"ssh_key_id,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// ListInstances returns all instances visible to the API key.
func (c *Client) ListInstances() ([]Instance, error) {
	var res Response[[]Instance]
	if err := c.get("/instances", &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// ListInstancesWithPagination returns instances with pagination metadata.
func (c *Client) ListInstancesWithPagination(limit, offset int) ([]Instance, *ListResponse[Instance], error) {
	var res Response[ListResponse[Instance]]
	if err := c.getWithPagination("/instances", &res, limit, offset); err != nil {
		return nil, nil, err
	}
	return res.Data.Data, &res.Data, nil
}

// ListInstancesWithContextAndPagination returns instances with context and pagination metadata.
func (c *Client) ListInstancesWithContextAndPagination(ctx context.Context, limit, offset int) ([]Instance, *ListResponse[Instance], error) {
	var res Response[ListResponse[Instance]]
	if err := c.getContextWithPagination(ctx, "/instances", &res, limit, offset); err != nil {
		return nil, nil, err
	}
	return res.Data.Data, &res.Data, nil
}

// ListInstancesWithContext returns all instances with context support.
func (c *Client) ListInstancesWithContext(ctx context.Context) ([]Instance, error) {
	var res Response[[]Instance]
	if err := c.getWithContext(ctx, "/instances", &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// resolveInstanceIDWithContext resolves an instance ID or name to a full ID with context support.
func (c *Client) resolveInstanceIDWithContext(ctx context.Context, idOrName string) (string, error) {
	return c.resolveID("instance", func() ([]interface{}, error) {
		instances, err := c.ListInstancesWithContext(ctx)
		return interfaceSlice(instances), err
	}, func(v interface{}) string { return v.(Instance).ID }, func(v interface{}) string { return v.(Instance).Name }, idOrName)
}

// GetInstance retrieves a compute instance by ID or name.
func (c *Client) GetInstance(idOrName string) (*Instance, error) {
	return c.GetInstanceWithContext(context.Background(), idOrName)
}

// GetInstanceWithContext retrieves a compute instance with context support.
func (c *Client) GetInstanceWithContext(ctx context.Context, idOrName string) (*Instance, error) {
	id, err := c.resolveInstanceIDWithContext(ctx, idOrName)
	if err != nil {
		return nil, err
	}
	var res Response[Instance]
	if err := c.getWithContext(ctx, fmt.Sprintf("/instances/%s", id), &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

func (c *Client) GetConsoleURL(idOrName string) (string, error) {
	return c.GetConsoleURLWithContext(context.Background(), idOrName)
}

func (c *Client) GetConsoleURLWithContext(ctx context.Context, idOrName string) (string, error) {
	id, err := c.resolveInstanceIDWithContext(ctx, idOrName)
	if err != nil {
		return "", err
	}
	var res Response[string]
	if err := c.getWithContext(ctx, fmt.Sprintf("/instances/%s/console", id), &res); err != nil {
		return "", err
	}
	return res.Data, nil
}

// VolumeAttachmentInput defines a volume attachment for instance launch.
type VolumeAttachmentInput struct {
	VolumeID  string `json:"volume_id"`
	MountPath string `json:"mount_path"`
}

// LaunchInstance provisions a new instance with optional metadata, labels, and volume attachments.
func (c *Client) LaunchInstance(name, image, ports, instanceType string, vpcID, subnetID string, volumes []VolumeAttachmentInput, metadata, labels map[string]string, sshKeyID string, cmd []string) (*Instance, error) {
	body := map[string]interface{}{
		"name":          name,
		"image":         image,
		"ports":         ports,
		"instance_type": instanceType,
		"vpc_id":        vpcID,
		"subnet_id":     subnetID,
		"volumes":       volumes,
		"metadata":      metadata,
		"labels":        labels,
		"ssh_key_id":    sshKeyID,
		"cmd":           cmd,
	}
	var res Response[Instance]
	if err := c.post("/instances", body, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// UpdateInstanceMetadata updates the metadata and labels of an instance.
func (c *Client) UpdateInstanceMetadata(idOrName string, metadata, labels map[string]string) error {
	return c.UpdateInstanceMetadataWithContext(context.Background(), idOrName, metadata, labels)
}

func (c *Client) UpdateInstanceMetadataWithContext(ctx context.Context, idOrName string, metadata, labels map[string]string) error {
	id, err := c.resolveInstanceIDWithContext(ctx, idOrName)
	if err != nil {
		return err
	}
	body := map[string]interface{}{
		"metadata": metadata,
		"labels":   labels,
	}
	return c.putWithContext(ctx, fmt.Sprintf("/instances/%s/metadata", id), body, nil)
}

// StopInstance stops a running instance by ID or name.
func (c *Client) StopInstance(idOrName string) error {
	return c.StopInstanceWithContext(context.Background(), idOrName)
}

func (c *Client) StopInstanceWithContext(ctx context.Context, idOrName string) error {
	id, err := c.resolveInstanceIDWithContext(ctx, idOrName)
	if err != nil {
		return err
	}
	return c.postWithContext(ctx, fmt.Sprintf("/instances/%s/stop", id), nil, nil)
}

// TerminateInstance deletes an instance by ID or name.
func (c *Client) TerminateInstance(idOrName string) error {
	return c.TerminateInstanceWithContext(context.Background(), idOrName)
}

// TerminateInstanceWithContext deletes an instance with context support.
func (c *Client) TerminateInstanceWithContext(ctx context.Context, idOrName string) error {
	id, err := c.resolveInstanceIDWithContext(ctx, idOrName)
	if err != nil {
		return err
	}
	return c.deleteWithContext(ctx, fmt.Sprintf("/instances/%s", id), nil)
}

// GetInstanceLogs retrieves the raw log output for an instance.
func (c *Client) GetInstanceLogs(idOrName string) (string, error) {
	return c.GetInstanceLogsWithContext(context.Background(), idOrName)
}

func (c *Client) GetInstanceLogsWithContext(ctx context.Context, idOrName string) (string, error) {
	id, err := c.resolveInstanceIDWithContext(ctx, idOrName)
	if err != nil {
		return "", err
	}
	resp, err := c.resty.R().SetContext(ctx).Get(c.apiURL + fmt.Sprintf("/instances/%s/logs", id))
	if err != nil {
		return "", err
	}
	if resp.IsError() {
		return "", fmt.Errorf("api error: %s", resp.String())
	}
	return string(resp.Body()), nil
}

// ResizeInstance changes the instance type of a running or stopped instance.
func (c *Client) ResizeInstance(idOrName, newInstanceType string) error {
	return c.ResizeInstanceWithContext(context.Background(), idOrName, newInstanceType)
}

func (c *Client) ResizeInstanceWithContext(ctx context.Context, idOrName, newInstanceType string) error {
	id, err := c.resolveInstanceIDWithContext(ctx, idOrName)
	if err != nil {
		return err
	}
	body := map[string]string{
		"instance_type": newInstanceType,
	}
	return c.postWithContext(ctx, fmt.Sprintf("/instances/%s/resize", id), body, nil)
}

// InstanceStats captures resource usage for an instance.
type InstanceStats struct {
	CPUPercentage    float64 `json:"cpu_percentage"`
	MemoryUsageBytes float64 `json:"memory_usage_bytes"`
	MemoryLimitBytes float64 `json:"memory_limit_bytes"`
	MemoryPercentage float64 `json:"memory_percentage"`
}

// GetInstanceStats returns resource usage metrics for an instance.
func (c *Client) GetInstanceStats(idOrName string) (*InstanceStats, error) {
	return c.GetInstanceStatsWithContext(context.Background(), idOrName)
}

func (c *Client) GetInstanceStatsWithContext(ctx context.Context, idOrName string) (*InstanceStats, error) {
	id, err := c.resolveInstanceIDWithContext(ctx, idOrName)
	if err != nil {
		return nil, err
	}
	var res Response[InstanceStats]
	if err := c.getWithContext(ctx, fmt.Sprintf("/instances/%s/stats", id), &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// SSHKey represents a registered SSH public key.
type SSHKey struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

// InstanceType describes available resource sizing.
type InstanceType struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	VCPUs       int     `json:"vcpus"`
	MemoryMB    int     `json:"memory_mb"`
	DiskGB      int     `json:"disk_gb"`
	NetworkMbps int     `json:"network_mbps"`
	PricePerHr  float64 `json:"price_per_hour"`
	Category    string  `json:"category"`
}

// ListInstanceTypes returns all available instance sizes.
func (c *Client) ListInstanceTypes() ([]InstanceType, error) {
	var res Response[[]InstanceType]
	if err := c.get("/instance-types", &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// ListInstanceTypesWithPagination returns instance types with pagination metadata.
func (c *Client) ListInstanceTypesWithPagination(limit, offset int) ([]InstanceType, *ListResponse[InstanceType], error) {
	var res Response[ListResponse[InstanceType]]
	if err := c.getWithPagination("/instance-types", &res, limit, offset); err != nil {
		return nil, nil, err
	}
	return res.Data.Data, &res.Data, nil
}

// RegisterSSHKey registers a new SSH public key.
func (c *Client) RegisterSSHKey(name, publicKey string) (*SSHKey, error) {
	body := map[string]string{
		"name":       name,
		"public_key": publicKey,
	}
	var res Response[SSHKey]
	if err := c.post("/ssh-keys", body, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// ListSSHKeys returns all registered SSH keys.
func (c *Client) ListSSHKeys() ([]SSHKey, error) {
	var res Response[[]SSHKey]
	if err := c.get("/ssh-keys", &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

// ListSSHKeysWithPagination returns SSH keys with pagination metadata.
func (c *Client) ListSSHKeysWithPagination(limit, offset int) ([]SSHKey, *ListResponse[SSHKey], error) {
	var res Response[ListResponse[SSHKey]]
	if err := c.getWithPagination("/ssh-keys", &res, limit, offset); err != nil {
		return nil, nil, err
	}
	return res.Data.Data, &res.Data, nil
}
