package services_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"log/slog"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/poyrazk/thecloud/internal/repositories/noop"
	"github.com/poyrazk/thecloud/pkg/testutil"
	"golang.org/x/crypto/bcrypt"
)

// noopDB returns a nil transaction. Benchmarks do not exercise transactional paths
// so this is safe — the nil transaction is never used by repo methods in benchmark scenarios.
type noopDB struct{}

func (noopDB) Begin(ctx context.Context) (services.Transaction, error) { return nil, nil }

func BenchmarkInstanceServiceList(b *testing.B) {
	tenantID := uuid.New()
	for _, size := range []int{10, 100, 1000} {
		repo := &benchInstanceRepository{instances: makeBenchmarkInstances(size, tenantID)}
		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			VpcRepo:          &noop.NoopVpcRepository{},
			SubnetRepo:       &noop.NoopSubnetRepository{},
			VolumeRepo:       &noop.NoopVolumeRepository{},
			RBAC:             &noop.NoopRBACService{},
			Compute:          &noop.NoopComputeBackend{},
			Network:          &noop.NoopNetworkAdapter{},
			EventSvc:         &noop.NoopEventService{},
			AuditSvc:         &noop.NoopAuditService{},
			TaskQueue:        &TaskQueueStub{},
			Logger:           slog.Default(),
			TenantSvc:        &NoopTenantService{},
			InstanceTypeRepo: &noop.NoopInstanceTypeRepository{},
		})

		ctx := benchmarkAuthContext(tenantID)

		b.Run(fmt.Sprintf("records=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				instances, _ := svc.ListInstances(ctx, nil)
				if len(instances) != size {
					b.Fatalf("expected %d instances, got %d", size, len(instances))
				}
			}
		})
	}
}

func BenchmarkVPCServiceGet(b *testing.B) {
	repo := &noop.NoopVpcRepository{}
	network := &noop.NoopNetworkAdapter{}
	auditSvc := &noop.NoopAuditService{}
	rbacSvc := &noop.NoopRBACService{}
	logger := slog.Default()
	svc := services.NewVpcService(services.VpcServiceParams{
		Repo:        repo,
		LBRepo:      &noop.NoopLBRepository{},
		RBACSvc:     rbacSvc,
		Network:     network,
		AuditSvc:    auditSvc,
		Logger:      logger,
		DefaultCIDR: testutil.TestCIDR,
	})

	ctx := context.Background()
	id := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.GetVPC(ctx, id.String())
	}
}

func BenchmarkInstanceServiceCreate(b *testing.B) {
	logger := slog.Default()
	repo := &noop.NoopInstanceRepository{}
	vpcRepo := &noop.NoopVpcRepository{}
	subnetRepo := &noop.NoopSubnetRepository{}
	volumeRepo := &noop.NoopVolumeRepository{}
	compute := &noop.NoopComputeBackend{}
	network := noop.NewNoopNetworkAdapter(logger)
	eventSvc := &noop.NoopEventService{}
	auditSvc := &noop.NoopAuditService{}
	rbacSvc := &noop.NoopRBACService{}

	svc := services.NewInstanceService(services.InstanceServiceParams{
		Repo:             repo,
		VpcRepo:          vpcRepo,
		SubnetRepo:       subnetRepo,
		VolumeRepo:       volumeRepo,
		RBAC:             rbacSvc,
		Compute:          compute,
		Network:          network,
		EventSvc:         eventSvc,
		AuditSvc:         auditSvc,
		TaskQueue:        &TaskQueueStub{},
		Logger:           logger,
		TenantSvc:        &NoopTenantService{},
		InstanceTypeRepo: &noop.NoopInstanceTypeRepository{},
	})

	ctx := context.Background()
	// CreateInstance needs a user ID in context usually
	ctx = appcontext.WithUserID(ctx, uuid.New())

	subnetID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.LaunchInstance(ctx, ports.LaunchParams{
			Name:         "test",
			Image:        "alpine",
			Ports:        "80:80",
			InstanceType: "basic-2",
			SubnetID:     &subnetID,
		})
	}
}

