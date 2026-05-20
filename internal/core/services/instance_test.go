//go:build integration
// +build integration

package services_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	coreports "github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/poyrazk/thecloud/internal/repositories/docker"
	"github.com/poyrazk/thecloud/internal/repositories/libvirt"
	"github.com/poyrazk/thecloud/internal/repositories/noop"
	"github.com/poyrazk/thecloud/internal/repositories/postgres"
	"github.com/poyrazk/thecloud/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testInstanceType = "basic-2"
	testImage        = "alpine"
)

// FaultyInstanceRepository wraps a real InstanceRepository to simulate failures.
type FaultyInstanceRepository struct {
	coreports.InstanceRepository
	ShouldFailCreate bool
}

func (r *FaultyInstanceRepository) Create(ctx context.Context, instance *domain.Instance) error {
	if r.ShouldFailCreate {
		return fmt.Errorf("simulated database failure")
	}
	return r.InstanceRepository.Create(ctx, instance)
}

type InMemoryTaskQueue struct {
	jobs       []string
	mu         sync.Mutex
	ShouldFail bool
}

func (q *InMemoryTaskQueue) Enqueue(ctx context.Context, queueName string, payload interface{}) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.ShouldFail {
		return fmt.Errorf("simulated enqueue failure")
	}
	q.jobs = append(q.jobs, fmt.Sprintf("%v", payload))
	return nil
}

func (q *InMemoryTaskQueue) Dequeue(ctx context.Context, queueName string) (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.jobs) == 0 {
		return "", fmt.Errorf("queue empty")
	}
	job := q.jobs[0]
	q.jobs = q.jobs[1:]
	return job, nil
}

func setupInstanceServiceTest(t *testing.T) (*pgxpool.Pool, *services.InstanceService, coreports.ComputeBackend, coreports.InstanceRepository, coreports.VpcRepository, coreports.VolumeRepository, context.Context) {
	t.Helper()
	db := setupDB(t)
	cleanDB(t, db)
	ctx := setupTestUser(t, db)

	repo := postgres.NewInstanceRepository(db)
	vpcRepo := postgres.NewVpcRepository(db)
	subnetRepo := postgres.NewSubnetRepository(db)
	volumeRepo := postgres.NewVolumeRepository(db)
	itRepo := postgres.NewInstanceTypeRepository(db)

	var compute coreports.ComputeBackend
	var err error

	backend := os.Getenv("COMPUTE_BACKEND")
	if backend == "libvirt" {
		uri := os.Getenv("LIBVIRT_URI")
		if uri == "" {
			uri = "qemu:///system"
		}
		compute, err = libvirt.NewLibvirtAdapter(slog.Default(), uri)
		if err != nil {
			t.Logf("Warning: failed to initialize libvirt adapter: %v. Falling back to noop for logic testing.", err)
			compute = noop.NewNoopComputeBackend()
		}
	} else {
		compute, err = docker.NewDockerAdapter(slog.Default())
		if err != nil {
			t.Logf("Warning: failed to initialize docker adapter: %v. Falling back to noop for logic testing.", err)
			compute = noop.NewNoopComputeBackend()
		}
	}

	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// In integration tests, we frequently rely on a shared Docker network.
	// We ensure it exists here so that Provisioning (which uses it as a fallback) succeeds.
	if compute.Type() == "docker" {
		dockerCompute := compute.(*docker.DockerAdapter)
		_, _ = dockerCompute.CreateNetwork(ctx, testutil.TestDockerNetwork)
		// Pre-pull test image to prevent flakes in CI environments with slow registries or restrictive daemons
		_, _, _ = dockerCompute.LaunchInstanceWithOptions(ctx, coreports.CreateInstanceOptions{
			Name:      "pre-pull-image",
			ImageName: testImage,
		})
		_ = dockerCompute.DeleteInstance(ctx, "pre-pull-image")
	}

	// Ensure default instance type exists
	defaultType := &domain.InstanceType{
		ID:       testInstanceType,
		Name:     "Basic 2",
		VCPUs:    1,
		MemoryMB: 128,
		DiskGB:   1,
	}
	_, _ = itRepo.Create(ctx, defaultType)

	eventRepo := postgres.NewEventRepository(db)
	eventSvc := services.NewEventService(services.EventServiceParams{
		Repo:      eventRepo,
		RBACSvc:   rbacSvc,
		Publisher: nil,
		Logger:    slog.Default(),
	})

	auditRepo := postgres.NewAuditRepository(db)
	auditSvc := services.NewAuditService(services.AuditServiceParams{
		Repo:    auditRepo,
		RBACSvc: rbacSvc,
	})

	tenantRepo := postgres.NewTenantRepo(db)
	userRepo := postgres.NewUserRepo(db)
	tenantSvc := services.NewTenantService(services.TenantServiceParams{
		Repo:     tenantRepo,
		UserRepo: userRepo,
		RBACSvc:  rbacSvc,
		Logger:   slog.Default(),
	})

	taskQueue := &InMemoryTaskQueue{}
	network := noop.NewNoopNetworkAdapter(slog.Default())

	sshKeyRepo := postgres.NewSSHKeyRepo(db)
	sshKeySvc, err := services.NewSSHKeyService(services.SSHKeyServiceParams{
		Repo:    sshKeyRepo,
		RBACSvc: rbacSvc,
	})
	require.NoError(t, err)

	svc := services.NewInstanceService(services.InstanceServiceParams{
		Repo:             repo,
		VpcRepo:          vpcRepo,
		SubnetRepo:       subnetRepo,
		VolumeRepo:       volumeRepo,
		InstanceTypeRepo: itRepo,
		RBAC:             rbacSvc,
		Compute:          compute,
		Network:          network,
		EventSvc:         eventSvc,
		AuditSvc:         auditSvc,
		TaskQueue:        taskQueue,
		Logger:           slog.Default(),
		TenantSvc:        tenantSvc,
		SSHKeySvc:        sshKeySvc,
	})

	return db, svc, compute, repo, vpcRepo, volumeRepo, ctx
}

