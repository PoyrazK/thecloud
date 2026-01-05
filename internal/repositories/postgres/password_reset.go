package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

type PasswordResetRepository struct {
	db *sql.DB
}

func NewPasswordResetRepository(db *sql.DB) ports.PasswordResetRepository {
	return &PasswordResetRepository{db: db}
}

func (r *PasswordResetRepository) Create(ctx context.Context, token *domain.PasswordResetToken) error {
	query := `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, used, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, query,
		token.ID,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
		token.Used,
		token.CreatedAt,
	)
	return err
}

func (r *PasswordResetRepository) GetByTokenHash(ctx context.Context, hash string) (*domain.PasswordResetToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, used, created_at
		FROM password_reset_tokens
		WHERE token_hash = $1
	`
	row := r.db.QueryRowContext(ctx, query, hash)

	var t domain.PasswordResetToken
	err := row.Scan(
		&t.ID,
		&t.UserID,
		&t.TokenHash,
		&t.ExpiresAt,
		&t.Used,
		&t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("token not found")
		}
		return nil, err
	}
	return &t, nil
}

func (r *PasswordResetRepository) MarkAsUsed(ctx context.Context, tokenID string) error {
	query := `UPDATE password_reset_tokens SET used = true WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, tokenID)
	return err
}

func (r *PasswordResetRepository) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM password_reset_tokens WHERE expires_at < $1`
	_, err := r.db.ExecContext(ctx, query, time.Now())
	return err
}
