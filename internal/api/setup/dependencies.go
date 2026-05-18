// Package setup wires API dependencies and routes.
package setup

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"strings"

	dnsadapter "github.com/poyrazk/thecloud/internal/adapters/dns"
	"github.com/poyrazk/thecloud/internal/adapters/vault"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/poyrazk/thecloud/internal/handlers/ws"
	"github.com/poyrazk/thecloud/internal/platform"
	"github.com/poyrazk/thecloud/internal/repositories/filesystem"
	"github.com/poyrazk/thecloud/internal/repositories/k8s"
	"github.com/poyrazk/thecloud/internal/repositories/postgres"
	"github.com/poyrazk/thecloud/internal/repositories/redis"
	"github.com/poyrazk/thecloud/internal/storage/coordinator"
	"github.com/poyrazk/thecloud/internal/storage/protocol"
	"github.com/poyrazk/thecloud/internal/workers"
	redisv9 "github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Repositories bundles all data access implementations.
type Repositories struct {
	Audit            ports.AuditRepository
	User             ports.UserRepository
	Tenant           ports.TenantRepository
	Identity         ports.IdentityRepository
	PasswordReset    ports.PasswordResetRepository
	RBAC             ports.RoleRepository
	Instance         ports.InstanceRepository
	Vpc              ports.VpcRepository
	Event            ports.EventRepository
	Volume           ports.VolumeRepository
	SecurityGroup    ports.SecurityGroupRepository
	Subnet           ports.SubnetRepository
	LB               ports.LBRepository
	Snapshot         ports.SnapshotRepository
	Stack            ports.StackRepository
	Storage          ports.StorageRepository
	Database         ports.DatabaseRepository
	Secret           ports.SecretRepository
	Function         ports.FunctionRepository
	FunctionSchedule ports.FunctionScheduleRepository
	Cache            ports.CacheRepository
	Queue            ports.QueueRepository
	Notify           ports.NotifyRepository
	Cron             ports.CronRepository
	Gateway          ports.GatewayRepository
	Container        ports.ContainerRepository
	AutoScaling      ports.AutoScalingRepository
	Accounting       ports.AccountingRepository
	TaskQueue        ports.TaskQueue
	DurableQueue     ports.DurableTaskQueue
	Ledger           ports.ExecutionLedger
	Image            ports.ImageRepository
	Cluster          ports.ClusterRepository
	Lifecycle        ports.LifecycleRepository
	DNS              ports.DNSRepository
	InstanceType     ports.InstanceTypeRepository
	GlobalLB         ports.GlobalLBRepository
	SSHKey           ports.SSHKeyRepository
	ElasticIP        ports.ElasticIPRepository
	Log              ports.LogRepository
	IAM              ports.IAMRepository
	ServiceAccount   ports.ServiceAccountRepository
	Pipeline         ports.PipelineRepository
	VPCPeering       ports.VPCPeeringRepository
	RouteTable       ports.RouteTableRepository
	IGW              ports.IGWRepository
	NATGateway       ports.NATGatewayRepository
}

