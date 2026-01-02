package services

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mocks
type MockRepo struct {
	mock.Mock
}

func (m *MockRepo) Create(ctx context.Context, inst *domain.Instance) error {
	args := m.Called(ctx, inst)
	return args.Error(0)
}

func (m *MockRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Instance, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Instance), args.Error(1)
}

func (m *MockRepo) GetByName(ctx context.Context, name string) (*domain.Instance, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Instance), args.Error(1)
}

func (m *MockRepo) List(ctx context.Context) ([]*domain.Instance, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Instance), args.Error(1)
}

func (m *MockRepo) Update(ctx context.Context, inst *domain.Instance) error {
	args := m.Called(ctx, inst)
	return args.Error(0)
}

func (m *MockRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockVpcRepo struct {
	mock.Mock
}

func (m *MockVpcRepo) Create(ctx context.Context, vpc *domain.VPC) error {
	args := m.Called(ctx, vpc)
	return args.Error(0)
}

func (m *MockVpcRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.VPC, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VPC), args.Error(1)
}

func (m *MockVpcRepo) GetByName(ctx context.Context, name string) (*domain.VPC, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VPC), args.Error(1)
}

func (m *MockVpcRepo) List(ctx context.Context) ([]*domain.VPC, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.VPC), args.Error(1)
}

func (m *MockVpcRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockDocker struct {
	mock.Mock
}

func (m *MockDocker) CreateContainer(ctx context.Context, name, image string, ports []string, networkID string, volumeBinds []string) (string, error) {
	args := m.Called(ctx, name, image, ports, networkID, volumeBinds)
	return args.String(0), args.Error(1)
}

func (m *MockDocker) StopContainer(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockDocker) RemoveContainer(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockDocker) GetLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockDocker) CreateNetwork(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}

func (m *MockDocker) GetContainerStats(ctx context.Context, containerID string) (io.ReadCloser, error) {
	args := m.Called(ctx, containerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

type MockEventService struct {
	mock.Mock
}

func (m *MockEventService) RecordEvent(ctx context.Context, action, resourceID, resourceType string, metadata map[string]interface{}) error {
	args := m.Called(ctx, action, resourceID, resourceType, metadata)
	return args.Error(0)
}

func (m *MockEventService) ListEvents(ctx context.Context, limit int) ([]*domain.Event, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]*domain.Event), args.Error(1)
}

func (m *MockDocker) RemoveNetwork(ctx context.Context, networkID string) error {
	args := m.Called(ctx, networkID)
	return args.Error(0)
}

func (m *MockDocker) CreateVolume(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockDocker) DeleteVolume(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

type MockVolumeRepo struct {
	mock.Mock
}

func (m *MockVolumeRepo) Create(ctx context.Context, v *domain.Volume) error {
	args := m.Called(ctx, v)
	return args.Error(0)
}

func (m *MockVolumeRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Volume, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Volume), args.Error(1)
}

func (m *MockVolumeRepo) GetByName(ctx context.Context, name string) (*domain.Volume, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Volume), args.Error(1)
}

func (m *MockVolumeRepo) List(ctx context.Context) ([]*domain.Volume, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Volume), args.Error(1)
}

func (m *MockVolumeRepo) ListByInstanceID(ctx context.Context, id uuid.UUID) ([]*domain.Volume, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Volume), args.Error(1)
}

func (m *MockVolumeRepo) Update(ctx context.Context, v *domain.Volume) error {
	args := m.Called(ctx, v)
	return args.Error(0)
}

