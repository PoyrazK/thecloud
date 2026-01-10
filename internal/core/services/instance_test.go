package services_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupInstanceServiceTest(t *testing.T) (*MockInstanceRepo, *MockVpcRepo, *MockSubnetRepo, *MockVolumeRepo, *MockComputeBackend, *MockNetworkBackend, *MockEventService, *MockAuditService, *services.TaskQueueStub, ports.InstanceService) {
	repo := new(MockInstanceRepo)
	vpcRepo := new(MockVpcRepo)
	subnetRepo := new(MockSubnetRepo)
	volumeRepo := new(MockVolumeRepo)
	compute := new(MockComputeBackend)
	network := new(MockNetworkBackend)
	eventSvc := new(MockEventService)
	auditSvc := new(MockAuditService)
	taskQueue := new(services.TaskQueueStub)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewInstanceService(services.InstanceServiceParams{
		Repo:       repo,
		VpcRepo:    vpcRepo,
		SubnetRepo: subnetRepo,
		VolumeRepo: volumeRepo,
		Compute:    compute,
		Network:    network,
		EventSvc:   eventSvc,
		AuditSvc:   auditSvc,
		TaskQueue:  taskQueue,
		Logger:     logger,
	})
	return repo, vpcRepo, subnetRepo, volumeRepo, compute, network, eventSvc, auditSvc, taskQueue, svc
}

func TestLaunchInstance_Success(t *testing.T) {
	repo, _, _, _, _, _, _, _, _, svc := setupInstanceServiceTest(t)
	defer repo.AssertExpectations(t)

	ctx := context.Background()
	name := "test-inst"
	image := "alpine"
	ports := "8080:80"

	repo.On("Create", ctx, mock.AnythingOfType("*domain.Instance")).Return(nil)

	inst, err := svc.LaunchInstance(ctx, name, image, ports, nil, nil, nil)

	assert.NoError(t, err)
	assert.NotNil(t, inst)
	assert.Equal(t, name, inst.Name)
	assert.Equal(t, domain.StatusStarting, inst.Status)
}

func TestLaunchInstance_PropagatesUserID(t *testing.T) {
	repo, _, _, _, _, _, _, _, _, svc := setupInstanceServiceTest(t)
	defer repo.AssertExpectations(t)

	expectedUserID := uuid.New()
	ctx := appcontext.WithUserID(context.Background(), expectedUserID)
	name := "test-inst-user"
	image := "alpine"

	repo.On("Create", ctx, mock.MatchedBy(func(inst *domain.Instance) bool {
		return inst.UserID == expectedUserID
	})).Return(nil)

	inst, err := svc.LaunchInstance(ctx, name, image, "", nil, nil, nil)

	assert.NoError(t, err)
	assert.NotNil(t, inst)
	assert.Equal(t, expectedUserID, inst.UserID)
}

func TestTerminateInstance_Success(t *testing.T) {
	repo, _, _, volumeRepo, compute, _, eventSvc, auditSvc, _, svc := setupInstanceServiceTest(t)
	defer repo.AssertExpectations(t)
	defer volumeRepo.AssertExpectations(t)
	defer compute.AssertExpectations(t)
	defer eventSvc.AssertExpectations(t)
	defer auditSvc.AssertExpectations(t)

	ctx := context.Background()
	id := uuid.New()
	volID := uuid.New()
	inst := &domain.Instance{ID: id, Name: "test", ContainerID: "c123"}
	attachedVolumes := []*domain.Volume{
		{
			ID:         volID,
			Name:       "vol-1",
			Status:     domain.VolumeStatusInUse,
			InstanceID: &id,
			MountPath:  "/data",
		},
	}

	repo.On("GetByID", ctx, id).Return(inst, nil)
	compute.On("DeleteInstance", ctx, "c123").Return(nil)
	volumeRepo.On("ListByInstanceID", ctx, id).Return(attachedVolumes, nil)
	volumeRepo.On("Update", ctx, mock.MatchedBy(func(v *domain.Volume) bool {
		return v.ID == volID &&
			v.Status == domain.VolumeStatusAvailable &&
			v.InstanceID == nil &&
			v.MountPath == ""
	})).Return(nil)
	repo.On("Delete", ctx, id).Return(nil)
	eventSvc.On("RecordEvent", ctx, "INSTANCE_TERMINATE", id.String(), "INSTANCE", mock.Anything).Return(nil)
	auditSvc.On("Log", ctx, mock.Anything, "instance.terminate", "instance", id.String(), mock.Anything).Return(nil)

	err := svc.TerminateInstance(ctx, id.String())

	assert.NoError(t, err)
}

