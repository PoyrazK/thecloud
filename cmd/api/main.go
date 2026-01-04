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

	queueRepo := postgres.NewPostgresQueueRepository(db)
	queueSvc := services.NewQueueService(queueRepo, eventSvc)
	queueHandler := httphandlers.NewQueueHandler(queueSvc)

	notifyRepo := postgres.NewPostgresNotifyRepository(db)
	notifySvc := services.NewNotifyService(notifyRepo, queueSvc, eventSvc)
	notifyHandler := httphandlers.NewNotifyHandler(notifySvc)

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

	// Instance Routes (Protected)
	instanceGroup := r.Group("/instances")
	instanceGroup.Use(httputil.Auth(identitySvc))
	{
		instanceGroup.POST("", instanceHandler.Launch)
		instanceGroup.GET("", instanceHandler.List)
		instanceGroup.GET("/:id", instanceHandler.Get)
		instanceGroup.POST("/:id/stop", instanceHandler.Stop)
		instanceGroup.GET("/:id/logs", instanceHandler.GetLogs)
		instanceGroup.GET("/:id/stats", instanceHandler.GetStats)
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

	// Event Routes (Protected)
	eventGroup := r.Group("/events")
	eventGroup.Use(httputil.Auth(identitySvc))
	{
		eventGroup.GET("", eventHandler.List)
	}

	// Volume Routes (Protected)
	volumeGroup := r.Group("/volumes")
	volumeGroup.Use(httputil.Auth(identitySvc))
	{
		volumeGroup.POST("", volumeHandler.Create)
		volumeGroup.GET("", volumeHandler.List)
		volumeGroup.GET("/:id", volumeHandler.Get)
		volumeGroup.DELETE("/:id", volumeHandler.Delete)
	}

	// Dashboard Routes (Protected)
	dashboardGroup := r.Group("/api/dashboard")
	dashboardGroup.Use(httputil.Auth(identitySvc))
	{
		dashboardGroup.GET("/summary", dashboardHandler.GetSummary)
		dashboardGroup.GET("/events", dashboardHandler.GetRecentEvents)
		dashboardGroup.GET("/stats", dashboardHandler.GetStats)
		dashboardGroup.GET("/stream", dashboardHandler.StreamEvents)
	}

	// Load Balancer Routes (Protected)
	lbGroup := r.Group("/lb")
	lbGroup.Use(httputil.Auth(identitySvc))
	{
		lbGroup.POST("", lbHandler.Create)
		lbGroup.GET("", lbHandler.List)
		lbGroup.GET("/:id", lbHandler.Get)
		lbGroup.DELETE("/:id", lbHandler.Delete)
		lbGroup.POST("/:id/targets", lbHandler.AddTarget)
		lbGroup.GET("/:id/targets", lbHandler.ListTargets)
		lbGroup.DELETE("/:id/targets/:instanceId", lbHandler.RemoveTarget)
	}

	// Database Routes (Protected)
	dbGroup := r.Group("/databases")
	dbGroup.Use(httputil.Auth(identitySvc))
	{
		dbGroup.POST("", databaseHandler.Create)
		dbGroup.GET("", databaseHandler.List)
		dbGroup.GET("/:id", databaseHandler.Get)
		dbGroup.DELETE("/:id", databaseHandler.Delete)
		dbGroup.GET("/:id/connection", databaseHandler.GetConnectionString)
	}

	// Secret Routes (Protected)
	secretGroup := r.Group("/secrets")
	secretGroup.Use(httputil.Auth(identitySvc))
	{
		secretGroup.POST("", secretHandler.Create)
		secretGroup.GET("", secretHandler.List)
		secretGroup.GET("/:id", secretHandler.Get)
		secretGroup.DELETE("/:id", secretHandler.Delete)
	}

	// Function Routes (Protected)
	fnGroup := r.Group("/functions")
	fnGroup.Use(httputil.Auth(identitySvc))
	{
		fnGroup.POST("", fnHandler.Create)
		fnGroup.GET("", fnHandler.List)
		fnGroup.GET("/:id", fnHandler.Get)
		fnGroup.DELETE("/:id", fnHandler.Delete)
		fnGroup.POST("/:id/invoke", fnHandler.Invoke)
		fnGroup.GET("/:id/logs", fnHandler.GetLogs)
	}

	// Cache Routes (Protected)
	cacheGroup := r.Group("/caches")
	cacheGroup.Use(httputil.Auth(identitySvc))
	{
		cacheGroup.POST("", cacheHandler.Create)
		cacheGroup.GET("", cacheHandler.List)
		cacheGroup.GET("/:id", cacheHandler.Get)
		cacheGroup.DELETE("/:id", cacheHandler.Delete)
		cacheGroup.GET("/:id/connection", cacheHandler.GetConnectionString)
		cacheGroup.POST("/:id/flush", cacheHandler.Flush)
		cacheGroup.GET("/:id/stats", cacheHandler.GetStats)
	}

	queueGroup := r.Group("/queues")
	queueGroup.Use(httputil.Auth(identitySvc))
	{
		queueGroup.POST("", queueHandler.Create)
		queueGroup.GET("", queueHandler.List)
		queueGroup.GET("/:id", queueHandler.Get)
		queueGroup.DELETE("/:id", queueHandler.Delete)
		queueGroup.POST("/:id/messages", queueHandler.SendMessage)
		queueGroup.GET("/:id/messages", queueHandler.ReceiveMessages)
		queueGroup.DELETE("/:id/messages/:handle", queueHandler.DeleteMessage)
		queueGroup.POST("/:id/purge", queueHandler.Purge) // Changed from DELETE to POST to avoid ambiguity
	}

	notifyGroup := r.Group("/notify")
	notifyGroup.Use(httputil.Auth(identitySvc))
	{
		notifyGroup.POST("/topics", notifyHandler.CreateTopic)
		notifyGroup.GET("/topics", notifyHandler.ListTopics)
		notifyGroup.DELETE("/topics/:id", notifyHandler.DeleteTopic)
		notifyGroup.POST("/topics/:id/subscriptions", notifyHandler.Subscribe)
		notifyGroup.GET("/topics/:id/subscriptions", notifyHandler.ListSubscriptions)
		notifyGroup.DELETE("/subscriptions/:id", notifyHandler.Unsubscribe)
		notifyGroup.POST("/topics/:id/publish", notifyHandler.Publish)
	}

	// Auto-Scaling Routes (Protected)
	asgRepo := postgres.NewAutoScalingRepo(db)
	asgSvc := services.NewAutoScalingService(asgRepo, vpcRepo)
	asgHandler := httphandlers.NewAutoScalingHandler(asgSvc)
	asgWorker := services.NewAutoScalingWorker(asgRepo, instanceSvc, lbSvc, eventSvc, ports.RealClock{})

	asgGroup := r.Group("/autoscaling")
	asgGroup.Use(httputil.Auth(identitySvc))
	{
		asgGroup.POST("/groups", asgHandler.CreateGroup)
		asgGroup.GET("/groups", asgHandler.ListGroups)
		asgGroup.GET("/groups/:id", asgHandler.GetGroup)
		asgGroup.DELETE("/groups/:id", asgHandler.DeleteGroup)
		asgGroup.POST("/groups/:id/policies", asgHandler.CreatePolicy)
		asgGroup.DELETE("/policies/:id", asgHandler.DeletePolicy)
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
