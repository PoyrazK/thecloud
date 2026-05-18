// Package sdk provides the official Go SDK for the platform.
package sdk

import (
	"context"
	"net/url"
	"time"

	"github.com/google/uuid"
)

const clustersPath = "/clusters"

// NodeGroup represents a pool of similar worker nodes in a cluster.
type NodeGroup struct {
	ID           uuid.UUID `json:"id"`
	ClusterID    uuid.UUID `json:"cluster_id"`
	Name         string    `json:"name"`
	InstanceType string    `json:"instance_type"`
	MinSize      int       `json:"min_size"`
	MaxSize      int       `json:"max_size"`
	CurrentSize  int       `json:"current_size"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Cluster represents a managed Kubernetes cluster in the SDK.
type Cluster struct {
	ID                 uuid.UUID   `json:"id"`
	Name               string      `json:"name"`
	UserID             uuid.UUID   `json:"user_id"`
	VpcID              uuid.UUID   `json:"vpc_id"`
	Version            string      `json:"version"`
	ControlPlaneIPs    []string    `json:"control_plane_ips"`
	WorkerCount        int         `json:"worker_count"`
	HAEnabled          bool        `json:"ha_enabled"`
	APIServerLBAddress *string     `json:"api_server_lb_address,omitempty"`
	Status             string      `json:"status"`
	NodeGroups         []NodeGroup `json:"node_groups,omitempty"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
}

// CreateClusterInput defines the input for creating a cluster.
type CreateClusterInput struct {
	Name             string    `json:"name"`
	VpcID            uuid.UUID `json:"vpc_id"`
	Version          string    `json:"version"`
	WorkerCount      int       `json:"workers"`
	NetworkIsolation bool      `json:"network_isolation"`
	HA               bool      `json:"ha"`
}

// NodeGroupInput defines the input for adding a node group.
type NodeGroupInput struct {
	Name         string `json:"name"`
	InstanceType string `json:"instance_type"`
	MinSize      int    `json:"min_size"`
	MaxSize      int    `json:"max_size"`
	DesiredSize  int    `json:"desired_size"`
}

// UpdateNodeGroupInput defines the input for updating a node group.
type UpdateNodeGroupInput struct {
	DesiredSize *int `json:"desired_size"`
	MinSize     *int `json:"min_size"`
	MaxSize     *int `json:"max_size"`
}

// ClusterHealth represents the operational status of a cluster.
type ClusterHealth struct {
	Status     string `json:"status"`
	APIServer  bool   `json:"api_server"`
	NodesTotal int    `json:"nodes_total"`
	NodesReady int    `json:"nodes_ready"`
	Message    string `json:"message,omitempty"`
}

// ScaleClusterInput defines the input for scaling worker nodes.
type ScaleClusterInput struct {
	Workers int `json:"workers"`
}