func (m *MockVolumeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Tests
func TestLaunchInstance_Success(t *testing.T) {
	repo := new(MockRepo)
	vpcRepo := new(MockVpcRepo)
	volumeRepo := new(MockVolumeRepo)
	docker := new(MockDocker)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewInstanceService(repo, vpcRepo, volumeRepo, docker, eventSvc, logger)

	ctx := context.Background()
	name := "test-inst"
	image := "alpine"
	ports := "8080:80"

	repo.On("Create", ctx, mock.AnythingOfType("*domain.Instance")).Return(nil)
	docker.On("CreateContainer", ctx, mock.Anything, image, []string{"8080:80"}, "", []string(nil)).Return("container-123", nil)
	repo.On("Update", ctx, mock.AnythingOfType("*domain.Instance")).Return(nil)
	eventSvc.On("RecordEvent", ctx, "INSTANCE_LAUNCH", mock.Anything, "INSTANCE", mock.Anything).Return(nil)

	inst, err := svc.LaunchInstance(ctx, name, image, ports, nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, name, inst.Name)
	assert.Equal(t, "container-123", inst.ContainerID)
	assert.Equal(t, domain.StatusRunning, inst.Status)
	repo.AssertExpectations(t)
	docker.AssertExpectations(t)
}

func TestLaunchInstance_PropagatesUserID(t *testing.T) {
	repo := new(MockRepo)
	vpcRepo := new(MockVpcRepo)
	volumeRepo := new(MockVolumeRepo)
	docker := new(MockDocker)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewInstanceService(repo, vpcRepo, volumeRepo, docker, eventSvc, logger)

	expectedUserID := uuid.New()
	ctx := appcontext.WithUserID(context.Background(), expectedUserID)
	name := "test-inst-user"
	image := "alpine"

	repo.On("Create", ctx, mock.MatchedBy(func(inst *domain.Instance) bool {
		return inst.UserID == expectedUserID
	})).Return(nil)
	docker.On("CreateContainer", ctx, mock.Anything, image, []string(nil), "", []string(nil)).Return("container-456", nil)
	repo.On("Update", ctx, mock.AnythingOfType("*domain.Instance")).Return(nil)
	eventSvc.On("RecordEvent", ctx, "INSTANCE_LAUNCH", mock.Anything, "INSTANCE", mock.Anything).Return(nil)

	inst, err := svc.LaunchInstance(ctx, name, image, "", nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, expectedUserID, inst.UserID)
	repo.AssertExpectations(t)
}

func TestTerminateInstance_Success(t *testing.T) {
	repo := new(MockRepo)
	vpcRepo := new(MockVpcRepo)
	volumeRepo := new(MockVolumeRepo)
	docker := new(MockDocker)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewInstanceService(repo, vpcRepo, volumeRepo, docker, eventSvc, logger)

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
	docker.On("RemoveContainer", ctx, "c123").Return(nil)
	volumeRepo.On("ListByInstanceID", ctx, id).Return(attachedVolumes, nil)
	volumeRepo.On("Update", ctx, mock.MatchedBy(func(v *domain.Volume) bool {
		return v.ID == volID &&
			v.Status == domain.VolumeStatusAvailable &&
			v.InstanceID == nil &&
			v.MountPath == ""
	})).Return(nil)
	repo.On("Delete", ctx, id).Return(nil)
	eventSvc.On("RecordEvent", ctx, "INSTANCE_TERMINATE", id.String(), "INSTANCE", mock.Anything).Return(nil)

	err := svc.TerminateInstance(ctx, id.String())

	assert.NoError(t, err)
	repo.AssertExpectations(t)
	docker.AssertExpectations(t)
	volumeRepo.AssertExpectations(t)
}

func TestTerminateInstance_RemoveContainerFails_DoesNotReleaseVolumes(t *testing.T) {
	repo := new(MockRepo)
	vpcRepo := new(MockVpcRepo)
	volumeRepo := new(MockVolumeRepo)
	docker := new(MockDocker)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewInstanceService(repo, vpcRepo, volumeRepo, docker, eventSvc, logger)

	ctx := context.Background()
	id := uuid.New()
	inst := &domain.Instance{ID: id, Name: "test", ContainerID: "c123"}

	repo.On("GetByID", ctx, id).Return(inst, nil)
	docker.On("RemoveContainer", ctx, "c123").Return(assert.AnError)

	err := svc.TerminateInstance(ctx, id.String())

	assert.Error(t, err)
	volumeRepo.AssertNotCalled(t, "ListByInstanceID", mock.Anything, id)
	repo.AssertNotCalled(t, "Delete", mock.Anything, id)
	eventSvc.AssertNotCalled(t, "RecordEvent", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestParseAndValidatePorts_RejectsInvalidPort(t *testing.T) {
	svc := &InstanceService{}

	_, err := svc.parseAndValidatePorts("80abc:90")

	assert.Error(t, err)
}