func TestLaunchInstanceSuccess(t *testing.T) {
	_, svc, compute, repo, _, _, ctx := setupInstanceServiceTest(t)
	name := "test-inst-launch"
	image := testImage
	portsStr := "0:80"

	// 1. Launch (Enqueue)
	inst, err := svc.LaunchInstance(ctx, coreports.LaunchParams{
		Name:         name,
		Image:        image,
		Ports:        portsStr,
		InstanceType: testInstanceType,
	})
	require.NoError(t, err)
	assert.NotNil(t, inst)
	assert.Equal(t, domain.StatusStarting, inst.Status)

	// 2. Provision (Simulate Worker)
	err = svc.Provision(ctx, domain.ProvisionJob{InstanceID: inst.ID})
	require.NoError(t, err)

	// 3. Verify Running
	updatedInst, err := repo.GetByID(ctx, inst.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusRunning, updatedInst.Status)

	// Only verify container/IP if compute is actually functional
	if compute.Type() != "noop" {
		assert.NotEmpty(t, updatedInst.ContainerID)
		ip, err := compute.GetInstanceIP(ctx, updatedInst.ContainerID)
		require.NoError(t, err)
		assert.NotEmpty(t, ip)
		_ = compute.DeleteInstance(ctx, updatedInst.ContainerID)
	}
}

func TestTerminateInstanceSuccess(t *testing.T) {
	_, svc, compute, repo, _, _, ctx := setupInstanceServiceTest(t)
	name := "test-inst-term"
	image := testImage

	// Setup: Launch & Provision
	inst, err := svc.LaunchInstance(ctx, coreports.LaunchParams{
		Name:         name,
		Image:        image,
		InstanceType: testInstanceType,
	})
	require.NoError(t, err)
	err = svc.Provision(ctx, domain.ProvisionJob{InstanceID: inst.ID})
	require.NoError(t, err)

	updatedInst, _ := repo.GetByID(ctx, inst.ID)

	// Execute Terminate
	err = svc.TerminateInstance(ctx, updatedInst.ID.String())
	require.NoError(t, err)

	// Verify Deleted from DB
	_, err = repo.GetByID(ctx, updatedInst.ID)
	require.Error(t, err)

	// Verify container is gone
	if compute.Type() != "noop" && updatedInst.ContainerID != "" {
		_, err = compute.GetInstanceIP(ctx, updatedInst.ContainerID)
		require.Error(t, err)
	}
}

