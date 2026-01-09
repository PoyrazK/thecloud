package setup

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/poyrazk/thecloud/internal/platform"
	"github.com/poyrazk/thecloud/internal/repositories/filesystem"
	"github.com/poyrazk/thecloud/internal/repositories/postgres"
)

type Repositories struct {
	Audit         ports.AuditRepository
	User          ports.UserRepository
	Identity      ports.IdentityRepository
	PasswordReset ports.PasswordResetRepository
	RBAC          ports.RoleRepository
	Instance      ports.InstanceRepository
	Vpc           ports.VpcRepository
	Event         ports.EventRepository
	Volume        ports.VolumeRepository
	SecurityGroup ports.SecurityGroupRepository
	Subnet        ports.SubnetRepository
	LB            ports.LBRepository
	Snapshot      ports.SnapshotRepository
	Stack         ports.StackRepository
	Storage       ports.StorageRepository
	Database      ports.DatabaseRepository
	Secret        ports.SecretRepository
	Function      ports.FunctionRepository
	Cache         ports.CacheRepository
	Queue         ports.QueueRepository
	Notify        ports.NotifyRepository
	Cron          ports.CronRepository
	Gateway       ports.GatewayRepository
	Container     ports.ContainerRepository
	AutoScaling   ports.AutoScalingRepository
}

func InitRepositories(db *pgxpool.Pool) *Repositories {
	return &Repositories{
		Audit:         postgres.NewAuditRepository(db),
		User:          postgres.NewUserRepo(db),
		Identity:      postgres.NewIdentityRepository(db),
		PasswordReset: postgres.NewPasswordResetRepository(db),
		RBAC:          postgres.NewRBACRepository(db),
		Instance:      postgres.NewInstanceRepository(db),
		Vpc:           postgres.NewVpcRepository(db),
		Event:         postgres.NewEventRepository(db),
		Volume:        postgres.NewVolumeRepository(db),
		SecurityGroup: postgres.NewSecurityGroupRepository(db),
		Subnet:        postgres.NewSubnetRepository(db),
		LB:            postgres.NewLBRepository(db),
		Snapshot:      postgres.NewSnapshotRepository(db),
		Stack:         postgres.NewStackRepository(db),
		Storage:       postgres.NewStorageRepository(db),
		Database:      postgres.NewDatabaseRepository(db),
		Secret:        postgres.NewSecretRepository(db),
		Function:      postgres.NewFunctionRepository(db),
		Cache:         postgres.NewCacheRepository(db),
		Queue:         postgres.NewPostgresQueueRepository(db),
		Notify:        postgres.NewPostgresNotifyRepository(db),
		Cron:          postgres.NewPostgresCronRepository(db),
		Gateway:       postgres.NewPostgresGatewayRepository(db),
		Container:     postgres.NewPostgresContainerRepository(db),
		AutoScaling:   postgres.NewAutoScalingRepo(db),
	}
}

type Services struct {
	Audit         ports.AuditService
	Identity      ports.IdentityService
	Auth          ports.AuthService
	PasswordReset ports.PasswordResetService
	RBAC          ports.RBACService
	Vpc           ports.VpcService
	Subnet        ports.SubnetService
	Event         ports.EventService
	Volume        ports.VolumeService
	Instance      ports.InstanceService
	SecurityGroup ports.SecurityGroupService
	LB            ports.LBService
	Dashboard     ports.DashboardService
	Snapshot      ports.SnapshotService
	Stack         ports.StackService
	Storage       ports.StorageService
	Database      ports.DatabaseService
	Secret        ports.SecretService
	Function      ports.FunctionService
	Cache         ports.CacheService
	Queue         ports.QueueService
	Notify        ports.NotifyService
	Cron          ports.CronService
	Gateway       ports.GatewayService
	Container     ports.ContainerService
	Health        ports.HealthService
	AutoScaling   ports.AutoScalingService
}

// Workers struct to return background workers
type Workers struct {
	LB          *services.LBWorker
	AutoScaling *services.AutoScalingWorker
	Cron        *services.CronWorker
	Container   *services.ContainerWorker
}

