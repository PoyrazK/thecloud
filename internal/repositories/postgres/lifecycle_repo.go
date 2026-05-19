// Package postgres provides Postgres-backed repository implementations.
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

// LifecycleRepository stores lifecycle rules in Postgres.
type LifecycleRepository struct {
	db DB
}

// NewLifecycleRepository constructs a LifecycleRepository.
func NewLifecycleRepository(db DB) *LifecycleRepository {
	return &LifecycleRepository{db: db}
}

func (r *LifecycleRepository) Create(ctx context.Context, rule *domain.LifecycleRule) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		INSERT INTO lifecycle_rules (id, user_id, tenant_id, bucket_name, prefix, expiration_days, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, query, rule.ID, rule.UserID, tenantID, rule.BucketName, rule.Prefix, rule.ExpirationDays, rule.Enabled, rule.CreatedAt, rule.UpdatedAt)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create lifecycle rule", err)
	}
	return nil
}

func (r *LifecycleRepository) Get(ctx context.Context, id uuid.UUID) (*domain.LifecycleRule, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT id, user_id, tenant_id, bucket_name, prefix, expiration_days, enabled, created_at, updated_at
		FROM lifecycle_rules
		WHERE id = $1 AND user_id = $2 AND tenant_id = $3
	`
	var rule domain.LifecycleRule
	err := r.db.QueryRow(ctx, query, id, userID, tenantID).Scan(
		&rule.ID, &rule.UserID, &rule.TenantID, &rule.BucketName, &rule.Prefix, &rule.ExpirationDays, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, "lifecycle rule not found")
		}
		return nil, errors.Wrap(errors.Internal, "failed to get lifecycle rule", err)
	}
	return &rule, nil
}

func (r *LifecycleRepository) List(ctx context.Context, bucketName string) ([]*domain.LifecycleRule, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT id, user_id, tenant_id, bucket_name, prefix, expiration_days, enabled, created_at, updated_at
		FROM lifecycle_rules
		WHERE bucket_name = $1 AND user_id = $2 AND tenant_id = $3
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, bucketName, userID, tenantID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list lifecycle rules", err)
	}
	defer rows.Close()

	rules := make([]*domain.LifecycleRule, 0)
	for rows.Next() {
		var rule domain.LifecycleRule
		if err := rows.Scan(&rule.ID, &rule.UserID, &rule.TenantID, &rule.BucketName, &rule.Prefix, &rule.ExpirationDays, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to scan lifecycle rule", err)
		}
		rules = append(rules, &rule)
	}
	return rules, nil
}

func (r *LifecycleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `DELETE FROM lifecycle_rules WHERE id = $1 AND user_id = $2 AND tenant_id = $3`
	cmd, err := r.db.Exec(ctx, query, id, userID, tenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete lifecycle rule", err)
	}
	if cmd.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "lifecycle rule not found")
	}
	return nil
}

// GetEnabledRules retrieves all enabled lifecycle rules across the system.
// This is intended for background workers and does not filter by user.
func (r *LifecycleRepository) GetEnabledRules(ctx context.Context) ([]*domain.LifecycleRule, error) {
	query := `
		SELECT id, user_id, tenant_id, bucket_name, prefix, expiration_days, enabled, created_at, updated_at
		FROM lifecycle_rules
		WHERE enabled = TRUE
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list enabled lifecycle rules", err)
	}
	defer rows.Close()

	rules := make([]*domain.LifecycleRule, 0)
	for rows.Next() {
		var rule domain.LifecycleRule
		if err := rows.Scan(&rule.ID, &rule.UserID, &rule.TenantID, &rule.BucketName, &rule.Prefix, &rule.ExpirationDays, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to scan lifecycle rule", err)
		}
		rules = append(rules, &rule)
	}
	return rules, nil
}
