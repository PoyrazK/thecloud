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

const (
	vpcPeeringColumns      = "id, requester_vpc_id, accepter_vpc_id, requester_tenant_id, accepter_tenant_id, status, arn, created_at, updated_at"
	errPeeringNotFound    = "vpc peering not found"
)

// VPCPeeringRepository provides a PostgreSQL implementation for managing VPC peering connections.
type VPCPeeringRepository struct {
	db DB
}

// NewVPCPeeringRepository creates a new VPCPeeringRepository with the given database pool.
func NewVPCPeeringRepository(db DB) *VPCPeeringRepository {
	return &VPCPeeringRepository{db: db}
}

// Create inserts a new VPC peering connection record into the database.
func (r *VPCPeeringRepository) Create(ctx context.Context, peering *domain.VPCPeering) error {
	query := `
		INSERT INTO vpc_peerings (id, requester_vpc_id, accepter_vpc_id, requester_tenant_id, accepter_tenant_id, status, arn, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, query,
		peering.ID, peering.RequesterVPCID, peering.AccepterVPCID,
		peering.RequesterTenantID, peering.AccepterTenantID,
		peering.Status, peering.ARN,
		peering.CreatedAt, peering.UpdatedAt,
	)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create vpc peering", err)
	}
	return nil
}

// GetByID retrieves a single VPC peering connection by its UUID.
// Returns the peering if the caller belongs to either the requester or accepter tenant.
func (r *VPCPeeringRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.VPCPeering, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT ` + vpcPeeringColumns + `
		FROM vpc_peerings
		WHERE id = $1 AND (requester_tenant_id = $2 OR accepter_tenant_id = $2)
	`
	return r.scanPeering(r.db.QueryRow(ctx, query, id, tenantID))
}

// List returns all VPC peering connections for a given tenant (including cross-tenant peerings).
func (r *VPCPeeringRepository) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.VPCPeering, error) {
	query := `
		SELECT ` + vpcPeeringColumns + `
		FROM vpc_peerings
		WHERE (requester_tenant_id = $1 OR accepter_tenant_id = $1) AND status != $2
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, tenantID, domain.PeeringStatusDeleted)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list vpc peerings", err)
	}
	return r.scanPeerings(rows)
}

// ListByVPC returns all peering connections involving a specific VPC.
func (r *VPCPeeringRepository) ListByVPC(ctx context.Context, vpcID uuid.UUID) ([]*domain.VPCPeering, error) {
	query := `
		SELECT ` + vpcPeeringColumns + `
		FROM vpc_peerings
		WHERE (requester_vpc_id = $1 OR accepter_vpc_id = $1) AND status != $2
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, vpcID, domain.PeeringStatusDeleted)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list vpc peerings by vpc", err)
	}
	return r.scanPeerings(rows)
}

// UpdateStatus changes the status of a peering connection.
// Allows update from either the requester or accepter tenant.
func (r *VPCPeeringRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		UPDATE vpc_peerings
		SET status = $1, updated_at = NOW()
		WHERE id = $2 AND (requester_tenant_id = $3 OR accepter_tenant_id = $3)
	`
	cmd, err := r.db.Exec(ctx, query, status, id, tenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to update vpc peering status", err)
	}
	if cmd.RowsAffected() == 0 {
		return errors.New(errors.NotFound, errPeeringNotFound)
	}
	return nil
}

// Delete removes a VPC peering connection record from the database.
// Allows deletion from either the requester or accepter tenant.
func (r *VPCPeeringRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `DELETE FROM vpc_peerings WHERE id = $1 AND (requester_tenant_id = $2 OR accepter_tenant_id = $2)`
	cmd, err := r.db.Exec(ctx, query, id, tenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete vpc peering", err)
	}
	if cmd.RowsAffected() == 0 {
		return errors.New(errors.NotFound, errPeeringNotFound)
	}
	return nil
}

// GetActiveByVPCPair returns an existing active or pending peering between two VPCs.
func (r *VPCPeeringRepository) GetActiveByVPCPair(ctx context.Context, vpc1, vpc2 uuid.UUID) (*domain.VPCPeering, error) {
	query := `
		SELECT ` + vpcPeeringColumns + `
		FROM vpc_peerings
		WHERE LEAST(requester_vpc_id, accepter_vpc_id) = LEAST($1::uuid, $2::uuid)
			AND GREATEST(requester_vpc_id, accepter_vpc_id) = GREATEST($1::uuid, $2::uuid)
			AND status IN ($3, $4)
	`
	return r.scanPeering(r.db.QueryRow(ctx, query, vpc1, vpc2, domain.PeeringStatusPendingAcceptance, domain.PeeringStatusActive))
}

func (r *VPCPeeringRepository) scanPeering(row pgx.Row) (*domain.VPCPeering, error) {
	var p domain.VPCPeering
	err := row.Scan(
		&p.ID, &p.RequesterVPCID, &p.AccepterVPCID,
		&p.RequesterTenantID, &p.AccepterTenantID,
		&p.Status, &p.ARN,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, errPeeringNotFound)
		}
		return nil, errors.Wrap(errors.Internal, "failed to scan vpc peering", err)
	}
	return &p, nil
}

func (r *VPCPeeringRepository) scanPeerings(rows pgx.Rows) ([]*domain.VPCPeering, error) {
	defer rows.Close()
	var peerings []*domain.VPCPeering
	for rows.Next() {
		p, err := r.scanPeering(rows)
		if err != nil {
			return nil, err
		}
		peerings = append(peerings, p)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to iterate vpc peerings", err)
	}
	return peerings, nil
}
