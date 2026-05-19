// Package postgres provides PostgreSQL-backed repository implementations.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	stdlib_errors "errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
)

// InstanceRepository provides a PostgreSQL implementation for managing instance metadata.
type InstanceRepository struct {
	db DB
}

// NewInstanceRepository creates a new InstanceRepository with the given database pool.
func NewInstanceRepository(db DB) *InstanceRepository {
	return &InstanceRepository{db: db}
}

// Create inserts a new instance record into the database.
func (r *InstanceRepository) Create(ctx context.Context, inst *domain.Instance) error {
	query := `
		INSERT INTO instances (
			id, user_id, tenant_id, name, image, container_id, status, ports, vpc_id, subnet_id, 
			private_ip, ovs_port, instance_type, volume_binds, env, cmd, cpu_limit, memory_limit, disk_limit,
			metadata, labels, ssh_key_id,
			version, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NULLIF($11, '')::inet, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)
	`
	_, err := r.db.Exec(ctx, query,
		inst.ID, inst.UserID, inst.TenantID, inst.Name, inst.Image, inst.ContainerID, string(inst.Status), inst.Ports, inst.VpcID, inst.SubnetID,
		inst.PrivateIP, inst.OvsPort, inst.InstanceType, inst.VolumeBinds, inst.Env, inst.Cmd, inst.CPULimit, inst.MemoryLimit, inst.DiskLimit,
		inst.Metadata, inst.Labels, inst.SSHKeyID,
		inst.Version, inst.CreatedAt, inst.UpdatedAt,
	)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create instance", err)
	}
	return nil
}

// GetByID retrieves a single instance by its UUID and ensures it belongs to the authenticated user.
func (r *InstanceRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Instance, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT id, user_id, tenant_id, name, image, COALESCE(container_id, ''), status, COALESCE(ports, ''), vpc_id, subnet_id, COALESCE(private_ip::text, ''), COALESCE(ovs_port, ''), COALESCE(instance_type, ''), 
		       volume_binds, env, cmd, COALESCE(cpu_limit, 0), COALESCE(memory_limit, 0), COALESCE(disk_limit, 0),
		       COALESCE(metadata, '{}'::jsonb), COALESCE(labels, '{}'::jsonb), ssh_key_id,
		       version, created_at, updated_at
		FROM instances
		WHERE id = $1 AND tenant_id = $2
	`
	return r.scanInstance(r.db.QueryRow(ctx, query, id, tenantID))
}

func (r *InstanceRepository) scanInstance(row pgx.Row) (*domain.Instance, error) {
	var inst domain.Instance
	var status string
	err := row.Scan(
		&inst.ID, &inst.UserID, &inst.TenantID, &inst.Name, &inst.Image, &inst.ContainerID, &status, &inst.Ports, &inst.VpcID, &inst.SubnetID, &inst.PrivateIP, &inst.OvsPort, &inst.InstanceType,
		&inst.VolumeBinds, &inst.Env, &inst.Cmd, &inst.CPULimit, &inst.MemoryLimit, &inst.DiskLimit,
		&inst.Metadata, &inst.Labels, &inst.SSHKeyID,
		&inst.Version, &inst.CreatedAt, &inst.UpdatedAt,
	)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, "instance not found")
		}
		return nil, errors.Wrap(errors.Internal, "failed to scan instance", err)
	}
	inst.Status = domain.InstanceStatus(status)
	return &inst, nil
}

// GetByName retrieves a single instance by its name and ensures it belongs to the authenticated user.
func (r *InstanceRepository) GetByName(ctx context.Context, name string) (*domain.Instance, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT id, user_id, tenant_id, name, image, COALESCE(container_id, ''), status, COALESCE(ports, ''), vpc_id, subnet_id, COALESCE(private_ip::text, ''), COALESCE(ovs_port, ''), COALESCE(instance_type, ''), 
		       volume_binds, env, cmd, COALESCE(cpu_limit, 0), COALESCE(memory_limit, 0), COALESCE(disk_limit, 0),
		       COALESCE(metadata, '{}'::jsonb), COALESCE(labels, '{}'::jsonb), ssh_key_id,
		       version, created_at, updated_at
		FROM instances
		WHERE name = $1 AND tenant_id = $2
	`
	return r.scanInstance(r.db.QueryRow(ctx, query, name, tenantID))
}