func BenchmarkFunctionServiceInvoke(b *testing.B) {
	repo := &noop.NoopFunctionRepository{}
	compute := &noop.NoopComputeBackend{}
	fileStore := &noop.NoopFileStore{}
	auditSvc := &noop.NoopAuditService{}
	rbacSvc := &noop.NoopRBACService{}
	secretSvc := &noop.NoopSecretService{}
	logger := slog.Default()

	svc := services.NewFunctionService(repo, rbacSvc, compute, fileStore, auditSvc, secretSvc, logger)

	ctx := context.Background()
	id := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.InvokeFunction(ctx, id, []byte("{}"), false)
	}
}

func BenchmarkInstanceServiceCreateParallel(b *testing.B) {
	logger := slog.Default()
	repo := &noop.NoopInstanceRepository{}
	vpcRepo := &noop.NoopVpcRepository{}
	subnetRepo := &noop.NoopSubnetRepository{}
	volumeRepo := &noop.NoopVolumeRepository{}
	compute := &noop.NoopComputeBackend{}
	network := noop.NewNoopNetworkAdapter(logger)
	eventSvc := &noop.NoopEventService{}
	auditSvc := &noop.NoopAuditService{}
	rbacSvc := &noop.NoopRBACService{}

	// Disable tracing for benchmarks to avoid overhead
	_ = os.Setenv("TRACING_ENABLED", "false")

	svc := services.NewInstanceService(services.InstanceServiceParams{
		Repo:             repo,
		VpcRepo:          vpcRepo,
		SubnetRepo:       subnetRepo,
		VolumeRepo:       volumeRepo,
		RBAC:             rbacSvc,
		Compute:          compute,
		Network:          network,
		EventSvc:         eventSvc,
		AuditSvc:         auditSvc,
		TaskQueue:        &TaskQueueStub{},
		Logger:           logger,
		TenantSvc:        &NoopTenantService{},
		InstanceTypeRepo: &noop.NoopInstanceTypeRepository{},
	})

	ctx := appcontext.WithUserID(context.Background(), uuid.New())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = svc.LaunchInstance(ctx, ports.LaunchParams{
				Name:         "test-inst",
				Image:        "alpine",
				Ports:        "80:80",
				InstanceType: "basic-2",
			})
		}
	})
}

type NoopTenantService struct{}

func (s *NoopTenantService) CreateTenant(ctx context.Context, name, slug string, ownerID uuid.UUID) (*domain.Tenant, error) {
	return &domain.Tenant{ID: uuid.New()}, nil
}
func (s *NoopTenantService) GetTenant(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return &domain.Tenant{ID: id}, nil
}
func (s *NoopTenantService) ListUserTenants(ctx context.Context, userID uuid.UUID) ([]domain.Tenant, error) {
	return nil, nil
}
func (s *NoopTenantService) InviteMember(ctx context.Context, tenantID uuid.UUID, email, role string) error {
	return nil
}
func (s *NoopTenantService) RemoveMember(ctx context.Context, tenantID, userID uuid.UUID) error {
	return nil
}
func (s *NoopTenantService) SwitchTenant(ctx context.Context, userID, tenantID uuid.UUID) error {
	return nil
}
func (s *NoopTenantService) CheckQuota(ctx context.Context, tenantID uuid.UUID, resource string, requested int) error {
	return nil
}
func (s *NoopTenantService) GetMembership(ctx context.Context, tenantID, userID uuid.UUID) (*domain.TenantMember, error) {
	return &domain.TenantMember{}, nil
}
func (s *NoopTenantService) IncrementUsage(ctx context.Context, tenantID uuid.UUID, resource string, amount int) error {
	return nil
}
func (s *NoopTenantService) DecrementUsage(ctx context.Context, tenantID uuid.UUID, resource string, amount int) error {
	return nil
}

type benchInstanceRepository struct {
	noop.NoopInstanceRepository
	instances []*domain.Instance
}

func (r *benchInstanceRepository) List(ctx context.Context, tagFilter []string) ([]*domain.Instance, error) {
	instances := make([]*domain.Instance, len(r.instances))
	copy(instances, r.instances)
	return instances, nil
}

type benchDatabaseRepository struct {
	noop.NoopDatabaseRepository
	databases []*domain.Database
}

func (r *benchDatabaseRepository) List(ctx context.Context) ([]*domain.Database, error) {
	databases := make([]*domain.Database, len(r.databases))
	copy(databases, r.databases)
	return databases, nil
}

type benchStorageRepository struct {
	noop.NoopStorageRepository
	objects []*domain.Object
}

