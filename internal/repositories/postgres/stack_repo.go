package postgres

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

type stackRepository struct {
	db *pgxpool.Pool
}

func NewStackRepository(db *pgxpool.Pool) *stackRepository {
	return &stackRepository{db: db}
}

func (r *stackRepository) Create(ctx context.Context, s *domain.Stack) error {
	_, err := r.db.Exec(ctx,
		"INSERT INTO stacks (id, user_id, name, template, parameters, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		s.ID, s.UserID, s.Name, s.Template, s.Parameters, s.Status, s.CreatedAt, s.UpdatedAt)
	
	// Check for unique constraint violation on (user_id, name)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			// Unique constraint violation
			if strings.Contains(pgErr.ConstraintName, "user_id") || strings.Contains(pgErr.ConstraintName, "name") {
				return domain.ErrStackNameAlreadyExists
			}
		}
	}
	return err
}

func (r *stackRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Stack, error) {
	s := &domain.Stack{}
	err := r.db.QueryRow(ctx,
		"SELECT id, user_id, name, template, parameters, status, status_reason, created_at, updated_at FROM stacks WHERE id = $1",
		id).Scan(&s.ID, &s.UserID, &s.Name, &s.Template, &s.Parameters, &s.Status, &s.StatusReason, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}

	resources, err := r.ListResources(ctx, id)
	if err == nil {
		s.Resources = resources
	}

	return s, nil
}

func (r *stackRepository) GetByName(ctx context.Context, userID uuid.UUID, name string) (*domain.Stack, error) {
	s := &domain.Stack{}
	err := r.db.QueryRow(ctx,
		"SELECT id, user_id, name, template, parameters, status, status_reason, created_at, updated_at FROM stacks WHERE user_id = $1 AND name = $2",
		userID, name).Scan(&s.ID, &s.UserID, &s.Name, &s.Template, &s.Parameters, &s.Status, &s.StatusReason, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *stackRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Stack, error) {
	rows, err := r.db.Query(ctx,
		"SELECT id, user_id, name, template, parameters, status, status_reason, created_at, updated_at FROM stacks WHERE user_id = $1 ORDER BY created_at DESC",
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stacks []*domain.Stack
	for rows.Next() {
		s := &domain.Stack{}
		if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &s.Template, &s.Parameters, &s.Status, &s.StatusReason, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		stacks = append(stacks, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stacks, nil
}

func (r *stackRepository) Update(ctx context.Context, s *domain.Stack) error {
	_, err := r.db.Exec(ctx,
		"UPDATE stacks SET status = $1, status_reason = $2, updated_at = $3 WHERE id = $4",
		s.Status, s.StatusReason, s.UpdatedAt, s.ID)
	return err
}

func (r *stackRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM stacks WHERE id = $1", id)
	return err
}

func (r *stackRepository) AddResource(ctx context.Context, res *domain.StackResource) error {
	_, err := r.db.Exec(ctx,
		"INSERT INTO stack_resources (id, stack_id, logical_id, physical_id, resource_type, status, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		res.ID, res.StackID, res.LogicalID, res.PhysicalID, res.ResourceType, res.Status, res.CreatedAt)
	return err
}

func (r *stackRepository) ListResources(ctx context.Context, stackID uuid.UUID) ([]domain.StackResource, error) {
	rows, err := r.db.Query(ctx,
		"SELECT id, stack_id, logical_id, physical_id, resource_type, status, created_at FROM stack_resources WHERE stack_id = $1",
		stackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var resources []domain.StackResource
	for rows.Next() {
		res := domain.StackResource{}
		if err := rows.Scan(&res.ID, &res.StackID, &res.LogicalID, &res.PhysicalID, &res.ResourceType, &res.Status, &res.CreatedAt); err != nil {
			return nil, err
		}
		resources = append(resources, res)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return resources, nil
}

func (r *stackRepository) DeleteResources(ctx context.Context, stackID uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM stack_resources WHERE stack_id = $1", stackID)
	return err
}