// InitRepositories constructs repositories using the provided database clients.
func InitRepositories(db postgres.DB, rdb *redisv9.Client) *Repositories {
	return &Repositories{
		Audit:            postgres.NewAuditRepository(db),
		User:             postgres.NewUserRepo(db),
		Tenant:           postgres.NewTenantRepo(db),
		Identity:         postgres.NewIdentityRepository(db),
		PasswordReset:    postgres.NewPasswordResetRepository(db),
		RBAC:             postgres.NewRBACRepository(db),
		Instance:         postgres.NewInstanceRepository(db),
		Vpc:              postgres.NewVpcRepository(db),
		Event:            postgres.NewEventRepository(db),
		Volume:           postgres.NewVolumeRepository(db),
		SecurityGroup:    postgres.NewSecurityGroupRepository(db),
		Subnet:           postgres.NewSubnetRepository(db),
		LB:               postgres.NewLBRepository(db),
		Snapshot:         postgres.NewSnapshotRepository(db),
		Stack:            postgres.NewStackRepository(db),
		Storage:          postgres.NewStorageRepository(db),
		Database:         postgres.NewDatabaseRepository(db),
		Secret:           postgres.NewSecretRepository(db),
		Function:         postgres.NewFunctionRepository(db),
		FunctionSchedule: postgres.NewPostgresFunctionScheduleRepository(db),
		Cache:            postgres.NewCacheRepository(db),
		Queue:            postgres.NewPostgresQueueRepository(db),
		Notify:           postgres.NewPostgresNotifyRepository(db),
		Cron:             postgres.NewPostgresCronRepository(db),
		Gateway:          postgres.NewPostgresGatewayRepository(db),
		Container:        postgres.NewPostgresContainerRepository(db),
		AutoScaling:      postgres.NewAutoScalingRepo(db),
		Accounting:       postgres.NewAccountingRepository(db),
		TaskQueue:        redis.NewRedisTaskQueue(rdb),
		DurableQueue:     redis.NewDurableTaskQueue(rdb),
		Ledger:           postgres.NewExecutionLedger(db),
		Image:            postgres.NewImageRepository(db),
		Cluster:          postgres.NewClusterRepository(db),
		Lifecycle:        postgres.NewLifecycleRepository(db),
		DNS:              postgres.NewDNSRepository(db),
		InstanceType:     postgres.NewInstanceTypeRepository(db),
		GlobalLB:         postgres.NewGlobalLBRepository(db),
		SSHKey:           postgres.NewSSHKeyRepo(db),
		ElasticIP:        postgres.NewElasticIPRepository(db),
		Log:              postgres.NewLogRepository(db),
		IAM:              postgres.NewIAMRepository(db),
		Pipeline:         postgres.NewPipelineRepository(db),
		VPCPeering:       postgres.NewVPCPeeringRepository(db),
		RouteTable:       postgres.NewRouteTableRepository(db),
		IGW:              postgres.NewIGWRepository(db),
		NATGateway:       postgres.NewNATGatewayRepository(db),
	}
}

// Services bundles the core application services.
type Services struct {
	WsHub            *ws.Hub
	Audit            ports.AuditService
	Identity         ports.IdentityService
	Tenant           ports.TenantService
	Auth             ports.AuthService
	PasswordReset    ports.PasswordResetService
	RBAC             ports.RBACService
	Vpc              ports.VpcService
	Subnet           ports.SubnetService
	Event            ports.EventService
	Volume           ports.VolumeService
	Instance         ports.InstanceService
	SecurityGroup    ports.SecurityGroupService
	LB               ports.LBService
	Dashboard        ports.DashboardService
	Snapshot         ports.SnapshotService
	Stack            ports.StackService
	Storage          ports.StorageService
	Database         ports.DatabaseService
	Secret           ports.SecretService
	Function         ports.FunctionService
	FunctionSchedule ports.FunctionScheduleService
	Cache            ports.CacheService
	Queue            ports.QueueService
	Notify           ports.NotifyService
	Cron             ports.CronService
	Gateway          ports.GatewayService
	Container        ports.ContainerService
	Health           ports.HealthService
	AutoScaling      ports.AutoScalingService
	Accounting       ports.AccountingService
	Image            ports.ImageService
	Cluster          ports.ClusterService
	Lifecycle        ports.LifecycleService
	DNS              ports.DNSService
	InstanceType     ports.InstanceTypeService
	GlobalLB         ports.GlobalLBService
	SSHKey           ports.SSHKeyService
	ElasticIP        ports.ElasticIPService
	Log              ports.LogService
	IAM              ports.IAMService
	Pipeline         ports.PipelineService
	VPCPeering       ports.VPCPeeringService
	RouteTable       *services.RouteTableService
	InternetGateway  *services.InternetGatewayService
	NATGateway       *services.NATGatewayService
}