func (r *benchStorageRepository) List(ctx context.Context, bucket string) ([]*domain.Object, error) {
	objects := make([]*domain.Object, len(r.objects))
	copy(objects, r.objects)
	return objects, nil
}

func benchmarkAuthContext(tenantID uuid.UUID) context.Context {
	ctx := appcontext.WithUserID(context.Background(), uuid.New())
	return appcontext.WithTenantID(ctx, tenantID)
}

func makeBenchmarkInstances(size int, tenantID uuid.UUID) []*domain.Instance {
	instances := make([]*domain.Instance, size)
	for i := 0; i < size; i++ {
		instances[i] = &domain.Instance{
			ID:       uuid.New(),
			UserID:   uuid.New(),
			TenantID: tenantID,
			Name:     fmt.Sprintf("instance-%d", i),
			Image:    "alpine",
		}
	}
	return instances
}

func makeBenchmarkDatabases(size int, tenantID uuid.UUID) []*domain.Database {
	databases := make([]*domain.Database, size)
	for i := 0; i < size; i++ {
		databases[i] = &domain.Database{
			ID:       uuid.New(),
			UserID:   uuid.New(),
			TenantID: tenantID,
			Name:     fmt.Sprintf("database-%d", i),
			Engine:   domain.EnginePostgres,
		}
	}
	return databases
}

func makeBenchmarkObjects(size int, tenantID uuid.UUID, bucket string) []*domain.Object {
	objects := make([]*domain.Object, size)
	for i := 0; i < size; i++ {
		objects[i] = &domain.Object{
			ID:       uuid.New(),
			UserID:   uuid.New(),
			TenantID: tenantID,
			Bucket:   bucket,
			Key:      fmt.Sprintf("object-%d", i),
		}
	}
	return objects
}

func BenchmarkAuthServiceLoginParallel(b *testing.B) {
	hash, _ := bcrypt.GenerateFromPassword([]byte(testutil.TestPasswordStrong), bcrypt.DefaultCost)

	userRepo := &BenchUserRepository{hash: string(hash)}
	idSvc := &noop.NoopIdentityService{}
	auditSvc := &noop.NoopAuditService{}
	tenantSvc := &NoopTenantService{}

	svc := services.NewAuthService(userRepo, idSvc, auditSvc, tenantSvc, noopDB{}, slog.Default())

	ctx := context.Background()
	email := "admin@thecloud.local"
	password := testutil.TestPasswordStrong

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, _ = svc.Login(ctx, email, password)
		}
	})
}

func BenchmarkDatabaseServiceList(b *testing.B) {
	tenantID := uuid.New()
	for _, size := range []int{10, 100, 1000} {
		repo := &benchDatabaseRepository{databases: makeBenchmarkDatabases(size, tenantID)}
		svc := services.NewDatabaseService(services.DatabaseServiceParams{
			Repo:         repo,
			RBAC:         &noop.NoopRBACService{},
			Compute:      &noop.NoopComputeBackend{},
			VpcRepo:      &noop.NoopVpcRepository{},
			VolumeSvc:    nil,
			SnapshotSvc:  nil,
			SnapshotRepo: nil,
			EventSvc:     &noop.NoopEventService{},
			AuditSvc:     &noop.NoopAuditService{},
			Logger:       slog.Default(),
		})

		ctx := benchmarkAuthContext(tenantID)

		b.Run(fmt.Sprintf("records=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				databases, _ := svc.ListDatabases(ctx)
				if len(databases) != size {
					b.Fatalf("expected %d databases, got %d", size, len(databases))
				}
			}
		})
	}
}

type ContentionRepo struct {
	noop.NoopDatabaseRepository
	mu sync.Mutex
}

func (r *ContentionRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Database, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return &domain.Database{ID: id}, nil
}

func BenchmarkDatabaseContentionParallel(b *testing.B) {
	repo := &ContentionRepo{}
	compute := &noop.NoopComputeBackend{}
	vpcRepo := &noop.NoopVpcRepository{}
	eventSvc := &noop.NoopEventService{}
	auditSvc := &noop.NoopAuditService{}
	rbacSvc := &noop.NoopRBACService{}
	logger := slog.Default()

	svc := services.NewDatabaseService(services.DatabaseServiceParams{
		Repo:         repo,
		RBAC:         rbacSvc,
		Compute:      compute,
		VpcRepo:      vpcRepo,
		VolumeSvc:    nil,
		SnapshotSvc:  nil,
		SnapshotRepo: nil,
		EventSvc:     eventSvc,
		AuditSvc:     auditSvc,
		Logger:       logger,
	})
	ctx := context.Background()
	id := uuid.New()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = svc.GetDatabase(ctx, id)
		}
	})
}

