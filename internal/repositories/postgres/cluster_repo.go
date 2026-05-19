// Package postgres provides Postgres-backed repository implementations.
package postgres

import (
	"context"
	"time"

	stdlib_errors "errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
)

// ClusterRepository implements ports.ClusterRepository.
type ClusterRepository struct {
	db DB
}

// NewClusterRepository constructs a new ClusterRepository.
func NewClusterRepository(db DB) *ClusterRepository {
	return &ClusterRepository{db: db}
}

func (r *ClusterRepository) Create(ctx context.Context, cluster *domain.Cluster) error {
	query := `
		INSERT INTO clusters (
			id, user_id, tenant_id, vpc_id, name, version, status, control_plane_ips, worker_count, ha_enabled,
			network_isolation, pod_cidr, service_cidr, api_server_lb_address,
			kubeconfig_encrypted, ssh_private_key_encrypted, join_token,
			token_expires_at, ca_cert_hash, job_id, backup_schedule, backup_retention_days,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
	`
	_, err := r.db.Exec(ctx, query,
		cluster.ID, cluster.UserID, cluster.TenantID, cluster.VpcID, cluster.Name, cluster.Version,
		string(cluster.Status), cluster.ControlPlaneIPs, cluster.WorkerCount, cluster.HAEnabled,
		cluster.NetworkIsolation, cluster.PodCIDR, cluster.ServiceCIDR,
		cluster.APIServerLBAddress, cluster.KubeconfigEncrypted,
		cluster.SSHPrivateKeyEncrypted, cluster.JoinToken, cluster.TokenExpiresAt,
		cluster.CACertHash, cluster.JobID, cluster.BackupSchedule, cluster.BackupRetentionDays,
		cluster.CreatedAt, cluster.UpdatedAt,
	)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create cluster", err)
	}
	return nil
}

func (r *ClusterRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Cluster, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT
			id, user_id, tenant_id, vpc_id, name, version, status, control_plane_ips, worker_count, ha_enabled,
			network_isolation, pod_cidr, service_cidr, api_server_lb_address,
			kubeconfig_encrypted, ssh_private_key_encrypted, join_token,
			token_expires_at, ca_cert_hash, job_id, backup_schedule, backup_retention_days,
			created_at, updated_at
		FROM clusters
		WHERE id = $1 AND tenant_id = $2 AND user_id = $3
	`
	cluster, err := r.scanCluster(r.db.QueryRow(ctx, query, id, tenantID, userID))
	if err != nil || cluster == nil {
		return cluster, err
	}

	// Fetch Node Groups
	ngs, err := r.GetNodeGroups(ctx, id)
	if err != nil {
		return nil, err
	}
	cluster.NodeGroups = ngs

	return cluster, nil
}

func (r *ClusterRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Cluster, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT
			id, user_id, tenant_id, vpc_id, name, version, status, control_plane_ips, worker_count, ha_enabled,
			network_isolation, pod_cidr, service_cidr, api_server_lb_address,
			kubeconfig_encrypted, ssh_private_key_encrypted, join_token,
			token_expires_at, ca_cert_hash, job_id, backup_schedule, backup_retention_days,
			created_at, updated_at
		FROM clusters
		WHERE tenant_id = $1 AND user_id = $2
		ORDER BY created_at DESC
	`
	return r.list(ctx, query, tenantID, userID)
}

func (r *ClusterRepository) ListAll(ctx context.Context) ([]*domain.Cluster, error) {
	query := `
		SELECT
			id, user_id, tenant_id, vpc_id, name, version, status, control_plane_ips, worker_count, ha_enabled,
			network_isolation, pod_cidr, service_cidr, api_server_lb_address,
			kubeconfig_encrypted, ssh_private_key_encrypted, join_token,
			token_expires_at, ca_cert_hash, job_id, backup_schedule, backup_retention_days,
			created_at, updated_at
		FROM clusters
		ORDER BY created_at DESC
	`
	return r.list(ctx, query)
}