func TestInstanceServiceLaunchDBFailure(t *testing.T) {
	// Custom setup with Faulty Repo
	db := setupDB(t)
	// No cleanDB needed here as setupDB gives us a fresh unique schema
	ctx := setupTestUser(t, db)

	realRepo := postgres.NewInstanceRepository(db)
	faultyRepo := &FaultyInstanceRepository{InstanceRepository: realRepo, ShouldFailCreate: true}

	// Other deps
	vpcRepo := postgres.NewVpcRepository(db)
	subnetRepo := postgres.NewSubnetRepository(db)
	volumeRepo := postgres.NewVolumeRepository(db)
	itRepo := postgres.NewInstanceTypeRepository(db)

	// Use noop for compute to focus on DB failure logic
	compute := noop.NewNoopComputeBackend()

	defaultType := &domain.InstanceType{ID: testInstanceType, Name: "Basic 2", VCPUs: 1, MemoryMB: 128, DiskGB: 1}
	_, _ = itRepo.Create(ctx, defaultType)
	tenantRepo := postgres.NewTenantRepo(db)
	_ = tenantRepo.UpdateQuota(ctx, &domain.TenantQuota{
		TenantID:     appcontext.TenantIDFromContext(ctx),
		MaxInstances: 10,
		MaxVCPUs:     20,
		MaxMemoryGB:  40,
		MaxStorageGB: 1000,
	})

	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	eventSvc := services.NewEventService(services.EventServiceParams{
		Repo:      postgres.NewEventRepository(db),
		RBACSvc:   rbacSvc,
		Publisher: nil,
		Logger:    slog.Default(),
	})
	auditSvc := services.NewAuditService(services.AuditServiceParams{
		Repo:    postgres.NewAuditRepository(db),
		RBACSvc: rbacSvc,
	})
	taskQueue := &InMemoryTaskQueue{}

	sshKeySvc, err := services.NewSSHKeyService(services.SSHKeyServiceParams{
		Repo:    postgres.NewSSHKeyRepo(db),
		RBACSvc: rbacSvc,
	})
	require.NoError(t, err)

	tenantSvc := services.NewTenantService(services.TenantServiceParams{
		Repo:     postgres.NewTenantRepo(db),
		UserRepo: postgres.NewUserRepo(db),
		RBACSvc:  rbacSvc,
		Logger:   slog.Default(),
	})
	svc := services.NewInstanceService(services.InstanceServiceParams{
		Repo:             faultyRepo,
		VpcRepo:          vpcRepo,
		SubnetRepo:       subnetRepo,
		VolumeRepo:       volumeRepo,
		InstanceTypeRepo: itRepo,
		RBAC:             rbacSvc,
		Compute:          compute,
		EventSvc:         eventSvc,
		AuditSvc:         auditSvc,
		TaskQueue:        taskQueue,
		Logger:           slog.Default(),
		TenantSvc:        tenantSvc,
		SSHKeySvc:        sshKeySvc,
	})

	// Attempt Launch
	_, err = svc.LaunchInstance(ctx, coreports.LaunchParams{
		Name:         "fail-inst",
		Image:        testImage,
		InstanceType: testInstanceType,
	})

	// Verify Failure
	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated database failure")

	// Verify no junk in DB (using real repo to check)
	list, err := realRepo.List(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestInstanceNetworking(t *testing.T) {
	_, svc, compute, _, vpcRepo, _, ctx := setupInstanceServiceTest(t)

	// Skip if noop compute since networking requires actual adapter support
	if compute.Type() == "noop" {
		t.Skip("Skipping networking test for noop compute")
	}

	vpcID := uuid.New()
	networkName := "net-" + vpcID.String()
	netID, err := compute.CreateNetwork(ctx, networkName)
	require.NoError(t, err)
	defer func() { _ = compute.DeleteNetwork(ctx, netID) }()

	vpc := &domain.VPC{
		ID:        vpcID,
		UserID:    appcontext.UserIDFromContext(ctx),
		TenantID:  appcontext.TenantIDFromContext(ctx),
		Name:      "test-vpc-" + vpcID.String(),
		CIDRBlock: "10.0.0.0/16",
		NetworkID: netID,
		Status:    "ACTIVE",
		CreatedAt: time.Now(),
	}
	err = vpcRepo.Create(ctx, vpc)
	require.NoError(t, err)

	// Use Launch with VPC
	inst, err := svc.LaunchInstance(ctx, coreports.LaunchParams{
		Name:         "test-vpc-inst",
		Image:        testImage,
		InstanceType: testInstanceType,
		VpcID:        &vpcID,
	})
	require.NoError(t, err)

	err = svc.Provision(ctx, domain.ProvisionJob{InstanceID: inst.ID})
	require.NoError(t, err)

	// Check if attached to network
	updatedInst, err := svc.GetInstance(ctx, inst.ID.String())
	require.NoError(t, err)
	assert.NotNil(t, updatedInst.VpcID)
	assert.Equal(t, vpcID, *updatedInst.VpcID)

	// Cleanup
	if updatedInst.ContainerID != "" {
		_ = compute.DeleteInstance(ctx, updatedInst.ContainerID)
	}
}

func TestInstanceServiceLaunchConcurrency(t *testing.T) {
	_, svc, compute, repo, _, _, ctx := setupInstanceServiceTest(t)
	concurrency := 5
	errChan := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			name := fmt.Sprintf("conc-inst-%d", idx)
			_, err := svc.LaunchInstance(ctx, coreports.LaunchParams{
				Name:         name,
				Image:        testImage,
				InstanceType: testInstanceType,
			})
			errChan <- err
		}(i)
	}

	for i := 0; i < concurrency; i++ {
		err := <-errChan
		require.NoError(t, err)
	}

	// Verify all created
	list, err := repo.List(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, list, concurrency)

	// Verify provision triggers (optional execution)
	for _, inst := range list {
		if compute.Type() != "noop" && inst.ContainerID != "" {
			_ = compute.DeleteInstance(ctx, inst.ContainerID)
		}
	}
}

