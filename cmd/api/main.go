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
	"github.com/poyrazk/thecloud/internal/core/domain"
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

// @securityDefinitions.apikey APIKeyAuth
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
	auditRepo := postgres.NewAuditRepository(db)
	userRepo := postgres.NewUserRepo(db)
	identityRepo := postgres.NewIdentityRepository(db)
	pwdResetRepo := postgres.NewPasswordResetRepository(db)

	auditSvc := services.NewAuditService(auditRepo)
	identitySvc := services.NewIdentityService(identityRepo, auditSvc)
	authSvc := services.NewAuthService(userRepo, identitySvc, auditSvc)
	pwdResetSvc := services.NewPasswordResetService(pwdResetRepo, userRepo)
	rbacRepo := postgres.NewRBACRepository(db)
	rbacSvc := services.NewRBACService(userRepo, rbacRepo, logger)

	auditHandler := httphandlers.NewAuditHandler(auditSvc)
	identityHandler := httphandlers.NewIdentityHandler(identitySvc)
	authHandler := httphandlers.NewAuthHandler(authSvc, pwdResetSvc)

	instanceRepo := postgres.NewInstanceRepository(db)
	vpcRepo := postgres.NewVpcRepository(db)
	eventRepo := postgres.NewEventRepository(db)
	volumeRepo := postgres.NewVolumeRepository(db)

	vpcSvc := services.NewVpcService(vpcRepo, dockerAdapter, auditSvc, logger)
	eventSvc := services.NewEventService(eventRepo, logger)
	volumeSvc := services.NewVolumeService(volumeRepo, dockerAdapter, eventSvc, auditSvc, logger)
	instanceSvc := services.NewInstanceService(instanceRepo, vpcRepo, volumeRepo, dockerAdapter, eventSvc, auditSvc, logger)

	lbRepo := postgres.NewLBRepository(db)
	lbProxy, err := docker.NewLBProxyAdapter(instanceRepo, vpcRepo)
	if err != nil {
		logger.Error("failed to initialize load balancer proxy adapter", "error", err)
		os.Exit(1)
	}
	lbSvc := services.NewLBService(lbRepo, vpcRepo, instanceRepo, auditSvc)
	lbWorker := services.NewLBWorker(lbRepo, instanceRepo, lbProxy)

	vpcHandler := httphandlers.NewVpcHandler(vpcSvc)
	instanceHandler := httphandlers.NewInstanceHandler(instanceSvc)
	eventHandler := httphandlers.NewEventHandler(eventSvc)
	volumeHandler := httphandlers.NewVolumeHandler(volumeSvc)
	lbHandler := httphandlers.NewLBHandler(lbSvc)

	// Dashboard Service (aggregates all repositories)
	dashboardSvc := services.NewDashboardService(instanceRepo, volumeRepo, vpcRepo, eventRepo, logger)
	dashboardHandler := httphandlers.NewDashboardHandler(dashboardSvc)

	// Snapshot Service
	snapshotRepo := postgres.NewSnapshotRepository(db)
	snapshotSvc := services.NewSnapshotService(snapshotRepo, volumeRepo, dockerAdapter, eventSvc, auditSvc, logger)
	snapshotHandler := httphandlers.NewSnapshotHandler(snapshotSvc)

	// Storage Service
	fileStore, err := filesystem.NewLocalFileStore("./thecloud-data/local/storage")
	if err != nil {
		logger.Error("failed to initialize file store", "error", err)
		os.Exit(1)
	}
	storageRepo := postgres.NewStorageRepository(db)
	storageSvc := services.NewStorageService(storageRepo, fileStore, auditSvc)
	storageHandler := httphandlers.NewStorageHandler(storageSvc)

	databaseRepo := postgres.NewDatabaseRepository(db)
	databaseSvc := services.NewDatabaseService(databaseRepo, dockerAdapter, vpcRepo, eventSvc, auditSvc, logger)
	databaseHandler := httphandlers.NewDatabaseHandler(databaseSvc)

	secretRepo := postgres.NewSecretRepository(db)
	secretSvc := services.NewSecretService(secretRepo, eventSvc, auditSvc, logger, cfg.SecretsEncryptionKey, cfg.Environment)
	secretHandler := httphandlers.NewSecretHandler(secretSvc)

	fnRepo := postgres.NewFunctionRepository(db)
	fnSvc := services.NewFunctionService(fnRepo, dockerAdapter, fileStore, auditSvc, logger)
	fnHandler := httphandlers.NewFunctionHandler(fnSvc)

	cacheRepo := postgres.NewCacheRepository(db)
	cacheSvc := services.NewCacheService(cacheRepo, dockerAdapter, vpcRepo, eventSvc, auditSvc, logger)
	cacheHandler := httphandlers.NewCacheHandler(cacheSvc)

	queueRepo := postgres.NewPostgresQueueRepository(db)
	queueSvc := services.NewQueueService(queueRepo, eventSvc, auditSvc)
	queueHandler := httphandlers.NewQueueHandler(queueSvc)

	notifyRepo := postgres.NewPostgresNotifyRepository(db)
	notifySvc := services.NewNotifyService(notifyRepo, queueSvc, eventSvc, auditSvc)
	notifyHandler := httphandlers.NewNotifyHandler(notifySvc)

	cronRepo := postgres.NewPostgresCronRepository(db)
	cronSvc := services.NewCronService(cronRepo, eventSvc, auditSvc)
	cronHandler := httphandlers.NewCronHandler(cronSvc)
	cronWorker := services.NewCronWorker(cronRepo)

	gwRepo := postgres.NewPostgresGatewayRepository(db)
	gwSvc := services.NewGatewayService(gwRepo, auditSvc)
	gwHandler := httphandlers.NewGatewayHandler(gwSvc)

	containerRepo := postgres.NewPostgresContainerRepository(db)
	containerSvc := services.NewContainerService(containerRepo, eventSvc, auditSvc)
	containerHandler := httphandlers.NewContainerHandler(containerSvc)
	containerWorker := services.NewContainerWorker(containerRepo, instanceSvc, eventSvc)

	// Health Service
	healthSvc := services.NewHealthServiceImpl(db, dockerAdapter)
	healthHandler := httphandlers.NewHealthHandler(healthSvc)

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
	r.GET("/health/live", healthHandler.Live)
	r.GET("/health/ready", healthHandler.Ready)
	r.GET("/health", healthHandler.Ready) // Alias for backward compatibility

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Auth Rate Limiter (5 req/min, burst 5)
	authLimiter := ratelimit.NewIPRateLimiter(rate.Limit(5.0/60.0), 5, logger)
	authMiddleware := ratelimit.Middleware(authLimiter)

	// Identity Routes
	r.POST("/auth/register", authMiddleware, authHandler.Register)
	r.POST("/auth/login", authMiddleware, authHandler.Login)
	r.POST("/auth/forgot-password", authMiddleware, authHandler.ForgotPassword)
	r.POST("/auth/reset-password", authMiddleware, authHandler.ResetPassword)

	keyGroup := r.Group("/auth/keys")
	keyGroup.Use(httputil.Auth(identitySvc))
	{
		keyGroup.POST("", identityHandler.CreateKey)
		keyGroup.GET("", identityHandler.ListKeys)
		keyGroup.DELETE("/:id", identityHandler.RevokeKey)
		keyGroup.POST("/:id/rotate", identityHandler.RotateKey)
		keyGroup.POST("/:id/regenerate", identityHandler.RegenerateKey)
	}

	// Instance Routes (Protected)
	instanceGroup := r.Group("/instances")
	instanceGroup.Use(httputil.Auth(identitySvc))
	{
		instanceGroup.POST("", httputil.Permission(rbacSvc, domain.PermissionInstanceLaunch), instanceHandler.Launch)
		instanceGroup.GET("", httputil.Permission(rbacSvc, domain.PermissionInstanceRead), instanceHandler.List)
		instanceGroup.GET("/:id", httputil.Permission(rbacSvc, domain.PermissionInstanceRead), instanceHandler.Get)
		instanceGroup.POST("/:id/stop", httputil.Permission(rbacSvc, domain.PermissionInstanceUpdate), instanceHandler.Stop)
		instanceGroup.GET("/:id/logs", httputil.Permission(rbacSvc, domain.PermissionInstanceRead), instanceHandler.GetLogs)
		instanceGroup.GET("/:id/stats", httputil.Permission(rbacSvc, domain.PermissionInstanceRead), instanceHandler.GetStats)
		instanceGroup.DELETE("/:id", httputil.Permission(rbacSvc, domain.PermissionInstanceTerminate), instanceHandler.Terminate)
	}

	// VPC Routes (Protected)
	vpcGroup := r.Group("/vpcs")
	vpcGroup.Use(httputil.Auth(identitySvc))
	{
		vpcGroup.POST("", httputil.Permission(rbacSvc, domain.PermissionVpcCreate), vpcHandler.Create)
		vpcGroup.GET("", httputil.Permission(rbacSvc, domain.PermissionVpcRead), vpcHandler.List)
		vpcGroup.GET("/:id", httputil.Permission(rbacSvc, domain.PermissionVpcRead), vpcHandler.Get)
		vpcGroup.DELETE("/:id", httputil.Permission(rbacSvc, domain.PermissionVpcDelete), vpcHandler.Delete)
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

	// Audit Routes (Protected)
	auditGroup := r.Group("/audit")
	auditGroup.Use(httputil.Auth(identitySvc))
	{
		auditGroup.GET("", auditHandler.ListLogs)
	}

	// Volume Routes (Protected)
	volumeGroup := r.Group("/volumes")
	volumeGroup.Use(httputil.Auth(identitySvc))
	{
		volumeGroup.POST("", httputil.Permission(rbacSvc, domain.PermissionVolumeCreate), volumeHandler.Create)
		volumeGroup.GET("", httputil.Permission(rbacSvc, domain.PermissionVolumeRead), volumeHandler.List)
		volumeGroup.GET("/:id", httputil.Permission(rbacSvc, domain.PermissionVolumeRead), volumeHandler.Get)
		volumeGroup.DELETE("/:id", httputil.Permission(rbacSvc, domain.PermissionVolumeDelete), volumeHandler.Delete)
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

	// Snapshot Routes (Protected)
	snapshotGroup := r.Group("/snapshots")
	snapshotGroup.Use(httputil.Auth(identitySvc))
	{
		snapshotGroup.POST("", httputil.Permission(rbacSvc, domain.PermissionSnapshotCreate), snapshotHandler.Create)
		snapshotGroup.GET("", httputil.Permission(rbacSvc, domain.PermissionSnapshotRead), snapshotHandler.List)
		snapshotGroup.GET("/:id", httputil.Permission(rbacSvc, domain.PermissionSnapshotRead), snapshotHandler.Get)
		snapshotGroup.DELETE("/:id", httputil.Permission(rbacSvc, domain.PermissionSnapshotDelete), snapshotHandler.Delete)
		snapshotGroup.POST("/:id/restore", httputil.Permission(rbacSvc, domain.PermissionSnapshotRestore), snapshotHandler.Restore)
	}

	// Load Balancer Routes (Protected)
	lbGroup := r.Group("/lb")
	lbGroup.Use(httputil.Auth(identitySvc))
	{
		lbGroup.POST("", httputil.Permission(rbacSvc, domain.PermissionLbCreate), lbHandler.Create)
		lbGroup.GET("", httputil.Permission(rbacSvc, domain.PermissionLbRead), lbHandler.List)
		lbGroup.GET("/:id", httputil.Permission(rbacSvc, domain.PermissionLbRead), lbHandler.Get)
		lbGroup.DELETE("/:id", httputil.Permission(rbacSvc, domain.PermissionLbDelete), lbHandler.Delete)
		lbGroup.POST("/:id/targets", httputil.Permission(rbacSvc, domain.PermissionLbUpdate), lbHandler.AddTarget)
		lbGroup.GET("/:id/targets", httputil.Permission(rbacSvc, domain.PermissionLbRead), lbHandler.ListTargets)
		lbGroup.DELETE("/:id/targets/:instanceId", httputil.Permission(rbacSvc, domain.PermissionLbUpdate), lbHandler.RemoveTarget)
	}

	// Database Routes (Protected)
	dbGroup := r.Group("/databases")
	dbGroup.Use(httputil.Auth(identitySvc))
	{
		dbGroup.POST("", httputil.Permission(rbacSvc, domain.PermissionDbCreate), databaseHandler.Create)
		dbGroup.GET("", httputil.Permission(rbacSvc, domain.PermissionDbRead), databaseHandler.List)
		dbGroup.GET("/:id", httputil.Permission(rbacSvc, domain.PermissionDbRead), databaseHandler.Get)
		dbGroup.DELETE("/:id", httputil.Permission(rbacSvc, domain.PermissionDbDelete), databaseHandler.Delete)
		dbGroup.GET("/:id/connection", httputil.Permission(rbacSvc, domain.PermissionDbRead), databaseHandler.GetConnectionString)
	}

	// Secret Routes (Protected)
	secretGroup := r.Group("/secrets")
	secretGroup.Use(httputil.Auth(identitySvc))
	{
		secretGroup.POST("", httputil.Permission(rbacSvc, domain.PermissionSecretCreate), secretHandler.Create)
		secretGroup.GET("", httputil.Permission(rbacSvc, domain.PermissionSecretRead), secretHandler.List)
		secretGroup.GET("/:id", httputil.Permission(rbacSvc, domain.PermissionSecretRead), secretHandler.Get)
		secretGroup.DELETE("/:id", httputil.Permission(rbacSvc, domain.PermissionSecretDelete), secretHandler.Delete)
	}

	// Function Routes (Protected)
	fnGroup := r.Group("/functions")
	fnGroup.Use(httputil.Auth(identitySvc))
	{
		fnGroup.POST("", httputil.Permission(rbacSvc, domain.PermissionFunctionCreate), fnHandler.Create)
		fnGroup.GET("", httputil.Permission(rbacSvc, domain.PermissionFunctionRead), fnHandler.List)
		fnGroup.GET("/:id", httputil.Permission(rbacSvc, domain.PermissionFunctionRead), fnHandler.Get)
		fnGroup.DELETE("/:id", httputil.Permission(rbacSvc, domain.PermissionFunctionDelete), fnHandler.Delete)
		fnGroup.POST("/:id/invoke", httputil.Permission(rbacSvc, domain.PermissionFunctionInvoke), fnHandler.Invoke)
		fnGroup.GET("/:id/logs", httputil.Permission(rbacSvc, domain.PermissionFunctionRead), fnHandler.GetLogs)
	}

	// Cache Routes (Protected)
	cacheGroup := r.Group("/caches")
	cacheGroup.Use(httputil.Auth(identitySvc))
	{
		cacheGroup.POST("", httputil.Permission(rbacSvc, domain.PermissionCacheCreate), cacheHandler.Create)
		cacheGroup.GET("", httputil.Permission(rbacSvc, domain.PermissionCacheRead), cacheHandler.List)
		cacheGroup.GET("/:id", httputil.Permission(rbacSvc, domain.PermissionCacheRead), cacheHandler.Get)
		cacheGroup.DELETE("/:id", httputil.Permission(rbacSvc, domain.PermissionCacheDelete), cacheHandler.Delete)
		cacheGroup.GET("/:id/connection", httputil.Permission(rbacSvc, domain.PermissionCacheRead), cacheHandler.GetConnectionString)
		cacheGroup.POST("/:id/flush", httputil.Permission(rbacSvc, domain.PermissionCacheUpdate), cacheHandler.Flush)
		cacheGroup.GET("/:id/stats", httputil.Permission(rbacSvc, domain.PermissionCacheRead), cacheHandler.GetStats)
	}

	queueGroup := r.Group("/queues")
	queueGroup.Use(httputil.Auth(identitySvc))
	{
		queueGroup.POST("", httputil.Permission(rbacSvc, domain.PermissionQueueCreate), queueHandler.Create)
		queueGroup.GET("", httputil.Permission(rbacSvc, domain.PermissionQueueRead), queueHandler.List)
		queueGroup.GET("/:id", httputil.Permission(rbacSvc, domain.PermissionQueueRead), queueHandler.Get)
		queueGroup.DELETE("/:id", httputil.Permission(rbacSvc, domain.PermissionQueueDelete), queueHandler.Delete)
		queueGroup.POST("/:id/messages", httputil.Permission(rbacSvc, domain.PermissionQueueWrite), queueHandler.SendMessage)
		queueGroup.GET("/:id/messages", httputil.Permission(rbacSvc, domain.PermissionQueueRead), queueHandler.ReceiveMessages)
		queueGroup.DELETE("/:id/messages/:handle", httputil.Permission(rbacSvc, domain.PermissionQueueWrite), queueHandler.DeleteMessage)
		queueGroup.POST("/:id/purge", httputil.Permission(rbacSvc, domain.PermissionQueueWrite), queueHandler.Purge)
	}

	notifyGroup := r.Group("/notify")
	notifyGroup.Use(httputil.Auth(identitySvc))
	{
		notifyGroup.POST("/topics", httputil.Permission(rbacSvc, domain.PermissionNotifyCreate), notifyHandler.CreateTopic)
		notifyGroup.GET("/topics", httputil.Permission(rbacSvc, domain.PermissionNotifyRead), notifyHandler.ListTopics)
		notifyGroup.DELETE("/topics/:id", httputil.Permission(rbacSvc, domain.PermissionNotifyDelete), notifyHandler.DeleteTopic)
		notifyGroup.POST("/topics/:id/subscriptions", httputil.Permission(rbacSvc, domain.PermissionNotifyWrite), notifyHandler.Subscribe)
		notifyGroup.GET("/topics/:id/subscriptions", httputil.Permission(rbacSvc, domain.PermissionNotifyRead), notifyHandler.ListSubscriptions)
		notifyGroup.DELETE("/subscriptions/:id", httputil.Permission(rbacSvc, domain.PermissionNotifyDelete), notifyHandler.Unsubscribe)
		notifyGroup.POST("/topics/:id/publish", httputil.Permission(rbacSvc, domain.PermissionNotifyWrite), notifyHandler.Publish)
	}

	cronGroup := r.Group("/cron")
	cronGroup.Use(httputil.Auth(identitySvc))
	{
		cronGroup.POST("/jobs", httputil.Permission(rbacSvc, domain.PermissionCronCreate), cronHandler.CreateJob)
		cronGroup.GET("/jobs", httputil.Permission(rbacSvc, domain.PermissionCronRead), cronHandler.ListJobs)
		cronGroup.GET("/jobs/:id", httputil.Permission(rbacSvc, domain.PermissionCronRead), cronHandler.GetJob)
		cronGroup.DELETE("/jobs/:id", httputil.Permission(rbacSvc, domain.PermissionCronDelete), cronHandler.DeleteJob)
		cronGroup.POST("/jobs/:id/pause", httputil.Permission(rbacSvc, domain.PermissionCronUpdate), cronHandler.PauseJob)
		cronGroup.POST("/jobs/:id/resume", httputil.Permission(rbacSvc, domain.PermissionCronUpdate), cronHandler.ResumeJob)
	}

	gatewayGroup := r.Group("/gateway")
	gatewayGroup.Use(httputil.Auth(identitySvc))
	{
		gatewayGroup.POST("/routes", httputil.Permission(rbacSvc, domain.PermissionGatewayCreate), gwHandler.CreateRoute)
		gatewayGroup.GET("/routes", httputil.Permission(rbacSvc, domain.PermissionGatewayRead), gwHandler.ListRoutes)
		gatewayGroup.DELETE("/routes/:id", httputil.Permission(rbacSvc, domain.PermissionGatewayDelete), gwHandler.DeleteRoute)
	}

	// The actual Gateway Proxy (Public)
	r.Any("/gw/*proxy", gwHandler.Proxy)

	containerGroup := r.Group("/containers")
	containerGroup.Use(httputil.Auth(identitySvc))
	{
		containerGroup.POST("/deployments", httputil.Permission(rbacSvc, domain.PermissionContainerCreate), containerHandler.CreateDeployment)
		containerGroup.GET("/deployments", httputil.Permission(rbacSvc, domain.PermissionContainerRead), containerHandler.ListDeployments)
		containerGroup.GET("/deployments/:id", httputil.Permission(rbacSvc, domain.PermissionContainerRead), containerHandler.GetDeployment)
		containerGroup.POST("/deployments/:id/scale", httputil.Permission(rbacSvc, domain.PermissionContainerUpdate), containerHandler.ScaleDeployment)
		containerGroup.DELETE("/deployments/:id", httputil.Permission(rbacSvc, domain.PermissionContainerDelete), containerHandler.DeleteDeployment)
	}
	// Auto-Scaling Routes (Protected)
	asgRepo := postgres.NewAutoScalingRepo(db)
	asgSvc := services.NewAutoScalingService(asgRepo, vpcRepo, auditSvc)
	asgHandler := httphandlers.NewAutoScalingHandler(asgSvc)
	asgWorker := services.NewAutoScalingWorker(asgRepo, instanceSvc, lbSvc, eventSvc, ports.RealClock{})

	// Auto-Scaling Routes (Protected)
	asgGroup := r.Group("/autoscaling")
	asgGroup.Use(httputil.Auth(identitySvc))
	{
		asgGroup.POST("/groups", httputil.Permission(rbacSvc, domain.PermissionAsCreate), asgHandler.CreateGroup)
		asgGroup.GET("/groups", httputil.Permission(rbacSvc, domain.PermissionAsRead), asgHandler.ListGroups)
		asgGroup.GET("/groups/:id", httputil.Permission(rbacSvc, domain.PermissionAsRead), asgHandler.GetGroup)
		asgGroup.DELETE("/groups/:id", httputil.Permission(rbacSvc, domain.PermissionAsDelete), asgHandler.DeleteGroup)
		asgGroup.POST("/groups/:id/policies", httputil.Permission(rbacSvc, domain.PermissionAsUpdate), asgHandler.CreatePolicy)
		asgGroup.DELETE("/policies/:id", httputil.Permission(rbacSvc, domain.PermissionAsDelete), asgHandler.DeletePolicy)
	}

	// 7. Background Workers
	wg := &sync.WaitGroup{}
	workerCtx, workerCancel := context.WithCancel(context.Background())
	wg.Add(4)
	go lbWorker.Run(workerCtx, wg)
	go asgWorker.Run(workerCtx, wg)
	go cronWorker.Run(workerCtx, wg)
	go containerWorker.Run(workerCtx, wg)

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
