package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/poyraz/cloud/internal/core/services"
	httphandlers "github.com/poyraz/cloud/internal/handlers"
	"github.com/poyraz/cloud/internal/platform"
	"github.com/poyraz/cloud/internal/repositories/docker"
	"github.com/poyraz/cloud/internal/repositories/filesystem"
	"github.com/poyraz/cloud/internal/repositories/postgres"
	"github.com/poyraz/cloud/pkg/httputil"
)

func main() {
	// 1. Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// 2. Config
	cfg, err := platform.NewConfig()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// 3. Infrastructure (Postgres + Docker)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := platform.NewDatabase(ctx, cfg.DatabaseURL, logger)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	dockerAdapter, err := docker.NewDockerAdapter()
	if err != nil {
		logger.Error("failed to initialize docker adapter", "error", err)
		os.Exit(1)
	}

	// 4. Layers (Repo -> Service -> Handler)
	identityRepo := postgres.NewIdentityRepository(db)
	identitySvc := services.NewIdentityService(identityRepo)
	identityHandler := httphandlers.NewIdentityHandler(identitySvc)

	instanceRepo := postgres.NewInstanceRepository(db)
	vpcRepo := postgres.NewVpcRepository(db)

	vpcSvc := services.NewVpcService(vpcRepo, dockerAdapter)
	instanceSvc := services.NewInstanceService(instanceRepo, vpcRepo, dockerAdapter)

	vpcHandler := httphandlers.NewVpcHandler(vpcSvc)
	instanceHandler := httphandlers.NewInstanceHandler(instanceSvc)

	// Storage Service
	fileStore, err := filesystem.NewLocalFileStore("./miniaws-data/local/storage")
	if err != nil {
		logger.Error("failed to initialize file store", "error", err)
		os.Exit(1)
	}
	storageRepo := postgres.NewStorageRepository(db)
	storageSvc := services.NewStorageService(storageRepo, fileStore)
	storageHandler := httphandlers.NewStorageHandler(storageSvc)

	// 5. Engine & Middleware
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(httputil.RequestID())
	r.Use(httputil.Logger(logger))
	r.Use(gin.Recovery())

	// 6. Routes
	r.GET("/health", func(c *gin.Context) {
		status := "UP"
		dbStatus := "CONNECTED"
		if err := db.Ping(c.Request.Context()); err != nil {
			dbStatus = "DISCONNECTED"
		}

		c.JSON(http.StatusOK, gin.H{
			"status":   status,
			"database": dbStatus,
			"time":     time.Now().Format(time.RFC3339),
		})
	})

	// Identity Routes (Public for bootstrapping)
	r.POST("/auth/keys", identityHandler.CreateKey)

	// Instance Routes (Protected)
	instanceGroup := r.Group("/instances")
	instanceGroup.Use(httputil.Auth(identitySvc))
	{
		instanceGroup.POST("", instanceHandler.Launch)
		instanceGroup.GET("", instanceHandler.List)
		instanceGroup.GET("/:id", instanceHandler.Get)
		instanceGroup.POST("/:id/stop", instanceHandler.Stop)
		instanceGroup.GET("/:id/logs", instanceHandler.GetLogs)
		instanceGroup.DELETE("/:id", instanceHandler.Terminate)
	}

	// VPC Routes (Protected)
	vpcGroup := r.Group("/vpcs")
	vpcGroup.Use(httputil.Auth(identitySvc))
	{
		vpcGroup.POST("", vpcHandler.Create)
		vpcGroup.GET("", vpcHandler.List)
		vpcGroup.GET("/:id", vpcHandler.Get)
		vpcGroup.DELETE("/:id", vpcHandler.Delete)
	}

	// Storage Routes (Protected)
	storageGroup := r.Group("/storage")
	storageGroup.Use(httputil.Auth(identitySvc))
	{
		storageGroup.PUT("/:bucket/:key", storageHandler.Upload)
		storageGroup.GET("/:bucket/:key", storageHandler.Download)
		storageGroup.GET("/:bucket", storageHandler.List)
		storageGroup.DELETE("/:bucket/:key", storageHandler.Delete)
	}

	// 6. Server setup
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// 7. Graceful Shutdown
	go func() {
		logger.Info("starting compute-api", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
	}

	logger.Info("server exited")
}
