package postgres

import (
	"context"
	stdlib_errors "errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
)

type FederatedIdentityRepository struct {
	db DB
}

func NewFederatedIdentityRepository(db DB) *FederatedIdentityRepository {
	return &FederatedIdentityRepository{db: db}
}

func (r *FederatedIdentityRepository) Create(ctx context.Context, fi *domain.FederatedIdentity) error {
	query := `
		INSERT INTO federated_identities (id, user_id, idp_id, subject, email, email_verified, groups, last_login_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, query, fi.ID, fi.UserID, fi.IdPID, fi.Subject, fi.Email, fi.EmailVerified, fi.Groups, fi.LastLoginAt, fi.CreatedAt)
	return err
}

func (r *FederatedIdentityRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.FederatedIdentity, error) {
	query := `
		SELECT id, user_id, idp_id, subject, email, email_verified, groups, last_login_at, created_at
		FROM federated_identities
		WHERE user_id = $1
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var identities []*domain.FederatedIdentity
	for rows.Next() {
		var fi domain.FederatedIdentity
		if err := rows.Scan(&fi.ID, &fi.UserID, &fi.IdPID, &fi.Subject, &fi.Email, &fi.EmailVerified, &fi.Groups, &fi.LastLoginAt, &fi.CreatedAt); err != nil {
			return nil, err
		}
		identities = append(identities, &fi)
	}
	return identities, rows.Err()
}

func (r *FederatedIdentityRepository) GetByIdPAndSubject(ctx context.Context, idpID uuid.UUID, subject string) (*domain.FederatedIdentity, error) {
	query := `
		SELECT id, user_id, idp_id, subject, email, email_verified, groups, last_login_at, created_at
		FROM federated_identities
		WHERE idp_id = $1 AND subject = $2
	`
	var fi domain.FederatedIdentity
	err := r.db.QueryRow(ctx, query, idpID, subject).Scan(
		&fi.ID, &fi.UserID, &fi.IdPID, &fi.Subject, &fi.Email, &fi.EmailVerified, &fi.Groups, &fi.LastLoginAt, &fi.CreatedAt,
	)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, "federated identity not found")
		}
		return nil, err
	}
	return &fi, nil
}

func (r *FederatedIdentityRepository) Update(ctx context.Context, fi *domain.FederatedIdentity) error {
	query := `
		UPDATE federated_identities
		SET email = $1, email_verified = $2, groups = $3, last_login_at = $4
		WHERE id = $5
	`
	_, err := r.db.Exec(ctx, query, fi.Email, fi.EmailVerified, fi.Groups, fi.LastLoginAt, fi.ID)
	return err
}

func (r *FederatedIdentityRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Exec(ctx, "DELETE FROM federated_identities WHERE id = $1", id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "federated identity not found")
	}
	return nil
}

func (r *FederatedIdentityRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM federated_identities WHERE user_id = $1", userID)
	return err
}