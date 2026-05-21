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

// LBRepository provides PostgreSQL-backed load balancer persistence.
type LBRepository struct {
	db DB
}

// NewLBRepository creates an LBRepository using the provided DB.
func NewLBRepository(db DB) *LBRepository {
	return &LBRepository{db: db}
}

func (r *LBRepository) Create(ctx context.Context, lb *domain.LoadBalancer) error {
	query := `
		INSERT INTO load_balancers (id, user_id, idempotency_key, name, vpc_id, port, algorithm, ip, status, version, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.Exec(ctx, query,
		lb.ID, lb.UserID, lb.IdempotencyKey, lb.Name, lb.VpcID, lb.Port, lb.Algorithm, lb.IP, lb.Status, lb.Version, lb.CreatedAt,
	)
	if err != nil {
		// Check for unique constraint violation on idempotency_key
		return errors.Wrap(errors.Internal, "failed to create load balancer", err)
	}
	return nil
}

func (r *LBRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.LoadBalancer, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT id, user_id, COALESCE(idempotency_key, ''), name, vpc_id, port, algorithm, COALESCE(ip, ''), status, version, created_at
		FROM load_balancers
		WHERE id = $1 AND tenant_id = $2
	`
	return r.scanLB(r.db.QueryRow(ctx, query, id, tenantID))
}

func (r *LBRepository) GetByName(ctx context.Context, name string) (*domain.LoadBalancer, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT id, user_id, COALESCE(idempotency_key, ''), name, vpc_id, port, algorithm, COALESCE(ip, ''), status, version, created_at
		FROM load_balancers
		WHERE name = $1 AND tenant_id = $2
	`
	return r.scanLB(r.db.QueryRow(ctx, query, name, tenantID))
}

func (r *LBRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.LoadBalancer, error) {
	if key == "" {
		return nil, errors.New(errors.NotFound, "idempotency key empty")
	}
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT id, user_id, idempotency_key, name, vpc_id, port, algorithm, COALESCE(ip, ''), status, version, created_at
		FROM load_balancers
		WHERE idempotency_key = $1 AND tenant_id = $2
	`
	return r.scanLB(r.db.QueryRow(ctx, query, key, tenantID))
}

func (r *LBRepository) List(ctx context.Context) ([]*domain.LoadBalancer, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT id, user_id, COALESCE(idempotency_key, ''), name, vpc_id, port, algorithm, COALESCE(ip, ''), status, version, created_at
		FROM load_balancers
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list load balancers", err)
	}
	return r.scanLBs(rows)
}

func (r *LBRepository) ListAll(ctx context.Context) ([]*domain.LoadBalancer, error) {
	query := `
		SELECT id, user_id, COALESCE(idempotency_key, ''), name, vpc_id, port, algorithm, COALESCE(ip, ''), status, version, created_at
		FROM load_balancers
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list all load balancers", err)
	}
	return r.scanLBs(rows)
}

func (r *LBRepository) ListByStatus(ctx context.Context, status string, limit, offset int) ([]*domain.LoadBalancer, error) {
	query := `
		SELECT id, user_id, COALESCE(idempotency_key, ''), name, vpc_id, port, algorithm, COALESCE(ip, ''), status, version, created_at
		FROM load_balancers
		WHERE status = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, status, limit, offset)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list load balancers by status", err)
	}
	return r.scanLBs(rows)
}

func (r *LBRepository) scanLB(row pgx.Row) (*domain.LoadBalancer, error) {
	var lb domain.LoadBalancer
	var status string
	err := row.Scan(
		&lb.ID, &lb.UserID, &lb.IdempotencyKey, &lb.Name, &lb.VpcID, &lb.Port, &lb.Algorithm, &lb.IP, &status, &lb.Version, &lb.CreatedAt,
	)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.ErrLBNotFound
		}
		return nil, errors.Wrap(errors.Internal, "failed to scan load balancer", err)
	}
	lb.Status = domain.LBStatus(status)
	return &lb, nil
}

