package main

// @title The Cloud API
// @version 1.0
// @description The Cloud - Multi-tenant Cloud Infrastructure Engine

// @securityDefinitions.apikey APIKeyAuth
// @in header
// @name X-API-Key

import (
	"context"
	stdlib_errors "errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/poyrazk/thecloud/internal/api/setup"
	"github.com/poyrazk/thecloud/internal/core/ports"
	apphandlers "github.com/poyrazk/thecloud/internal/handlers"
	"github.com/poyrazk/thecloud/internal/platform"
	"github.com/poyrazk/thecloud/internal/repositories/postgres"
	"github.com/poyrazk/thecloud/pkg/tracing"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const (
	defaultDBInitTimeout     = 120 * time.Second
	defaultReadHeaderTimeout = 10 * time.Second
	defaultShutdownTimeout   = 30 * time.Second
)

// ErrMigrationDone signals that migrations have already completed.
var ErrMigrationDone = stdlib_errors.New("migrations done")

// AppDeps holds the dependencies for the application, enabling DI for testing.
type AppDeps struct {
	LoadConfig         func(*slog.Logger) (*platform.Config, error)
	InitDatabase       func(context.Context, *platform.Config, *slog.Logger) (postgres.DB, error)
	RunMigrations      func(context.Context, postgres.DB, *slog.Logger) error
	InitRedis          func(context.Context, *platform.Config, *slog.Logger) (*redis.Client, error)
	InitComputeBackend func(*platform.Config, *slog.Logger) (ports.ComputeBackend, error)
	InitStorageBackend func(*platform.Config, *slog.Logger) (ports.StorageBackend, error)
	InitNetworkBackend func(*platform.Config, *slog.Logger) ports.NetworkBackend
	InitLBProxy        func(*platform.Config, ports.ComputeBackend, ports.InstanceRepository, ports.VpcRepository) (ports.LBProxyAdapter, error)
	InitRepositories   func(postgres.DB, *redis.Client) *setup.Repositories
	InitServices       func(setup.ServiceConfig) (*setup.Services, *setup.Workers, error)
	InitHandlers       func(*setup.Services, *platform.Config, *slog.Logger) *setup.Handlers
	SetupRouter        func(*platform.Config, *slog.Logger, *setup.Handlers, *setup.Services, ports.NetworkBackend) *gin.Engine
	NewHTTPServer      func(string, http.Handler) *http.Server
	StartHTTPServer    func(*http.Server) error
	ShutdownHTTPServer func(context.Context, *http.Server) error
	NotifySignals      func(chan<- os.Signal, ...os.Signal)
}

// DefaultDeps returns the production dependency wiring for the API server.
func DefaultDeps() AppDeps {
	return AppDeps{
		LoadConfig:         setup.LoadConfig,
		InitDatabase:       setup.InitDatabase,
		RunMigrations:      setup.RunMigrations,
		InitRedis:          setup.InitRedis,
		InitComputeBackend: setup.InitComputeBackend,
		InitStorageBackend: setup.InitStorageBackend,
		InitNetworkBackend: setup.InitNetworkBackend,
		InitLBProxy:        setup.InitLBProxy,
		InitRepositories:   setup.InitRepositories,
		InitServices:       setup.InitServices,
		InitHandlers:       setup.InitHandlers,
		SetupRouter:        setup.SetupRouter,
		NewHTTPServer: func(addr string, handler http.Handler) *http.Server {
			return &http.Server{
				Addr:              addr,
				Handler:           handler,
				ReadHeaderTimeout: defaultReadHeaderTimeout,
			}
		},
		StartHTTPServer: func(s *http.Server) error {
			return s.ListenAndServe()
		},
		ShutdownHTTPServer: func(ctx context.Context, s *http.Server) error {
			return s.Shutdown(ctx)
		},
		NotifySignals: func(c chan<- os.Signal, sigs ...os.Signal) {
			signal.Notify(c, sigs...)
		},
	}
}

func main() {
	if err := run(); err != nil {
		if !stdlib_errors.Is(err, ErrMigrationDone) {
			os.Exit(1)
		}
	}
}

func run() error {
	logger := setup.InitLogger()
	migrateOnly := flag.Bool("migrate-only", false, "run database migrations and exit")
	flag.Parse()

	deps := DefaultDeps()

	// Initialize Tracing (Opt-in)
	if tp := initTracing(logger); tp != nil {
		defer func() {
			if err := tp.Shutdown(context.Background()); err != nil {
				logger.Error("failed to shutdown tracer", "error", err)
			}
		}()
	}

	cfg, db, rdb, err := initInfrastructure(deps, logger, *migrateOnly)
	if err != nil {
		if *migrateOnly && stdlib_errors.Is(err, ErrMigrationDone) {
			return ErrMigrationDone
		}
		logger.Error("initialization failed", "error", err)
		return err
	}
	defer db.Close()
	defer func() { _ = rdb.Close() }()

	rawCompute, rawStorage, rawNetwork, rawLBProxy, err := initBackends(deps, cfg, logger, db, rdb)
	if err != nil {
		logger.Error("backend initialization failed", "error", err)
		return err
	}

	// Wrap raw backends with resilience decorators (circuit breaker, bulkhead, timeouts).
	compute := platform.NewResilientCompute(rawCompute, logger, platform.ResilientComputeOpts{})
	storage := platform.NewResilientStorage(rawStorage, logger, platform.ResilientStorageOpts{})
	network := platform.NewResilientNetwork(rawNetwork, logger, platform.ResilientNetworkOpts{})
	lbProxy := platform.NewResilientLB(rawLBProxy, logger, platform.ResilientLBOpts{})

	repos := deps.InitRepositories(db, rdb)

	// Create leader elector for singleton worker coordination.
	// When multiple worker replicas run, only one will hold leadership per key.
	leaderElector := postgres.NewPgLeaderElector(db, logger)

	svcs, workers, err := deps.InitServices(setup.ServiceConfig{
		Config: cfg, Repos: repos, Compute: compute, Storage: storage,
		Network: network, LBProxy: lbProxy, DB: db, RDB: rdb, Logger: logger,
		LeaderElector: leaderElector,
	})
	if err != nil {
		logger.Error("service initialization failed", "error", err)
		return err
	}

	handlers := deps.InitHandlers(svcs, cfg, logger)
	handlers.Admin = apphandlers.NewAdminHandler(compute)
	r := deps.SetupRouter(cfg, logger, handlers, svcs, network)

	// Add Tracing Middleware if enabled
	if os.Getenv("TRACING_ENABLED") == "true" {
		r.Use(otelgin.Middleware("compute-api"))
	}

	return runApplication(deps, cfg, logger, r, svcs, workers)
}

func runApplication(deps AppDeps, cfg *platform.Config, logger *slog.Logger, r *gin.Engine, svcs *setup.Services, workers *setup.Workers) error {
	role := os.Getenv("ROLE")
	if role == "" {
		role = "all"
	}

	validRoles := map[string]bool{"api": true, "worker": true, "all": true}
	if !validRoles[role] {
		logger.Error("invalid ROLE value, must be one of: api, worker, all", "role", role)
		return fmt.Errorf("invalid ROLE value %q, must be one of: api, worker, all", role)
	}
	logger.Info("starting with role", "role", role)

	wg := &sync.WaitGroup{}
	workerCtx, workerCancel := context.WithCancel(context.Background())

	if role == "worker" || role == "all" {
		runWorkers(workerCtx, wg, workers)
	}

	var srv *http.Server
	if role == "api" || role == "all" {
		srv = deps.NewHTTPServer(":"+cfg.Port, r)
		go func() {
			logger.Info("starting compute-api", "port", cfg.Port)
			if err := deps.StartHTTPServer(srv); err != nil && !stdlib_errors.Is(err, http.ErrServerClosed) {
				logger.Error("failed to start server", "error", err)
			}
		}()
	} else {
		logger.Info("running in worker-only mode, HTTP server disabled")
	}

	quit := make(chan os.Signal, 1)
	deps.NotifySignals(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()

	if srv != nil {
		if err := deps.ShutdownHTTPServer(ctx, srv); err != nil {
			logger.Error("server forced to shutdown", "error", err)
		}
	}

	workerCancel()
	wg.Wait()
	if svcs != nil {
		svcs.Shutdown()
	}
	logger.Info("shutdown complete")
	return nil
}

type runner interface {
	Run(context.Context, *sync.WaitGroup)
}

func startWorker(ctx context.Context, wg *sync.WaitGroup, w runner) {
	if w != nil {
		wg.Add(1)
		go w.Run(ctx, wg)
	}
}

func runWorkers(ctx context.Context, wg *sync.WaitGroup, workers *setup.Workers) {
	if workers == nil {
		return
	}
	if workers.LB != nil {
		startWorker(ctx, wg, workers.LB)
	}
	if workers.AutoScaling != nil {
		startWorker(ctx, wg, workers.AutoScaling)
	}
	if workers.Cron != nil {
		startWorker(ctx, wg, workers.Cron)
	}
	if workers.Container != nil {
		startWorker(ctx, wg, workers.Container)
	}
	if workers.Pipeline != nil {
		startWorker(ctx, wg, workers.Pipeline)
	}
	if workers.Provision != nil {
		startWorker(ctx, wg, workers.Provision)
	}
	if workers.Accounting != nil {
		startWorker(ctx, wg, workers.Accounting)
	}
	if workers.Cluster != nil {
		startWorker(ctx, wg, workers.Cluster)
	}
	if workers.Lifecycle != nil {
		startWorker(ctx, wg, workers.Lifecycle)
	}
	if workers.FunctionSchedule != nil {
		startWorker(ctx, wg, workers.FunctionSchedule)
	}
	if workers.ReplicaMonitor != nil {
		startWorker(ctx, wg, workers.ReplicaMonitor)
	}
	if workers.ClusterReconciler != nil {
		startWorker(ctx, wg, workers.ClusterReconciler)
	}
	if workers.Healing != nil {
		startWorker(ctx, wg, workers.Healing)
	}
	if workers.DatabaseFailover != nil {
		startWorker(ctx, wg, workers.DatabaseFailover)
	}
	if workers.Log != nil {
		startWorker(ctx, wg, workers.Log)
	}
}

func initTracing(logger *slog.Logger) *sdktrace.TracerProvider {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		return nil
	}

	tp, err := tracing.InitTracer(context.Background(), "compute-api", endpoint)
	if err != nil {
		logger.Warn("failed to initialize tracing", "error", err)
		return nil
	}
	return tp
}

func initInfrastructure(deps AppDeps, logger *slog.Logger, migrateOnly bool) (*platform.Config, postgres.DB, *redis.Client, error) {
	cfg, err := deps.LoadConfig(logger)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultDBInitTimeout)
	defer cancel()

	db, err := deps.InitDatabase(ctx, cfg, logger)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to init database: %w", err)
	}

	if err := deps.RunMigrations(ctx, db, logger); err != nil {
		return nil, nil, nil, fmt.Errorf("migration failed: %w", err)
	}

	if migrateOnly {
		db.Close()
		return nil, nil, nil, ErrMigrationDone
	}

	rdb, err := deps.InitRedis(ctx, cfg, logger)
	if err != nil {
		db.Close()
		return nil, nil, nil, fmt.Errorf("failed to init redis: %w", err)
	}

	return cfg, db, rdb, nil
}

func initBackends(deps AppDeps, cfg *platform.Config, logger *slog.Logger, db postgres.DB, rdb *redis.Client) (ports.ComputeBackend, ports.StorageBackend, ports.NetworkBackend, ports.LBProxyAdapter, error) {
	compute, err := deps.InitComputeBackend(cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to init compute: %w", err)
	}

	storage, err := deps.InitStorageBackend(cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to init storage: %w", err)
	}

	network := deps.InitNetworkBackend(cfg, logger)

	repos := deps.InitRepositories(db, rdb)
	lbProxy, err := deps.InitLBProxy(cfg, compute, repos.Instance, repos.Vpc)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to init lb proxy: %w", err)
	}

	return compute, storage, network, lbProxy, nil
}
