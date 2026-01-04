package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

type PostgresGatewayRepository struct {
	db *pgxpool.Pool
}

func NewPostgresGatewayRepository(db *pgxpool.Pool) ports.GatewayRepository {
	return &PostgresGatewayRepository{db: db}
}

func (r *PostgresGatewayRepository) CreateRoute(ctx context.Context, route *domain.GatewayRoute) error {
	query := `
		INSERT INTO gateway_routes (id, user_id, name, path_prefix, target_url, strip_prefix, rate_limit, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, query,
		route.ID,
		route.UserID,
		route.Name,
		route.PathPrefix,
		route.TargetURL,
		route.StripPrefix,
		route.RateLimit,
		route.CreatedAt,
		route.UpdatedAt,
	)
	return err
}

func (r *PostgresGatewayRepository) GetRouteByID(ctx context.Context, id, userID uuid.UUID) (*domain.GatewayRoute, error) {
	query := `SELECT id, user_id, name, path_prefix, target_url, strip_prefix, rate_limit, created_at, updated_at FROM gateway_routes WHERE id = $1 AND user_id = $2`
	var route domain.GatewayRoute
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&route.ID,
		&route.UserID,
		&route.Name,
		&route.PathPrefix,
		&route.TargetURL,
		&route.StripPrefix,
		&route.RateLimit,
		&route.CreatedAt,
		&route.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &route, nil
}

func (r *PostgresGatewayRepository) ListRoutes(ctx context.Context, userID uuid.UUID) ([]*domain.GatewayRoute, error) {
	query := `SELECT id, user_id, name, path_prefix, target_url, strip_prefix, rate_limit, created_at, updated_at FROM gateway_routes WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []*domain.GatewayRoute
	for rows.Next() {
		var route domain.GatewayRoute
		if err := rows.Scan(
			&route.ID,
			&route.UserID,
			&route.Name,
			&route.PathPrefix,
			&route.TargetURL,
			&route.StripPrefix,
			&route.RateLimit,
			&route.CreatedAt,
			&route.UpdatedAt,
		); err != nil {
			return nil, err
		}
		routes = append(routes, &route)
	}
	return routes, nil
}

func (r *PostgresGatewayRepository) DeleteRoute(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM gateway_routes WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

func (r *PostgresGatewayRepository) GetAllActiveRoutes(ctx context.Context) ([]*domain.GatewayRoute, error) {
	query := `SELECT id, user_id, name, path_prefix, target_url, strip_prefix, rate_limit, created_at, updated_at FROM gateway_routes`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []*domain.GatewayRoute
	for rows.Next() {
		var route domain.GatewayRoute
		if err := rows.Scan(
			&route.ID,
			&route.UserID,
			&route.Name,
			&route.PathPrefix,
			&route.TargetURL,
			&route.StripPrefix,
			&route.RateLimit,
			&route.CreatedAt,
			&route.UpdatedAt,
		); err != nil {
			return nil, err
		}
		routes = append(routes, &route)
	}
	return routes, nil
}
