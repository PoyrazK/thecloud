package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/poyraz/cloud/internal/core/domain"
	"github.com/poyraz/cloud/internal/errors"
)

type InstanceRepository struct {
	db *pgxpool.Pool
}

func NewInstanceRepository(db *pgxpool.Pool) *InstanceRepository {
	return &InstanceRepository{db: db}
}

func (r *InstanceRepository) Create(ctx context.Context, inst *domain.Instance) error {
	query := `
		INSERT INTO instances (id, name, image, container_id, status, ports, vpc_id, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(ctx, query,
		inst.ID, inst.Name, inst.Image, inst.ContainerID, inst.Status, inst.Ports, inst.VpcID, inst.Version, inst.CreatedAt, inst.UpdatedAt,
	)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create instance", err)
	}
	return nil
}

func (r *InstanceRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Instance, error) {
	query := `
		SELECT id, name, image, COALESCE(container_id, ''), status, COALESCE(ports, ''), vpc_id, version, created_at, updated_at
		FROM instances
		WHERE id = $1
	`
	var inst domain.Instance
	err := r.db.QueryRow(ctx, query, id).Scan(
		&inst.ID, &inst.Name, &inst.Image, &inst.ContainerID, &inst.Status, &inst.Ports, &inst.VpcID, &inst.Version, &inst.CreatedAt, &inst.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New(errors.NotFound, fmt.Sprintf("instance %s not found", id))
		}
		return nil, errors.Wrap(errors.Internal, "failed to get instance", err)
	}
	return &inst, nil
}

func (r *InstanceRepository) GetByName(ctx context.Context, name string) (*domain.Instance, error) {
	query := `
		SELECT id, name, image, COALESCE(container_id, ''), status, COALESCE(ports, ''), vpc_id, version, created_at, updated_at
		FROM instances
		WHERE name = $1
	`
	var inst domain.Instance
	err := r.db.QueryRow(ctx, query, name).Scan(
		&inst.ID, &inst.Name, &inst.Image, &inst.ContainerID, &inst.Status, &inst.Ports, &inst.VpcID, &inst.Version, &inst.CreatedAt, &inst.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New(errors.NotFound, fmt.Sprintf("instance name %s not found", name))
		}
		return nil, errors.Wrap(errors.Internal, "failed to get instance by name", err)
	}
	return &inst, nil
}

func (r *InstanceRepository) List(ctx context.Context) ([]*domain.Instance, error) {
	query := `
		SELECT id, name, image, COALESCE(container_id, ''), status, COALESCE(ports, ''), vpc_id, version, created_at, updated_at
		FROM instances
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list instances", err)
	}
	defer rows.Close()

	var instances []*domain.Instance
	for rows.Next() {
		var inst domain.Instance
		err := rows.Scan(
			&inst.ID, &inst.Name, &inst.Image, &inst.ContainerID, &inst.Status, &inst.Ports, &inst.VpcID, &inst.Version, &inst.CreatedAt, &inst.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to scan instance", err)
		}
		instances = append(instances, &inst)
	}
	return instances, nil
}

func (r *InstanceRepository) Update(ctx context.Context, inst *domain.Instance) error {
	// Implements Optimistic Locking via 'version'
	query := `
		UPDATE instances
		SET name = $1, status = $2, version = version + 1, updated_at = $3, container_id = $4, ports = $5, vpc_id = $6
		WHERE id = $7 AND version = $8
	`
	now := time.Now()
	cmd, err := r.db.Exec(ctx, query, inst.Name, inst.Status, now, inst.ContainerID, inst.Ports, inst.VpcID, inst.ID, inst.Version)
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

func (r *InstanceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM instances WHERE id = $1`
	cmd, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete instance", err)
	}
	if cmd.RowsAffected() == 0 {
		return errors.New(errors.NotFound, fmt.Sprintf("instance %s not found", id))
	}
	return nil
}
