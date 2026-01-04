package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
)

type IdentityRepository struct {
	db *pgxpool.Pool
}

func NewIdentityRepository(db *pgxpool.Pool) *IdentityRepository {
	return &IdentityRepository{db: db}
}

func (r *IdentityRepository) CreateAPIKey(ctx context.Context, key *domain.APIKey) error {
	query := `
		INSERT INTO api_keys (id, user_id, key, name, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Exec(ctx, query, key.ID, key.UserID, key.Key, key.Name, key.CreatedAt)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create api key", err)
	}
	return nil
}

func (r *IdentityRepository) GetAPIKeyByKey(ctx context.Context, keyStr string) (*domain.APIKey, error) {
	query := `
		SELECT id, user_id, key, name, created_at, last_used
		FROM api_keys
		WHERE key = $1
	`
	var key domain.APIKey
	var lastUsed *time.Time
	err := r.db.QueryRow(ctx, query, keyStr).Scan(
		&key.ID, &key.UserID, &key.Key, &key.Name, &key.CreatedAt, &lastUsed,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New(errors.Unauthorized, "invalid api key")
		}
		return nil, errors.Wrap(errors.Internal, "failed to get api key", err)
	}
	if lastUsed != nil {
		key.LastUsed = *lastUsed
	}
	return &key, nil
}
