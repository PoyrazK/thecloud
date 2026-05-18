package services_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	svcerrors "github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockSSHKeyService struct{ mock.Mock }

func (m *MockSSHKeyService) CreateKey(ctx context.Context, name, publicKey string) (*domain.SSHKey, error) {
	return nil, nil
}
func (m *MockSSHKeyService) GetKey(ctx context.Context, id uuid.UUID) (*domain.SSHKey, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.SSHKey)
	return r0, args.Error(1)
}
func (m *MockSSHKeyService) ListKeys(ctx context.Context) ([]*domain.SSHKey, error) { return nil, nil }
func (m *MockSSHKeyService) DeleteKey(ctx context.Context, id uuid.UUID) error      { return nil }

type MockDNSService struct{ mock.Mock }

func (m *MockDNSService) CreateZone(ctx context.Context, vpcID uuid.UUID, name, description string) (*domain.DNSZone, error) {
	return nil, nil
}
func (m *MockDNSService) ListZones(ctx context.Context) ([]*domain.DNSZone, error) {
	return nil, nil
}
func (m *MockDNSService) GetZone(ctx context.Context, idOrName string) (*domain.DNSZone, error) {
	return nil, nil
}
func (m *MockDNSService) GetZoneByVPC(ctx context.Context, vpcID uuid.UUID) (*domain.DNSZone, error) {
	return nil, nil
}
func (m *MockDNSService) DeleteZone(ctx context.Context, idOrName string) error {
	return nil
}
func (m *MockDNSService) CreateRecord(ctx context.Context, zoneID uuid.UUID, name string, recordType domain.RecordType, content string, ttl int, priority *int) (*domain.DNSRecord, error) {
	return nil, nil
}
func (m *MockDNSService) ListRecords(ctx context.Context, zoneID uuid.UUID) ([]*domain.DNSRecord, error) {
	return nil, nil
}
func (m *MockDNSService) GetRecord(ctx context.Context, id uuid.UUID) (*domain.DNSRecord, error) {
	return nil, nil
}
func (m *MockDNSService) UpdateRecord(ctx context.Context, id uuid.UUID, content string, ttl int, priority *int) (*domain.DNSRecord, error) {
	return nil, nil
}
func (m *MockDNSService) DeleteRecord(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockDNSService) RegisterInstance(ctx context.Context, instance *domain.Instance, ip string) error {
	args := m.Called(ctx, instance, ip)
	return args.Error(0)
}
func (m *MockDNSService) UnregisterInstance(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestInstanceService_Unit(t *testing.T) {
	t.Run("LaunchInstance", testInstanceServiceLaunchInstanceUnit)
	t.Run("Lifecycle", testInstanceServiceLifecycleUnit)
	t.Run("Exec", testInstanceServiceExecUnit)
	t.Run("ProvisionFinalize", testInstanceServiceProvisionFinalize)
	t.Run("ProvisionFinalize_LibvirtNoopNetwork", testInstanceServiceProvisionFinalize_LibvirtNoopNetwork)
	t.Run("Terminate", testInstanceServiceTerminateUnit)
	t.Run("VolumeRelease", testInstanceServiceVolumeReleaseUnit)
	t.Run("PauseResume", testInstanceServicePauseResumeUnit)
	t.Run("RBACErrors", testInstanceServiceUnitRbacErrors)
	t.Run("RepoErrors", testInstanceServiceUnitRepoErrors)
	t.Run("ResizeInstance", testInstanceServiceResizeInstanceUnit)
}

func testInstanceServiceLaunchInstanceUnit(t *testing.T) {
	repo := new(MockInstanceRepo)
	vpcRepo := new(MockVpcRepo)
	subnetRepo := new(MockSubnetRepo)
	volRepo := new(MockVolumeRepo)
	typeRepo := new(MockInstanceTypeRepo)
	compute := new(MockComputeBackend)
	network := new(MockNetworkBackend)
	rbacSvc := new(MockRBACService)
	eventSvc := new(MockEventService)
	auditSvc := new(MockAuditService)
	dnsSvc := new(MockDNSService)
	taskQueue := new(MockTaskQueue)
	tenantSvc := new(MockTenantService)
	sshKeySvc := new(MockSSHKeyService)

	svc := services.NewInstanceService(services.InstanceServiceParams{
		Repo:             repo,
		VpcRepo:          vpcRepo,
		SubnetRepo:       subnetRepo,
		VolumeRepo:       volRepo,
		InstanceTypeRepo: typeRepo,
		Compute:          compute,
		Network:          network,
		RBAC:             rbacSvc,
		EventSvc:         eventSvc,
		AuditSvc:         auditSvc,
		DNSSvc:           dnsSvc,
		TaskQueue:        taskQueue,
		TenantSvc:        tenantSvc,
		SSHKeySvc:        sshKeySvc,
		DockerNetwork:    "bridge",
		Logger:           slog.Default(),
	})

	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	ctx = appcontext.WithTenantID(ctx, tenantID)

	t.Run("Success_Basic", func(t *testing.T) {
		params := ports.LaunchParams{
			Name:         "test-inst",
			Image:        "alpine",
			InstanceType: "t2.micro",
		}

		rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		typeRepo.On("GetByID", mock.Anything, "t2.micro").Return(&domain.InstanceType{
			ID: "t2.micro", VCPUs: 1, MemoryMB: 1024,
		}, nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "instances", 1).Return(nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "vcpus", 1).Return(nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "memory", 1).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "vcpus", 1).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "memory", 1).Return(nil).Once()

		repo.On("Create", mock.Anything, mock.MatchedBy(func(i *domain.Instance) bool {
			return i.Name == params.Name && i.UserID == userID
		})).Return(nil).Once()

		taskQueue.On("Enqueue", mock.Anything, "provision_queue", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "instance.launch", "instance", mock.Anything, mock.Anything).Return(nil).Once()

		inst, err := svc.LaunchInstance(ctx, params)

		require.NoError(t, err)
		assert.NotNil(t, inst)
		assert.Equal(t, params.Name, inst.Name)
		repo.AssertExpectations(t)
		tenantSvc.AssertExpectations(t)
		rbacSvc.AssertExpectations(t)
	})

	t.Run("QuotaExceeded_Instances", func(t *testing.T) {
		params := ports.LaunchParams{
			Name:         "no-quota",
			Image:        "alpine",
			InstanceType: "t2.micro",
		}

		rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		typeRepo.On("GetByID", mock.Anything, "t2.micro").Return(&domain.InstanceType{
			ID: "t2.micro", VCPUs: 1, MemoryMB: 1024,
		}, nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "instances", 1).Return(fmt.Errorf("quota exceeded")).Once()

		_, err := svc.LaunchInstance(ctx, params)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "quota exceeded")
		rbacSvc.AssertExpectations(t)
	})

	t.Run("QuotaExceeded_VCPUs", func(t *testing.T) {
		params := ports.LaunchParams{
			Name:         "no-vcpu-quota",
			Image:        "ubuntu",
			InstanceType: "t2.large",
		}

		rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		typeRepo.On("GetByID", mock.Anything, "t2.large").Return(&domain.InstanceType{
			ID: "t2.large", VCPUs: 4, MemoryMB: 4096,
		}, nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "instances", 1).Return(nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "vcpus", 4).Return(fmt.Errorf("quota exceeded")).Once()

		_, err := svc.LaunchInstance(ctx, params)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "quota exceeded")
	})

	t.Run("QuotaExceeded_Memory", func(t *testing.T) {
		params := ports.LaunchParams{
			Name:         "no-mem-quota",
			Image:        "ubuntu",
			InstanceType: "t2.large",
		}

		rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		typeRepo.On("GetByID", mock.Anything, "t2.large").Return(&domain.InstanceType{
			ID: "t2.large", VCPUs: 4, MemoryMB: 4096,
		}, nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "instances", 1).Return(nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "vcpus", 4).Return(nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "memory", 4).Return(fmt.Errorf("quota exceeded")).Once()

		_, err := svc.LaunchInstance(ctx, params)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "quota exceeded")
	})
}

func testInstanceServiceLifecycleUnit(t *testing.T) {
	repo := new(MockInstanceRepo)
	vpcRepo := new(MockVpcRepo)
	subnetRepo := new(MockSubnetRepo)
	volRepo := new(MockVolumeRepo)
	typeRepo := new(MockInstanceTypeRepo)
	compute := new(MockComputeBackend)
	network := new(MockNetworkBackend)
	rbacSvc := new(MockRBACService)
	eventSvc := new(MockEventService)
	auditSvc := new(MockAuditService)
	dnsSvc := new(MockDNSService)
	taskQueue := new(MockTaskQueue)
	tenantSvc := new(MockTenantService)
	sshKeySvc := new(MockSSHKeyService)

	svc := services.NewInstanceService(services.InstanceServiceParams{
		Repo:             repo,
		VpcRepo:          vpcRepo,
		SubnetRepo:       subnetRepo,
		VolumeRepo:       volRepo,
		InstanceTypeRepo: typeRepo,
		Compute:          compute,
		Network:          network,
		RBAC:             rbacSvc,
		EventSvc:         eventSvc,
		AuditSvc:         auditSvc,
		DNSSvc:           dnsSvc,
		TaskQueue:        taskQueue,
		TenantSvc:        tenantSvc,
		SSHKeySvc:        sshKeySvc,
		DockerNetwork:    "bridge",
		Logger:           slog.Default(),
	})

	ctx := context.Background()
	instanceID := uuid.New()
	userID := uuid.New()
	tenantID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	ctx = appcontext.WithTenantID(ctx, tenantID)

	t.Run("StartInstance", func(t *testing.T) {
		inst := &domain.Instance{ID: instanceID, UserID: userID, TenantID: tenantID, Status: domain.StatusStopped, ContainerID: "cid-1"}
		repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceUpdate, instanceID.String()).Return(nil).Once()
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceRead, instanceID.String()).Return(nil).Once()
		compute.On("StartInstance", mock.Anything, "cid-1").Return(nil).Once()
		compute.On("Type").Return("docker").Maybe()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Instance) bool {
			return i.Status == domain.StatusRunning
		})).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "instance.start", "instance", instanceID.String(), mock.Anything).Return(nil).Once()

		err := svc.StartInstance(ctx, instanceID.String())
		require.NoError(t, err)
	})

	t.Run("StopInstance", func(t *testing.T) {
		inst := &domain.Instance{ID: instanceID, UserID: userID, TenantID: tenantID, Status: domain.StatusRunning, ContainerID: "cid-1"}
		repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceUpdate, instanceID.String()).Return(nil).Once()
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceRead, instanceID.String()).Return(nil).Once()
		compute.On("StopInstance", mock.Anything, "cid-1").Return(nil).Once()
		compute.On("Type").Return("docker").Maybe()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Instance) bool {
			return i.Status == domain.StatusStopped
		})).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "instance.stop", "instance", instanceID.String(), mock.Anything).Return(nil).Once()

		err := svc.StopInstance(ctx, instanceID.String())
		require.NoError(t, err)
	})

	t.Run("TerminateInstance", func(t *testing.T) {
		inst := &domain.Instance{
			ID: instanceID, UserID: userID, TenantID: tenantID, Status: domain.StatusRunning, ContainerID: "cid-1",
			InstanceType: "t2.micro",
		}
		typeRepo.On("GetByID", mock.Anything, "t2.micro").Return(&domain.InstanceType{VCPUs: 1, MemoryMB: 1024}, nil).Maybe()
		repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
		repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceTerminate, instanceID.String()).Return(nil).Once()
		compute.On("DeleteInstance", mock.Anything, "cid-1").Return(nil).Once()
		compute.On("Type").Return("docker").Maybe()
		volRepo.On("ListByInstanceID", mock.Anything, instanceID).Return([]*domain.Volume{}, nil).Once()
		repo.On("Delete", mock.Anything, instanceID).Return(nil).Once()

		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "instances", 1).Return(nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "vcpus", 1).Return(nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "memory", 1).Return(nil).Once()

		dnsSvc.On("UnregisterInstance", mock.Anything, instanceID).Return(nil).Maybe()
		eventSvc.On("RecordEvent", mock.Anything, "INSTANCE_TERMINATE", instanceID.String(), "INSTANCE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "instance.terminate", "instance", instanceID.String(), mock.Anything).Return(nil).Once()

		err := svc.TerminateInstance(ctx, instanceID.String())
		require.NoError(t, err)
	})
}