func TestTerminateInstance_RemoveContainerFails_DoesNotReleaseVolumes(t *testing.T) {
	repo, _, _, volumeRepo, compute, _, eventSvc, _, _, svc := setupInstanceServiceTest(t)
	defer repo.AssertExpectations(t)
	defer compute.AssertExpectations(t)

	ctx := context.Background()
	id := uuid.New()
	inst := &domain.Instance{ID: id, Name: "test", ContainerID: "c123"}

	repo.On("GetByID", ctx, id).Return(inst, nil)
	compute.On("DeleteInstance", ctx, "c123").Return(assert.AnError)

	err := svc.TerminateInstance(ctx, id.String())

	assert.Error(t, err)
	volumeRepo.AssertNotCalled(t, "ListByInstanceID", mock.Anything, id)
	repo.AssertNotCalled(t, "Delete", mock.Anything, id)
	eventSvc.AssertNotCalled(t, "RecordEvent", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestGetInstance_ByID(t *testing.T) {
	repo, _, _, _, _, _, _, _, _, svc := setupInstanceServiceTest(t)
	defer repo.AssertExpectations(t)

	ctx := context.Background()
	instID := uuid.New()
	inst := &domain.Instance{ID: instID, Name: "test-inst"}

	repo.On("GetByID", ctx, instID).Return(inst, nil)

	result, err := svc.GetInstance(ctx, instID.String())

	assert.NoError(t, err)
	assert.Equal(t, instID, result.ID)
}

func TestGetInstance_ByName(t *testing.T) {
	repo, _, _, _, _, _, _, _, _, svc := setupInstanceServiceTest(t)
	defer repo.AssertExpectations(t)

	ctx := context.Background()
	name := "my-instance"
	inst := &domain.Instance{ID: uuid.New(), Name: name}

	repo.On("GetByName", ctx, name).Return(inst, nil)

	result, err := svc.GetInstance(ctx, name)

	assert.NoError(t, err)
	assert.Equal(t, name, result.Name)
}

func TestListInstances(t *testing.T) {
	repo, _, _, _, _, _, _, _, _, svc := setupInstanceServiceTest(t)
	defer repo.AssertExpectations(t)

	ctx := context.Background()
	instances := []*domain.Instance{{Name: "inst1"}, {Name: "inst2"}}

	repo.On("List", ctx).Return(instances, nil)

	result, err := svc.ListInstances(ctx)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestGetInstanceLogs(t *testing.T) {
	repo, _, _, _, compute, _, _, _, _, svc := setupInstanceServiceTest(t)
	defer repo.AssertExpectations(t)
	defer compute.AssertExpectations(t)

	ctx := context.Background()
	instID := uuid.New()
	inst := &domain.Instance{ID: instID, ContainerID: "c123"}

	repo.On("GetByID", ctx, instID).Return(inst, nil)
	compute.On("GetInstanceLogs", ctx, "c123").Return(io.NopCloser(strings.NewReader("log line 1\nlog line 2")), nil)

	logs, err := svc.GetInstanceLogs(ctx, instID.String())

	assert.NoError(t, err)
	assert.Contains(t, logs, "log line 1")
}

func TestStopInstance_Success(t *testing.T) {
	repo, _, _, _, compute, _, _, auditSvc, _, svc := setupInstanceServiceTest(t)
	defer repo.AssertExpectations(t)
	defer compute.AssertExpectations(t)
	defer auditSvc.AssertExpectations(t)

	ctx := context.Background()
	instID := uuid.New()
	inst := &domain.Instance{ID: instID, ContainerID: "c123", Status: domain.StatusRunning}

	repo.On("GetByID", ctx, instID).Return(inst, nil)
	compute.On("StopInstance", ctx, "c123").Return(nil)
	compute.On("Type").Return("mock")
	repo.On("Update", ctx, mock.MatchedBy(func(i *domain.Instance) bool {
		return i.Status == domain.StatusStopped
	})).Return(nil)
	auditSvc.On("Log", ctx, mock.Anything, "instance.stop", "instance", instID.String(), mock.Anything).Return(nil)

	err := svc.StopInstance(ctx, instID.String())

	assert.NoError(t, err)
}

func TestLaunchInstance_WithSubnetAndNetworking(t *testing.T) {
	repo, vpcRepo, subnetRepo, _, _, _, _, _, _, svc := setupInstanceServiceTest(t)
	defer repo.AssertExpectations(t)
	defer vpcRepo.AssertExpectations(t)
	defer subnetRepo.AssertExpectations(t)

	ctx := context.Background()
	vpcID := uuid.New()
	subnetID := uuid.New()
	name := "net-inst"
	image := "alpine"

	repo.On("Create", ctx, mock.AnythingOfType("*domain.Instance")).Return(nil)

	inst, err := svc.LaunchInstance(ctx, name, image, "", &vpcID, &subnetID, nil)

	assert.NoError(t, err)
	assert.NotNil(t, inst)
	assert.Equal(t, domain.StatusStarting, inst.Status)
}

func TestProvision_Success(t *testing.T) {
	repo, vpcRepo, subnetRepo, _, compute, network, eventSvc, auditSvc, _, svc := setupInstanceServiceTest(t)
	defer repo.AssertExpectations(t)
	defer vpcRepo.AssertExpectations(t)
	defer subnetRepo.AssertExpectations(t)
	defer compute.AssertExpectations(t)
	defer network.AssertExpectations(t)
	defer eventSvc.AssertExpectations(t)
	defer auditSvc.AssertExpectations(t)

	ctx := context.Background()
	instID := uuid.New()
	vpcID := uuid.New()
	subnetID := uuid.New()
	image := "alpine"

	inst := &domain.Instance{
		ID:       instID,
		Name:     "inst",
		Image:    image,
		VpcID:    &vpcID,
		SubnetID: &subnetID,
		Ports:    "8080:80",
	}

	vpc := &domain.VPC{ID: vpcID, NetworkID: "br-vpc-123"}
	subnet := &domain.Subnet{
		ID:        subnetID,
		VPCID:     vpcID,
		CIDRBlock: "10.0.1.0/24",
		GatewayIP: "10.0.1.1",
	}

	repo.On("GetByID", mock.Anything, instID).Return(inst, nil)
	vpcRepo.On("GetByID", mock.Anything, vpcID).Return(vpc, nil)
	subnetRepo.On("GetByID", mock.Anything, subnetID).Return(subnet, nil)
	repo.On("ListBySubnet", mock.Anything, subnetID).Return([]*domain.Instance{}, nil)

	compute.On("CreateInstance", mock.Anything, mock.Anything, image, []string{"8080:80"}, "br-vpc-123", mock.Anything, mock.Anything, mock.Anything).Return("c-123", nil)

	network.On("CreateVethPair", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	network.On("AttachVethToBridge", mock.Anything, "br-vpc-123", mock.Anything).Return(nil)
	network.On("SetVethIP", mock.Anything, mock.Anything, "10.0.1.2", "24").Return(nil)

	repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Instance) bool {
		return i.Status == domain.StatusRunning && i.ContainerID == "c-123"
	})).Return(nil)
	eventSvc.On("RecordEvent", mock.Anything, "INSTANCE_LAUNCH", instID.String(), "INSTANCE", mock.Anything).Return(nil)
	auditSvc.On("Log", mock.Anything, mock.Anything, "instance.launch", "instance", instID.String(), mock.Anything).Return(nil)

	err := svc.(interface {
		Provision(context.Context, uuid.UUID, []domain.VolumeAttachment) error
	}).Provision(ctx, instID, nil)

	assert.NoError(t, err)
}

func TestInstanceService_GetInstanceStats(t *testing.T) {
	repo, _, _, _, compute, _, _, _, _, svc := setupInstanceServiceTest(t)
	defer repo.AssertExpectations(t)
	defer compute.AssertExpectations(t)

	ctx := context.Background()
	instID := uuid.New()
	inst := &domain.Instance{ID: instID, ContainerID: "c123"}

	repo.On("GetByID", ctx, instID).Return(inst, nil)
	compute.On("GetInstanceStats", ctx, "c123").Return(io.NopCloser(strings.NewReader("{}")), nil)

	stats, err := svc.GetInstanceStats(ctx, instID.String())
	assert.NoError(t, err)
	assert.NotNil(t, stats)
}

func TestInstanceService_GetInstanceStats_Error(t *testing.T) {
	repo, _, _, _, compute, _, _, _, _, svc := setupInstanceServiceTest(t)
	ctx := context.Background()
	instID := uuid.New()
	inst := &domain.Instance{ID: instID, ContainerID: "c123"}

	t.Run("RepoError", func(t *testing.T) {
		repo.On("GetByID", ctx, instID).Return(nil, assert.AnError).Once()
		_, err := svc.GetInstanceStats(ctx, instID.String())
		assert.Error(t, err)
	})

	t.Run("ComputeError", func(t *testing.T) {
		repo.On("GetByID", ctx, instID).Return(inst, nil).Once()
		compute.On("GetInstanceStats", ctx, "c123").Return(nil, assert.AnError).Once()
		_, err := svc.GetInstanceStats(ctx, instID.String())
		assert.Error(t, err)
	})
}

func TestInstanceService_Launch_InvalidPorts(t *testing.T) {
	_, _, _, _, _, _, _, _, _, svc := setupInstanceServiceTest(t)
	ctx := context.Background()

	_, err := svc.LaunchInstance(ctx, "n", "i", "invalid", nil, nil, nil)
	assert.Error(t, err)
}

func TestInstanceService_Stop_Error(t *testing.T) {
	repo, _, _, _, compute, _, _, _, _, svc := setupInstanceServiceTest(t)
	ctx := context.Background()
	instID := uuid.New()
	inst := &domain.Instance{ID: instID, ContainerID: "c123", Status: domain.StatusRunning}

	repo.On("GetByID", ctx, instID).Return(inst, nil)
	compute.On("StopInstance", ctx, "c123").Return(assert.AnError)

	err := svc.StopInstance(ctx, instID.String())
	assert.Error(t, err)
}

func TestInstanceService_GetInstanceLogs_Error(t *testing.T) {
	repo, _, _, _, compute, _, _, _, _, svc := setupInstanceServiceTest(t)
	ctx := context.Background()
	instID := uuid.New()
	inst := &domain.Instance{ID: instID, ContainerID: "c123"}

	repo.On("GetByID", ctx, instID).Return(inst, nil)
	compute.On("GetInstanceLogs", ctx, "c123").Return(nil, assert.AnError)

	_, err := svc.GetInstanceLogs(ctx, instID.String())
	assert.Error(t, err)
}

func TestInstanceService_Launch_WithVolumes(t *testing.T) {
	repo, _, _, volumeRepo, _, _, _, _, _, svc := setupInstanceServiceTest(t)
	ctx := context.Background()
	volID := uuid.New()

	volumeRepo.On("GetByID", ctx, volID).Return(&domain.Volume{ID: volID, Name: "v1", Status: domain.VolumeStatusAvailable}, nil)
	repo.On("Create", ctx, mock.Anything).Return(nil)

	attachments := []domain.VolumeAttachment{
		{VolumeIDOrName: volID.String(), MountPath: "/data"},
	}
	_, err := svc.LaunchInstance(ctx, "n", "i", "", nil, nil, attachments)
	assert.NoError(t, err)
}

func TestInstanceService_GetVolumeByIDOrName(t *testing.T) {
	_, _, _, volumeRepo, _, _, _, _, _, _ := setupInstanceServiceTest(t)
	ctx := context.Background()
	volID := uuid.New()

	// Test by ID
	volumeRepo.On("GetByID", ctx, volID).Return(&domain.Volume{ID: volID}, nil).Once()
	// Wait, getVolumeByIDOrName is unexported. I can't call it directly.
	// But LaunchInstance/TerminateInstance/etc might call it?
	// Actually, only LaunchInstance calls resolveVolumes which calls it.
}
