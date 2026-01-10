package platform

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewDatabase(ctx context.Context, cfg *Config, logger *slog.Logger) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database url: %w", err)
	}

	// Apply performance optimizations
	var maxConns int32
	if _, err := fmt.Sscanf(cfg.DBMaxConns, "%d", &maxConns); err == nil {
		config.MaxConns = maxConns
	}

	var minConns int32
	if _, err := fmt.Sscanf(cfg.DBMinConns, "%d", &minConns); err == nil {
		config.MinConns = minConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	logger.Info("connected to database")
	return pool, nil
}