func testInstanceServiceExecUnit(t *testing.T) {
	repo := new(MockInstanceRepo)
	compute := new(MockComputeBackend)
	rbacSvc := new(MockRBACService)
	svc := services.NewInstanceService(services.InstanceServiceParams{
		Repo:    repo,
		Compute: compute,
		RBAC:    rbacSvc,
		Logger:  slog.Default(),
	})

	ctx := context.Background()
	instanceID := uuid.New()
	userID := uuid.New()
	tenantID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	ctx = appcontext.WithTenantID(ctx, tenantID)

	t.Run("NotRunning", func(t *testing.T) {
		inst := &domain.Instance{ID: instanceID, UserID: userID, TenantID: tenantID, Status: domain.StatusStopped, ContainerID: ""}
		repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
		repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceUpdate, instanceID.String()).Return(nil).Once()

		_, err := svc.Exec(ctx, instanceID.String(), []string{"ls"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "instance not running")
	})

	t.Run("BackendError", func(t *testing.T) {
		inst := &domain.Instance{ID: instanceID, UserID: userID, TenantID: tenantID, Status: domain.StatusRunning, ContainerID: "cid-1"}
		repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
		repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceUpdate, instanceID.String()).Return(nil).Once()
		compute.On("Exec", mock.Anything, "cid-1", []string{"ls"}).Return("", errors.New("exec failed")).Once()

		_, err := svc.Exec(ctx, instanceID.String(), []string{"ls"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exec failed")
	})
}

// TestInstanceService_Provision_Finalize tests the Provision method focusing on finalizeProvision path
func testInstanceServiceProvisionFinalize(t *testing.T) {
	t.Run("Finalize_Success", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		vpcRepo := new(MockVpcRepo)
		subnetRepo := new(MockSubnetRepo)
		volRepo := new(MockVolumeRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		network := new(MockNetworkBackend)
		rbacSvc := new(MockRBACService)
		eventSvc := new(MockEventService)
		auditSvc := new(MockAuditService)
		dnsSvc := new(MockDNSService)
		taskQueue := new(MockTaskQueue)
		tenantSvc := new(MockTenantService)
		sshKeySvc := new(MockSSHKeyService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			VpcRepo:          vpcRepo,
			SubnetRepo:       subnetRepo,
			VolumeRepo:       volRepo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			Network:          network,
			RBAC:             rbacSvc,
			EventSvc:         eventSvc,
			AuditSvc:         auditSvc,
			DNSSvc:           dnsSvc,
			TaskQueue:        taskQueue,
			TenantSvc:        tenantSvc,
			SSHKeySvc:        sshKeySvc,
			DockerNetwork:    "bridge",
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		vpcID := uuid.New()
		subnetID := uuid.New()
		inst := &domain.Instance{
			ID:           uuid.New(),
			UserID:       userID,
			TenantID:     tenantID,
			Name:         "test-inst",
			Image:        "alpine",
			InstanceType: "t2.micro",
			VpcID:        &vpcID,
			SubnetID:     &subnetID,
			Status:       domain.StatusStarting,
			PrivateIP:    "10.0.0.100", // Pre-allocated IP
			OvsPort:      "ovs-port-1",
		}

		// Mock GetByID to return instance
		repo.On("GetByID", mock.Anything, inst.ID).Return(inst, nil).Once()

		// Mock network provisioning - some methods called multiple times so use Maybe()
		compute.On("Type").Return("docker").Maybe()
		vpcRepo.On("GetByID", mock.Anything, vpcID).Return(&domain.VPC{ID: vpcID, NetworkID: "net1"}, nil).Maybe() // called twice: provisionNetwork + plumbNetwork
		subnetRepo.On("GetByID", mock.Anything, subnetID).Return(&domain.Subnet{ID: subnetID, CIDRBlock: "10.0.0.0/24", GatewayIP: "10.0.0.1"}, nil).Maybe()
		repo.On("ListBySubnet", mock.Anything, subnetID).Return([]*domain.Instance{}, nil).Maybe()
		network.On("CreateVethPair", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		network.On("AttachVethToBridge", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		network.On("SetVethIP", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		// Mock volume resolution (no volumes)
		volRepo.On("ListByInstanceID", mock.Anything, inst.ID).Return([]*domain.Volume{}, nil).Once()

		// Mock instance type
		typeRepo.On("GetByID", mock.Anything, "t2.micro").Return(&domain.InstanceType{ID: "t2.micro", VCPUs: 1, MemoryMB: 1024, DiskGB: 10}, nil).Once()

		// Mock container launch
		compute.On("LaunchInstanceWithOptions", mock.Anything, mock.Anything).Return("container-123", []string{}, nil).Once()

		// Mock finalizeProvision dependencies - use mock.Anything for IPs since allocation changes them
		dnsSvc.On("RegisterInstance", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()
		eventSvc.On("RecordEvent", mock.Anything, "INSTANCE_LAUNCH", mock.Anything, "INSTANCE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "instance.launch", "instance", mock.Anything, mock.Anything).Return(nil).Once()

		// Run Provision
		job := domain.ProvisionJob{
			InstanceID: inst.ID,
			UserData:   "",
			Volumes:    nil,
		}

		err := svc.Provision(ctx, job)
		require.NoError(t, err)

		// Verify final state
		assert.Equal(t, domain.StatusRunning, inst.Status)
		assert.Equal(t, "container-123", inst.ContainerID)

		repo.AssertExpectations(t)
	})

	t.Run("Finalize_RepoUpdateFails", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		vpcRepo := new(MockVpcRepo)
		subnetRepo := new(MockSubnetRepo)
		volRepo := new(MockVolumeRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		network := new(MockNetworkBackend)
		eventSvc := new(MockEventService)
		auditSvc := new(MockAuditService)
		dnsSvc := new(MockDNSService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			VpcRepo:          vpcRepo,
			SubnetRepo:       subnetRepo,
			VolumeRepo:       volRepo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			Network:          network,
			EventSvc:         eventSvc,
			AuditSvc:         auditSvc,
			DNSSvc:           dnsSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		vpcID := uuid.New()
		subnetID := uuid.New()
		inst := &domain.Instance{
			ID:           uuid.New(),
			UserID:       userID,
			TenantID:     tenantID,
			Name:         "test-inst",
			Image:        "alpine",
			InstanceType: "t2.micro",
			VpcID:        &vpcID,
			SubnetID:     &subnetID,
			Status:       domain.StatusStarting,
			PrivateIP:    "10.0.0.100",
			OvsPort:      "ovs-port-1",
		}

		repo.On("GetByID", mock.Anything, mock.Anything).Return(inst, nil).Maybe()
		compute.On("Type").Return("docker").Maybe()
		vpcRepo.On("GetByID", mock.Anything, mock.Anything).Return(&domain.VPC{ID: vpcID, NetworkID: "net1"}, nil).Maybe()
		subnetRepo.On("GetByID", mock.Anything, mock.Anything).Return(&domain.Subnet{ID: subnetID, CIDRBlock: "10.0.0.0/24", GatewayIP: "10.0.0.1"}, nil).Maybe()
		repo.On("ListBySubnet", mock.Anything, mock.Anything).Return([]*domain.Instance{}, nil).Maybe()
		network.On("CreateVethPair", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		network.On("AttachVethToBridge", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		network.On("SetVethIP", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		volRepo.On("ListByInstanceID", mock.Anything, mock.Anything).Return([]*domain.Volume{}, nil).Maybe()
		typeRepo.On("GetByID", mock.Anything, mock.Anything).Return(&domain.InstanceType{ID: "t2.micro", VCPUs: 1, MemoryMB: 1024, DiskGB: 10}, nil).Maybe()
		compute.On("LaunchInstanceWithOptions", mock.Anything, mock.Anything).Return("container-123", []string{}, nil).Maybe()
		dnsSvc.On("RegisterInstance", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		repo.On("Update", mock.Anything, mock.Anything).Return(errors.New("database error")).Maybe()

		job := domain.ProvisionJob{
			InstanceID: inst.ID,
			UserData:   "",
			Volumes:    nil,
		}

		err := svc.Provision(ctx, job)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database error")

		repo.AssertExpectations(t)
	})

	t.Run("Finalize_NetworkProvisioningAllocatesIP", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		vpcRepo := new(MockVpcRepo)
		subnetRepo := new(MockSubnetRepo)
		volRepo := new(MockVolumeRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		network := new(MockNetworkBackend)
		eventSvc := new(MockEventService)
		auditSvc := new(MockAuditService)
		dnsSvc := new(MockDNSService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			VpcRepo:          vpcRepo,
			SubnetRepo:       subnetRepo,
			VolumeRepo:       volRepo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			Network:          network,
			EventSvc:         eventSvc,
			AuditSvc:         auditSvc,
			DNSSvc:           dnsSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		vpcID := uuid.New()
		subnetID := uuid.New()
		inst := &domain.Instance{
			ID:           uuid.New(),
			UserID:       userID,
			TenantID:     tenantID,
			Name:         "test-inst",
			Image:        "alpine",
			InstanceType: "t2.micro",
			VpcID:        &vpcID,
			SubnetID:     &subnetID,
			Status:       domain.StatusStarting,
			PrivateIP:    "", // Empty - will trigger GetInstanceIP
			OvsPort:      "ovs-port-1",
		}

		repo.On("GetByID", mock.Anything, mock.Anything).Return(inst, nil).Maybe()
		compute.On("Type").Return("docker").Maybe()
		vpcRepo.On("GetByID", mock.Anything, mock.Anything).Return(&domain.VPC{ID: vpcID, NetworkID: "net1"}, nil).Maybe()
		subnetRepo.On("GetByID", mock.Anything, mock.Anything).Return(&domain.Subnet{ID: subnetID, CIDRBlock: "10.0.0.0/24", GatewayIP: "10.0.0.1"}, nil).Maybe()
		repo.On("ListBySubnet", mock.Anything, mock.Anything).Return([]*domain.Instance{}, nil).Maybe()
		network.On("CreateVethPair", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		network.On("AttachVethToBridge", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		network.On("SetVethIP", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		volRepo.On("ListByInstanceID", mock.Anything, mock.Anything).Return([]*domain.Volume{}, nil).Maybe()
		typeRepo.On("GetByID", mock.Anything, mock.Anything).Return(&domain.InstanceType{ID: "t2.micro", VCPUs: 1, MemoryMB: 1024, DiskGB: 10}, nil).Maybe()
		compute.On("LaunchInstanceWithOptions", mock.Anything, mock.Anything).Return("container-123", []string{}, nil).Maybe()
		compute.On("GetInstanceIP", mock.Anything, mock.Anything).Return("10.0.0.50", nil).Maybe()
		dnsSvc.On("RegisterInstance", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		repo.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
		eventSvc.On("RecordEvent", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		auditSvc.On("Log", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		job := domain.ProvisionJob{
			InstanceID: inst.ID,
			UserData:   "",
			Volumes:    nil,
		}

		err := svc.Provision(ctx, job)
		require.NoError(t, err)

		// Note: provisionNetwork allocates PrivateIP before finalizeProvision runs,
		// so GetInstanceIP in finalizeProvision is not exercised by this test.
		// This test verifies success path when PrivateIP starts empty and is allocated during network provisioning.
		assert.Equal(t, domain.StatusRunning, inst.Status)

		repo.AssertExpectations(t)
	})
}

func testInstanceServiceProvisionFinalize_LibvirtNoopNetwork(t *testing.T) {
	t.Run("Libvirt_NoopNetwork_ReturnsDefaultNetwork", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		vpcRepo := new(MockVpcRepo)
		subnetRepo := new(MockSubnetRepo)
		volRepo := new(MockVolumeRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		eventSvc := new(MockEventService)
		auditSvc := new(MockAuditService)
		dnsSvc := new(MockDNSService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			VpcRepo:          vpcRepo,
			SubnetRepo:       subnetRepo,
			VolumeRepo:       volRepo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			Network:          nil, // noop network backend
			EventSvc:         eventSvc,
			AuditSvc:         auditSvc,
			DNSSvc:           dnsSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		vpcID := uuid.New()
		subnetID := uuid.New()
		inst := &domain.Instance{
			ID:           uuid.New(),
			UserID:       userID,
			TenantID:     tenantID,
			Name:         "test-inst",
			Image:        "alpine",
			InstanceType: "t2.micro",
			VpcID:        &vpcID,
			SubnetID:     &subnetID,
			Status:       domain.StatusStarting,
		}

		repo.On("GetByID", mock.Anything, inst.ID).Return(inst, nil).Once()
		compute.On("Type").Return("libvirt").Maybe()
		vpcRepo.On("GetByID", mock.Anything, vpcID).Return(&domain.VPC{ID: vpcID, NetworkID: "br-vpc-12345678"}, nil).Maybe()
		subnetRepo.On("GetByID", mock.Anything, subnetID).Return(&domain.Subnet{ID: subnetID, CIDRBlock: "10.0.0.0/24", GatewayIP: "10.0.0.1"}, nil).Maybe()
		volRepo.On("ListByInstanceID", mock.Anything, mock.Anything).Return([]*domain.Volume{}, nil).Maybe()
		typeRepo.On("GetByID", mock.Anything, "t2.micro").Return(&domain.InstanceType{ID: "t2.micro", VCPUs: 1, MemoryMB: 1024, DiskGB: 10}, nil).Maybe()

		// Capture the NetworkID passed to LaunchInstanceWithOptions
		var capturedNetworkID string
		compute.On("LaunchInstanceWithOptions", mock.Anything, mock.MatchedBy(func(opts ports.CreateInstanceOptions) bool {
			capturedNetworkID = opts.NetworkID
			return true
		})).Return("domain-123", []string{}, nil).Once()

		// GetInstanceIP is called in finalizeProvision when PrivateIP is empty
		compute.On("GetInstanceIP", mock.Anything, "domain-123").Return("192.168.122.50", nil).Maybe()

		dnsSvc.On("RegisterInstance", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()
		eventSvc.On("RecordEvent", mock.Anything, "INSTANCE_LAUNCH", mock.Anything, "INSTANCE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "instance.launch", "instance", mock.Anything, mock.Anything).Return(nil).Once()

		job := domain.ProvisionJob{
			InstanceID: inst.ID,
			UserData:   "",
			Volumes:    nil,
		}

		err := svc.Provision(ctx, job)
		require.NoError(t, err)

		// Verify that libvirt used "default" network instead of the OVS bridge name
		assert.Equal(t, "default", capturedNetworkID)
		assert.Equal(t, domain.StatusRunning, inst.Status)

		repo.AssertExpectations(t)
		compute.AssertExpectations(t)
	})
}

func testInstanceServiceTerminateUnit(t *testing.T) {
	t.Run("TerminateInstance_WithVolumesAttached", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		volRepo := new(MockVolumeRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		rbacSvc := new(MockRBACService)
		dnsSvc := new(MockDNSService)
		eventSvc := new(MockEventService)
		auditSvc := new(MockAuditService)
		tenantSvc := new(MockTenantService)
		logSvc := new(MockLogService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			VolumeRepo:       volRepo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			RBAC:             rbacSvc,
			DNSSvc:           dnsSvc,
			EventSvc:         eventSvc,
			AuditSvc:         auditSvc,
			TenantSvc:        tenantSvc,
			LogSvc:           logSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		vpcID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		vol1 := &domain.Volume{ID: uuid.New(), TenantID: tenantID, Status: domain.VolumeStatusInUse, InstanceID: &instanceID, MountPath: "/mnt/vol1"}
		vol2 := &domain.Volume{ID: uuid.New(), TenantID: tenantID, Status: domain.VolumeStatusInUse, InstanceID: &instanceID, MountPath: "/mnt/vol2"}

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusRunning,
			ContainerID:  "cid-1",
			InstanceType: "t2.micro",
			VpcID:        &vpcID,
		}

		repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
		repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceTerminate, instanceID.String()).Return(nil).Once()
		compute.On("GetInstanceLogs", mock.Anything, "cid-1").Return(io.NopCloser(strings.NewReader("log line 1\nlog line 2\n")), nil).Once()
		logSvc.On("IngestLogs", mock.Anything, mock.Anything).Return(nil).Once()
		compute.On("DeleteInstance", mock.Anything, "cid-1").Return(nil).Once()
		compute.On("Type").Return("docker").Maybe()
		volRepo.On("ListByInstanceID", mock.Anything, instanceID).Return([]*domain.Volume{vol1, vol2}, nil).Once()
		volRepo.On("Update", mock.Anything, mock.MatchedBy(func(v *domain.Volume) bool {
			return v.Status == domain.VolumeStatusAvailable && v.InstanceID == nil && v.MountPath == ""
		})).Return(nil).Times(2)
		repo.On("Delete", mock.Anything, instanceID).Return(nil).Once()
		typeRepo.On("GetByID", mock.Anything, "t2.micro").Return(&domain.InstanceType{VCPUs: 1, MemoryMB: 1024}, nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "instances", 1).Return(nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "vcpus", 1).Return(nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "memory", 1).Return(nil).Once()
		eventSvc.On("RecordEvent", mock.Anything, "INSTANCE_TERMINATE", instanceID.String(), "INSTANCE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "instance.terminate", "instance", instanceID.String(), mock.Anything).Return(nil).Once()
		dnsSvc.On("UnregisterInstance", mock.Anything, instanceID).Return(nil).Maybe()

		err := svc.TerminateInstance(ctx, instanceID.String())
		require.NoError(t, err)

		mock.AssertExpectationsForObjects(t, repo, volRepo, typeRepo, compute, rbacSvc, eventSvc, auditSvc, dnsSvc)
	})

	t.Run("TerminateInstance_ContainerDeleteFails", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		compute := new(MockComputeBackend)
		rbacSvc := new(MockRBACService)
		tenantSvc := new(MockTenantService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:      repo,
			Compute:   compute,
			RBAC:      rbacSvc,
			TenantSvc: tenantSvc,
			Logger:    slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:          instanceID,
			UserID:      userID,
			TenantID:    tenantID,
			Status:      domain.StatusRunning,
			ContainerID: "cid-1",
		}

		repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
		repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceTerminate, instanceID.String()).Return(nil).Once()
		compute.On("DeleteInstance", mock.Anything, "cid-1").Return(fmt.Errorf("docker error")).Once()
		compute.On("Type").Return("docker").Maybe()

		err := svc.TerminateInstance(ctx, instanceID.String())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to remove docker instance")
		mock.AssertExpectationsForObjects(t, repo, rbacSvc, compute)
	})

	t.Run("TerminateInstance_InstanceTypeNotFound", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		volRepo := new(MockVolumeRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		rbacSvc := new(MockRBACService)
		eventSvc := new(MockEventService)
		auditSvc := new(MockAuditService)
		tenantSvc := new(MockTenantService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			VolumeRepo:       volRepo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			RBAC:             rbacSvc,
			EventSvc:         eventSvc,
			AuditSvc:         auditSvc,
			TenantSvc:        tenantSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusStopped,
			ContainerID:  "cid-1",
			InstanceType: "unknown-type",
		}

		repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
		repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceTerminate, instanceID.String()).Return(nil).Once()
		compute.On("DeleteInstance", mock.Anything, "cid-1").Return(nil).Once()
		compute.On("Type").Return("docker").Maybe()
		volRepo.On("ListByInstanceID", mock.Anything, instanceID).Return([]*domain.Volume{}, nil).Once()
		repo.On("Delete", mock.Anything, instanceID).Return(nil).Once()
		typeRepo.On("GetByID", mock.Anything, "unknown-type").Return(nil, fmt.Errorf("not found")).Once()
		eventSvc.On("RecordEvent", mock.Anything, "INSTANCE_TERMINATE", instanceID.String(), "INSTANCE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "instance.terminate", "instance", instanceID.String(), mock.Anything).Return(nil).Once()

		err := svc.TerminateInstance(ctx, instanceID.String())
		require.NoError(t, err)

		mock.AssertExpectationsForObjects(t, repo, volRepo, typeRepo, compute, rbacSvc, eventSvc, auditSvc)
	})

	t.Run("TerminateInstance_StoppedStatus", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		volRepo := new(MockVolumeRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		rbacSvc := new(MockRBACService)
		eventSvc := new(MockEventService)
		auditSvc := new(MockAuditService)
		tenantSvc := new(MockTenantService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			VolumeRepo:       volRepo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			RBAC:             rbacSvc,
			EventSvc:         eventSvc,
			AuditSvc:         auditSvc,
			TenantSvc:        tenantSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusStopped,
			ContainerID:  "cid-1",
			InstanceType: "t2.micro",
		}

		repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
		repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceTerminate, instanceID.String()).Return(nil).Once()
		compute.On("DeleteInstance", mock.Anything, "cid-1").Return(nil).Once()
		compute.On("Type").Return("docker").Maybe()
		volRepo.On("ListByInstanceID", mock.Anything, instanceID).Return([]*domain.Volume{}, nil).Once()
		repo.On("Delete", mock.Anything, instanceID).Return(nil).Once()
		typeRepo.On("GetByID", mock.Anything, "t2.micro").Return(&domain.InstanceType{VCPUs: 1, MemoryMB: 1024}, nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "instances", 1).Return(nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "vcpus", 1).Return(nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "memory", 1).Return(nil).Once()
		eventSvc.On("RecordEvent", mock.Anything, "INSTANCE_TERMINATE", instanceID.String(), "INSTANCE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "instance.terminate", "instance", instanceID.String(), mock.Anything).Return(nil).Once()

		err := svc.TerminateInstance(ctx, instanceID.String())
		require.NoError(t, err)

		mock.AssertExpectationsForObjects(t, repo, volRepo, typeRepo, compute, rbacSvc, eventSvc, auditSvc, tenantSvc)
	})
}

func testInstanceServiceVolumeReleaseUnit(t *testing.T) {
	t.Run("releaseAttachedVolumes_ListError", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		volRepo := new(MockVolumeRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		rbacSvc := new(MockRBACService)
		eventSvc := new(MockEventService)
		auditSvc := new(MockAuditService)
		tenantSvc := new(MockTenantService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			VolumeRepo:       volRepo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			RBAC:             rbacSvc,
			EventSvc:         eventSvc,
			AuditSvc:         auditSvc,
			TenantSvc:        tenantSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusRunning,
			ContainerID:  "cid-1",
			InstanceType: "t2.micro",
		}

		repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
		repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceTerminate, instanceID.String()).Return(nil).Once()
		compute.On("DeleteInstance", mock.Anything, "cid-1").Return(nil).Once()
		compute.On("Type").Return("docker").Maybe()
		volRepo.On("ListByInstanceID", mock.Anything, instanceID).Return(nil, fmt.Errorf("db error")).Once()
		repo.On("Delete", mock.Anything, instanceID).Return(nil).Once()
		typeRepo.On("GetByID", mock.Anything, "t2.micro").Return(&domain.InstanceType{VCPUs: 1, MemoryMB: 1024}, nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "instances", 1).Return(nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "vcpus", 1).Return(nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "memory", 1).Return(nil).Once()
		eventSvc.On("RecordEvent", mock.Anything, "INSTANCE_TERMINATE", instanceID.String(), "INSTANCE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "instance.terminate", "instance", instanceID.String(), mock.Anything).Return(nil).Once()

		err := svc.TerminateInstance(ctx, instanceID.String())
		require.NoError(t, err) // releaseAttachedVolumes error is logged but doesn't fail termination
		mock.AssertExpectationsForObjects(t, repo, volRepo, typeRepo, compute, rbacSvc, eventSvc, auditSvc, tenantSvc)
	})

	t.Run("releaseAttachedVolumes_PartialFailure", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		volRepo := new(MockVolumeRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		rbacSvc := new(MockRBACService)
		eventSvc := new(MockEventService)
		auditSvc := new(MockAuditService)
		tenantSvc := new(MockTenantService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			VolumeRepo:       volRepo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			RBAC:             rbacSvc,
			EventSvc:         eventSvc,
			AuditSvc:         auditSvc,
			TenantSvc:        tenantSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		vol1 := &domain.Volume{ID: uuid.New(), TenantID: tenantID, Status: domain.VolumeStatusInUse, InstanceID: &instanceID}
		vol2 := &domain.Volume{ID: uuid.New(), TenantID: tenantID, Status: domain.VolumeStatusInUse, InstanceID: &instanceID}

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusRunning,
			ContainerID:  "cid-1",
			InstanceType: "t2.micro",
		}

		repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
		repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceTerminate, instanceID.String()).Return(nil).Once()
		compute.On("DeleteInstance", mock.Anything, "cid-1").Return(nil).Once()
		compute.On("Type").Return("docker").Maybe()
		volRepo.On("ListByInstanceID", mock.Anything, instanceID).Return([]*domain.Volume{vol1, vol2}, nil).Once()
		volRepo.On("Update", mock.Anything, mock.MatchedBy(func(v *domain.Volume) bool {
			return v.ID == vol1.ID
		})).Return(nil).Once()
		volRepo.On("Update", mock.Anything, mock.MatchedBy(func(v *domain.Volume) bool {
			return v.ID == vol2.ID
		})).Return(fmt.Errorf("update error")).Once()
		repo.On("Delete", mock.Anything, instanceID).Return(nil).Once()
		typeRepo.On("GetByID", mock.Anything, "t2.micro").Return(&domain.InstanceType{VCPUs: 1, MemoryMB: 1024}, nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "instances", 1).Return(nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "vcpus", 1).Return(nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "memory", 1).Return(nil).Once()
		eventSvc.On("RecordEvent", mock.Anything, "INSTANCE_TERMINATE", instanceID.String(), "INSTANCE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "instance.terminate", "instance", instanceID.String(), mock.Anything).Return(nil).Once()

		err := svc.TerminateInstance(ctx, instanceID.String())
		require.NoError(t, err)
		mock.AssertExpectationsForObjects(t, repo, volRepo, typeRepo, compute, rbacSvc, eventSvc, auditSvc, tenantSvc)
	})
}

func testInstanceServicePauseResumeUnit(t *testing.T) {
	ctx := context.Background()
	instanceID := uuid.New()
	userID := uuid.New()
	tenantID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	ctx = appcontext.WithTenantID(ctx, tenantID)

	type pauseResumeCase struct {
		name   string
		method func(*services.InstanceService) func(context.Context, string) error
		setup  func(*domain.Instance, *MockInstanceRepo, *MockComputeBackend, *MockRBACService, *MockAuditService)
		assert func(*testing.T, error, *domain.Instance)
	}

	pauseCases := []pauseResumeCase{
		{
			name:   "PauseInstance_Success",
			method: func(svc *services.InstanceService) func(context.Context, string) error { return svc.PauseInstance },
			setup: func(inst *domain.Instance, repo *MockInstanceRepo, compute *MockComputeBackend, rbacSvc *MockRBACService, auditSvc *MockAuditService) {
				inst.Status = domain.StatusRunning
				repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
				repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceUpdate, instanceID.String()).Return(nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceRead, instanceID.String()).Return(nil).Once()
				compute.On("PauseInstance", mock.Anything, "cid-1").Return(nil).Once()
				compute.On("Type").Return("docker").Maybe()
				repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Instance) bool { return i.Status == domain.StatusPaused })).Return(nil).Once()
				auditSvc.On("Log", mock.Anything, userID, "instance.pause", "instance", instanceID.String(), mock.Anything).Return(nil).Once()
			},
			assert: func(t *testing.T, err error, inst *domain.Instance) {
				t.Helper()
				require.NoError(t, err)
				assert.Equal(t, domain.StatusPaused, inst.Status)
			},
		},
		{
			name:   "PauseInstance_WrongState",
			method: func(svc *services.InstanceService) func(context.Context, string) error { return svc.PauseInstance },
			setup: func(inst *domain.Instance, repo *MockInstanceRepo, compute *MockComputeBackend, rbacSvc *MockRBACService, auditSvc *MockAuditService) {
				inst.Status = domain.StatusPaused
				repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
				repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceUpdate, instanceID.String()).Return(nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceRead, instanceID.String()).Return(nil).Once()
			},
			assert: func(t *testing.T, err error, inst *domain.Instance) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "must be RUNNING to pause")
			},
		},
		{
			name:   "PauseInstance_ComputeError",
			method: func(svc *services.InstanceService) func(context.Context, string) error { return svc.PauseInstance },
			setup: func(inst *domain.Instance, repo *MockInstanceRepo, compute *MockComputeBackend, rbacSvc *MockRBACService, auditSvc *MockAuditService) {
				inst.Status = domain.StatusRunning
				repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
				repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceUpdate, instanceID.String()).Return(nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceRead, instanceID.String()).Return(nil).Once()
				compute.On("PauseInstance", mock.Anything, "cid-1").Return(fmt.Errorf("pause failed")).Once()
				compute.On("Type").Return("docker").Maybe()
				auditSvc.On("Log", mock.Anything, userID, "instance.pause", "instance", instanceID.String(), mock.Anything).Return(nil).Maybe()
			},
			assert: func(t *testing.T, err error, inst *domain.Instance) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "pause failed")
			},
		},
		{
			name:   "PauseInstance_RepoError_Rollback",
			method: func(svc *services.InstanceService) func(context.Context, string) error { return svc.PauseInstance },
			setup: func(inst *domain.Instance, repo *MockInstanceRepo, compute *MockComputeBackend, rbacSvc *MockRBACService, auditSvc *MockAuditService) {
				inst.Status = domain.StatusRunning
				repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
				repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceUpdate, instanceID.String()).Return(nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceRead, instanceID.String()).Return(nil).Once()
				compute.On("PauseInstance", mock.Anything, "cid-1").Return(nil).Once()
				compute.On("Type").Return("docker").Maybe()
				// First Update (to PAUSED) fails
				repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Instance) bool { return i.Status == domain.StatusPaused })).Return(fmt.Errorf("db error")).Once()
				// Rollback: backend undo first, then repo status restore
				compute.On("ResumeInstance", mock.Anything, "cid-1").Return(nil).Once()
				repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Instance) bool { return i.Status == domain.StatusRunning })).Return(nil).Once()
				auditSvc.On("Log", mock.Anything, userID, "instance.pause", "instance", instanceID.String(), mock.Anything).Return(nil).Maybe()
			},
			assert: func(t *testing.T, err error, inst *domain.Instance) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "db error")
				assert.Equal(t, domain.StatusRunning, inst.Status)
			},
		},
	}

	resumeCases := []pauseResumeCase{
		{
			name:   "ResumeInstance_Success",
			method: func(svc *services.InstanceService) func(context.Context, string) error { return svc.ResumeInstance },
			setup: func(inst *domain.Instance, repo *MockInstanceRepo, compute *MockComputeBackend, rbacSvc *MockRBACService, auditSvc *MockAuditService) {
				inst.Status = domain.StatusPaused
				repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
				repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceUpdate, instanceID.String()).Return(nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceRead, instanceID.String()).Return(nil).Once()
				compute.On("ResumeInstance", mock.Anything, "cid-1").Return(nil).Once()
				compute.On("Type").Return("docker").Maybe()
				repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Instance) bool { return i.Status == domain.StatusRunning })).Return(nil).Once()
				auditSvc.On("Log", mock.Anything, userID, "instance.resume", "instance", instanceID.String(), mock.Anything).Return(nil).Once()
			},
			assert: func(t *testing.T, err error, inst *domain.Instance) {
				t.Helper()
				require.NoError(t, err)
				assert.Equal(t, domain.StatusRunning, inst.Status)
			},
		},
		{
			name:   "ResumeInstance_WrongState",
			method: func(svc *services.InstanceService) func(context.Context, string) error { return svc.ResumeInstance },
			setup: func(inst *domain.Instance, repo *MockInstanceRepo, compute *MockComputeBackend, rbacSvc *MockRBACService, auditSvc *MockAuditService) {
				inst.Status = domain.StatusRunning
				repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
				repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceUpdate, instanceID.String()).Return(nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceRead, instanceID.String()).Return(nil).Once()
			},
			assert: func(t *testing.T, err error, inst *domain.Instance) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "must be PAUSED to resume")
			},
		},
		{
			name:   "ResumeInstance_ComputeError",
			method: func(svc *services.InstanceService) func(context.Context, string) error { return svc.ResumeInstance },
			setup: func(inst *domain.Instance, repo *MockInstanceRepo, compute *MockComputeBackend, rbacSvc *MockRBACService, auditSvc *MockAuditService) {
				inst.Status = domain.StatusPaused
				repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
				repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceUpdate, instanceID.String()).Return(nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceRead, instanceID.String()).Return(nil).Once()
				compute.On("ResumeInstance", mock.Anything, "cid-1").Return(fmt.Errorf("resume failed")).Once()
				compute.On("Type").Return("docker").Maybe()
				repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Instance) bool { return i.Status == domain.StatusPaused })).Return(nil).Once()
				auditSvc.On("Log", mock.Anything, userID, "instance.resume", "instance", instanceID.String(), mock.Anything).Return(nil).Maybe()
			},
			assert: func(t *testing.T, err error, inst *domain.Instance) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "resume failed")
			},
		},
		{
			name:   "ResumeInstance_ConflictError",
			method: func(svc *services.InstanceService) func(context.Context, string) error { return svc.ResumeInstance },
			setup: func(inst *domain.Instance, repo *MockInstanceRepo, compute *MockComputeBackend, rbacSvc *MockRBACService, auditSvc *MockAuditService) {
				inst.Status = domain.StatusPaused
				repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
				repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceUpdate, instanceID.String()).Return(nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceRead, instanceID.String()).Return(nil).Once()
				compute.On("ResumeInstance", mock.Anything, "cid-1").Return(svcerrors.ErrInstanceNotResumable).Once()
				compute.On("Type").Return("docker").Maybe()
			},
			assert: func(t *testing.T, err error, inst *domain.Instance) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "instance cannot be resumed")
				assert.True(t, svcerrors.Is(err, svcerrors.Conflict))
			},
		},
		{
			name:   "ResumeInstance_RepoError_Rollback",
			method: func(svc *services.InstanceService) func(context.Context, string) error { return svc.ResumeInstance },
			setup: func(inst *domain.Instance, repo *MockInstanceRepo, compute *MockComputeBackend, rbacSvc *MockRBACService, auditSvc *MockAuditService) {
				inst.Status = domain.StatusPaused
				repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, fmt.Errorf("not found")).Maybe()
				repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceUpdate, instanceID.String()).Return(nil).Once()
				rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceRead, instanceID.String()).Return(nil).Once()
				compute.On("ResumeInstance", mock.Anything, "cid-1").Return(nil).Once()
				compute.On("Type").Return("docker").Maybe()
				// First Update (to RUNNING) fails
				repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Instance) bool { return i.Status == domain.StatusRunning })).Return(fmt.Errorf("db error")).Once()
				// Rollback: backend undo first, then repo status restore
				compute.On("PauseInstance", mock.Anything, "cid-1").Return(nil).Once()
				repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Instance) bool { return i.Status == domain.StatusPaused })).Return(nil).Once()
				auditSvc.On("Log", mock.Anything, userID, "instance.resume", "instance", instanceID.String(), mock.Anything).Return(nil).Maybe()
			},
			assert: func(t *testing.T, err error, inst *domain.Instance) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "db error")
				assert.Equal(t, domain.StatusPaused, inst.Status)
			},
		},
	}

	for _, tc := range pauseCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(MockInstanceRepo)
			compute := new(MockComputeBackend)
			rbacSvc := new(MockRBACService)
			auditSvc := new(MockAuditService)
			svc := services.NewInstanceService(services.InstanceServiceParams{
				Repo:     repo,
				Compute:  compute,
				RBAC:     rbacSvc,
				AuditSvc: auditSvc,
				Logger:   slog.Default(),
			})
			inst := &domain.Instance{ID: instanceID, UserID: userID, TenantID: tenantID, Status: domain.StatusRunning, ContainerID: "cid-1"}
			tc.setup(inst, repo, compute, rbacSvc, auditSvc)
			err := tc.method(svc)(ctx, instanceID.String())
			tc.assert(t, err, inst)
			mock.AssertExpectationsForObjects(t, repo, compute, rbacSvc, auditSvc)
		})
	}

	for _, tc := range resumeCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(MockInstanceRepo)
			compute := new(MockComputeBackend)
			rbacSvc := new(MockRBACService)
			auditSvc := new(MockAuditService)
			svc := services.NewInstanceService(services.InstanceServiceParams{
				Repo:     repo,
				Compute:  compute,
				RBAC:     rbacSvc,
				AuditSvc: auditSvc,
				Logger:   slog.Default(),
			})
			inst := &domain.Instance{ID: instanceID, UserID: userID, TenantID: tenantID, Status: domain.StatusPaused, ContainerID: "cid-1"}
			tc.setup(inst, repo, compute, rbacSvc, auditSvc)
			err := tc.method(svc)(ctx, instanceID.String())
			tc.assert(t, err, inst)
			mock.AssertExpectationsForObjects(t, repo, compute, rbacSvc, auditSvc)
		})
	}
}

