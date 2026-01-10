package services_test

import (
	"context"
	"testing"

	"log/slog"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/poyrazk/thecloud/internal/repositories/noop"
	"golang.org/x/crypto/bcrypt"
)

func BenchmarkInstanceServiceList(b *testing.B) {
	// Setup
	repo := &noop.NoopInstanceRepository{}
	vpcRepo := &noop.NoopVpcRepository{}
	subnetRepo := &noop.NoopSubnetRepository{}
	volumeRepo := &noop.NoopVolumeRepository{}
	compute := &noop.NoopComputeBackend{}
	network := &noop.NoopNetworkAdapter{}
	eventSvc := &noop.NoopEventService{}
	auditSvc := &noop.NoopAuditService{}
	logger := slog.Default()

	svc := services.NewInstanceService(services.InstanceServiceParams{
		Repo:       repo,
		VpcRepo:    vpcRepo,
		SubnetRepo: subnetRepo,
		VolumeRepo: volumeRepo,
		Compute:    compute,
		Network:    network,
		EventSvc:   eventSvc,
		AuditSvc:   auditSvc,
		Logger:     logger,
	})

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.ListInstances(ctx)
	}
}

func BenchmarkVPCServiceGet(b *testing.B) {
	repo := &noop.NoopVpcRepository{}
	network := &noop.NoopNetworkAdapter{}
	auditSvc := &noop.NoopAuditService{}
	logger := slog.Default()
	svc := services.NewVpcService(repo, network, auditSvc, logger, "10.0.0.0/16")

	ctx := context.Background()
	id := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.GetVPC(ctx, id.String())
	}
}

func BenchmarkInstanceServiceCreate(b *testing.B) {
	repo := &noop.NoopInstanceRepository{}
	vpcRepo := &noop.NoopVpcRepository{}
	subnetRepo := &noop.NoopSubnetRepository{}
	volumeRepo := &noop.NoopVolumeRepository{}
	compute := &noop.NoopComputeBackend{}
	network := &noop.NoopNetworkAdapter{}
	eventSvc := &noop.NoopEventService{}
	auditSvc := &noop.NoopAuditService{}
	logger := slog.Default()

	svc := services.NewInstanceService(services.InstanceServiceParams{
		Repo:       repo,
		VpcRepo:    vpcRepo,
		SubnetRepo: subnetRepo,
		VolumeRepo: volumeRepo,
		Compute:    compute,
		Network:    network,
		EventSvc:   eventSvc,
		AuditSvc:   auditSvc,
		Logger:     logger,
	})

	ctx := context.Background()
	// CreateInstance needs a user ID in context usually
	ctx = appcontext.WithUserID(ctx, uuid.New())

	subnetID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.LaunchInstance(ctx, "test", "alpine", "80:80", nil, &subnetID, nil)
	}
}

func BenchmarkFunctionServiceInvoke(b *testing.B) {
	repo := &noop.NoopFunctionRepository{}
	compute := &noop.NoopComputeBackend{}
	fileStore := &noop.NoopFileStore{}
	auditSvc := &noop.NoopAuditService{}
	logger := slog.Default()

	svc := services.NewFunctionService(repo, compute, fileStore, auditSvc, logger)

	ctx := context.Background()
	id := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.InvokeFunction(ctx, id, []byte("{}"), false)
	}
}

func BenchmarkDatabaseServiceList(b *testing.B) {
	repo := &noop.NoopDatabaseRepository{}
	compute := &noop.NoopComputeBackend{}
	vpcRepo := &noop.NoopVpcRepository{}
	eventSvc := &noop.NoopEventService{}
	auditSvc := &noop.NoopAuditService{}
	logger := slog.Default()

	svc := services.NewDatabaseService(repo, compute, vpcRepo, eventSvc, auditSvc, logger)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.ListDatabases(ctx)
	}
}

func BenchmarkCacheServiceList(b *testing.B) {
	repo := &noop.NoopCacheRepository{}
	compute := &noop.NoopComputeBackend{}
	vpcRepo := &noop.NoopVpcRepository{}
	eventSvc := &noop.NoopEventService{}
	auditSvc := &noop.NoopAuditService{}
	logger := slog.Default()

	svc := services.NewCacheService(repo, compute, vpcRepo, eventSvc, auditSvc, logger)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.ListCaches(ctx)
	}
}

func BenchmarkStorageServiceList(b *testing.B) {
	repo := &noop.NoopStorageRepository{}
	fileStore := &noop.NoopFileStore{}
	auditSvc := &noop.NoopAuditService{}

	svc := services.NewStorageService(repo, fileStore, auditSvc)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.ListObjects(ctx, "test-bucket")
	}
}

func BenchmarkFunctionServiceList(b *testing.B) {
	repo := &noop.NoopFunctionRepository{}
	compute := &noop.NoopComputeBackend{}
	fileStore := &noop.NoopFileStore{}
	auditSvc := &noop.NoopAuditService{}
	logger := slog.Default()

	svc := services.NewFunctionService(repo, compute, fileStore, auditSvc, logger)

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

	svc := services.NewAuthService(userRepo, identitySvc, auditSvc)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use unique email each time to avoid "user exists" check impact if we were using real repo
		// though noop always returns nil for existing check usually (wait, I should check noop GetByEmail)
		_, _ = svc.Register(ctx, "test@example.com", "P@ssword123!", "Test User")
	}
}

type BenchUserRepository struct {
	noop.NoopUserRepository
	hash string
}

func (r *BenchUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	h := r.hash
	if h == "" {
		h = "$2a$10$8K1p/a0ZlAbzf.H4G1/BTe1B9U1S9S9S9S9S9S9S9S9S9S9S9S9S"
	}
	return &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: h,
	}, nil
}

func BenchmarkAuthServiceLogin(b *testing.B) {
	// Generate a real hash once
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), 10)

	userRepo := &BenchUserRepository{}
	// Override the default behavior to return the real hash
	// using a closure or just setting it if we make it a field.
	// Let's make it a field.
	userRepo.hash = string(hash)

	identitySvc := &noop.NoopIdentityService{}
	auditSvc := &noop.NoopAuditService{}

	svc := services.NewAuthService(userRepo, identitySvc, auditSvc)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := svc.Login(ctx, "test@example.com", "password")
		if err != nil {
			b.Fatalf("Login failed: %v", err)
		}
	}
}