// ListClusters returns all clusters for the current user.
func (c *Client) ListClusters() ([]*Cluster, error) {
	var resp Response[[]*Cluster]
	if err := c.get(clustersPath, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// ListClustersWithContext returns all clusters for the current user with context support.
func (c *Client) ListClustersWithContext(ctx context.Context) ([]*Cluster, error) {
	var resp Response[[]*Cluster]
	if err := c.getWithContext(ctx, clustersPath, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// resolveClusterIDWithContext resolves a cluster ID or name to a full UUID with context support.
func (c *Client) resolveClusterIDWithContext(ctx context.Context, idOrName string) (string, error) {
	return c.resolveID("cluster", func() ([]interface{}, error) {
		clusters, err := c.ListClustersWithContext(ctx)
		return interfaceSlicePtr(clusters), err
	}, func(v interface{}) string { return v.(*Cluster).ID.String() }, func(v interface{}) string { return v.(*Cluster).Name }, idOrName)
}

// CreateCluster initiates cluster provisioning.
func (c *Client) CreateCluster(input *CreateClusterInput) (*Cluster, error) {
	var resp Response[*Cluster]
	if err := c.post(clustersPath, input, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetCluster retrieves cluster details by ID or name.
func (c *Client) GetCluster(idOrName string) (*Cluster, error) {
	return c.GetClusterWithContext(context.Background(), idOrName)
}

// GetClusterWithContext retrieves cluster details by ID or name with context support.
func (c *Client) GetClusterWithContext(ctx context.Context, idOrName string) (*Cluster, error) {
	id, err := c.resolveClusterIDWithContext(ctx, idOrName)
	if err != nil {
		return nil, err
	}
	var resp Response[*Cluster]
	if err := c.getWithContext(ctx, clustersPath+"/"+id, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// DeleteCluster removes a cluster.
func (c *Client) DeleteCluster(idOrName string) error {
	return c.DeleteClusterWithContext(context.Background(), idOrName)
}

// DeleteClusterWithContext removes a cluster with context support.
func (c *Client) DeleteClusterWithContext(ctx context.Context, idOrName string) error {
	id, err := c.resolveClusterIDWithContext(ctx, idOrName)
	if err != nil {
		return err
	}
	var resp Response[any]
	return c.deleteWithContext(ctx, clustersPath+"/"+id, &resp)
}

// GetKubeconfig retrieves the cluster kubeconfig, optionally for a specific role.
func (c *Client) GetKubeconfig(idOrName string, role string) (string, error) {
	return c.GetKubeconfigWithContext(context.Background(), idOrName, role)
}

// GetKubeconfigWithContext retrieves the cluster kubeconfig with context support.
func (c *Client) GetKubeconfigWithContext(ctx context.Context, idOrName string, role string) (string, error) {
	id, err := c.resolveClusterIDWithContext(ctx, idOrName)
	if err != nil {
		return "", err
	}
	path := clustersPath + "/" + id + "/kubeconfig"
	if role != "" {
		path += "?role=" + url.QueryEscape(role)
	}
	var resp Response[string]
	if err := c.getWithContext(ctx, path, &resp); err != nil {
		return "", err
	}
	return resp.Data, nil
}

// RepairCluster triggers a re-run of critical provisioning steps.
func (c *Client) RepairCluster(idOrName string) error {
	return c.RepairClusterWithContext(context.Background(), idOrName)
}

// RepairClusterWithContext triggers a re-run of critical provisioning steps with context support.
func (c *Client) RepairClusterWithContext(ctx context.Context, idOrName string) error {
	id, err := c.resolveClusterIDWithContext(ctx, idOrName)
	if err != nil {
		return err
	}
	var resp Response[any]
	return c.postWithContext(ctx, clustersPath+"/"+id+"/repair", nil, &resp)
}

// ScaleCluster adjusts the number of worker nodes.
func (c *Client) ScaleCluster(idOrName string, workers int) error {
	return c.ScaleClusterWithContext(context.Background(), idOrName, workers)
}

// ScaleClusterWithContext adjusts the number of worker nodes with context support.
func (c *Client) ScaleClusterWithContext(ctx context.Context, idOrName string, workers int) error {
	id, err := c.resolveClusterIDWithContext(ctx, idOrName)
	if err != nil {
		return err
	}
	var resp Response[any]
	input := &ScaleClusterInput{Workers: workers}
	return c.postWithContext(ctx, clustersPath+"/"+id+"/scale", input, &resp)
}

// GetClusterHealth retrieved the operational health of the cluster.
func (c *Client) GetClusterHealth(idOrName string) (*ClusterHealth, error) {
	return c.GetClusterHealthWithContext(context.Background(), idOrName)
}

// GetClusterHealthWithContext retrieves the operational health of the cluster with context support.
func (c *Client) GetClusterHealthWithContext(ctx context.Context, idOrName string) (*ClusterHealth, error) {
	id, err := c.resolveClusterIDWithContext(ctx, idOrName)
	if err != nil {
		return nil, err
	}
	var resp Response[*ClusterHealth]
	if err := c.getWithContext(ctx, clustersPath+"/"+id+"/health", &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// UpgradeClusterInput defines the input for upgrading a cluster.
type UpgradeClusterInput struct {
	Version string `json:"version"`
}

// UpgradeCluster initiates an asynchronous version upgrade.
func (c *Client) UpgradeCluster(idOrName string, version string) error {
	return c.UpgradeClusterWithContext(context.Background(), idOrName, version)
}

// UpgradeClusterWithContext initiates an asynchronous version upgrade with context support.
func (c *Client) UpgradeClusterWithContext(ctx context.Context, idOrName string, version string) error {
	id, err := c.resolveClusterIDWithContext(ctx, idOrName)
	if err != nil {
		return err
	}
	var resp Response[any]
	input := &UpgradeClusterInput{Version: version}
	return c.postWithContext(ctx, clustersPath+"/"+id+"/upgrade", input, &resp)
}

// RotateSecrets triggers a renewal of cluster certificates.
func (c *Client) RotateSecrets(idOrName string) error {
	return c.RotateSecretsWithContext(context.Background(), idOrName)
}

// RotateSecretsWithContext triggers a renewal of cluster certificates with context support.
func (c *Client) RotateSecretsWithContext(ctx context.Context, idOrName string) error {
	id, err := c.resolveClusterIDWithContext(ctx, idOrName)
	if err != nil {
		return err
	}
	var resp Response[any]
	return c.postWithContext(ctx, clustersPath+"/"+id+"/rotate-secrets", nil, &resp)
}

// CreateBackup initiates a cluster state snapshot.
func (c *Client) CreateBackup(idOrName string) error {
	return c.CreateBackupWithContext(context.Background(), idOrName)
}

// CreateBackupWithContext initiates a cluster state snapshot with context support.
func (c *Client) CreateBackupWithContext(ctx context.Context, idOrName string) error {
	id, err := c.resolveClusterIDWithContext(ctx, idOrName)
	if err != nil {
		return err
	}
	var resp Response[any]
	return c.postWithContext(ctx, clustersPath+"/"+id+"/backups", nil, &resp)
}

// RestoreBackupInput defines the input for restoring a cluster from backup.
type RestoreBackupInput struct {
	BackupPath string `json:"backup_path"`
}

// RestoreBackup initiates a cluster restoration from a specific path.
func (c *Client) RestoreBackup(idOrName string, backupPath string) error {
	return c.RestoreBackupWithContext(context.Background(), idOrName, backupPath)
}

// RestoreBackupWithContext initiates a cluster restoration from a specific path with context support.
func (c *Client) RestoreBackupWithContext(ctx context.Context, idOrName string, backupPath string) error {
	id, err := c.resolveClusterIDWithContext(ctx, idOrName)
	if err != nil {
		return err
	}
	var resp Response[any]
	input := &RestoreBackupInput{BackupPath: backupPath}
	return c.postWithContext(ctx, clustersPath+"/"+id+"/restore", input, &resp)
}

// AddNodeGroup adds a new node pool to the cluster.
func (c *Client) AddNodeGroup(clusterIDOrName string, input NodeGroupInput) (*NodeGroup, error) {
	return c.AddNodeGroupWithContext(context.Background(), clusterIDOrName, input)
}

// AddNodeGroupWithContext adds a new node pool to the cluster with context support.
func (c *Client) AddNodeGroupWithContext(ctx context.Context, clusterIDOrName string, input NodeGroupInput) (*NodeGroup, error) {
	id, err := c.resolveClusterIDWithContext(ctx, clusterIDOrName)
	if err != nil {
		return nil, err
	}
	var resp Response[*NodeGroup]
	if err := c.postWithContext(ctx, clustersPath+"/"+id+"/nodegroups", input, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// UpdateNodeGroupWithContext updates a node group's parameters with context support.
func (c *Client) UpdateNodeGroupWithContext(ctx context.Context, clusterIDOrName string, name string, input UpdateNodeGroupInput) (*NodeGroup, error) {
	id, err := c.resolveClusterIDWithContext(ctx, clusterIDOrName)
	if err != nil {
		return nil, err
	}
	var resp Response[*NodeGroup]
	if err := c.putWithContext(ctx, clustersPath+"/"+id+"/nodegroups/"+url.PathEscape(name), input, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// DeleteNodeGroup removes a node group.
func (c *Client) DeleteNodeGroup(clusterIDOrName string, name string) error {
	return c.DeleteNodeGroupWithContext(context.Background(), clusterIDOrName, name)
}

// DeleteNodeGroupWithContext removes a node group with context support.
func (c *Client) DeleteNodeGroupWithContext(ctx context.Context, clusterIDOrName string, name string) error {
	id, err := c.resolveClusterIDWithContext(ctx, clusterIDOrName)
	if err != nil {
		return err
	}
	var resp Response[any]
	return c.deleteWithContext(ctx, clustersPath+"/"+id+"/nodegroups/"+url.PathEscape(name), &resp)
}
