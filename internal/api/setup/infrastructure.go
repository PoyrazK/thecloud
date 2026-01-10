package setup

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/platform"
	"github.com/poyrazk/thecloud/internal/repositories/docker"
	"github.com/poyrazk/thecloud/internal/repositories/libvirt"
	"github.com/poyrazk/thecloud/internal/repositories/noop"
	"github.com/poyrazk/thecloud/internal/repositories/ovs"
	"github.com/poyrazk/thecloud/internal/repositories/postgres"
)

// InitDatabase initializes the database connection
func InitDatabase(ctx context.Context, cfg *platform.Config, logger *slog.Logger) (*pgxpool.Pool, error) {
	db, err := platform.NewDatabase(ctx, cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return db, nil
}

// RunMigrations runs database migrations
func RunMigrations(ctx context.Context, db *pgxpool.Pool, logger *slog.Logger) error {
	return postgres.RunMigrations(ctx, db, logger)
}

// InitComputeBackend initializes the compute backend (Docker or Libvirt)
func InitComputeBackend(cfg *platform.Config, logger *slog.Logger) (ports.ComputeBackend, error) {
	if cfg.ComputeBackend == "libvirt" {
		logger.Info("using libvirt compute backend")
		return libvirt.NewLibvirtAdapter(logger, "")
	}
	logger.Info("using docker compute backend")
	return docker.NewDockerAdapter()
}

// InitNetworkBackend initializes the network backend (OVS or No-op)
func InitNetworkBackend(logger *slog.Logger) ports.NetworkBackend {
	ovsAdapter, err := ovs.NewOvsAdapter(logger)
	if err != nil {
		logger.Warn("failed to initialize OVS adapter, using no-op network backend", "error", err)
		return noop.NewNoopNetworkAdapter(logger)
	}
	return ovsAdapter
}

// InitLBProxy initializes the Load Balancer proxy adapter
func InitLBProxy(cfg *platform.Config, computeBackend ports.ComputeBackend, instanceRepo ports.InstanceRepository, vpcRepo ports.VpcRepository) (ports.LBProxyAdapter, error) {
	if cfg.ComputeBackend == "libvirt" {
		return libvirt.NewLBProxyAdapter(computeBackend), nil
	}
	return docker.NewLBProxyAdapter(instanceRepo, vpcRepo)
}