// Shutdown cleanly stops all services.
func (s *Services) Shutdown() {
	if s == nil || s.WsHub == nil {
		return
	}
	s.WsHub.Stop()
}

// Runner is the interface that all background workers implement.
type Runner interface {
	Run(context.Context, *sync.WaitGroup)
}

// Workers struct to return background workers.
// Singleton workers are typed as Runner so they can be wrapped with LeaderGuard.
// Parallel consumers retain concrete types for direct configuration access.
type Workers struct {
	// Singleton workers (must run on exactly one node via leader election)
	LB                Runner
	AutoScaling       Runner
	Cron              Runner
	Container         Runner
	Accounting        Runner
	Lifecycle         Runner
	ReplicaMonitor    Runner
	ClusterReconciler Runner
	Healing           Runner
	DatabaseFailover  Runner
	Log               Runner

	// Parallel consumer workers (safe to run on multiple nodes)
	Pipeline         *workers.PipelineWorker
	Provision        *workers.ProvisionWorker
	Cluster          *workers.ClusterWorker
	FunctionSchedule *services.FunctionScheduleWorker
}

// ServiceConfig holds the dependencies required to initialize services
type ServiceConfig struct {
	Config        *platform.Config
	Repos         *Repositories
	Compute       ports.ComputeBackend
	Storage       ports.StorageBackend
	Network       ports.NetworkBackend
	LBProxy       ports.LBProxyAdapter
	DB            postgres.DB
	RDB           *redisv9.Client
	Logger        *slog.Logger
	LeaderElector ports.LeaderElector // nil disables leader election (single-instance mode)
}