func (r *LBRepository) scanLBs(rows pgx.Rows) ([]*domain.LoadBalancer, error) {
	defer rows.Close()
	var lbs []*domain.LoadBalancer
	for rows.Next() {
		lb, err := r.scanLB(rows)
		if err != nil {
			return nil, err
		}
		lbs = append(lbs, lb)
	}
	return lbs, nil
}

func (r *LBRepository) Update(ctx context.Context, lb *domain.LoadBalancer) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		UPDATE load_balancers
		SET name = $1, port = $2, algorithm = $3, ip = $4, status = $5, version = version + 1
		WHERE id = $6 AND version = $7 AND tenant_id = $8
	`
	cmd, err := r.db.Exec(ctx, query, lb.Name, lb.Port, lb.Algorithm, lb.IP, lb.Status, lb.ID, lb.Version, tenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to update load balancer", err)
	}
	if cmd.RowsAffected() == 0 {
		return errors.New(errors.Conflict, "update conflict: load balancer was modified or not found")
	}
	lb.Version++
	return nil
}

func (r *LBRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `DELETE FROM load_balancers WHERE id = $1 AND tenant_id = $2`
	cmd, err := r.db.Exec(ctx, query, id, tenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete load balancer", err)
	}
	if cmd.RowsAffected() == 0 {
		return errors.ErrLBNotFound
	}
	return nil
}

func (r *LBRepository) AddTarget(ctx context.Context, target *domain.LBTarget) error {
	query := `
		INSERT INTO lb_targets (id, lb_id, instance_id, port, weight, health)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, query,
		target.ID, target.LBID, target.InstanceID, target.Port, target.Weight, target.Health,
	)
	if err != nil {
		// Handle unique constraint on (lb_id, instance_id)
		return errors.Wrap(errors.Internal, "failed to add load balancer target", err)
	}
	return nil
}

func (r *LBRepository) RemoveTarget(ctx context.Context, lbID, instanceID uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		DELETE FROM lb_targets
		WHERE lb_id = $1 AND instance_id = $2
		AND lb_id IN (SELECT id FROM load_balancers WHERE tenant_id = $3)
	`
	cmd, err := r.db.Exec(ctx, query, lbID, instanceID, tenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to remove load balancer target", err)
	}
	if cmd.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "target not found")
	}
	return nil
}

func (r *LBRepository) ListTargets(ctx context.Context, lbID uuid.UUID) ([]*domain.LBTarget, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT t.id, t.lb_id, t.instance_id, t.port, t.weight, t.health
		FROM lb_targets t
		WHERE t.lb_id = $1 AND t.lb_id IN (SELECT id FROM load_balancers WHERE tenant_id = $2)
	`
	rows, err := r.db.Query(ctx, query, lbID, tenantID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list load balancer targets", err)
	}
	return r.scanTargets(rows)
}

func (r *LBRepository) UpdateTargetHealth(ctx context.Context, lbID, instanceID uuid.UUID, health string) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		UPDATE lb_targets
		SET health = $1
		WHERE lb_id = $2 AND instance_id = $3
		AND lb_id IN (SELECT id FROM load_balancers WHERE tenant_id = $4)
	`
	_, err := r.db.Exec(ctx, query, health, lbID, instanceID, tenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to update target health", err)
	}
	return nil
}

func (r *LBRepository) GetTargetsForInstance(ctx context.Context, instanceID uuid.UUID) ([]*domain.LBTarget, error) {
	query := `
		SELECT id, lb_id, instance_id, port, weight, health
		FROM lb_targets
		WHERE instance_id = $1
	`
	rows, err := r.db.Query(ctx, query, instanceID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to get targets for instance", err)
	}
	return r.scanTargets(rows)
}

func (r *LBRepository) scanTarget(row pgx.Row) (*domain.LBTarget, error) {
	var t domain.LBTarget
	err := row.Scan(&t.ID, &t.LBID, &t.InstanceID, &t.Port, &t.Weight, &t.Health)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to scan load balancer target", err)
	}
	return &t, nil
}

func (r *LBRepository) scanTargets(rows pgx.Rows) ([]*domain.LBTarget, error) {
	defer rows.Close()
	var targets []*domain.LBTarget
	for rows.Next() {
		t, err := r.scanTarget(rows)
		if err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, nil
}