func testInstanceServiceUnitRbacErrors(t *testing.T) {
	repo := new(MockInstanceRepo)
	vpcRepo := new(MockVpcRepo)
	subnetRepo := new(MockSubnetRepo)
	volRepo := new(MockVolumeRepo)
	typeRepo := new(MockInstanceTypeRepo)
	compute := new(MockComputeBackend)
	network := new(MockNetworkBackend)
	rbacSvc := new(MockRBACService)
	eventSvc := new(MockEventService)
	auditSvc := new(MockAuditService)
	dnsSvc := new(MockDNSService)
	taskQueue := new(MockTaskQueue)
	tenantSvc := new(MockTenantService)
	sshKeySvc := new(MockSSHKeyService)

	svc := services.NewInstanceService(services.InstanceServiceParams{
		Repo:             repo,
		VpcRepo:          vpcRepo,
		SubnetRepo:       subnetRepo,
		VolumeRepo:       volRepo,
		InstanceTypeRepo: typeRepo,
		Compute:          compute,
		Network:          network,
		RBAC:             rbacSvc,
		EventSvc:         eventSvc,
		AuditSvc:         auditSvc,
		DNSSvc:           dnsSvc,
		TaskQueue:        taskQueue,
		TenantSvc:        tenantSvc,
		SSHKeySvc:        sshKeySvc,
		DockerNetwork:    "bridge",
		Logger:           slog.Default(),
	})

	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	ctx = appcontext.WithTenantID(ctx, tenantID)

	instID := uuid.New()
	instIDStr := instID.String()

	type rbacCase struct {
		name       string
		permission domain.Permission
		resourceID string
		invoke     func() error
	}

	cases := []rbacCase{
		{
			name:       "LaunchInstance_Unauthorized",
			permission: domain.PermissionInstanceLaunch,
			resourceID: "*",
			invoke: func() error {
				_, err := svc.LaunchInstance(ctx, ports.LaunchParams{Name: "test", Image: "alpine", InstanceType: "t2.micro"})
				return err
			},
		},
		{
			name:       "StartInstance_Unauthorized",
			permission: domain.PermissionInstanceUpdate,
			resourceID: instIDStr,
			invoke: func() error {
				return svc.StartInstance(ctx, instIDStr)
			},
		},
		{
			name:       "StopInstance_Unauthorized",
			permission: domain.PermissionInstanceUpdate,
			resourceID: instIDStr,
			invoke: func() error {
				return svc.StopInstance(ctx, instIDStr)
			},
		},
		{
			name:       "ListInstances_Unauthorized",
			permission: domain.PermissionInstanceRead,
			resourceID: "*",
			invoke: func() error {
				_, err := svc.ListInstances(ctx, nil)
				return err
			},
		},
		{
			name:       "GetInstance_Unauthorized",
			permission: domain.PermissionInstanceRead,
			resourceID: instIDStr,
			invoke: func() error {
				_, err := svc.GetInstance(ctx, instIDStr)
				return err
			},
		},
		{
			name:       "GetInstanceLogs_Unauthorized",
			permission: domain.PermissionInstanceRead,
			resourceID: instIDStr,
			invoke: func() error {
				_, err := svc.GetInstanceLogs(ctx, instIDStr)
				return err
			},
		},
		{
			name:       "GetConsoleURL_Unauthorized",
			permission: domain.PermissionInstanceRead,
			resourceID: instIDStr,
			invoke: func() error {
				_, err := svc.GetConsoleURL(ctx, instIDStr)
				return err
			},
		},
		{
			name:       "TerminateInstance_Unauthorized",
			permission: domain.PermissionInstanceTerminate,
			resourceID: instIDStr,
			invoke: func() error {
				return svc.TerminateInstance(ctx, instIDStr)
			},
		},
		{
			name:       "GetInstanceStats_Unauthorized",
			permission: domain.PermissionInstanceRead,
			resourceID: instIDStr,
			invoke: func() error {
				_, err := svc.GetInstanceStats(ctx, instIDStr)
				return err
			},
		},
		{
			name:       "Exec_Unauthorized",
			permission: domain.PermissionInstanceUpdate,
			resourceID: instIDStr,
			invoke: func() error {
				_, err := svc.Exec(ctx, instIDStr, []string{"ls"})
				return err
			},
		},
		{
			name:       "UpdateInstanceMetadata_Unauthorized",
			permission: domain.PermissionInstanceUpdate,
			resourceID: instIDStr,
			invoke: func() error {
				return svc.UpdateInstanceMetadata(ctx, instID, nil, nil)
			},
		},
	}

	authErr := svcerrors.New(svcerrors.Forbidden, "permission denied")
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rbacSvc.On("Authorize", mock.Anything, userID, tenantID, c.permission, c.resourceID).Return(authErr).Once()
			err := c.invoke()
			require.Error(t, err)
			assert.True(t, svcerrors.Is(err, svcerrors.Forbidden))
		})
	}
}