func (r *ClusterRepository) list(ctx context.Context, query string, args ...any) ([]*domain.Cluster, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list clusters", err)
	}
	defer rows.Close()

	var clusters []*domain.Cluster
	for rows.Next() {
		c, err := r.scanCluster(rows)
		if err != nil {
			return nil, err
		}
		// Fetch Node Groups for each cluster in the list
		ngs, err := r.GetNodeGroups(ctx, c.ID)
		if err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to fetch node groups for cluster in list", err)
		}
		c.NodeGroups = ngs
		clusters = append(clusters, c)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(errors.Internal, "row iteration error", err)
	}
	return clusters, nil
}

func (r *ClusterRepository) Update(ctx context.Context, cluster *domain.Cluster) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		UPDATE clusters
		SET
			vpc_id = $1, name = $2, version = $3, status = $4, control_plane_ips = $5,
			worker_count = $6, ha_enabled = $7, network_isolation = $8, pod_cidr = $9,
			service_cidr = $10, api_server_lb_address = $11, kubeconfig_encrypted = $12,
			ssh_private_key_encrypted = $13, join_token = $14, token_expires_at = $15,
			ca_cert_hash = $16, job_id = $17, backup_schedule = $18, backup_retention_days = $19,
			updated_at = $20
		WHERE id = $21 AND tenant_id = $22 AND user_id = $23
	`
	_, err := r.db.Exec(ctx, query,
		cluster.VpcID, cluster.Name, cluster.Version, string(cluster.Status),
		cluster.ControlPlaneIPs, cluster.WorkerCount, cluster.HAEnabled,
		cluster.NetworkIsolation, cluster.PodCIDR, cluster.ServiceCIDR,
		cluster.APIServerLBAddress, cluster.KubeconfigEncrypted,
		cluster.SSHPrivateKeyEncrypted, cluster.JoinToken, cluster.TokenExpiresAt,
		cluster.CACertHash, cluster.JobID, cluster.BackupSchedule, cluster.BackupRetentionDays,
		time.Now(), cluster.ID, tenantID, cluster.UserID,
	)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to update cluster", err)
	}
	return nil
}

func (r *ClusterRepository) Delete(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `DELETE FROM clusters WHERE id = $1 AND tenant_id = $2 AND user_id = $3`
	_, err := r.db.Exec(ctx, query, id, tenantID, userID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete cluster", err)
	}
	return nil
}

func (r *ClusterRepository) AddNode(ctx context.Context, node *domain.ClusterNode) error {
	query := `
		INSERT INTO cluster_nodes (id, cluster_id, instance_id, role, status, joined_at, last_heartbeat)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query, node.ID, node.ClusterID, node.InstanceID, string(node.Role), node.Status, node.JoinedAt, node.LastHeartbeat)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to add cluster node", err)
	}
	return nil
}

func (r *ClusterRepository) GetNodes(ctx context.Context, clusterID uuid.UUID) ([]*domain.ClusterNode, error) {
	query := `
		SELECT id, cluster_id, instance_id, role, status, joined_at, last_heartbeat
		FROM cluster_nodes
		WHERE cluster_id = $1
	`
	rows, err := r.db.Query(ctx, query, clusterID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to get cluster nodes", err)
	}
	defer rows.Close()

	var nodes []*domain.ClusterNode
	for rows.Next() {
		var n domain.ClusterNode
		var role string
		if err := rows.Scan(&n.ID, &n.ClusterID, &n.InstanceID, &role, &n.Status, &n.JoinedAt, &n.LastHeartbeat); err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to scan cluster node", err)
		}
		n.Role = domain.NodeRole(role)
		nodes = append(nodes, &n)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(errors.Internal, "row iteration error", err)
	}
	return nodes, nil
}

