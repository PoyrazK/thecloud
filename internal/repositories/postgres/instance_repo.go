package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
)

// InstanceRepository provides a PostgreSQL implementation for managing instance metadata.
type InstanceRepository struct {
	db *pgxpool.Pool
}

// NewInstanceRepository creates a new InstanceRepository with the given database pool.
func NewInstanceRepository(db *pgxpool.Pool) *InstanceRepository {
	return &InstanceRepository{db: db}
}

// Create inserts a new instance record into the database.
func (r *InstanceRepository) Create(ctx context.Context, inst *domain.Instance) error {
	query := `
		INSERT INTO instances (id, user_id, name, image, container_id, status, ports, vpc_id, subnet_id, private_ip, ovs_port, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, '')::inet, $11, $12, $13, $14)
	`
	_, err := r.db.Exec(ctx, query,
		inst.ID, inst.UserID, inst.Name, inst.Image, inst.ContainerID, inst.Status, inst.Ports, inst.VpcID, inst.SubnetID,
		inst.PrivateIP, inst.OvsPort, inst.Version, inst.CreatedAt, inst.UpdatedAt,
	)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create instance", err)
	}
	return nil
}

// GetByID retrieves a single instance by its UUID and ensures it belongs to the authenticated user.
func (r *InstanceRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Instance, error) {
	userID := appcontext.UserIDFromContext(ctx)
	query := `
		SELECT id, user_id, name, image, COALESCE(container_id, ''), status, COALESCE(ports, ''), vpc_id, subnet_id, COALESCE(private_ip::text, ''), COALESCE(ovs_port, ''), version, created_at, updated_at
		FROM instances
		WHERE id = $1 AND user_id = $2
	`
	var inst domain.Instance
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&inst.ID, &inst.UserID, &inst.Name, &inst.Image, &inst.ContainerID, &inst.Status, &inst.Ports, &inst.VpcID, &inst.SubnetID, &inst.PrivateIP, &inst.OvsPort, &inst.Version, &inst.CreatedAt, &inst.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New(errors.NotFound, fmt.Sprintf("instance %s not found", id))
		}
		return nil, errors.Wrap(errors.Internal, "failed to get instance", err)
	}
	return &inst, nil
}

// GetByName retrieves a single instance by its name and ensures it belongs to the authenticated user.
func (r *InstanceRepository) GetByName(ctx context.Context, name string) (*domain.Instance, error) {
	userID := appcontext.UserIDFromContext(ctx)
	query := `
		SELECT id, user_id, name, image, COALESCE(container_id, ''), status, COALESCE(ports, ''), vpc_id, subnet_id, COALESCE(private_ip::text, ''), COALESCE(ovs_port, ''), version, created_at, updated_at
		FROM instances
		WHERE name = $1 AND user_id = $2
	`
	var inst domain.Instance
	err := r.db.QueryRow(ctx, query, name, userID).Scan(
		&inst.ID, &inst.UserID, &inst.Name, &inst.Image, &inst.ContainerID, &inst.Status, &inst.Ports, &inst.VpcID, &inst.SubnetID, &inst.PrivateIP, &inst.OvsPort, &inst.Version, &inst.CreatedAt, &inst.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New(errors.NotFound, fmt.Sprintf("instance name %s not found", name))
		}
		return nil, errors.Wrap(errors.Internal, "failed to get instance by name", err)
	}
	return &inst, nil
}

// List returns all instances belonging to the authenticated user.
func (r *InstanceRepository) List(ctx context.Context) ([]*domain.Instance, error) {
	userID := appcontext.UserIDFromContext(ctx)
	query := `
		SELECT id, user_id, name, image, COALESCE(container_id, ''), status, COALESCE(ports, ''), vpc_id, subnet_id, COALESCE(private_ip::text, ''), COALESCE(ovs_port, ''), version, created_at, updated_at
		FROM instances
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list instances", err)
	}
	defer rows.Close()

	var instances []*domain.Instance
	for rows.Next() {
		var inst domain.Instance
		err := rows.Scan(
			&inst.ID, &inst.UserID, &inst.Name, &inst.Image, &inst.ContainerID, &inst.Status, &inst.Ports, &inst.VpcID, &inst.SubnetID, &inst.PrivateIP, &inst.OvsPort, &inst.Version, &inst.CreatedAt, &inst.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to scan instance", err)
		}
		instances = append(instances, &inst)
	}
	return instances, nil
}

// Update modifies an existing instance record using optimistic locking (via the version field).
func (r *InstanceRepository) Update(ctx context.Context, inst *domain.Instance) error {
	// Implements Optimistic Locking via 'version'
	query := `
		UPDATE instances
		SET name = $1, status = $2, version = version + 1, updated_at = $3, container_id = $4, ports = $5, vpc_id = $6, subnet_id = $7, private_ip = NULLIF($8, '')::inet, ovs_port = $9
		WHERE id = $10 AND version = $11 AND user_id = $12
	`
	now := time.Now()
	cmd, err := r.db.Exec(ctx, query, inst.Name, inst.Status, now, inst.ContainerID, inst.Ports, inst.VpcID, inst.SubnetID, inst.PrivateIP, inst.OvsPort, inst.ID, inst.Version, inst.UserID)
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
	userID := appcontext.UserIDFromContext(ctx)
	query := `
		SELECT id, user_id, name, image, COALESCE(container_id, ''), status, COALESCE(ports, ''), vpc_id, subnet_id, COALESCE(private_ip::text, ''), COALESCE(ovs_port, ''), version, created_at, updated_at
		FROM instances
		WHERE subnet_id = $1 AND user_id = $2
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, subnetID, userID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list instances by subnet", err)
	}
	defer rows.Close()

	var instances []*domain.Instance
	for rows.Next() {
		var inst domain.Instance
		err := rows.Scan(
			&inst.ID, &inst.UserID, &inst.Name, &inst.Image, &inst.ContainerID, &inst.Status, &inst.Ports, &inst.VpcID, &inst.SubnetID, &inst.PrivateIP, &inst.OvsPort, &inst.Version, &inst.CreatedAt, &inst.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to scan instance", err)
		}
		instances = append(instances, &inst)
	}
	return instances, nil
}

// Delete removes an instance record from the database.
func (r *InstanceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	query := `DELETE FROM instances WHERE id = $1 AND user_id = $2`
	cmd, err := r.db.Exec(ctx, query, id, userID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete instance", err)
	}
	if cmd.RowsAffected() == 0 {
		return errors.New(errors.NotFound, fmt.Sprintf("instance %s not found", id))
	}
	return nil
}