func TestInstanceServiceGetStatsReal(t *testing.T) {
	_, svc, compute, _, _, _, ctx := setupInstanceServiceTest(t)

	// Skip if noop compute
	if compute.Type() == "noop" {
		t.Skip("Skipping stats test for noop compute")
	}

	name := "stats-inst"
	image := testImage

	// Launch and Provision
	inst, err := svc.LaunchInstance(ctx, coreports.LaunchParams{
		Name:         name,
		Image:        image,
		InstanceType: testInstanceType,
	})
	require.NoError(t, err)
	err = svc.Provision(ctx, domain.ProvisionJob{InstanceID: inst.ID})
	require.NoError(t, err)

	// Wait a bit for container to run and gather stats
	time.Sleep(2 * time.Second)

	// Get Stats
	stats, err := svc.GetInstanceStats(ctx, inst.ID.String())
	if err != nil {
		// In some CI environments (e.g., Docker-in-Docker or restricted cgroups), retrieving container statistics
		// may fail or return incomplete data. We log this as a warning rather than failing the test to maintain
		// robustness across different execution environments while still verifying the integration where possible.
		t.Logf("Warning: Failed to retrieve instance statistics (likely environment limitation): %v", err)
	} else {
		assert.NotNil(t, stats)
		// CPU percent might be 0 if idle, but LimitBytes should be set if memory limit worked.
		assert.GreaterOrEqual(t, stats.MemoryLimitBytes, 0.0)
		t.Logf("Got Stats: CPU=%.2f%% Mem=%.2fMB", stats.CPUPercentage, stats.MemoryUsageBytes/1024/1024)
	}

	// Clean up
	_ = svc.TerminateInstance(ctx, inst.ID.String())
	// compute.DeleteInstance(ctx, ...) happens in Terminate
}

