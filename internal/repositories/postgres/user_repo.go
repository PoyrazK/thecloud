// Package postgres implements the PostgreSQL repositories.
package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/poyrazk/thecloud/internal/core"
	"github.com/poyrazk/thecloud/internal/core/domain"
	internalerrors "github.com/poyrazk/thecloud/internal/errors"
)

// txCapableDB is implemented by both pgx.Tx and DB (pgxpool.Pool/DualDB).
// This allows repos to check context for a transaction and use it transparently.
type txCapableDB interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

// UserRepo provides PostgreSQL-backed user persistence.
type UserRepo struct {
	db DB
}

// NewUserRepo creates a UserRepo using the provided DB.
func NewUserRepo(db DB) *UserRepo {
	return &UserRepo{db: db}
}

// getDB returns the underlying DB, or a transaction from context if present.
func (r *UserRepo) getDB(ctx context.Context) txCapableDB {
	if tx := core.TransactionFromContext(ctx); tx != nil {
		return tx.(pgx.Tx)
	}
	return r.db
}

func (r *UserRepo) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (id, email, password_hash, name, role, default_tenant_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.getDB(ctx).Exec(ctx, query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.Name,
		user.Role,
		user.DefaultTenantID,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return internalerrors.Wrap(internalerrors.Internal, "failed to create user", err)
	}
	return nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, name, role, default_tenant_id, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	return r.scanUser(r.getDB(ctx).QueryRow(ctx, query, email))
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, name, role, default_tenant_id, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	return r.scanUser(r.getDB(ctx).QueryRow(ctx, query, id))
}

func (r *UserRepo) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET email = $1, password_hash = $2, name = $3, role = $4, default_tenant_id = $5, updated_at = $6
		WHERE id = $7
	`
	_, err := r.getDB(ctx).Exec(ctx, query,
		user.Email,
		user.PasswordHash,
		user.Name,
		user.Role,
		user.DefaultTenantID,
		user.UpdatedAt,
		user.ID,
	)
	if err != nil {
		return internalerrors.Wrap(internalerrors.Internal, "failed to update user", err)
	}
	return nil
}

func (r *UserRepo) List(ctx context.Context) ([]*domain.User, error) {
	query := `
		SELECT id, email, password_hash, name, role, default_tenant_id, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
	`
	rows, err := r.getDB(ctx).Query(ctx, query)
	if err != nil {
		return nil, internalerrors.Wrap(internalerrors.Internal, "failed to list users", err)
	}
	return r.scanUsers(rows)
}

func (r *UserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		DELETE FROM users
		WHERE id = $1
	`
	_, err := r.getDB(ctx).Exec(ctx, query, id)
	if err != nil {
		return internalerrors.Wrap(internalerrors.Internal, "failed to delete user", err)
	}
	return nil
}

func (r *UserRepo) scanUser(row pgx.Row) (*domain.User, error) {
	user := &domain.User{}
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
		&user.Role,
		&user.DefaultTenantID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, internalerrors.New(internalerrors.NotFound, "user not found")
		}
		return nil, internalerrors.Wrap(internalerrors.Internal, "failed to scan user", err)
	}
	return user, nil
}

func (r *UserRepo) scanUsers(rows pgx.Rows) ([]*domain.User, error) {
	defer rows.Close()
	var users []*domain.User
	for rows.Next() {
		user, err := r.scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}
