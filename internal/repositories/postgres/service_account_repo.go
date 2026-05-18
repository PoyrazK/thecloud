package postgres

import (
	"context"

	stdlib_errors "errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
)

// ServiceAccountRepository provides PostgreSQL-backed service account persistence.
type ServiceAccountRepository struct {
	db DB
}

// NewServiceAccountRepository creates a ServiceAccountRepository using the provided DB.
func NewServiceAccountRepository(db DB) *ServiceAccountRepository {
	return &ServiceAccountRepository{db: db}
}

// Create creates a new service account.
func (r *ServiceAccountRepository) Create(ctx context.Context, sa *domain.ServiceAccount) error {
	query := `
		INSERT INTO service_accounts (id, tenant_id, name, description, role, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query, sa.ID, sa.TenantID, sa.Name, sa.Description, sa.Role, sa.Enabled, sa.CreatedAt, sa.UpdatedAt)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create service account", err)
	}
	return nil
}

// GetByID retrieves a service account by ID.
func (r *ServiceAccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.ServiceAccount, error) {
	query := `
		SELECT id, tenant_id, name, description, role, enabled, created_at, updated_at
		FROM service_accounts
		WHERE id = $1
	`
	var sa domain.ServiceAccount
	err := r.db.QueryRow(ctx, query, id).Scan(
		&sa.ID, &sa.TenantID, &sa.Name, &sa.Description, &sa.Role, &sa.Enabled, &sa.CreatedAt, &sa.UpdatedAt,
	)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, "service account not found")
		}
		return nil, errors.Wrap(errors.Internal, "failed to get service account", err)
	}
	return &sa, nil
}

// GetByName retrieves a service account by name within a tenant.
func (r *ServiceAccountRepository) GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*domain.ServiceAccount, error) {
	query := `
		SELECT id, tenant_id, name, description, role, enabled, created_at, updated_at
		FROM service_accounts
		WHERE tenant_id = $1 AND name = $2
	`
	var sa domain.ServiceAccount
	err := r.db.QueryRow(ctx, query, tenantID, name).Scan(
		&sa.ID, &sa.TenantID, &sa.Name, &sa.Description, &sa.Role, &sa.Enabled, &sa.CreatedAt, &sa.UpdatedAt,
	)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, "service account not found")
		}
		return nil, errors.Wrap(errors.Internal, "failed to get service account", err)
	}
	return &sa, nil
}

// ListByTenant returns all service accounts for a tenant.
func (r *ServiceAccountRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]*domain.ServiceAccount, error) {
	query := `
		SELECT id, tenant_id, name, description, role, enabled, created_at, updated_at
		FROM service_accounts
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list service accounts", err)
	}
	defer rows.Close()

	var accounts []*domain.ServiceAccount
	for rows.Next() {
		var sa domain.ServiceAccount
		if err := rows.Scan(&sa.ID, &sa.TenantID, &sa.Name, &sa.Description, &sa.Role, &sa.Enabled, &sa.CreatedAt, &sa.UpdatedAt); err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to scan service account", err)
		}
		accounts = append(accounts, &sa)
	}
	return accounts, nil
}

// Update updates a service account.
func (r *ServiceAccountRepository) Update(ctx context.Context, sa *domain.ServiceAccount) error {
	query := `
		UPDATE service_accounts
		SET name = $1, description = $2, role = $3, enabled = $4, updated_at = NOW()
		WHERE id = $5
	`
	result, err := r.db.Exec(ctx, query, sa.Name, sa.Description, sa.Role, sa.Enabled, sa.ID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to update service account", err)
	}
	if result.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "service account not found")
	}
	return nil
}

// Delete removes a service account.
func (r *ServiceAccountRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM service_accounts WHERE id = $1`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete service account", err)
	}
	if result.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "service account not found")
	}
	return nil
}

// CreateSecret creates a new secret for a service account.
func (r *ServiceAccountRepository) CreateSecret(ctx context.Context, secret *domain.ServiceAccountSecret) error {
	query := `
		INSERT INTO service_account_secrets (id, service_account_id, secret_hash, name, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, query, secret.ID, secret.ServiceAccountID, secret.SecretHash, secret.Name, secret.ExpiresAt, secret.CreatedAt)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create service account secret", err)
	}
	return nil
}

// GetSecretByHash retrieves a secret by its hash.
func (r *ServiceAccountRepository) GetSecretByHash(ctx context.Context, secretHash string) (*domain.ServiceAccountSecret, error) {
	query := `
		SELECT id, service_account_id, secret_hash, name, expires_at, created_at, last_used_at
		FROM service_account_secrets
		WHERE secret_hash = $1
	`
	var secret domain.ServiceAccountSecret
	err := r.db.QueryRow(ctx, query, secretHash).Scan(
		&secret.ID, &secret.ServiceAccountID, &secret.SecretHash, &secret.Name,
		&secret.ExpiresAt, &secret.CreatedAt, &secret.LastUsedAt,
	)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.Unauthorized, "invalid client credentials")
		}
		return nil, errors.Wrap(errors.Internal, "failed to get secret", err)
	}
	return &secret, nil
}

// ListSecretsByServiceAccount returns all secrets for a service account.
func (r *ServiceAccountRepository) ListSecretsByServiceAccount(ctx context.Context, saID uuid.UUID) ([]*domain.ServiceAccountSecret, error) {
	query := `
		SELECT id, service_account_id, secret_hash, name, expires_at, created_at, last_used_at
		FROM service_account_secrets
		WHERE service_account_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, saID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list secrets", err)
	}
	defer rows.Close()

	var secrets []*domain.ServiceAccountSecret
	for rows.Next() {
		var secret domain.ServiceAccountSecret
		if err := rows.Scan(
			&secret.ID, &secret.ServiceAccountID, &secret.SecretHash, &secret.Name,
			&secret.ExpiresAt, &secret.CreatedAt, &secret.LastUsedAt,
		); err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to scan secret", err)
		}
		secrets = append(secrets, &secret)
	}
	return secrets, nil
}

// UpdateSecretLastUsed updates the last_used_at timestamp.
func (r *ServiceAccountRepository) UpdateSecretLastUsed(ctx context.Context, secretID uuid.UUID) error {
	query := `UPDATE service_account_secrets SET last_used_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(ctx, query, secretID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to update secret last used", err)
	}
	return nil
}

// DeleteSecret removes a secret.
func (r *ServiceAccountRepository) DeleteSecret(ctx context.Context, secretID uuid.UUID) error {
	query := `DELETE FROM service_account_secrets WHERE id = $1`
	result, err := r.db.Exec(ctx, query, secretID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete secret", err)
	}
	if result.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "secret not found")
	}
	return nil
}

// DeleteAllSecrets removes all secrets for a service account.
func (r *ServiceAccountRepository) DeleteAllSecrets(ctx context.Context, saID uuid.UUID) error {
	query := `DELETE FROM service_account_secrets WHERE service_account_id = $1`
	_, err := r.db.Exec(ctx, query, saID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete secrets", err)
	}
	return nil
}