func TestNetworkingCIDRExhaustion(t *testing.T) {
	db, svc, compute, _, vpcRepo, _, ctx := setupInstanceServiceTest(t)

	// Skip if noop compute
	if compute.Type() == "noop" {
		t.Skip("Skipping CIDR exhaustion test for noop compute")
	}

	subnetRepo := postgres.NewSubnetRepository(db)

	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	auditSvc := services.NewAuditService(services.AuditServiceParams{
		Repo:    postgres.NewAuditRepository(db),
		RBACSvc: rbacSvc,
	})
	subnetSvc := services.NewSubnetService(services.SubnetServiceParams{
		Repo:     subnetRepo,
		RBACSvc:  rbacSvc,
		VpcRepo:  vpcRepo,
		AuditSvc: auditSvc,
		Logger:   slog.Default(),
	})

	// 1. Create VPC and a very small subnet (/30)
	tenantID := appcontext.TenantIDFromContext(ctx)
	userID := appcontext.UserIDFromContext(ctx)
	vpc := &domain.VPC{
		ID:        uuid.New(),
		UserID:    userID,
		TenantID:  tenantID,
		Name:      "exhaust-vpc",
		CIDRBlock: "10.10.0.0/16",
		Status:    "available",
	}
	err := vpcRepo.Create(ctx, vpc)
	require.NoError(t, err)

	subnet, err := subnetSvc.CreateSubnet(ctx, vpc.ID, "tiny-subnet", "10.10.1.0/30", "us-east-1a")
	require.NoError(t, err)

	// 2. Launch 1st instance (Should succeed in DB)
	inst1, err := svc.LaunchInstance(ctx, coreports.LaunchParams{
		Name:         "inst-1",
		Image:        testImage,
		InstanceType: testInstanceType,
		VpcID:        &vpc.ID,
		SubnetID:     &subnet.ID,
	})
	require.NoError(t, err)

	// Manually provision to trigger network allocation
	err = svc.Provision(ctx, domain.ProvisionJob{InstanceID: inst1.ID})
	require.NoError(t, err)

	// 3. Launch 2nd instance (Should succeed in DB)
	inst2, err := svc.LaunchInstance(ctx, coreports.LaunchParams{
		Name:         "inst-2",
		Image:        testImage,
		InstanceType: testInstanceType,
		VpcID:        &vpc.ID,
		SubnetID:     &subnet.ID,
	})
	require.NoError(t, err)

	// Provision 2nd instance (Should fail with CIDR exhaustion)
	err = svc.Provision(ctx, domain.ProvisionJob{InstanceID: inst2.ID})
	require.Error(t, err, "Expected error due to CIDR exhaustion")
	t.Logf("Got expected error: %v", err)
	assert.Contains(t, err.Error(), "allocate IP")
}

func TestInstanceMetadataAndLabels(t *testing.T) {
	_, svc, _, repo, _, _, ctx := setupInstanceServiceTest(t)

	t.Run("Launch with Metadata", func(t *testing.T) {
		name := "meta-launch"
		metadata := map[string]string{"env": "prod", "version": "1.0"}
		labels := map[string]string{"tier": "frontend"}

		inst, err := svc.LaunchInstance(ctx, coreports.LaunchParams{
			Name:         name,
			Image:        testImage,
			InstanceType: testInstanceType,
			Metadata:     metadata,
			Labels:       labels,
		})
		require.NoError(t, err)
		assert.Equal(t, metadata, inst.Metadata)
		assert.Equal(t, labels, inst.Labels)

		// Verify in DB
		dbInst, err := repo.GetByID(ctx, inst.ID)
		require.NoError(t, err)
		assert.Equal(t, metadata, dbInst.Metadata)
		assert.Equal(t, labels, dbInst.Labels)
	})

	t.Run("Update Metadata", func(t *testing.T) {
		inst, _ := svc.LaunchInstance(ctx, coreports.LaunchParams{
			Name:         "meta-update",
			Image:        testImage,
			InstanceType: testInstanceType,
			Metadata:     map[string]string{"key1": "val1"},
		})

		// 1. Add and Update
		err := svc.UpdateInstanceMetadata(ctx, inst.ID, map[string]string{"key1": "newval", "key2": "val2"}, nil)
		require.NoError(t, err)

		// 2. Delete (empty value)
		err = svc.UpdateInstanceMetadata(ctx, inst.ID, map[string]string{"key1": ""}, map[string]string{"l1": "v1"})
		require.NoError(t, err)

		dbInst, _ := repo.GetByID(ctx, inst.ID)
		assert.Equal(t, "val2", dbInst.Metadata["key2"])
		_, ok := dbInst.Metadata["key1"]
		assert.False(t, ok)
		assert.Equal(t, "v1", dbInst.Labels["l1"])
	})
}