// List returns all instances belonging to the authenticated user.
func (r *InstanceRepository) List(ctx context.Context, tagFilter []string) ([]*domain.Instance, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT id, user_id, tenant_id, name, image, COALESCE(container_id, ''), status, COALESCE(ports, ''), vpc_id, subnet_id, COALESCE(private_ip::text, ''), COALESCE(ovs_port, ''), COALESCE(instance_type, ''),
		       volume_binds, env, cmd, COALESCE(cpu_limit, 0), COALESCE(memory_limit, 0), COALESCE(disk_limit, 0),
		       COALESCE(metadata, '{}'::jsonb), COALESCE(labels, '{}'::jsonb), ssh_key_id,
		       version, created_at, updated_at
		FROM instances
		WHERE tenant_id = $1`
	args := []interface{}{tenantID}
	for i, tag := range tagFilter {
		// tag format: "key:value" — use JSONB containment (@>) to match instances where labels contains this entry
		query += fmt.Sprintf(" AND labels @> $%d", i+2)
		parts := strings.SplitN(tag, ":", 2)
		if len(parts) == 2 {
			labelJSON, _ := json.Marshal(map[string]string{parts[0]: parts[1]})
			args = append(args, string(labelJSON))
		} else {
			// bare key filter: labels ? key
			query = strings.Replace(query, fmt.Sprintf(" AND labels @> $%d", i+2), fmt.Sprintf(" AND labels ? $%d", i+2), 1)
			args = append(args, tag)
		}
	}
	query += " ORDER BY created_at DESC"
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list instances", err)
	}
	return r.scanInstances(rows)
}

func (r *InstanceRepository) ListAll(ctx context.Context) ([]*domain.Instance, error) {
	query := `
		SELECT id, user_id, tenant_id, name, image, COALESCE(container_id, ''), status, COALESCE(ports, ''), vpc_id, subnet_id, COALESCE(private_ip::text, ''), COALESCE(ovs_port, ''), COALESCE(instance_type, ''), 
		       volume_binds, env, cmd, COALESCE(cpu_limit, 0), COALESCE(memory_limit, 0), COALESCE(disk_limit, 0),
		       COALESCE(metadata, '{}'::jsonb), COALESCE(labels, '{}'::jsonb), ssh_key_id,
		       version, created_at, updated_at
		FROM instances
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list all instances", err)
	}
	return r.scanInstances(rows)
}

// Update modifies an existing instance record using optimistic locking (via the version field).
func (r *InstanceRepository) Update(ctx context.Context, inst *domain.Instance) error {
	// Implements Optimistic Locking via 'version'
	query := `
		UPDATE instances
		SET name = $1, status = $2, version = version + 1, updated_at = $3, container_id = $4, ports = $5, vpc_id = $6, subnet_id = $7, private_ip = NULLIF($8, '')::inet, ovs_port = $9, instance_type = $10,
		    volume_binds = $11, env = $12, cmd = $13, cpu_limit = $14, memory_limit = $15, disk_limit = $16,
		    metadata = $17, labels = $18, ssh_key_id = $19
		WHERE id = $20 AND version = $21 AND tenant_id = $22
	`
	now := time.Now()
	cmd, err := r.db.Exec(ctx, query, inst.Name, string(inst.Status), now, inst.ContainerID, inst.Ports, inst.VpcID, inst.SubnetID, inst.PrivateIP, inst.OvsPort, inst.InstanceType,
		inst.VolumeBinds, inst.Env, inst.Cmd, inst.CPULimit, inst.MemoryLimit, inst.DiskLimit,
		inst.Metadata, inst.Labels, inst.SSHKeyID,
		inst.ID, inst.Version, inst.TenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to update instance", err)
	}

	if cmd.RowsAffected() == 0 {
		return errors.New(errors.Conflict, "update conflict: instance was modified or not found")
	}

	inst.UpdatedAt = now
	inst.Version++
	return nil
}

// ListBySubnet returns all instances associated with a specific subnet.
func (r *InstanceRepository) ListBySubnet(ctx context.Context, subnetID uuid.UUID) ([]*domain.Instance, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT id, user_id, tenant_id, name, image, COALESCE(container_id, ''), status, COALESCE(ports, ''), vpc_id, subnet_id, COALESCE(private_ip::text, ''), COALESCE(ovs_port, ''), COALESCE(instance_type, ''), 
		       volume_binds, env, cmd, COALESCE(cpu_limit, 0), COALESCE(memory_limit, 0), COALESCE(disk_limit, 0),
		       COALESCE(metadata, '{}'::jsonb), COALESCE(labels, '{}'::jsonb), ssh_key_id,
		       version, created_at, updated_at
		FROM instances
		WHERE subnet_id = $1 AND tenant_id = $2
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, subnetID, tenantID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list instances by subnet", err)
	}
	return r.scanInstances(rows)
}

func (r *InstanceRepository) scanInstances(rows pgx.Rows) ([]*domain.Instance, error) {
	defer rows.Close()
	var instances []*domain.Instance
	for rows.Next() {
		inst, err := r.scanInstance(rows)
		if err != nil {
			return nil, err
		}
		instances = append(instances, inst)
	}
	return instances, nil
}

// Delete removes an instance record from the database.
func (r *InstanceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `DELETE FROM instances WHERE id = $1 AND tenant_id = $2`
	cmd, err := r.db.Exec(ctx, query, id, tenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete instance", err)
	}
	if cmd.RowsAffected() == 0 {
		return errors.New(errors.NotFound, fmt.Sprintf("instance %s not found", id))
	}
	return nil
}