func testInstanceServiceUnitRepoErrors(t *testing.T) {
	repo := new(MockInstanceRepo)
	vpcRepo := new(MockVpcRepo)
	subnetRepo := new(MockSubnetRepo)
	volRepo := new(MockVolumeRepo)
	typeRepo := new(MockInstanceTypeRepo)
	compute := new(MockComputeBackend)
	network := new(MockNetworkBackend)
	rbacSvc := new(MockRBACService)
	eventSvc := new(MockEventService)
	auditSvc := new(MockAuditService)
	dnsSvc := new(MockDNSService)
	taskQueue := new(MockTaskQueue)
	tenantSvc := new(MockTenantService)
	sshKeySvc := new(MockSSHKeyService)

	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	compute.On("Type").Return("docker", nil).Maybe()

	svc := services.NewInstanceService(services.InstanceServiceParams{
		Repo:             repo,
		VpcRepo:          vpcRepo,
		SubnetRepo:       subnetRepo,
		VolumeRepo:       volRepo,
		InstanceTypeRepo: typeRepo,
		Compute:          compute,
		Network:          network,
		RBAC:             rbacSvc,
		EventSvc:         eventSvc,
		AuditSvc:         auditSvc,
		DNSSvc:           dnsSvc,
		TaskQueue:        taskQueue,
		TenantSvc:        tenantSvc,
		SSHKeySvc:        sshKeySvc,
		DockerNetwork:    "bridge",
		Logger:           slog.Default(),
	})

	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	ctx = appcontext.WithTenantID(ctx, tenantID)

	instID := uuid.New()
	inst := &domain.Instance{ID: instID, UserID: userID, TenantID: tenantID, Status: domain.StatusRunning, ContainerID: "cid-1", Name: "test-inst"}
	instStoppedNoCID := &domain.Instance{ID: instID, UserID: userID, TenantID: tenantID, Status: domain.StatusStopped, ContainerID: "", Name: "test-inst"}

	t.Run("GetInstance_NotFound", func(t *testing.T) {
		repo.On("GetByName", mock.Anything, "not-found").Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()
		repo.On("GetByID", mock.Anything, mock.Anything).Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()

		_, err := svc.GetInstance(ctx, "not-found")
		require.Error(t, err)
	})

	t.Run("GetInstance_RepoError", func(t *testing.T) {
		repo.On("GetByName", mock.Anything, "error").Return(nil, fmt.Errorf("db error")).Once()

		_, err := svc.GetInstance(ctx, "error")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("ListInstances_RepoError", func(t *testing.T) {
		repo.On("List", mock.Anything, []string(nil)).Return(nil, fmt.Errorf("db error")).Once()

		_, err := svc.ListInstances(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("StartInstance_NotFound", func(t *testing.T) {
		repo.On("GetByName", mock.Anything, mock.Anything).Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()
		repo.On("GetByID", mock.Anything, mock.Anything).Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()

		err := svc.StartInstance(ctx, "not-found")
		require.Error(t, err)
	})

	t.Run("StartInstance_ComputeError", func(t *testing.T) {
		// Use name string (not UUID) so GetByName is called; ContainerID empty so it formats name
		repo.On("GetByName", mock.Anything, "test-inst").Return(instStoppedNoCID, nil).Once()
		compute.On("StartInstance", mock.Anything, mock.Anything).Return(fmt.Errorf("compute error")).Once()
		repo.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
		auditSvc.On("Log", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		err := svc.StartInstance(ctx, "test-inst")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "compute error")
	})

	t.Run("StopInstance_NotFound", func(t *testing.T) {
		repo.On("GetByName", mock.Anything, mock.Anything).Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()
		repo.On("GetByID", mock.Anything, mock.Anything).Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()

		err := svc.StopInstance(ctx, "not-found")
		require.Error(t, err)
	})

	t.Run("StopInstance_ComputeError", func(t *testing.T) {
		// Use name string (not UUID) so GetByName is called; StatusRunning so compute is called
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		compute.On("StopInstance", mock.Anything, mock.Anything).Return(fmt.Errorf("compute error")).Once()
		repo.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
		auditSvc.On("Log", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		err := svc.StopInstance(ctx, "test-inst")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "compute error")
	})

	t.Run("TerminateInstance_NotFound", func(t *testing.T) {
		repo.On("GetByName", mock.Anything, mock.Anything).Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()
		repo.On("GetByID", mock.Anything, mock.Anything).Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()

		err := svc.TerminateInstance(ctx, "not-found")
		require.Error(t, err)
	})

	t.Run("TerminateInstance_RepoDeleteError", func(t *testing.T) {
		// Use name string so GetByName is called
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		compute.On("DeleteInstance", mock.Anything, mock.Anything).Return(nil).Once()
		volRepo.On("ListByInstanceID", mock.Anything, instID).Return([]*domain.Volume{}, nil).Once()
		typeRepo.On("GetByID", mock.Anything, inst.InstanceType).Return(&domain.InstanceType{ID: inst.InstanceType, VCPUs: 1, MemoryMB: 1024}, nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "instances", 1).Return(nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "vcpus", 1).Return(nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "memory", 1).Return(nil).Once()
		repo.On("Delete", mock.Anything, instID).Return(fmt.Errorf("db error")).Once()
		compute.On("Type").Return("docker").Maybe()
		dnsSvc.On("UnregisterInstance", mock.Anything, instID).Return(nil).Maybe()

		err := svc.TerminateInstance(ctx, "test-inst")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("LaunchInstance_SSHKeyNotFound", func(t *testing.T) {
		sshKeyID := uuid.New()
		params := ports.LaunchParams{Name: "test", Image: "alpine", InstanceType: "t2.micro", SSHKeyID: &sshKeyID}
		typeRepo.On("GetByID", mock.Anything, "t2.micro").Return(&domain.InstanceType{ID: "t2.micro", VCPUs: 1, MemoryMB: 1024}, nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "instances", 1).Return(nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "vcpus", 1).Return(nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "memory", 1).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "vcpus", 1).Return(nil).Maybe()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "memory", 1).Return(nil).Maybe()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "vcpus", 1).Return(nil).Maybe()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "memory", 1).Return(nil).Maybe()
		sshKeySvc.On("GetKey", mock.Anything, sshKeyID).Return(nil, svcerrors.New(svcerrors.NotFound, "ssh key not found")).Once()

		_, err := svc.LaunchInstance(ctx, params)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ssh key not found")
	})

	t.Run("LaunchInstance_PortValidationError", func(t *testing.T) {
		params := ports.LaunchParams{Name: "test", Image: "alpine", InstanceType: "t2.micro", Ports: "invalid-port"}
		typeRepo.On("GetByID", mock.Anything, "t2.micro").Return(&domain.InstanceType{ID: "t2.micro", VCPUs: 1, MemoryMB: 1024}, nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "instances", 1).Return(nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "vcpus", 1).Return(nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "memory", 1).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "vcpus", 1).Return(nil).Maybe()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "memory", 1).Return(nil).Maybe()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "vcpus", 1).Return(nil).Maybe()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "memory", 1).Return(nil).Maybe()

		_, err := svc.LaunchInstance(ctx, params)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "port format must be host:container")
	})

	t.Run("GetInstanceLogs_NotFound", func(t *testing.T) {
		repo.On("GetByName", mock.Anything, mock.Anything).Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()
		repo.On("GetByID", mock.Anything, mock.Anything).Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()

		_, err := svc.GetInstanceLogs(ctx, "not-found")
		require.Error(t, err)
	})

	t.Run("GetConsoleURL_NotFound", func(t *testing.T) {
		repo.On("GetByName", mock.Anything, mock.Anything).Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()
		repo.On("GetByID", mock.Anything, mock.Anything).Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()

		_, err := svc.GetConsoleURL(ctx, "not-found")
		require.Error(t, err)
	})

	t.Run("GetInstanceStats_NotFound", func(t *testing.T) {
		repo.On("GetByName", mock.Anything, mock.Anything).Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()
		repo.On("GetByID", mock.Anything, mock.Anything).Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()

		_, err := svc.GetInstanceStats(ctx, "not-found")
		require.Error(t, err)
	})

	t.Run("GetInstanceStats_ComputeError", func(t *testing.T) {
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		compute.On("GetInstanceStats", mock.Anything, "cid-1").Return(nil, fmt.Errorf("stats unavailable")).Once()

		_, err := svc.GetInstanceStats(ctx, "test-inst")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "stats unavailable")
	})

	t.Run("GetInstanceLogs_ComputeError", func(t *testing.T) {
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		compute.On("GetInstanceLogs", mock.Anything, "cid-1").Return(nil, fmt.Errorf("log stream error")).Once()

		_, err := svc.GetInstanceLogs(ctx, "test-inst")
		require.Error(t, err)
		assert.True(t, svcerrors.Is(err, svcerrors.Internal))
		assert.Contains(t, err.Error(), "failed to get instance logs")
	})

	t.Run("GetConsoleURL_ComputeError", func(t *testing.T) {
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		compute.On("GetConsoleURL", mock.Anything, "cid-1").Return("", fmt.Errorf("console unavailable")).Once()

		_, err := svc.GetConsoleURL(ctx, "test-inst")
		require.Error(t, err)
		assert.True(t, svcerrors.Is(err, svcerrors.Internal))
		assert.Contains(t, err.Error(), "failed to get console URL")
	})

	t.Run("Exec_NotFound", func(t *testing.T) {
		repo.On("GetByName", mock.Anything, mock.Anything).Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()
		repo.On("GetByID", mock.Anything, mock.Anything).Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()

		_, err := svc.Exec(ctx, "not-found", []string{"ls"})
		require.Error(t, err)
	})

	t.Run("UpdateInstanceMetadata_NotFound", func(t *testing.T) {
		repo.On("GetByID", mock.Anything, mock.Anything).Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Maybe()
		repo.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()

		err := svc.UpdateInstanceMetadata(ctx, instID, nil, nil)
		require.Error(t, err)
	})
}