// InitServices constructs core services and background workers.
func InitServices(c ServiceConfig) (*Services, *Workers, error) {
	// 1. Core Services (Audit, Identity, Auth, RBAC)
	rbacSvc := initRBACServices(c)
	auditSvc := services.NewAuditService(services.AuditServiceParams{Repo: c.Repos.Audit, RBACSvc: rbacSvc, Logger: c.Logger})
	identitySvc := initIdentityServices(c, rbacSvc, auditSvc)
	tenantSvc := services.NewTenantService(services.TenantServiceParams{Repo: c.Repos.Tenant, UserRepo: c.Repos.User, RBACSvc: rbacSvc, Logger: c.Logger})
	authSvc := services.NewAuthService(c.Repos.User, identitySvc, auditSvc, tenantSvc, c.DB, c.Logger)
	pwdResetSvc := services.NewPasswordResetService(c.Repos.PasswordReset, c.Repos.User, c.Logger)

	// 2. WebSocket & Core Infrastructure
	wsHub := ws.NewHub(c.Logger)
	go wsHub.Run()
	eventSvc := services.NewEventService(services.EventServiceParams{Repo: c.Repos.Event, RBACSvc: rbacSvc, Publisher: wsHub, Logger: c.Logger})

	// 3. Cloud Infrastructure Services (VPC, Subnet, Instance, Volume, SG, LB)
	vpcSvc := services.NewVpcService(services.VpcServiceParams{Repo: c.Repos.Vpc, LBRepo: c.Repos.LB, PeeringRepo: c.Repos.VPCPeering, AsRepo: c.Repos.AutoScaling, RBACSvc: rbacSvc, Network: c.Network, AuditSvc: auditSvc, Logger: c.Logger, DefaultCIDR: c.Config.DefaultVPCCIDR, ComputeBackend: c.Config.ComputeBackend})
	subnetSvc := services.NewSubnetService(services.SubnetServiceParams{Repo: c.Repos.Subnet, RBACSvc: rbacSvc, VpcRepo: c.Repos.Vpc, AuditSvc: auditSvc, Logger: c.Logger})
	volumeSvc := services.NewVolumeService(services.VolumeServiceParams{Repo: c.Repos.Volume, RBACSvc: rbacSvc, Storage: c.Storage, EventSvc: eventSvc, AuditSvc: auditSvc, Logger: c.Logger})

	// DNS Service
	pdnsBackend, err := dnsadapter.NewPowerDNSBackend(c.Config.PowerDNSAPIURL, c.Config.PowerDNSAPIKey, c.Config.PowerDNSServerID, c.Logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to init powerdns backend: %w", err)
	}
	// Wrap DNS backend with resilience (circuit breaker + timeout).
	resilientDNS := platform.NewResilientDNS(pdnsBackend, c.Logger, platform.ResilientDNSOpts{})
	dnsSvc := services.NewDNSService(services.DNSServiceParams{
		Repo: c.Repos.DNS, RBAC: rbacSvc, Backend: resilientDNS, VpcRepo: c.Repos.Vpc,
		AuditSvc: auditSvc, EventSvc: eventSvc, Logger: c.Logger,
	})

	sshKeySvc, err := services.NewSSHKeyService(services.SSHKeyServiceParams{Repo: c.Repos.SSHKey, Logger: c.Logger, RBACSvc: rbacSvc})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to init ssh key service: %w", err)
	}

	logSvc := services.NewCloudLogsService(c.Repos.Log, rbacSvc, c.Logger)

	instSvcConcrete := services.NewInstanceService(services.InstanceServiceParams{Repo: c.Repos.Instance, VpcRepo: c.Repos.Vpc, SubnetRepo: c.Repos.Subnet, VolumeRepo: c.Repos.Volume, InstanceTypeRepo: c.Repos.InstanceType, RBAC: rbacSvc, Compute: c.Compute, Network: c.Network, EventSvc: eventSvc, AuditSvc: auditSvc, DNSSvc: dnsSvc, TaskQueue: c.Repos.DurableQueue, DockerNetwork: c.Config.DockerDefaultNetwork, Logger: c.Logger, TenantSvc: tenantSvc, SSHKeySvc: sshKeySvc, LogSvc: logSvc})
	sgSvc := services.NewSecurityGroupService(c.Repos.SecurityGroup, rbacSvc, c.Repos.Vpc, c.Network, auditSvc, c.Logger)

	lbSvc := services.NewLBService(c.Repos.LB, rbacSvc, c.Repos.Vpc, c.Repos.Instance, auditSvc, c.Logger)
	lbWorker := services.NewLBWorker(c.Repos.LB, c.Repos.Instance, c.LBProxy)

	// Global LB Service
	glbSvc := services.NewGlobalLBService(services.GlobalLBServiceParams{Repo: c.Repos.GlobalLB, RBAC: rbacSvc, LBRepo: c.Repos.LB, GeoDNS: pdnsBackend, AuditSvc: auditSvc, Logger: c.Logger})

	// Encryption Service
	encryptionRepo := postgres.NewEncryptionRepository(c.DB)
	encryptionSvc, err := services.NewEncryptionService(encryptionRepo, c.Config.SecretsEncryptionKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to init encryption service: %w", err)
	}

	// 4. Advanced Services (Storage, DB, Secrets, FaaS, Cache, Queue)
	storageSvc, fileStore, err := initStorageServices(c, rbacSvc, auditSvc, encryptionSvc)
	if err != nil {
		return nil, nil, err
	}

	snapshotSvc := services.NewSnapshotService(c.Repos.Snapshot, rbacSvc, c.Repos.Volume, c.Storage, eventSvc, auditSvc, c.Logger)

	var secretsSvc ports.SecretsManager
	if c.Config.VaultToken == "" {
		if c.Config.Environment == "production" || c.Config.Environment == "staging" {
			return nil, nil, fmt.Errorf("VAULT_TOKEN is required in %s environment", c.Config.Environment)
		}
		c.Logger.Warn("VAULT_TOKEN not set, using NoOp secrets manager. Credentials will NOT be stored in Vault.")
		secretsSvc = vault.NewNoOpSecretsManager()
	} else {
		vaultSvc, err := vault.NewVaultAdapter(c.Config.VaultAddress, c.Config.VaultToken, c.Logger)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to init vault adapter: %w", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := vaultSvc.Ping(ctx); err != nil {
			return nil, nil, fmt.Errorf("vault health check failed on startup: %w", err)
		}
		secretsSvc = vaultSvc
	}

	databaseSvc := services.NewDatabaseService(services.DatabaseServiceParams{Repo: c.Repos.Database, RBAC: rbacSvc, Compute: c.Compute, VpcRepo: c.Repos.Vpc, VolumeSvc: volumeSvc, SnapshotSvc: snapshotSvc, SnapshotRepo: c.Repos.Snapshot, EventSvc: eventSvc, AuditSvc: auditSvc, Secrets: secretsSvc, VolumeEncryption: nil, Logger: c.Logger, VaultMountPath: c.Config.VaultMountPath})
	secretSvc, err := services.NewSecretService(services.SecretServiceParams{Repo: c.Repos.Secret, RBACSvc: rbacSvc, EventSvc: eventSvc, AuditSvc: auditSvc, Logger: c.Logger, MasterKey: c.Config.SecretsEncryptionKey, Environment: c.Config.Environment})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to init secret service: %w", err)
	}
	fnSvc := services.NewFunctionService(c.Repos.Function, rbacSvc, c.Compute, fileStore, auditSvc, secretSvc, c.Logger)
	fnSchedSvc := services.NewFunctionScheduleService(c.Repos.FunctionSchedule, c.Repos.Function, rbacSvc, eventSvc, auditSvc, c.Logger)
	cacheSvc := services.NewCacheService(c.Repos.Cache, rbacSvc, c.Compute, c.Repos.Vpc, eventSvc, auditSvc, c.Logger)
	queueSvc := services.NewQueueService(c.Repos.Queue, rbacSvc, eventSvc, auditSvc, c.Logger)
	pipelineSvc := services.NewPipelineService(c.Repos.Pipeline, c.Repos.DurableQueue, eventSvc, auditSvc, c.Logger)
	notifySvc := services.NewNotifyService(services.NotifyServiceParams{Repo: c.Repos.Notify, RBACSvc: rbacSvc, QueueSvc: queueSvc, EventSvc: eventSvc, AuditSvc: auditSvc, Logger: c.Logger})

	// 5. DevOps & Automation Services
	cronSvc := services.NewCronService(c.Repos.Cron, rbacSvc, eventSvc, auditSvc, c.Logger)
	cronWorker := services.NewCronWorker(c.Repos.Cron)
	gwSvc := services.NewGatewayService(c.Repos.Gateway, rbacSvc, auditSvc, c.Logger)
	containerSvc := services.NewContainerService(c.Repos.Container, rbacSvc, eventSvc, auditSvc, c.Logger)
	containerWorker := services.NewContainerWorker(c.Repos.Container, instSvcConcrete, eventSvc)
	stackSvc := services.NewStackService(c.Repos.Stack, rbacSvc, instSvcConcrete, vpcSvc, volumeSvc, snapshotSvc, c.Logger)

	// 6. Business & Scaling Services
	asgSvc := services.NewAutoScalingService(c.Repos.AutoScaling, rbacSvc, c.Repos.Vpc, auditSvc, c.Logger)
	asgWorker := services.NewAutoScalingWorker(c.Repos.AutoScaling, instSvcConcrete, lbSvc, eventSvc, ports.RealClock{})
	accountingSvc := services.NewAccountingService(c.Repos.Accounting, rbacSvc, c.Repos.Instance, c.Logger)
	accountingWorker := workers.NewAccountingWorker(accountingSvc, c.Logger)
	imageSvc := services.NewImageService(services.ImageServiceParams{Repo: c.Repos.Image, RBACSvc: rbacSvc, FileStore: fileStore, Logger: c.Logger})
	iamSvc := services.NewIAMService(c.Repos.IAM, auditSvc, eventSvc, c.Logger)
	provisionWorker := workers.NewProvisionWorker(instSvcConcrete, c.Repos.DurableQueue, c.Repos.Ledger, c.Logger)
	healingWorker := workers.NewHealingWorker(instSvcConcrete, c.Repos.Instance, c.Logger)

	clusterSvc, clusterProvisioner, err := initClusterServices(c, rbacSvc, vpcSvc, instSvcConcrete, secretSvc, storageSvc, lbSvc, sgSvc)
	if err != nil {
		return nil, nil, err
	}

	svcs := &Services{WsHub: wsHub, Audit: auditSvc, Identity: identitySvc, Tenant: tenantSvc, Auth: authSvc, PasswordReset: pwdResetSvc, RBAC: rbacSvc, Vpc: vpcSvc, Subnet: subnetSvc, Event: eventSvc, Volume: volumeSvc, Instance: instSvcConcrete, SecurityGroup: sgSvc, LB: lbSvc, Snapshot: snapshotSvc, Stack: stackSvc, Storage: storageSvc, Database: databaseSvc, Secret: secretSvc, Function: fnSvc, FunctionSchedule: fnSchedSvc, Cache: cacheSvc, Queue: queueSvc, Notify: notifySvc, Cron: cronSvc, Gateway: gwSvc, Container: containerSvc, Pipeline: pipelineSvc, Health: services.NewHealthServiceImpl(c.DB, c.Compute, clusterSvc), AutoScaling: asgSvc, Accounting: accountingSvc, Image: imageSvc, Cluster: clusterSvc, Dashboard: services.NewDashboardService(rbacSvc, c.Repos.Instance, c.Repos.Volume, c.Repos.Vpc, c.Repos.Event, c.Logger), Lifecycle: services.NewLifecycleService(c.Repos.Lifecycle, rbacSvc, c.Repos.Storage), InstanceType: services.NewInstanceTypeService(c.Repos.InstanceType, rbacSvc), GlobalLB: glbSvc, DNS: dnsSvc, SSHKey: sshKeySvc, ElasticIP: services.NewElasticIPService(services.ElasticIPServiceParams{Repo: c.Repos.ElasticIP, RBAC: rbacSvc, InstanceRepo: c.Repos.Instance, AuditSvc: auditSvc, Logger: c.Logger}), Log: logSvc, IAM: iamSvc, VPCPeering: services.NewVPCPeeringService(services.VPCPeeringServiceParams{Repo: c.Repos.VPCPeering, VpcRepo: c.Repos.Vpc, Network: c.Network, AuditSvc: auditSvc, Logger: c.Logger}), RouteTable: services.NewRouteTableService(services.RouteTableServiceParams{Repo: c.Repos.RouteTable, VpcRepo: c.Repos.Vpc, RBACSvc: rbacSvc, Network: c.Network, AuditSvc: auditSvc, Logger: c.Logger}), InternetGateway: services.NewInternetGatewayService(services.InternetGatewayServiceParams{Repo: c.Repos.IGW, RTRepo: c.Repos.RouteTable, VpcRepo: c.Repos.Vpc, RBACSvc: rbacSvc, AuditSvc: auditSvc, Logger: c.Logger}), NATGateway: services.NewNATGatewayService(services.NATGatewayServiceParams{Repo: c.Repos.NATGateway, EIPRepo: c.Repos.ElasticIP, SubnetRepo: c.Repos.Subnet, VpcRepo: c.Repos.Vpc, RBACSvc: rbacSvc, Network: c.Network, AuditSvc: auditSvc, Logger: c.Logger})}

	// 7. High Availability & Monitoring
	replicaMonitor := initReplicaMonitor(c)

	// Helper: wrap a singleton worker with LeaderGuard if leader election is enabled.
	// Accepts a concrete pointer to avoid nil-interface pitfalls — callers must
	// explicitly pass nil Runner when the worker should be skipped.
	guardSingleton := func(key string, w Runner) Runner {
		if w == nil || c.LeaderElector == nil {
			return w
		}
		return workers.NewLeaderGuard(c.LeaderElector, key, w, c.Logger)
	}

	lifecycleWorker := workers.NewLifecycleWorker(c.Repos.Lifecycle, storageSvc, c.Repos.Storage, c.Logger)
	clusterReconciler := workers.NewClusterReconciler(c.Repos.Cluster, clusterProvisioner, c.Logger)
	dbFailoverWorker := workers.NewDatabaseFailoverWorker(databaseSvc, c.Repos.Database, c.Compute, c.Logger)
	logWorker := workers.NewLogWorker(logSvc, c.Logger)

	// For replicaMonitor, we must convert nil *ReplicaMonitor to nil Runner to avoid
	// a non-nil interface wrapping a nil pointer.
	var replicaMonitorRunner Runner
	if replicaMonitor != nil {
		replicaMonitorRunner = replicaMonitor
	}

	workersCollection := &Workers{
		// Singleton workers — wrapped with leader election
		LB:                guardSingleton("singleton:lb", lbWorker),
		AutoScaling:       guardSingleton("singleton:autoscaling", asgWorker),
		Cron:              guardSingleton("singleton:cron", cronWorker),
		Container:         guardSingleton("singleton:container", containerWorker),
		Accounting:        guardSingleton("singleton:accounting", accountingWorker),
		Lifecycle:         guardSingleton("singleton:lifecycle", lifecycleWorker),
		ReplicaMonitor:    guardSingleton("singleton:replica-monitor", replicaMonitorRunner),
		ClusterReconciler: guardSingleton("singleton:cluster-reconciler", clusterReconciler),
		Healing:           guardSingleton("singleton:healing", healingWorker),
		DatabaseFailover:  guardSingleton("singleton:db-failover", dbFailoverWorker),
		Log:               guardSingleton("singleton:log", logWorker),

		// Parallel consumer workers — no leader election needed
		Pipeline:         workers.NewPipelineWorker(c.Repos.Pipeline, c.Repos.DurableQueue, c.Repos.Ledger, c.Compute, c.Logger),
		Provision:        provisionWorker,
		Cluster:          workers.NewClusterWorker(c.Repos.Cluster, clusterProvisioner, c.Repos.DurableQueue, c.Repos.Ledger, c.Logger),
		FunctionSchedule: services.NewFunctionScheduleWorker(c.Repos.FunctionSchedule, fnSvc),
	}

	return svcs, workersCollection, nil
}