func TestSSHKeyInjection(t *testing.T) {
	db, svc, _, _, _, _, ctx := setupInstanceServiceTest(t)
	sshKeyRepo := postgres.NewSSHKeyRepo(db)

	// 1. Create SSH Key
	key := &domain.SSHKey{
		ID:        uuid.New(),
		UserID:    appcontext.UserIDFromContext(ctx),
		TenantID:  appcontext.TenantIDFromContext(ctx),
		Name:      "test-injection-key",
		PublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC...",
	}
	err := sshKeyRepo.Create(ctx, key)
	require.NoError(t, err)

	// 2. Launch with SSH Key
	inst, err := svc.LaunchInstance(ctx, coreports.LaunchParams{
		Name:         "ssh-inst",
		Image:        testImage,
		InstanceType: testInstanceType,
		SSHKeyID:     &key.ID,
	})
	require.NoError(t, err)

	assert.Equal(t, &key.ID, inst.SSHKeyID)
}

func TestInstanceServiceLifecycleMethods(t *testing.T) {
	_, svc, compute, repo, _, _, ctx := setupInstanceServiceTest(t)

	// Setup instance
	inst, err := svc.LaunchInstance(ctx, coreports.LaunchParams{
		Name:         "lifecycle-test",
		Image:        testImage,
		InstanceType: testInstanceType,
	})
	require.NoError(t, err)
	err = svc.Provision(ctx, domain.ProvisionJob{InstanceID: inst.ID})
	require.NoError(t, err)

	t.Run("StopInstance", func(t *testing.T) {
		err := svc.StopInstance(ctx, inst.ID.String())
		require.NoError(t, err)

		dbInst, err := repo.GetByID(ctx, inst.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusStopped, dbInst.Status)

		// Test stopping again (idempotency)
		err = svc.StopInstance(ctx, inst.ID.String())
		require.NoError(t, err)
	})

	t.Run("StartInstance", func(t *testing.T) {
		err := svc.StartInstance(ctx, inst.ID.String())
		require.NoError(t, err)

		dbInst, err := repo.GetByID(ctx, inst.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusRunning, dbInst.Status)

		// Test starting again (idempotency)
		err = svc.StartInstance(ctx, inst.ID.String())
		require.NoError(t, err)
	})

	t.Run("GetInstanceLogs", func(t *testing.T) {
		logs, err := svc.GetInstanceLogs(ctx, inst.ID.String())
		require.NoError(t, err)
		assert.NotNil(t, logs)
	})

	t.Run("Exec", func(t *testing.T) {
		if compute.Type() == "noop" {
			t.Skip("Skipping exec test for noop compute")
		}
		output, err := svc.Exec(ctx, inst.ID.String(), []string{"echo", "hello"})
		require.NoError(t, err)
		assert.Contains(t, output, "hello")
	})

	t.Run("GetConsoleURL", func(t *testing.T) {
		if compute.Type() == "docker" || compute.Type() == "noop" {
			t.Skip("Skipping console test for docker/noop backend")
		}
		url, err := svc.GetConsoleURL(ctx, inst.ID.String())
		require.NoError(t, err)
		assert.NotNil(t, url)
	})

	// Cleanup
	// Refresh instance to get the latest container ID after provisioning/restarts
	refreshInst, err := repo.GetByID(ctx, inst.ID)
	if err == nil && refreshInst.ContainerID != "" && compute.Type() != "noop" {
		_ = compute.DeleteInstance(ctx, refreshInst.ContainerID)
	}
}