func testInstanceServiceResizeInstanceUnit(t *testing.T) {
	// Each subtest creates its own mocks to avoid cross-test pollution
	t.Run("Success_Upsize", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		rbacSvc := new(MockRBACService)
		tenantSvc := new(MockTenantService)
		auditSvc := new(MockAuditService)
		eventSvc := new(MockEventService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			RBAC:             rbacSvc,
			TenantSvc:        tenantSvc,
			AuditSvc:         auditSvc,
			EventSvc:         eventSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusRunning,
			ContainerID:  "cid-1",
			InstanceType: "basic-2",
			Name:         "test-inst",
		}

		oldType := &domain.InstanceType{ID: "basic-2", VCPUs: 2, MemoryMB: 2048}
		newType := &domain.InstanceType{ID: "basic-4", VCPUs: 4, MemoryMB: 4096}

		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceResize, "test-inst").Return(nil).Once()
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-2").Return(oldType, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-4").Return(newType, nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "memory", 2).Return(nil).Once()
		compute.On("ResizeInstance", mock.Anything, "cid-1", int64(4*1e9), int64(4096*1024*1024)).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "memory", 2).Return(nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Instance) bool {
			return i.InstanceType == "basic-4"
		})).Return(nil).Once()
		eventSvc.On("RecordEvent", mock.Anything, "INSTANCE_RESIZE", instanceID.String(), "INSTANCE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "instance.resize", "instance", instanceID.String(), mock.Anything).Return(nil).Once()

		_, err := svc.ResizeInstance(ctx, "test-inst", "basic-4")

		require.NoError(t, err)
		mock.AssertExpectationsForObjects(t, repo, typeRepo, compute, rbacSvc, tenantSvc, eventSvc, auditSvc)
	})

	t.Run("Success_Downsize", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		rbacSvc := new(MockRBACService)
		tenantSvc := new(MockTenantService)
		auditSvc := new(MockAuditService)
		eventSvc := new(MockEventService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			RBAC:             rbacSvc,
			TenantSvc:        tenantSvc,
			AuditSvc:         auditSvc,
			EventSvc:         eventSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusRunning,
			ContainerID:  "cid-1",
			InstanceType: "basic-4",
			Name:         "test-inst",
		}

		oldType := &domain.InstanceType{ID: "basic-4", VCPUs: 4, MemoryMB: 4096}
		newType := &domain.InstanceType{ID: "basic-2", VCPUs: 2, MemoryMB: 2048}

		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceResize, "test-inst").Return(nil).Once()
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-4").Return(oldType, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-2").Return(newType, nil).Once()
		compute.On("ResizeInstance", mock.Anything, "cid-1", int64(2*1e9), int64(2048*1024*1024)).Return(nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "memory", 2).Return(nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Instance) bool {
			return i.InstanceType == "basic-2"
		})).Return(nil).Once()
		eventSvc.On("RecordEvent", mock.Anything, "INSTANCE_RESIZE", instanceID.String(), "INSTANCE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "instance.resize", "instance", instanceID.String(), mock.Anything).Return(nil).Once()

		_, err := svc.ResizeInstance(ctx, "test-inst", "basic-2")

		require.NoError(t, err)
		mock.AssertExpectationsForObjects(t, repo, typeRepo, compute, rbacSvc, tenantSvc, eventSvc, auditSvc)
	})

	t.Run("Success_SameSize", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		rbacSvc := new(MockRBACService)
		auditSvc := new(MockAuditService)
		eventSvc := new(MockEventService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			RBAC:             rbacSvc,
			AuditSvc:         auditSvc,
			EventSvc:         eventSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusRunning,
			ContainerID:  "cid-1",
			InstanceType: "basic-2",
			Name:         "test-inst",
		}

		oldType := &domain.InstanceType{ID: "basic-2", VCPUs: 2, MemoryMB: 2048}

		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceResize, "test-inst").Return(nil).Once()
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-2").Return(oldType, nil).Maybe()

		_, err := svc.ResizeInstance(ctx, "test-inst", "basic-2")

		require.NoError(t, err)
		compute.AssertNotCalled(t, "ResizeInstance", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
		mock.AssertExpectationsForObjects(t, repo, typeRepo, compute, rbacSvc, auditSvc)
	})

	t.Run("Success_ByUUID", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		rbacSvc := new(MockRBACService)
		tenantSvc := new(MockTenantService)
		auditSvc := new(MockAuditService)
		eventSvc := new(MockEventService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			RBAC:             rbacSvc,
			TenantSvc:        tenantSvc,
			AuditSvc:         auditSvc,
			EventSvc:         eventSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusRunning,
			ContainerID:  "cid-1",
			InstanceType: "basic-2",
			Name:         "test-inst",
		}

		oldType := &domain.InstanceType{ID: "basic-2", VCPUs: 2, MemoryMB: 2048}
		newType := &domain.InstanceType{ID: "basic-4", VCPUs: 4, MemoryMB: 4096}

		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceResize, instanceID.String()).Return(nil).Once()
		repo.On("GetByName", mock.Anything, instanceID.String()).Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()
		repo.On("GetByID", mock.Anything, instanceID).Return(inst, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-2").Return(oldType, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-4").Return(newType, nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "memory", 2).Return(nil).Once()
		compute.On("ResizeInstance", mock.Anything, "cid-1", int64(4*1e9), int64(4096*1024*1024)).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "memory", 2).Return(nil).Once()
		repo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()
		eventSvc.On("RecordEvent", mock.Anything, "INSTANCE_RESIZE", instanceID.String(), "INSTANCE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "instance.resize", "instance", instanceID.String(), mock.Anything).Return(nil).Once()

		_, err := svc.ResizeInstance(ctx, instanceID.String(), "basic-4")

		require.NoError(t, err)
		mock.AssertExpectationsForObjects(t, repo, typeRepo, compute, rbacSvc, tenantSvc, eventSvc, auditSvc)
	})

	t.Run("NotFound", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		rbacSvc := new(MockRBACService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:   repo,
			RBAC:   rbacSvc,
			Logger: slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceResize, "not-found").Return(nil).Once()
		repo.On("GetByName", mock.Anything, "not-found").Return(nil, svcerrors.New(svcerrors.NotFound, "not found")).Once()

		_, err := svc.ResizeInstance(ctx, "not-found", "basic-4")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		mock.AssertExpectationsForObjects(t, repo, rbacSvc)
	})

	t.Run("OldTypeNotFound", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		typeRepo := new(MockInstanceTypeRepo)
		rbacSvc := new(MockRBACService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			InstanceTypeRepo: typeRepo,
			RBAC:             rbacSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		instWithUnknownType := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			InstanceType: "unknown-type",
			ContainerID:  "cid-1",
		}

		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceResize, "test-inst").Return(nil).Once()
		repo.On("GetByName", mock.Anything, "test-inst").Return(instWithUnknownType, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "unknown-type").Return(nil, fmt.Errorf("not found")).Once()

		_, err := svc.ResizeInstance(ctx, "test-inst", "basic-4")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "current instance type not found")
		mock.AssertExpectationsForObjects(t, repo, typeRepo, rbacSvc)
	})

	t.Run("NewTypeNotFound", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		typeRepo := new(MockInstanceTypeRepo)
		rbacSvc := new(MockRBACService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			InstanceTypeRepo: typeRepo,
			RBAC:             rbacSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			InstanceType: "basic-2",
			ContainerID:  "cid-1",
		}

		oldType := &domain.InstanceType{ID: "basic-2", VCPUs: 2, MemoryMB: 2048}

		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceResize, "test-inst").Return(nil).Once()
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-2").Return(oldType, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "invalid-type").Return(nil, fmt.Errorf("not found")).Once()

		_, err := svc.ResizeInstance(ctx, "test-inst", "invalid-type")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid instance type")
		mock.AssertExpectationsForObjects(t, repo, typeRepo, rbacSvc)
	})

	t.Run("QuotaExceeded_CPU", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		typeRepo := new(MockInstanceTypeRepo)
		rbacSvc := new(MockRBACService)
		tenantSvc := new(MockTenantService)
		compute := new(MockComputeBackend)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			InstanceTypeRepo: typeRepo,
			RBAC:             rbacSvc,
			TenantSvc:        tenantSvc,
			Compute:          compute,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusRunning,
			InstanceType: "basic-2",
			ContainerID:  "cid-1",
		}

		oldType := &domain.InstanceType{ID: "basic-2", VCPUs: 2, MemoryMB: 2048}
		newType := &domain.InstanceType{ID: "basic-4", VCPUs: 4, MemoryMB: 4096}

		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceResize, "test-inst").Return(nil).Once()
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-2").Return(oldType, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-4").Return(newType, nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "vcpus", 2).Return(fmt.Errorf("insufficient vCPU quota")).Once()

		_, err := svc.ResizeInstance(ctx, "test-inst", "basic-4")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient vCPU quota")
		mock.AssertExpectationsForObjects(t, repo, typeRepo, rbacSvc, tenantSvc)
	})

	t.Run("QuotaExceeded_Memory", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		typeRepo := new(MockInstanceTypeRepo)
		rbacSvc := new(MockRBACService)
		tenantSvc := new(MockTenantService)
		compute := new(MockComputeBackend)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			InstanceTypeRepo: typeRepo,
			RBAC:             rbacSvc,
			TenantSvc:        tenantSvc,
			Compute:          compute,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusRunning,
			InstanceType: "basic-2",
			ContainerID:  "cid-1",
		}

		oldType := &domain.InstanceType{ID: "basic-2", VCPUs: 2, MemoryMB: 2048}
		newType := &domain.InstanceType{ID: "basic-4", VCPUs: 4, MemoryMB: 4096}

		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceResize, "test-inst").Return(nil).Once()
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-2").Return(oldType, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-4").Return(newType, nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "memory", 2).Return(fmt.Errorf("insufficient memory quota")).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()

		_, err := svc.ResizeInstance(ctx, "test-inst", "basic-4")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient memory quota")
		mock.AssertExpectationsForObjects(t, repo, typeRepo, rbacSvc, tenantSvc)
	})

	t.Run("Failure_DownsizeQuotaDecrementFails", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		typeRepo := new(MockInstanceTypeRepo)
		rbacSvc := new(MockRBACService)
		tenantSvc := new(MockTenantService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			InstanceTypeRepo: typeRepo,
			RBAC:             rbacSvc,
			TenantSvc:        tenantSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusRunning,
			InstanceType: "basic-4",
			ContainerID:  "cid-1",
		}

		oldType := &domain.InstanceType{ID: "basic-4", VCPUs: 4, MemoryMB: 4096}
		newType := &domain.InstanceType{ID: "basic-2", VCPUs: 2, MemoryMB: 2048}

		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceResize, "test-inst").Return(nil).Once()
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-4").Return(oldType, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-2").Return(newType, nil).Once()
		// Downsize: deltaCPU = -2, deltaMemMB = -2048
		// DecrementUsage fails for vCPUs — quota change fails before any compute touch
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "vcpus", 2).Return(fmt.Errorf("quota record locked")).Once()

		_, err := svc.ResizeInstance(ctx, "test-inst", "basic-2")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decrement vCPU quota for resize")
		repo.AssertNotCalled(t, "Update")
		mock.AssertExpectationsForObjects(t, repo, typeRepo, rbacSvc, tenantSvc)
	})

	t.Run("Failure_DownsizeComputeResizeFails", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		rbacSvc := new(MockRBACService)
		tenantSvc := new(MockTenantService)
		eventSvc := new(MockEventService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			RBAC:             rbacSvc,
			TenantSvc:        tenantSvc,
			EventSvc:         eventSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusRunning,
			InstanceType: "basic-4",
			ContainerID:  "cid-1",
		}

		oldType := &domain.InstanceType{ID: "basic-4", VCPUs: 4, MemoryMB: 4096}
		newType := &domain.InstanceType{ID: "basic-2", VCPUs: 2, MemoryMB: 2048}

		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceResize, "test-inst").Return(nil).Once()
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-4").Return(oldType, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-2").Return(newType, nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "memory", 2).Return(nil).Once()
		compute.On("ResizeInstance", mock.Anything, "cid-1", int64(2*1e9), int64(2048*1024*1024)).Return(fmt.Errorf("libvirt error")).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "vcpus", 2).Return(nil).Maybe()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "memory", 2).Return(nil).Maybe()

		_, err := svc.ResizeInstance(ctx, "test-inst", "basic-2")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resize instance")
		compute.AssertNotCalled(t, "ResizeInstance") // no compute rollback on compute failure, only quota rollback
		mock.AssertExpectationsForObjects(t, repo, typeRepo, compute, rbacSvc, tenantSvc, eventSvc)
	})

	t.Run("Success_VersionIncrementedOnResize", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		rbacSvc := new(MockRBACService)
		tenantSvc := new(MockTenantService)
		eventSvc := new(MockEventService)
		auditSvc := new(MockAuditService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			RBAC:             rbacSvc,
			TenantSvc:        tenantSvc,
			EventSvc:         eventSvc,
			AuditSvc:         auditSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusRunning,
			InstanceType: "basic-2",
			ContainerID:  "cid-1",
			Version:      1,
		}

		oldType := &domain.InstanceType{ID: "basic-2", VCPUs: 2, MemoryMB: 2048}
		newType := &domain.InstanceType{ID: "basic-4", VCPUs: 4, MemoryMB: 4096}

		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceResize, "test-inst").Return(nil).Once()
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-2").Return(oldType, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-4").Return(newType, nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "memory", 2).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "memory", 2).Return(nil).Once()
		compute.On("ResizeInstance", mock.Anything, "cid-1", int64(4*1e9), int64(4096*1024*1024)).Return(nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Instance) bool {
			return i.InstanceType == "basic-4" && i.Version == 2
		})).Return(nil).Once()
		eventSvc.On("RecordEvent", mock.Anything, "INSTANCE_RESIZE", instanceID.String(), "INSTANCE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "instance.resize", "instance", instanceID.String(), mock.Anything).Return(nil).Once()

		_, err := svc.ResizeInstance(ctx, "test-inst", "basic-4")

		require.NoError(t, err)
		mock.AssertExpectationsForObjects(t, repo, typeRepo, compute, rbacSvc, tenantSvc, eventSvc, auditSvc)
	})

	t.Run("Failure_ConcurrentResizeConflict", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		rbacSvc := new(MockRBACService)
		tenantSvc := new(MockTenantService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			RBAC:             rbacSvc,
			TenantSvc:        tenantSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusRunning,
			InstanceType: "basic-2",
			ContainerID:  "cid-1",
			Version:      1,
		}

		oldType := &domain.InstanceType{ID: "basic-2", VCPUs: 2, MemoryMB: 2048}
		newType := &domain.InstanceType{ID: "basic-4", VCPUs: 4, MemoryMB: 4096}

		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceResize, "test-inst").Return(nil).Once()
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-2").Return(oldType, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-4").Return(newType, nil).Once()
		// Quota calls for upsize
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "memory", 2).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "memory", 2).Return(nil).Once()
		// Compute resize succeeds
		compute.On("ResizeInstance", mock.Anything, "cid-1", int64(4*1e9), int64(4096*1024*1024)).Return(nil).Once()
		// repo.Update returns Conflict (simulating another resize modified the instance)
		// On conflict, we log warning and return success since compute already succeeded
		repo.On("Update", mock.Anything, mock.Anything).Return(svcerrors.New(svcerrors.Conflict, "update conflict")).Once()

		result, err := svc.ResizeInstance(ctx, "test-inst", "basic-4")

		// Conflict is treated as success since compute resize already happened
		require.NoError(t, err)
		assert.Equal(t, "basic-4", result.InstanceType)
	})

	t.Run("Failure_NonConflictDBUpdateWithRollbackFailure", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		rbacSvc := new(MockRBACService)
		tenantSvc := new(MockTenantService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			RBAC:             rbacSvc,
			TenantSvc:        tenantSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusRunning,
			InstanceType: "basic-2",
			ContainerID:  "cid-1",
			Version:      1,
		}

		oldType := &domain.InstanceType{ID: "basic-2", VCPUs: 2, MemoryMB: 2048}
		newType := &domain.InstanceType{ID: "basic-4", VCPUs: 4, MemoryMB: 4096}

		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceResize, "test-inst").Return(nil).Once()
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-2").Return(oldType, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-4").Return(newType, nil).Once()
		// Quota calls for upsize
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "memory", 2).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "memory", 2).Return(nil).Once()
		// Compute resize succeeds
		compute.On("ResizeInstance", mock.Anything, "cid-1", int64(4*1e9), int64(4096*1024*1024)).Return(nil).Once()
		// repo.Update returns Internal error (not Conflict - this tests DB failure scenario)
		repo.On("Update", mock.Anything, mock.Anything).Return(svcerrors.New(svcerrors.Internal, "db error")).Once()
		// Compute rollback FAILS
		compute.On("ResizeInstance", mock.Anything, "cid-1", int64(2*1e9), int64(2048*1024*1024)).Return(fmt.Errorf("libvirt error")).Once()
		// Quota rollback succeeds
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "memory", 2).Return(nil).Once()

		_, err := svc.ResizeInstance(ctx, "test-inst", "basic-4")

		// Should return error since DB update failed and rollback failed
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
		assert.Contains(t, err.Error(), "rollback")
	})

	t.Run("ComputeError", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		rbacSvc := new(MockRBACService)
		tenantSvc := new(MockTenantService)
		eventSvc := new(MockEventService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			RBAC:             rbacSvc,
			TenantSvc:        tenantSvc,
			EventSvc:         eventSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusRunning,
			InstanceType: "basic-2",
			ContainerID:  "cid-1",
		}

		oldType := &domain.InstanceType{ID: "basic-2", VCPUs: 2, MemoryMB: 2048}
		newType := &domain.InstanceType{ID: "basic-4", VCPUs: 4, MemoryMB: 4096}

		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceResize, "test-inst").Return(nil).Once()
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-2").Return(oldType, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-4").Return(newType, nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "memory", 2).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "memory", 2).Return(nil).Once()
		compute.On("ResizeInstance", mock.Anything, "cid-1", int64(4*1e9), int64(4096*1024*1024)).Return(fmt.Errorf("docker error")).Once()
		// Quota rollback when compute resize fails
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "vcpus", 2).Return(nil).Maybe()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "memory", 2).Return(nil).Maybe()

		_, err := svc.ResizeInstance(ctx, "test-inst", "basic-4")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resize instance")
		compute.AssertNotCalled(t, "ResizeInstance") // no compute rollback on compute failure, only quota rollback
		mock.AssertExpectationsForObjects(t, repo, typeRepo, compute, rbacSvc, tenantSvc, eventSvc)
	})

	t.Run("UpdateRecordError", func(t *testing.T) {
		repo := new(MockInstanceRepo)
		typeRepo := new(MockInstanceTypeRepo)
		compute := new(MockComputeBackend)
		rbacSvc := new(MockRBACService)
		tenantSvc := new(MockTenantService)
		eventSvc := new(MockEventService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			Repo:             repo,
			InstanceTypeRepo: typeRepo,
			Compute:          compute,
			RBAC:             rbacSvc,
			TenantSvc:        tenantSvc,
			EventSvc:         eventSvc,
			Logger:           slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		instanceID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		inst := &domain.Instance{
			ID:           instanceID,
			UserID:       userID,
			TenantID:     tenantID,
			Status:       domain.StatusRunning,
			InstanceType: "basic-2",
			ContainerID:  "cid-1",
		}

		oldType := &domain.InstanceType{ID: "basic-2", VCPUs: 2, MemoryMB: 2048}
		newType := &domain.InstanceType{ID: "basic-4", VCPUs: 4, MemoryMB: 4096}

		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceResize, "test-inst").Return(nil).Once()
		repo.On("GetByName", mock.Anything, "test-inst").Return(inst, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-2").Return(oldType, nil).Once()
		typeRepo.On("GetByID", mock.Anything, "basic-4").Return(newType, nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "vcpus", 2).Return(nil).Once()
		tenantSvc.On("CheckQuota", mock.Anything, tenantID, "memory", 2).Return(nil).Once()
		tenantSvc.On("IncrementUsage", mock.Anything, tenantID, "memory", 2).Return(nil).Once()
		compute.On("ResizeInstance", mock.Anything, "cid-1", int64(4*1e9), int64(4096*1024*1024)).Return(nil).Once()
		compute.On("ResizeInstance", mock.Anything, "cid-1", int64(2*1e9), int64(2048*1024*1024)).Return(nil).Maybe()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "vcpus", 2).Return(nil).Maybe()
		tenantSvc.On("DecrementUsage", mock.Anything, tenantID, "memory", 2).Return(nil).Maybe()
		repo.On("Update", mock.Anything, mock.Anything).Return(fmt.Errorf("db error")).Once()

		_, err := svc.ResizeInstance(ctx, "test-inst", "basic-4")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update instance record")
		mock.AssertExpectationsForObjects(t, repo, typeRepo, compute, rbacSvc, tenantSvc, eventSvc)
	})

	t.Run("Unauthorized", func(t *testing.T) {
		rbacSvc := new(MockRBACService)

		svc := services.NewInstanceService(services.InstanceServiceParams{
			RBAC:   rbacSvc,
			Logger: slog.Default(),
		})

		ctx := context.Background()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx = appcontext.WithUserID(ctx, userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)

		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionInstanceResize, "test-inst").Return(fmt.Errorf("access denied")).Once()

		_, err := svc.ResizeInstance(ctx, "test-inst", "basic-4")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
		rbacSvc.AssertExpectations(t)
	})
}