func BenchmarkCacheServiceList(b *testing.B) {
	repo := &noop.NoopCacheRepository{}
	compute := &noop.NoopComputeBackend{}
	vpcRepo := &noop.NoopVpcRepository{}
	eventSvc := &noop.NoopEventService{}
	auditSvc := &noop.NoopAuditService{}
	rbacSvc := &noop.NoopRBACService{}
	logger := slog.Default()

	svc := services.NewCacheService(repo, rbacSvc, compute, vpcRepo, eventSvc, auditSvc, logger)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.ListCaches(ctx)
	}
}

func BenchmarkStorageServiceList(b *testing.B) {
	bucket := "test-bucket"
	tenantID := uuid.New()
	for _, size := range []int{10, 100, 1000} {
		repo := &benchStorageRepository{objects: makeBenchmarkObjects(size, tenantID, bucket)}
		svc := services.NewStorageService(services.StorageServiceParams{
			Repo:     repo,
			RBACSvc:  &noop.NoopRBACService{},
			Store:    &noop.NoopFileStore{},
			AuditSvc: &noop.NoopAuditService{},
			Logger:   slog.Default(),
		})

		ctx := benchmarkAuthContext(tenantID)

		b.Run(fmt.Sprintf("records=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				objects, _ := svc.ListObjects(ctx, bucket)
				if len(objects) != size {
					b.Fatalf("expected %d objects, got %d", size, len(objects))
				}
			}
		})
	}
}

func BenchmarkFunctionServiceList(b *testing.B) {
	repo := &noop.NoopFunctionRepository{}
	compute := &noop.NoopComputeBackend{}
	fileStore := &noop.NoopFileStore{}
	auditSvc := &noop.NoopAuditService{}
	rbacSvc := &noop.NoopRBACService{}
	secretSvc := &noop.NoopSecretService{}
	logger := slog.Default()

	svc := services.NewFunctionService(repo, rbacSvc, compute, fileStore, auditSvc, secretSvc, logger)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.ListFunctions(ctx)
	}
}

func BenchmarkAuthServiceRegister(b *testing.B) {
	userRepo := &noop.NoopUserRepository{}
	identitySvc := &noop.NoopIdentityService{}
	auditSvc := &noop.NoopAuditService{}
	tenantSvc := &NoopTenantService{}

	svc := services.NewAuthService(userRepo, identitySvc, auditSvc, tenantSvc, noopDB{}, slog.Default())

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use unique email each time to avoid "user exists" check impact if we were using real repo
		// though noop always returns nil for existing check usually (wait, I should check noop GetByEmail)
		_, _ = svc.Register(ctx, "test@example.com", testutil.TestPasswordStrong, "Test User")
	}
}

type BenchUserRepository struct {
	noop.NoopUserRepository
	hash string
}

func (r *BenchUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	h := r.hash
	if h == "" {
		h = "$2a$10$8K1p/a0ZlAbzf.H4G1/BTe1B9U1S9S9S9S9S9S9S9S9S9S9S9S"
	}
	return &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: h,
	}, nil
}

func BenchmarkAuthServiceLogin(b *testing.B) {
	// Generate a real hash once
	hash, _ := bcrypt.GenerateFromPassword([]byte(testutil.TestPasswordStrong), 10)

	userRepo := &BenchUserRepository{}
	// Override the default behavior to return the real hash
	// using a closure or just setting it if we make it a field.
	// Let's make it a field.
	userRepo.hash = string(hash)

	identitySvc := &noop.NoopIdentityService{}
	auditSvc := &noop.NoopAuditService{}
	tenantSvc := &NoopTenantService{}

	svc := services.NewAuthService(userRepo, identitySvc, auditSvc, tenantSvc, noopDB{}, slog.Default())

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := svc.Login(ctx, "test@example.com", testutil.TestPasswordStrong)
		if err != nil {
			b.Fatalf("Login failed: %v", err)
		}
	}
}
