package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/poyraz/cloud/internal/core/domain"
)

type VolumeRepository struct {
	db *pgxpool.Pool
}

func NewVolumeRepository(db *pgxpool.Pool) *VolumeRepository {
	return &VolumeRepository{db: db}
}

func (r *VolumeRepository) Create(ctx context.Context, v *domain.Volume) error {
	query := `INSERT INTO volumes (id, name, size_gb, status, instance_id, mount_path, created_at, updated_at) 
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.db.Exec(ctx, query, v.ID, v.Name, v.SizeGB, v.Status, v.InstanceID, v.MountPath, v.CreatedAt, v.UpdatedAt)
	return err
}

func (r *VolumeRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Volume, error) {
	query := `SELECT id, name, size_gb, status, instance_id, mount_path, created_at, updated_at FROM volumes WHERE id = $1`
	v := &domain.Volume{}
	err := r.db.QueryRow(ctx, query, id).Scan(&v.ID, &v.Name, &v.SizeGB, &v.Status, &v.InstanceID, &v.MountPath, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (r *VolumeRepository) GetByName(ctx context.Context, name string) (*domain.Volume, error) {
	query := `SELECT id, name, size_gb, status, instance_id, mount_path, created_at, updated_at FROM volumes WHERE name = $1`
	v := &domain.Volume{}
	err := r.db.QueryRow(ctx, query, name).Scan(&v.ID, &v.Name, &v.SizeGB, &v.Status, &v.InstanceID, &v.MountPath, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (r *VolumeRepository) List(ctx context.Context) ([]*domain.Volume, error) {
	query := `SELECT id, name, size_gb, status, instance_id, mount_path, created_at, updated_at FROM volumes ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var volumes []*domain.Volume
	for rows.Next() {
		v := &domain.Volume{}
		if err := rows.Scan(&v.ID, &v.Name, &v.SizeGB, &v.Status, &v.InstanceID, &v.MountPath, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		volumes = append(volumes, v)
	}
	return volumes, nil
}

func (r *VolumeRepository) Update(ctx context.Context, v *domain.Volume) error {
	query := `UPDATE volumes SET status = $1, instance_id = $2, mount_path = $3, updated_at = $4 WHERE id = $5`
	_, err := r.db.Exec(ctx, query, v.Status, v.InstanceID, v.MountPath, v.UpdatedAt, v.ID)
	return err
}

func (r *VolumeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM volumes WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}