func InitServices(
	cfg *platform.Config,
	repos *Repositories,
	compute ports.ComputeBackend,
	network ports.NetworkBackend,
	lbProxy ports.LBProxyAdapter,
	db *pgxpool.Pool, // needed for HealthService
	logger *slog.Logger,
) (*Services, *Workers, error) {
	// Audit Service
	auditSvc := services.NewAuditService(repos.Audit)

	// Identity & Auth
	identitySvc := services.NewIdentityService(repos.Identity, auditSvc)
	authSvc := services.NewAuthService(repos.User, identitySvc, auditSvc)
	pwdResetSvc := services.NewPasswordResetService(repos.PasswordReset, repos.User, logger)
	rbacSvc := services.NewRBACService(repos.User, repos.RBAC, logger)

	// Core Cloud Services
	eventSvc := services.NewEventService(repos.Event, logger)
	vpcSvc := services.NewVpcService(repos.Vpc, network, auditSvc, logger, cfg.DefaultVPCCIDR)
	subnetSvc := services.NewSubnetService(repos.Subnet, repos.Vpc, auditSvc, logger)
	volumeSvc := services.NewVolumeService(repos.Volume, compute, eventSvc, auditSvc, logger)
	instanceSvc := services.NewInstanceService(services.InstanceServiceParams{
		Repo:       repos.Instance,
		VpcRepo:    repos.Vpc,
		SubnetRepo: repos.Subnet,
		VolumeRepo: repos.Volume,
		Compute:    compute,
		Network:    network,
		EventSvc:   eventSvc,
		AuditSvc:   auditSvc,
		Logger:     logger,
	})
	sgSvc := services.NewSecurityGroupService(repos.SecurityGroup, repos.Vpc, network, auditSvc, logger)

	// Load Balancer
	lbSvc := services.NewLBService(repos.LB, repos.Vpc, repos.Instance, auditSvc)
	lbWorker := services.NewLBWorker(repos.LB, repos.Instance, lbProxy)

	// Dashboard
	dashboardSvc := services.NewDashboardService(repos.Instance, repos.Volume, repos.Vpc, repos.Event, logger)

	// Snapshot
	snapshotSvc := services.NewSnapshotService(repos.Snapshot, repos.Volume, compute, eventSvc, auditSvc, logger)

	// Stack (IaC)
	stackSvc := services.NewStackService(repos.Stack, instanceSvc, vpcSvc, volumeSvc, snapshotSvc, logger)

	// Storage
	fileStore, err := filesystem.NewLocalFileStore("./thecloud-data/local/storage")
	if err != nil {
		return nil, nil, err
	}
	storageSvc := services.NewStorageService(repos.Storage, fileStore, auditSvc)

	// Database (PaaS)
	databaseSvc := services.NewDatabaseService(repos.Database, compute, repos.Vpc, eventSvc, auditSvc, logger)

	// Secrets
	secretSvc := services.NewSecretService(repos.Secret, eventSvc, auditSvc, logger, cfg.SecretsEncryptionKey, cfg.Environment)

	// Functions (FaaS)
	fnSvc := services.NewFunctionService(repos.Function, compute, fileStore, auditSvc, logger)

	// Cache (Redis/Memcached)
	cacheSvc := services.NewCacheService(repos.Cache, compute, repos.Vpc, eventSvc, auditSvc, logger)

	// Queue (SQS-like)
	queueSvc := services.NewQueueService(repos.Queue, eventSvc, auditSvc)

	// Notify (SNS-like)
	notifySvc := services.NewNotifyService(repos.Notify, queueSvc, eventSvc, auditSvc, logger)

	// Cron (Scheduled Tasks)
	cronSvc := services.NewCronService(repos.Cron, eventSvc, auditSvc)
	cronWorker := services.NewCronWorker(repos.Cron)

	// Gateway
	gwSvc := services.NewGatewayService(repos.Gateway, auditSvc)

	// Container (K8s-lite)
	containerSvc := services.NewContainerService(repos.Container, eventSvc, auditSvc)
	containerWorker := services.NewContainerWorker(repos.Container, instanceSvc, eventSvc)

	// Health
	healthSvc := services.NewHealthServiceImpl(db, compute)

	// AutoScaling
	asgSvc := services.NewAutoScalingService(repos.AutoScaling, repos.Vpc, auditSvc)
	asgWorker := services.NewAutoScalingWorker(repos.AutoScaling, instanceSvc, lbSvc, eventSvc, ports.RealClock{})

	svcs := &Services{
		Audit: auditSvc, Identity: identitySvc, Auth: authSvc, PasswordReset: pwdResetSvc,
		RBAC: rbacSvc, Vpc: vpcSvc, Subnet: subnetSvc, Event: eventSvc, Volume: volumeSvc,
		Instance: instanceSvc, SecurityGroup: sgSvc, LB: lbSvc, Dashboard: dashboardSvc,
		Snapshot: snapshotSvc, Stack: stackSvc, Storage: storageSvc, Database: databaseSvc,
		Secret: secretSvc, Function: fnSvc, Cache: cacheSvc, Queue: queueSvc, Notify: notifySvc,
		Cron: cronSvc, Gateway: gwSvc, Container: containerSvc, Health: healthSvc,
		AutoScaling: asgSvc,
	}

	workers := &Workers{
		LB: lbWorker, AutoScaling: asgWorker, Cron: cronWorker, Container: containerWorker,
	}

	return svcs, workers, nil
}
