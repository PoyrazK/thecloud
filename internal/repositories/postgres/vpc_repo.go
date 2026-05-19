// Package postgres provides PostgreSQL-backed repository implementations.
package postgres

import (
	"context"

	stdlib_errors "errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
)

// VpcRepository provides a PostgreSQL implementation for managing VPC metadata.
type VpcRepository struct {
	db DB
}

// NewVpcRepository creates a new VpcRepository with the given database pool.
func NewVpcRepository(db DB) *VpcRepository {
	return &VpcRepository{db: db}
}

// Create inserts a new VPC record into the database.
func (r *VpcRepository) Create(ctx context.Context, vpc *domain.VPC) error {
	query := `
		INSERT INTO vpcs (id, user_id, tenant_id, name, cidr_block, network_id, vxlan_id, status, arn, created_at, idempotency_key)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::cidr, $6, $7, $8, $9, $10, NULLIF($11, ''))
	`
	_, err := r.db.Exec(ctx, query, vpc.ID, vpc.UserID, vpc.TenantID, vpc.Name, vpc.CIDRBlock, vpc.NetworkID, vpc.VXLANID, vpc.Status, vpc.ARN, vpc.CreatedAt, vpc.IdempotencyKey)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create vpc", err)
	}
	return nil
}

// GetByID retrieves a single VPC by its UUID and ensures it belongs to the authenticated user.
func (r *VpcRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.VPC, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `SELECT id, user_id, tenant_id, name, COALESCE(cidr_block::text, ''), network_id, vxlan_id, status, arn, created_at FROM vpcs WHERE id = $1 AND tenant_id = $2`
	return r.scanVPC(r.db.QueryRow(ctx, query, id, tenantID))
}

// GetByName retrieves a single VPC by its name and ensures it belongs to the authenticated user.
func (r *VpcRepository) GetByName(ctx context.Context, name string) (*domain.VPC, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `SELECT id, user_id, tenant_id, name, COALESCE(cidr_block::text, ''), network_id, vxlan_id, status, arn, created_at FROM vpcs WHERE name = $1 AND tenant_id = $2`
	return r.scanVPC(r.db.QueryRow(ctx, query, name, tenantID))
}

// GetByIdempotencyKey retrieves a VPC by its idempotency key (scoped to user).
func (r *VpcRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.VPC, error) {
	if key == "" {
		return nil, errors.New(errors.NotFound, "idempotency key empty")
	}
	userID := appcontext.UserIDFromContext(ctx)
	query := `SELECT id, user_id, tenant_id, name, COALESCE(cidr_block::text, ''), network_id, vxlan_id, status, arn, created_at FROM vpcs WHERE idempotency_key = $1 AND user_id = $2`
	return r.scanVPC(r.db.QueryRow(ctx, query, key, userID))
}

// List returns all VPCs belonging to the authenticated user.
func (r *VpcRepository) List(ctx context.Context) ([]*domain.VPC, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `SELECT id, user_id, tenant_id, name, COALESCE(cidr_block::text, ''), network_id, vxlan_id, status, arn, created_at FROM vpcs WHERE tenant_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list vpcs", err)
	}
	return r.scanVPCs(rows)
}

func (r *VpcRepository) scanVPC(row pgx.Row) (*domain.VPC, error) {
	var vpc domain.VPC
	err := row.Scan(&vpc.ID, &vpc.UserID, &vpc.TenantID, &vpc.Name, &vpc.CIDRBlock, &vpc.NetworkID, &vpc.VXLANID, &vpc.Status, &vpc.ARN, &vpc.CreatedAt)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, "vpc not found")
		}
		return nil, errors.Wrap(errors.Internal, "failed to scan vpc", err)
	}
	return &vpc, nil
}

func (r *VpcRepository) scanVPCs(rows pgx.Rows) ([]*domain.VPC, error) {
	defer rows.Close()
	var vpcs []*domain.VPC
	for rows.Next() {
		vpc, err := r.scanVPC(rows)
		if err != nil {
			return nil, err
		}
		vpcs = append(vpcs, vpc)
	}
	return vpcs, nil
}

// Delete removes a VPC record from the database.
func (r *VpcRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `DELETE FROM vpcs WHERE id = $1 AND tenant_id = $2`
	cmd, err := r.db.Exec(ctx, query, id, tenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete vpc", err)
	}
	if cmd.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "vpc not found")
	}
	return nil
}

// Update modifies an existing VPC record.
func (r *VpcRepository) Update(ctx context.Context, vpc *domain.VPC) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `UPDATE vpcs SET name = $1 WHERE id = $2 AND tenant_id = $3`
	cmd, err := r.db.Exec(ctx, query, vpc.Name, vpc.ID, tenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to update vpc", err)
	}
	if cmd.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "vpc not found")
	}
	return nil
}
