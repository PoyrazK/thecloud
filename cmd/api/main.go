package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sync"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"golang.org/x/time/rate"

	"github.com/gin-gonic/gin"
	_ "github.com/poyrazk/thecloud/docs/swagger"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	httphandlers "github.com/poyrazk/thecloud/internal/handlers"
	"github.com/poyrazk/thecloud/internal/platform"
	"github.com/poyrazk/thecloud/internal/repositories/docker"
	"github.com/poyrazk/thecloud/internal/repositories/filesystem"
	"github.com/poyrazk/thecloud/internal/repositories/postgres"
	"github.com/poyrazk/thecloud/pkg/httputil"
	"github.com/poyrazk/thecloud/pkg/ratelimit"
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

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key

func main() {
	// 1. Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	migrateOnly := flag.Bool("migrate-only", false, "run database migrations and exit")
	flag.Parse()

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

	// 3.1 Run Migrations
	if err := postgres.RunMigrations(ctx, db); err != nil {
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

	dockerAdapter, err := docker.NewDockerAdapter()
	if err != nil {
		logger.Error("failed to initialize docker adapter", "error", err)
		os.Exit(1)
	}

	// 4. Layers (Repo -> Service -> Handler)
	userRepo := postgres.NewUserRepo(db)
	identityRepo := postgres.NewIdentityRepository(db)
	identitySvc := services.NewIdentityService(identityRepo)
	authSvc := services.NewAuthService(userRepo, identitySvc)
	identityHandler := httphandlers.NewIdentityHandler(identitySvc)
	authHandler := httphandlers.NewAuthHandler(authSvc)
	rbacHandler := httphandlers.NewRBACHandler(authSvc)

	instanceRepo := postgres.NewInstanceRepository(db)
	vpcRepo := postgres.NewVpcRepository(db)
	eventRepo := postgres.NewEventRepository(db)
	volumeRepo := postgres.NewVolumeRepository(db)

	vpcSvc := services.NewVpcService(vpcRepo, dockerAdapter, logger)
	eventSvc := services.NewEventService(eventRepo, logger)
	volumeSvc := services.NewVolumeService(volumeRepo, dockerAdapter, eventSvc, logger)
	instanceSvc := services.NewInstanceService(instanceRepo, vpcRepo, volumeRepo, dockerAdapter, eventSvc, logger)

	lbRepo := postgres.NewLBRepository(db)
	lbProxy, err := docker.NewLBProxyAdapter(instanceRepo, vpcRepo)
	if err != nil {
		logger.Error("failed to initialize load balancer proxy adapter", "error", err)
		os.Exit(1)
	}
	lbSvc := services.NewLBService(lbRepo, vpcRepo, instanceRepo)
	lbWorker := services.NewLBWorker(lbRepo, instanceRepo, lbProxy)

	vpcHandler := httphandlers.NewVpcHandler(vpcSvc)
	instanceHandler := httphandlers.NewInstanceHandler(instanceSvc)
	eventHandler := httphandlers.NewEventHandler(eventSvc)
	volumeHandler := httphandlers.NewVolumeHandler(volumeSvc)
	lbHandler := httphandlers.NewLBHandler(lbSvc)

	// Dashboard Service (aggregates all repositories)
	dashboardSvc := services.NewDashboardService(instanceRepo, volumeRepo, vpcRepo, eventRepo, logger)
	dashboardHandler := httphandlers.NewDashboardHandler(dashboardSvc)

	// Storage Service
	fileStore, err := filesystem.NewLocalFileStore("./thecloud-data/local/storage")
	if err != nil {
		logger.Error("failed to initialize file store", "error", err)
		os.Exit(1)
	}
	storageRepo := postgres.NewStorageRepository(db)
	storageSvc := services.NewStorageService(storageRepo, fileStore)
	storageHandler := httphandlers.NewStorageHandler(storageSvc)

	databaseRepo := postgres.NewDatabaseRepository(db)
	databaseSvc := services.NewDatabaseService(databaseRepo, dockerAdapter, vpcRepo, eventSvc, logger)
	databaseHandler := httphandlers.NewDatabaseHandler(databaseSvc)

	secretRepo := postgres.NewSecretRepository(db)
	secretSvc := services.NewSecretService(secretRepo, eventSvc, logger)
	secretHandler := httphandlers.NewSecretHandler(secretSvc)

	fnRepo := postgres.NewFunctionRepository(db)
	fnSvc := services.NewFunctionService(fnRepo, dockerAdapter, fileStore, logger)
	fnHandler := httphandlers.NewFunctionHandler(fnSvc)

	cacheRepo := postgres.NewCacheRepository(db)
	cacheSvc := services.NewCacheService(cacheRepo, dockerAdapter, vpcRepo, eventSvc, logger)
	cacheHandler := httphandlers.NewCacheHandler(cacheSvc)

	// 5. Engine & Middleware
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(httputil.RequestID())
	r.Use(httputil.Logger(logger))
	r.Use(httputil.CORS())
	r.Use(gin.Recovery())

	// Security Middleware
	r.Use(httputil.SecurityHeadersMiddleware())

	// Rate Limiter (5 req/sec, burst 10)
	limiter := ratelimit.NewIPRateLimiter(rate.Limit(5), 10, logger)
	r.Use(ratelimit.Middleware(limiter))

	// 6. Routes
	r.GET("/health", func(c *gin.Context) {
		overallStatus := "UP"

		// Check database
		dbStatus := "CONNECTED"
		if err := db.Ping(c.Request.Context()); err != nil {
			dbStatus = "DISCONNECTED"
			overallStatus = "DEGRADED"
		}

		// Check Docker daemon
		dockerStatus := "CONNECTED"
		if err := dockerAdapter.Ping(c.Request.Context()); err != nil {
			dockerStatus = "DISCONNECTED"
			overallStatus = "DEGRADED"
		}

		statusCode := http.StatusOK
		if overallStatus == "DEGRADED" {
			statusCode = http.StatusServiceUnavailable
		}

		c.JSON(statusCode, gin.H{
			"status": overallStatus,
			"checks": gin.H{
				"database": dbStatus,
				"docker":   dockerStatus,
			},
			"time": time.Now().Format(time.RFC3339),
		})
	})

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Identity Routes (Public for bootstrapping)
	r.POST("/auth/register", authHandler.Register)
	r.POST("/auth/login", authHandler.Login)
	r.POST("/auth/keys", identityHandler.CreateKey)

	// RBAC Routes (Protected)
	authGroup := r.Group("/auth")
	authGroup.Use(httputil.Auth(identitySvc, authSvc))
	{
		authGroup.GET("/roles", httputil.RequirePermission("auth", httputil.ActionRead), rbacHandler.ListRoles)
		authGroup.GET("/me/role", rbacHandler.GetMyRole)
		authGroup.GET("/users/:id/role", httputil.RequirePermission("auth", httputil.ActionRead), rbacHandler.GetUserRole)
		authGroup.PUT("/users/:id/role", httputil.RequirePermission("auth", httputil.ActionUpdate), rbacHandler.UpdateUserRole)
	}

	// Instance Routes (Protected)
	instanceGroup := r.Group("/instances")
	instanceGroup.Use(httputil.Auth(identitySvc, authSvc))
	{
		instanceGroup.POST("", httputil.RequirePermission("instances", httputil.ActionCreate), instanceHandler.Launch)
		instanceGroup.GET("", httputil.RequirePermission("instances", httputil.ActionRead), instanceHandler.List)
		instanceGroup.GET("/:id", httputil.RequirePermission("instances", httputil.ActionRead), instanceHandler.Get)
		instanceGroup.POST("/:id/stop", httputil.RequirePermission("instances", httputil.ActionUpdate), instanceHandler.Stop)
		instanceGroup.GET("/:id/logs", httputil.RequirePermission("instances", httputil.ActionExecute), instanceHandler.GetLogs)
		instanceGroup.GET("/:id/stats", httputil.RequirePermission("instances", httputil.ActionRead), instanceHandler.GetStats)
		instanceGroup.DELETE("/:id", httputil.RequirePermission("instances", httputil.ActionDelete), instanceHandler.Terminate)
	}

	// VPC Routes (Protected)
	vpcGroup := r.Group("/vpcs")
	vpcGroup.Use(httputil.Auth(identitySvc, authSvc))
	{
		vpcGroup.POST("", httputil.RequirePermission("vpcs", httputil.ActionCreate), vpcHandler.Create)
		vpcGroup.GET("", httputil.RequirePermission("vpcs", httputil.ActionRead), vpcHandler.List)
		vpcGroup.GET("/:id", httputil.RequirePermission("vpcs", httputil.ActionRead), vpcHandler.Get)
		vpcGroup.DELETE("/:id", httputil.RequirePermission("vpcs", httputil.ActionDelete), vpcHandler.Delete)
	}

	// Storage Routes (Protected)
	storageGroup := r.Group("/storage")
	storageGroup.Use(httputil.Auth(identitySvc, authSvc))
	{
		storageGroup.PUT("/:bucket/:key", httputil.RequirePermission("storage", httputil.ActionCreate), storageHandler.Upload)
		storageGroup.GET("/:bucket/:key", httputil.RequirePermission("storage", httputil.ActionRead), storageHandler.Download)
		storageGroup.GET("/:bucket", httputil.RequirePermission("storage", httputil.ActionRead), storageHandler.List)
		storageGroup.DELETE("/:bucket/:key", httputil.RequirePermission("storage", httputil.ActionDelete), storageHandler.Delete)
	}

	// Event Routes (Protected)
	eventGroup := r.Group("/events")
	eventGroup.Use(httputil.Auth(identitySvc, authSvc))
	{
		eventGroup.GET("", httputil.RequirePermission("events", httputil.ActionRead), eventHandler.List)
	}

	// Volume Routes (Protected)
	volumeGroup := r.Group("/volumes")
	volumeGroup.Use(httputil.Auth(identitySvc, authSvc))
	{
		volumeGroup.POST("", httputil.RequirePermission("volumes", httputil.ActionCreate), volumeHandler.Create)
		volumeGroup.GET("", httputil.RequirePermission("volumes", httputil.ActionRead), volumeHandler.List)
		volumeGroup.GET("/:id", httputil.RequirePermission("volumes", httputil.ActionRead), volumeHandler.Get)
		volumeGroup.DELETE("/:id", httputil.RequirePermission("volumes", httputil.ActionDelete), volumeHandler.Delete)
	}

	// Dashboard Routes (Protected)
	dashboardGroup := r.Group("/api/dashboard")
	dashboardGroup.Use(httputil.Auth(identitySvc, authSvc))
	{
		dashboardGroup.GET("/summary", httputil.RequirePermission("dashboard", httputil.ActionRead), dashboardHandler.GetSummary)
		dashboardGroup.GET("/events", httputil.RequirePermission("dashboard", httputil.ActionRead), dashboardHandler.GetRecentEvents)
		dashboardGroup.GET("/stats", httputil.RequirePermission("dashboard", httputil.ActionRead), dashboardHandler.GetStats)
		dashboardGroup.GET("/stream", httputil.RequirePermission("dashboard", httputil.ActionRead), dashboardHandler.StreamEvents)
	}

	// Load Balancer Routes (Protected)
	lbGroup := r.Group("/lb")
	lbGroup.Use(httputil.Auth(identitySvc, authSvc))
	{
		lbGroup.POST("", httputil.RequirePermission("loadbalancers", httputil.ActionCreate), lbHandler.Create)
		lbGroup.GET("", httputil.RequirePermission("loadbalancers", httputil.ActionRead), lbHandler.List)
		lbGroup.GET("/:id", httputil.RequirePermission("loadbalancers", httputil.ActionRead), lbHandler.Get)
		lbGroup.DELETE("/:id", httputil.RequirePermission("loadbalancers", httputil.ActionDelete), lbHandler.Delete)
		lbGroup.POST("/:id/targets", httputil.RequirePermission("loadbalancers", httputil.ActionUpdate), lbHandler.AddTarget)
		lbGroup.GET("/:id/targets", httputil.RequirePermission("loadbalancers", httputil.ActionRead), lbHandler.ListTargets)
		lbGroup.DELETE("/:id/targets/:instanceId", httputil.RequirePermission("loadbalancers", httputil.ActionUpdate), lbHandler.RemoveTarget)
	}

	// Database Routes (Protected)
	dbGroup := r.Group("/databases")
	dbGroup.Use(httputil.Auth(identitySvc, authSvc))
	{
		dbGroup.POST("", httputil.RequirePermission("databases", httputil.ActionCreate), databaseHandler.Create)
		dbGroup.GET("", httputil.RequirePermission("databases", httputil.ActionRead), databaseHandler.List)
		dbGroup.GET("/:id", httputil.RequirePermission("databases", httputil.ActionRead), databaseHandler.Get)
		dbGroup.DELETE("/:id", httputil.RequirePermission("databases", httputil.ActionDelete), databaseHandler.Delete)
		dbGroup.GET("/:id/connection", httputil.RequirePermission("databases", httputil.ActionRead), databaseHandler.GetConnectionString)
		dbGroup.GET("/:id/logs", httputil.RequirePermission("databases", httputil.ActionExecute), databaseHandler.GetLogs)
	}

	// Secret Routes (Protected)
	secretGroup := r.Group("/secrets")
	secretGroup.Use(httputil.Auth(identitySvc, authSvc))
	{
		secretGroup.POST("", httputil.RequirePermission("secrets", httputil.ActionCreate), secretHandler.Create)
		secretGroup.GET("", httputil.RequirePermission("secrets", httputil.ActionRead), secretHandler.List)
		secretGroup.GET("/:id", httputil.RequirePermission("secrets", httputil.ActionRead), secretHandler.Get)
		secretGroup.DELETE("/:id", httputil.RequirePermission("secrets", httputil.ActionDelete), secretHandler.Delete)
	}

	// Function Routes (Protected)
	fnGroup := r.Group("/functions")
	fnGroup.Use(httputil.Auth(identitySvc, authSvc))
	{
		fnGroup.POST("", httputil.RequirePermission("functions", httputil.ActionCreate), fnHandler.Create)
		fnGroup.GET("", httputil.RequirePermission("functions", httputil.ActionRead), fnHandler.List)
		fnGroup.GET("/:id", httputil.RequirePermission("functions", httputil.ActionRead), fnHandler.Get)
		fnGroup.DELETE("/:id", httputil.RequirePermission("functions", httputil.ActionDelete), fnHandler.Delete)
		fnGroup.POST("/:id/invoke", httputil.RequirePermission("functions", httputil.ActionExecute), fnHandler.Invoke)
		fnGroup.GET("/:id/logs", httputil.RequirePermission("functions", httputil.ActionRead), fnHandler.GetLogs)
	}

	// Cache Routes (Protected)
	cacheGroup := r.Group("/caches")
	cacheGroup.Use(httputil.Auth(identitySvc, authSvc))
	{
		cacheGroup.POST("", httputil.RequirePermission("caches", httputil.ActionCreate), cacheHandler.Create)
		cacheGroup.GET("", httputil.RequirePermission("caches", httputil.ActionRead), cacheHandler.List)
		cacheGroup.GET("/:id", httputil.RequirePermission("caches", httputil.ActionRead), cacheHandler.Get)
		cacheGroup.DELETE("/:id", httputil.RequirePermission("caches", httputil.ActionDelete), cacheHandler.Delete)
		cacheGroup.GET("/:id/connection", httputil.RequirePermission("caches", httputil.ActionRead), cacheHandler.GetConnectionString)
		cacheGroup.POST("/:id/flush", httputil.RequirePermission("caches", httputil.ActionExecute), cacheHandler.Flush)
		cacheGroup.GET("/:id/stats", httputil.RequirePermission("caches", httputil.ActionRead), cacheHandler.GetStats)
	}

	// Auto-Scaling Routes (Protected)
	asgRepo := postgres.NewAutoScalingRepo(db)
	asgSvc := services.NewAutoScalingService(asgRepo, vpcRepo)
	asgHandler := httphandlers.NewAutoScalingHandler(asgSvc)
	asgWorker := services.NewAutoScalingWorker(asgRepo, instanceSvc, lbSvc, eventSvc, ports.RealClock{})

	asgGroup := r.Group("/autoscaling")
	asgGroup.Use(httputil.Auth(identitySvc, authSvc))
	{
		asgGroup.POST("/groups", httputil.RequirePermission("autoscaling", httputil.ActionCreate), asgHandler.CreateGroup)
		asgGroup.GET("/groups", httputil.RequirePermission("autoscaling", httputil.ActionRead), asgHandler.ListGroups)
		asgGroup.GET("/groups/:id", httputil.RequirePermission("autoscaling", httputil.ActionRead), asgHandler.GetGroup)
		asgGroup.DELETE("/groups/:id", httputil.RequirePermission("autoscaling", httputil.ActionDelete), asgHandler.DeleteGroup)
		asgGroup.POST("/groups/:id/policies", httputil.RequirePermission("autoscaling", httputil.ActionUpdate), asgHandler.CreatePolicy)
		asgGroup.DELETE("/policies/:id", httputil.RequirePermission("autoscaling", httputil.ActionDelete), asgHandler.DeletePolicy)
	}

	// 7. Background Workers
	wg := &sync.WaitGroup{}
	workerCtx, workerCancel := context.WithCancel(context.Background())
	wg.Add(2)
	go lbWorker.Run(workerCtx, wg)
	go asgWorker.Run(workerCtx, wg)

	// 8. Server setup
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

	// Shutdown workers
	workerCancel()
	wg.Wait()

	logger.Info("server exited")
}