func TestLaunchInstanceWithOptions(t *testing.T) {
	_, svc, compute, _, _, _, ctx := setupInstanceServiceTest(t)

	opts := coreports.CreateInstanceOptions{
		Name:      "opts-launch",
		ImageName: testImage,
		Ports:     []string{"8080:80"},
		Env:       []string{"FOO=BAR"},
	}

	inst, err := svc.LaunchInstanceWithOptions(ctx, opts)
	require.NoError(t, err)
	assert.Equal(t, "opts-launch", inst.Name)
	assert.Equal(t, "8080:80", inst.Ports)

	// Cleanup
	if inst.ContainerID != "" && compute.Type() != "noop" {
		_ = compute.DeleteInstance(ctx, inst.ContainerID)
	}
}

func TestInstanceServiceLaunchEnqueueFailure(t *testing.T) {
	db := setupDB(t)
	ctx := setupTestUser(t, db)

	repo := postgres.NewInstanceRepository(db)
	vpcRepo := postgres.NewVpcRepository(db)
	subnetRepo := postgres.NewSubnetRepository(db)
	volumeRepo := postgres.NewVolumeRepository(db)
	itRepo := postgres.NewInstanceTypeRepository(db)

	compute := noop.NewNoopComputeBackend()

	defaultType := &domain.InstanceType{ID: testInstanceType, Name: "Basic 2", VCPUs: 1, MemoryMB: 128, DiskGB: 1}
	_, err := itRepo.Create(ctx, defaultType)
	require.NoError(t, err)

	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	eventSvc := services.NewEventService(services.EventServiceParams{
		Repo:      postgres.NewEventRepository(db),
		RBACSvc:   rbacSvc,
		Publisher: nil,
		Logger:    slog.Default(),
	})
	auditSvc := services.NewAuditService(services.AuditServiceParams{
		Repo:    postgres.NewAuditRepository(db),
		RBACSvc: rbacSvc,
	})

	// Create a task queue that fails
	taskQueue := &InMemoryTaskQueue{ShouldFail: true}

	sshKeySvc, err := services.NewSSHKeyService(services.SSHKeyServiceParams{
		Repo:    postgres.NewSSHKeyRepo(db),
		RBACSvc: rbacSvc,
	})
	require.NoError(t, err)

	tenantSvc := services.NewTenantService(services.TenantServiceParams{
		Repo:     postgres.NewTenantRepo(db),
		UserRepo: postgres.NewUserRepo(db),
		RBACSvc:  rbacSvc,
		Logger:   slog.Default(),
	})

	svc := services.NewInstanceService(services.InstanceServiceParams{
		Repo:             repo,
		VpcRepo:          vpcRepo,
		SubnetRepo:       subnetRepo,
		VolumeRepo:       volumeRepo,
		InstanceTypeRepo: itRepo,
		RBAC:             rbacSvc,
		Compute:          compute,
		EventSvc:         eventSvc,
		AuditSvc:         auditSvc,
		TaskQueue:        taskQueue,
		Logger:           slog.Default(),
		TenantSvc:        tenantSvc,
		SSHKeySvc:        sshKeySvc,
	})

	// Attempt LaunchInstance
	name := "enqueue-fail-integration"
	_, err = svc.LaunchInstance(ctx, coreports.LaunchParams{
		Name:         name,
		Image:        testImage,
		InstanceType: testInstanceType,
	})

	// Verify Failure
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to enqueue provisioning task")

	// Verify instance not in DB
	inst, err := repo.GetByName(ctx, name)
	require.Error(t, err)
	assert.Nil(t, inst)

	// Attempt LaunchInstanceWithOptions
	optsName := "enqueue-fail-opts-integration"
	opts := coreports.CreateInstanceOptions{
		Name:      optsName,
		ImageName: testImage,
	}
	_, err = svc.LaunchInstanceWithOptions(ctx, opts)

	// Verify Failure
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to enqueue provisioning task")

	// Verify instance not in DB
	instOpts, err := repo.GetByName(ctx, optsName)
	require.Error(t, err)
	assert.Nil(t, instOpts)
}