func initIdentityServices(c ServiceConfig, rbacSvc ports.RBACService, audit ports.AuditService) ports.IdentityService {
	tokenTTL := time.Duration(c.Config.ServiceAccountTokenTTL) * time.Second
	if tokenTTL == 0 {
		tokenTTL = time.Hour
	}
	base := services.NewIdentityService(services.IdentityServiceParams{Repo: c.Repos.Identity, SARepo: c.Repos.ServiceAccount, RbacSvc: rbacSvc, AuditSvc: audit, Logger: c.Logger, TokenTTL: tokenTTL})
	return services.NewCachedIdentityService(base, c.RDB, c.Logger)
}

func initRBACServices(c ServiceConfig) ports.RBACService {
	iamRepo := c.Repos.IAM
	evaluator := services.NewIAMEvaluator()
	base := services.NewRBACService(services.RBACServiceParams{UserRepo: c.Repos.User, RoleRepo: c.Repos.RBAC, TenantRepo: c.Repos.Tenant, IAMRepo: iamRepo, Evaluator: evaluator, Logger: c.Logger, SARepo: c.Repos.ServiceAccount})
	return services.NewCachedRBACService(base, c.Repos.ServiceAccount, c.RDB, c.Logger)
}

func buildStorageDialOpts(cfg *platform.Config) ([]grpc.DialOption, error) {
	if cfg.StorageTLSEnabled {
		cert, err := tls.LoadX509KeyPair(cfg.StorageTLSCertFile, cfg.StorageTLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load storage TLS cert: %w", err)
		}
		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
		}
		if cfg.StorageTLSCACertFile != "" {
			caCert, err := os.ReadFile(cfg.StorageTLSCACertFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read storage TLS CA cert: %w", err)
			}
			caPool := x509.NewCertPool()
			if !caPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("failed to parse storage TLS CA cert PEM from %s", cfg.StorageTLSCACertFile)
			}
			tlsCfg.RootCAs = caPool
		}
		if cfg.StorageTLSSkipVerify {
			tlsCfg.InsecureSkipVerify = true
		}
		creds := credentials.NewTLS(tlsCfg)
		return []grpc.DialOption{grpc.WithTransportCredentials(creds)}, nil
	}
	return []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}, nil
}

