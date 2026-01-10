package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/poyrazk/thecloud/internal/api/setup"
)

// @title The Cloud API
// @version 1.0
// @description This is The Cloud Compute API server.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /

// @securityDefinitions.apikey APIKeyAuth
// @in header
// @name X-API-Key

func main() {
	// 1. Logger
	logger := setup.InitLogger()

	migrateOnly := flag.Bool("migrate-only", false, "run database migrations and exit")
	flag.Parse()

	// 2. Config
	cfg, err := setup.LoadConfig(logger)
	if err != nil {
		os.Exit(1)
	}

	// 3. Infrastructure
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := setup.InitDatabase(ctx, cfg, logger)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	rdb, err := setup.InitRedis(ctx, cfg, logger)
	if err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	// 3.1 Run Migrations
	if err := setup.RunMigrations(ctx, db, logger); err != nil {
		logger.Warn("failed to run migrations", "error", err)
		if *migrateOnly {
			db.Close()
			os.Exit(1)
		}
	}

	if *migrateOnly {
		logger.Info("migrations completed, exiting")
		return
	}

	computeBackend, err := setup.InitComputeBackend(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize compute backend", "error", err)
		os.Exit(1)
	}

	networkBackend := setup.InitNetworkBackend(cfg, logger)

	// 4. Dependencies
	repos := setup.InitRepositories(db, rdb)

	// We need LBProxy for services initialization
	lbProxy, err := setup.InitLBProxy(cfg, computeBackend, repos.Instance, repos.Vpc)
	if err != nil {
		logger.Error("failed to initialize load balancer proxy adapter", "error", err)
		os.Exit(1)
	}

	svcs, workers, err := setup.InitServices(cfg, repos, computeBackend, networkBackend, lbProxy, db, rdb, logger)
	if err != nil {
		logger.Error("failed to initialize services", "error", err)
		os.Exit(1)
	}

	handlers := setup.InitHandlers(svcs)

	// 5. Router
	r := setup.SetupRouter(cfg, logger, handlers, svcs, networkBackend)

	// 6. Background Workers & Server
	role := os.Getenv("ROLE")
	if role == "" {
		role = "all"
	}

	wg := &sync.WaitGroup{}
	workerCtx, workerCancel := context.WithCancel(context.Background())

	if role == "worker" || role == "all" {
		wg.Add(5)
		go workers.LB.Run(workerCtx, wg)
		go workers.AutoScaling.Run(workerCtx, wg)
		go workers.Cron.Run(workerCtx, wg)
		go workers.Container.Run(workerCtx, wg)
		go workers.Provision.Run(workerCtx, wg)
	}

	// 7. Server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	if role == "api" || role == "all" {
		go func() {
			logger.Info("starting compute-api", "port", cfg.Port)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("failed to start server", "error", err)
				os.Exit(1)
			}
		}()
	} else {
		logger.Info("running in worker-only mode")
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
	}

	// Shutdown workers
	workerCancel()
	wg.Wait()

	logger.Info("server exited")
}
