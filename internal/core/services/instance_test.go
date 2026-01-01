package services

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/poyraz/cloud/internal/core/domain"
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

func (m *MockDocker) CreateContainer(ctx context.Context, name, image string, ports []string, networkID string) (string, error) {
	args := m.Called(ctx, name, image, ports, networkID)
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

func (m *MockDocker) RemoveNetwork(ctx context.Context, networkID string) error {
	args := m.Called(ctx, networkID)
	return args.Error(0)
}

// Tests
func TestLaunchInstance_Success(t *testing.T) {
	repo := new(MockRepo)
	vpcRepo := new(MockVpcRepo)
	docker := new(MockDocker)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewInstanceService(repo, vpcRepo, docker, logger)

	ctx := context.Background()
	name := "test-inst"
	image := "alpine"
	ports := "8080:80"

	repo.On("Create", ctx, mock.AnythingOfType("*domain.Instance")).Return(nil)
	docker.On("CreateContainer", ctx, mock.Anything, image, []string{"8080:80"}, "").Return("container-123", nil)
	repo.On("Update", ctx, mock.AnythingOfType("*domain.Instance")).Return(nil)

	inst, err := svc.LaunchInstance(ctx, name, image, ports, nil)

	assert.NoError(t, err)
	assert.Equal(t, name, inst.Name)
	assert.Equal(t, "container-123", inst.ContainerID)
	assert.Equal(t, domain.StatusRunning, inst.Status)
	repo.AssertExpectations(t)
	docker.AssertExpectations(t)
}

func TestTerminateInstance_Success(t *testing.T) {
	repo := new(MockRepo)
	vpcRepo := new(MockVpcRepo)
	docker := new(MockDocker)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewInstanceService(repo, vpcRepo, docker, logger)

	ctx := context.Background()
	id := uuid.New()
	inst := &domain.Instance{ID: id, Name: "test", ContainerID: "c123"}

	repo.On("GetByID", ctx, id).Return(inst, nil)
	docker.On("RemoveContainer", ctx, "c123").Return(nil)
	repo.On("Delete", ctx, id).Return(nil)

	err := svc.TerminateInstance(ctx, id.String())

	assert.NoError(t, err)
	repo.AssertExpectations(t)
	docker.AssertExpectations(t)
}