func initStorageServices(c ServiceConfig, rbacSvc ports.RBACService, audit ports.AuditService, encryption ports.EncryptionService) (ports.StorageService, ports.FileStore, error) {
	var fileStore ports.FileStore
	var err error

	dialOpts, err := buildStorageDialOpts(c.Config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build storage dial options: %w", err)
	}

	if c.Config.ObjectStorageMode == "distributed" {
		c.Logger.Info("initializing distributed storage backend")
		ring := coordinator.NewConsistentHashRing(100) // 100 virtual nodes

		nodes := strings.Split(c.Config.ObjectStorageNodes, ",")
		clients := make(map[string]protocol.StorageNodeClient)

		for i, addr := range nodes {
			if addr == "" {
				continue
			}
			nodeID := fmt.Sprintf("node-%d", i+1)

			conn, err := grpc.NewClient(addr, dialOpts...)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to connect to storage node %s: %w", addr, err)
			}
			clients[nodeID] = protocol.NewStorageNodeClient(conn)
			ring.AddNode(nodeID)
			c.Logger.Info("added storage node", "id", nodeID, "addr", addr)
		}

		fileStore = coordinator.NewCoordinator(context.Background(), ring, clients, 3)
	} else {
		fileStore, err = filesystem.NewLocalFileStore("./thecloud-data/local/storage")
		if err != nil {
			return nil, nil, err
		}
	}

	storageSvc := services.NewStorageService(services.StorageServiceParams{
		Repo:       c.Repos.Storage,
		RBACSvc:    rbacSvc,
		Store:      fileStore,
		AuditSvc:   audit,
		EncryptSvc: encryption,
		Config:     c.Config,
		Logger:     c.Logger,
	})
	return storageSvc, fileStore, nil
}