func (r *ClusterRepository) UpdateNode(ctx context.Context, node *domain.ClusterNode) error {
	query := `
		UPDATE cluster_nodes
		SET status = $1, joined_at = $2, last_heartbeat = $3
		WHERE id = $4
	`
	_, err := r.db.Exec(ctx, query, node.Status, node.JoinedAt, node.LastHeartbeat, node.ID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to update cluster node", err)
	}
	return nil
}

func (r *ClusterRepository) DeleteNode(ctx context.Context, nodeID uuid.UUID) error {
	query := `DELETE FROM cluster_nodes WHERE id = $1`
	_, err := r.db.Exec(ctx, query, nodeID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete cluster node", err)
	}
	return nil
}

func (r *ClusterRepository) AddNodeGroup(ctx context.Context, ng *domain.NodeGroup) error {
	query := `
		INSERT INTO cluster_node_groups (id, cluster_id, name, instance_type, min_size, max_size, current_size, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, query, ng.ID, ng.ClusterID, ng.Name, ng.InstanceType, ng.MinSize, ng.MaxSize, ng.CurrentSize, ng.CreatedAt, ng.UpdatedAt)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to add node group", err)
	}
	return nil
}

func (r *ClusterRepository) GetNodeGroups(ctx context.Context, clusterID uuid.UUID) ([]domain.NodeGroup, error) {
	query := `
		SELECT id, cluster_id, name, instance_type, min_size, max_size, current_size, created_at, updated_at
		FROM cluster_node_groups
		WHERE cluster_id = $1
	`
	rows, err := r.db.Query(ctx, query, clusterID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to get node groups", err)
	}
	defer rows.Close()

	var groups []domain.NodeGroup
	for rows.Next() {
		var ng domain.NodeGroup
		if err := rows.Scan(&ng.ID, &ng.ClusterID, &ng.Name, &ng.InstanceType, &ng.MinSize, &ng.MaxSize, &ng.CurrentSize, &ng.CreatedAt, &ng.UpdatedAt); err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to scan node group", err)
		}
		groups = append(groups, ng)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(errors.Internal, "row iteration error", err)
	}
	return groups, nil
}

func (r *ClusterRepository) UpdateNodeGroup(ctx context.Context, ng *domain.NodeGroup) error {
	query := `
		UPDATE cluster_node_groups
		SET instance_type = $1, min_size = $2, max_size = $3, current_size = $4, updated_at = $5
		WHERE cluster_id = $6 AND name = $7
	`
	_, err := r.db.Exec(ctx, query, ng.InstanceType, ng.MinSize, ng.MaxSize, ng.CurrentSize, time.Now(), ng.ClusterID, ng.Name)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to update node group", err)
	}
	return nil
}

func (r *ClusterRepository) DeleteNodeGroup(ctx context.Context, clusterID uuid.UUID, name string) error {
	query := `DELETE FROM cluster_node_groups WHERE cluster_id = $1 AND name = $2`
	_, err := r.db.Exec(ctx, query, clusterID, name)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete node group", err)
	}
	return nil
}

func (r *ClusterRepository) scanCluster(row pgx.Row) (*domain.Cluster, error) {
	var c domain.Cluster
	var status string
	err := row.Scan(
		&c.ID, &c.UserID, &c.TenantID, &c.VpcID, &c.Name, &c.Version, &status, &c.ControlPlaneIPs, &c.WorkerCount,
		&c.HAEnabled, &c.NetworkIsolation, &c.PodCIDR, &c.ServiceCIDR,
		&c.APIServerLBAddress, &c.KubeconfigEncrypted, &c.SSHPrivateKeyEncrypted,
		&c.JoinToken, &c.TokenExpiresAt, &c.CACertHash, &c.JobID, &c.BackupSchedule, &c.BackupRetentionDays,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, errors.Wrap(errors.Internal, "failed to scan cluster", err)
	}
	c.Status = domain.ClusterStatus(status)
	return &c, nil
}
