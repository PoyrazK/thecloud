package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
)

type SubnetRepository struct {
	db *pgxpool.Pool
}

// NewSubnetRepository creates a SubnetRepository backed by the provided pgxpool.Pool.
// The returned repository uses the pool for all database operations related to subnets.
func NewSubnetRepository(db *pgxpool.Pool) *SubnetRepository {
	return &SubnetRepository{db: db}
}

func (r *SubnetRepository) Create(ctx context.Context, subnet *domain.Subnet) error {
	query := `
		INSERT INTO subnets (id, user_id, vpc_id, name, cidr_block, availability_zone, gateway_ip, arn, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(ctx, query, subnet.ID, subnet.UserID, subnet.VPCID, subnet.Name, subnet.CIDRBlock, subnet.AvailabilityZone, subnet.GatewayIP, subnet.ARN, subnet.Status, subnet.CreatedAt)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create subnet", err)
	}
	return nil
}

func (r *SubnetRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Subnet, error) {
	userID := appcontext.UserIDFromContext(ctx)
	query := `SELECT id, user_id, vpc_id, name, cidr_block, availability_zone, gateway_ip, arn, status, created_at FROM subnets WHERE id = $1 AND user_id = $2`
	var s domain.Subnet
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&s.ID, &s.UserID, &s.VPCID, &s.Name, &s.CIDRBlock, &s.AvailabilityZone, &s.GatewayIP, &s.ARN, &s.Status, &s.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New(errors.NotFound, fmt.Sprintf("subnet %s not found", id))
		}
		return nil, errors.Wrap(errors.Internal, "failed to get subnet", err)
	}
	return &s, nil
}

func (r *SubnetRepository) GetByName(ctx context.Context, vpcID uuid.UUID, name string) (*domain.Subnet, error) {
	userID := appcontext.UserIDFromContext(ctx)
	query := `SELECT id, user_id, vpc_id, name, cidr_block, availability_zone, gateway_ip, arn, status, created_at FROM subnets WHERE vpc_id = $1 AND name = $2 AND user_id = $3`
	var s domain.Subnet
	err := r.db.QueryRow(ctx, query, vpcID, name, userID).Scan(
		&s.ID, &s.UserID, &s.VPCID, &s.Name, &s.CIDRBlock, &s.AvailabilityZone, &s.GatewayIP, &s.ARN, &s.Status, &s.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New(errors.NotFound, fmt.Sprintf("subnet %s not found in vpc %s", name, vpcID))
		}
		return nil, errors.Wrap(errors.Internal, "failed to get subnet by name", err)
	}
	return &s, nil
}

func (r *SubnetRepository) ListByVPC(ctx context.Context, vpcID uuid.UUID) ([]*domain.Subnet, error) {
	userID := appcontext.UserIDFromContext(ctx)
	query := `SELECT id, user_id, vpc_id, name, cidr_block, availability_zone, gateway_ip, arn, status, created_at FROM subnets WHERE vpc_id = $1 AND user_id = $2 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, vpcID, userID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list subnets", err)
	}
	defer rows.Close()

	var subnets []*domain.Subnet
	for rows.Next() {
		var s domain.Subnet
		err := rows.Scan(
			&s.ID, &s.UserID, &s.VPCID, &s.Name, &s.CIDRBlock, &s.AvailabilityZone, &s.GatewayIP, &s.ARN, &s.Status, &s.CreatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to scan subnet", err)
		}
		subnets = append(subnets, &s)
	}
	return subnets, nil
}

func (r *SubnetRepository) Delete(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	query := `DELETE FROM subnets WHERE id = $1 AND user_id = $2`
	cmd, err := r.db.Exec(ctx, query, id, userID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete subnet", err)
	}
	if cmd.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "subnet not found")
	}
	return nil
}