func initClusterServices(c ServiceConfig, rbacSvc ports.RBACService, vpcSvc ports.VpcService, instSvc ports.InstanceService, secretSvc ports.SecretService, storageSvc ports.StorageService, lbSvc ports.LBService, sgSvc ports.SecurityGroupService) (ports.ClusterService, ports.ClusterProvisioner, error) {
	clusterProvisioner := k8s.NewKubeadmProvisioner(instSvc, c.Repos.Cluster, secretSvc, sgSvc, storageSvc, lbSvc, c.Logger)
	clusterSvc, err := services.NewClusterService(services.ClusterServiceParams{
		Repo: c.Repos.Cluster, RBAC: rbacSvc, Provisioner: clusterProvisioner, VpcSvc: vpcSvc, InstanceSvc: instSvc, SecretSvc: secretSvc, TaskQueue: c.Repos.DurableQueue, Logger: c.Logger,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to init cluster service: %w", err)
	}
	return clusterSvc, clusterProvisioner, nil
}

func initReplicaMonitor(c ServiceConfig) *workers.ReplicaMonitor {
	if dualDB, ok := c.DB.(*postgres.DualDB); ok {
		replica := dualDB.GetReplica()
		// Only monitor if we actually have a separate replica
		if replica != nil && replica != dualDB {
			return workers.NewReplicaMonitor(dualDB, replica, c.Logger)
		}
	}
	return nil
}
