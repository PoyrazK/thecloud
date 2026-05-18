// Package postgres provides PostgreSQL-backed repository implementations.
package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

// PostgresGatewayRepository provides PostgreSQL-backed gateway route persistence.
type PostgresGatewayRepository struct {
	db DB
}

// NewPostgresGatewayRepository creates a gateway repository using the provided DB.
func NewPostgresGatewayRepository(db DB) ports.GatewayRepository {
	return &PostgresGatewayRepository{db: db}
}

func (r *PostgresGatewayRepository) CreateRoute(ctx context.Context, route *domain.GatewayRoute) error {
	query := `
		INSERT INTO gateway_routes (
			id, user_id, name, path_prefix, path_pattern, pattern_type,
			param_names, target_url, methods, strip_prefix, rate_limit, priority,
			circuit_breaker_threshold, circuit_breaker_timeout, max_retries, retry_timeout,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`
	_, err := r.db.Exec(ctx, query,
		route.ID,
		route.UserID,
		route.Name,
		route.PathPrefix,
		route.PathPattern,
		route.PatternType,
		route.ParamNames,
		route.TargetURL,
		route.Methods,
		route.StripPrefix,
		route.RateLimit,
		route.Priority,
		route.CircuitBreakerThreshold,
		route.CircuitBreakerTimeout,
		route.MaxRetries,
		route.RetryTimeout,
		route.CreatedAt,
		route.UpdatedAt,
	)
	return err
}

func (r *PostgresGatewayRepository) GetRouteByID(ctx context.Context, id, userID uuid.UUID) (*domain.GatewayRoute, error) {
	query := `SELECT id, user_id, name, path_prefix, path_pattern, pattern_type, param_names, target_url, methods, strip_prefix, rate_limit, priority, circuit_breaker_threshold, circuit_breaker_timeout, max_retries, retry_timeout, created_at, updated_at FROM gateway_routes WHERE id = $1 AND user_id = $2`
	return r.scanRoute(r.db.QueryRow(ctx, query, id, userID))
}

func (r *PostgresGatewayRepository) ListRoutes(ctx context.Context, userID uuid.UUID) ([]*domain.GatewayRoute, error) {
	query := `SELECT id, user_id, name, path_prefix, path_pattern, pattern_type, param_names, target_url, methods, strip_prefix, rate_limit, priority, circuit_breaker_threshold, circuit_breaker_timeout, max_retries, retry_timeout, created_at, updated_at FROM gateway_routes WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	return r.scanRoutes(rows)
}

func (r *PostgresGatewayRepository) DeleteRoute(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM gateway_routes WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

func (r *PostgresGatewayRepository) GetAllActiveRoutes(ctx context.Context) ([]*domain.GatewayRoute, error) {
	query := `SELECT id, user_id, name, path_prefix, path_pattern, pattern_type, param_names, target_url, methods, strip_prefix, rate_limit, priority, circuit_breaker_threshold, circuit_breaker_timeout, max_retries, retry_timeout, created_at, updated_at FROM gateway_routes`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	return r.scanRoutes(rows)
}

func (r *PostgresGatewayRepository) scanRoute(row pgx.Row) (*domain.GatewayRoute, error) {
	var route domain.GatewayRoute
	err := row.Scan(
		&route.ID,
		&route.UserID,
		&route.Name,
		&route.PathPrefix,
		&route.PathPattern,
		&route.PatternType,
		&route.ParamNames,
		&route.TargetURL,
		&route.Methods,
		&route.StripPrefix,
		&route.RateLimit,
		&route.Priority,
		&route.CircuitBreakerThreshold,
		&route.CircuitBreakerTimeout,
		&route.MaxRetries,
		&route.RetryTimeout,
		&route.CreatedAt,
		&route.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &route, nil
}

func (r *PostgresGatewayRepository) scanRoutes(rows pgx.Rows) ([]*domain.GatewayRoute, error) {
	defer rows.Close()
	var routes []*domain.GatewayRoute
	for rows.Next() {
		route, err := r.scanRoute(rows)
		if err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	return routes, nil
}
