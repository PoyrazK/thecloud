package services

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock repositories (local to dashboard_test as it is in package services)
type mockInstanceRepo struct {
	mock.Mock
}

func (m *mockInstanceRepo) Create(ctx context.Context, instance *domain.Instance) error {
	args := m.Called(ctx, instance)
	return args.Error(0)
}
func (m *mockInstanceRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Instance, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Instance), args.Error(1)
}
func (m *mockInstanceRepo) GetByName(ctx context.Context, name string) (*domain.Instance, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Instance), args.Error(1)
}
func (m *mockInstanceRepo) List(ctx context.Context) ([]*domain.Instance, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Instance), args.Error(1)
}
func (m *mockInstanceRepo) ListBySubnet(ctx context.Context, subnetID uuid.UUID) ([]*domain.Instance, error) {
	args := m.Called(ctx, subnetID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Instance), args.Error(1)
}
func (m *mockInstanceRepo) Update(ctx context.Context, instance *domain.Instance) error {
	// Mock implementation for Update
	args := m.Called(ctx, instance)
	return args.Error(0)
}
func (m *mockInstanceRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *mockInstanceRepo) ListByVPC(ctx context.Context, vpcID uuid.UUID) ([]*domain.Instance, error) {
	args := m.Called(ctx, vpcID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Instance), args.Error(1)
}

type mockVolumeRepo struct {
	mock.Mock
}

func (m *mockVolumeRepo) Create(ctx context.Context, v *domain.Volume) error {
	args := m.Called(ctx, v)
	return args.Error(0)
}
func (m *mockVolumeRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Volume, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Volume), args.Error(1)
}
func (m *mockVolumeRepo) GetByName(ctx context.Context, name string) (*domain.Volume, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Volume), args.Error(1)
}
func (m *mockVolumeRepo) List(ctx context.Context) ([]*domain.Volume, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Volume), args.Error(1)
}
func (m *mockVolumeRepo) ListByInstanceID(ctx context.Context, instanceID uuid.UUID) ([]*domain.Volume, error) {
	args := m.Called(ctx, instanceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Volume), args.Error(1)
}
func (m *mockVolumeRepo) Update(ctx context.Context, v *domain.Volume) error {
	// Mock implementation for Update
	args := m.Called(ctx, v)
	return args.Error(0)
}
func (m *mockVolumeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type mockVpcRepo struct {
	mock.Mock
}

func (m *mockVpcRepo) Create(ctx context.Context, vpc *domain.VPC) error {
	args := m.Called(ctx, vpc)
	return args.Error(0)
}
func (m *mockVpcRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.VPC, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VPC), args.Error(1)
}
func (m *mockVpcRepo) GetByName(ctx context.Context, name string) (*domain.VPC, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VPC), args.Error(1)
}
func (m *mockVpcRepo) List(ctx context.Context) ([]*domain.VPC, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VPC), args.Error(1)
}
func (m *mockVpcRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type mockEventRepo struct {
	mock.Mock
}

func (m *mockEventRepo) Create(ctx context.Context, event *domain.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}
func (m *mockEventRepo) List(ctx context.Context, limit int) ([]*domain.Event, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Event), args.Error(1)
}

func setupDashboardServiceTest(t *testing.T) (*mockInstanceRepo, *mockVolumeRepo, *mockVpcRepo, *mockEventRepo, ports.DashboardService) {
	instanceRepo := new(mockInstanceRepo)
	volumeRepo := new(mockVolumeRepo)
	vpcRepo := new(mockVpcRepo)
	eventRepo := new(mockEventRepo)
	svc := NewDashboardService(instanceRepo, volumeRepo, vpcRepo, eventRepo, slog.Default())
	return instanceRepo, volumeRepo, vpcRepo, eventRepo, svc
}

func TestDashboardServiceGetSummary(t *testing.T) {
	tests := []struct {
		name        string
		instances   []*domain.Instance
		volumes     []*domain.Volume
		vpcs        []*domain.VPC
		wantRunning int
		wantStopped int
		wantVolumes int
		wantVPCs    int
	}{
		{
			name: "mixed resources",
			instances: []*domain.Instance{
				{ID: uuid.New(), Status: domain.StatusRunning},
				{ID: uuid.New(), Status: domain.StatusRunning},
				{ID: uuid.New(), Status: domain.StatusStopped},
			},
			volumes: []*domain.Volume{
				{ID: uuid.New(), SizeGB: 10},
				{ID: uuid.New(), SizeGB: 20, InstanceID: func() *uuid.UUID { id := uuid.New(); return &id }()},
			},
			vpcs: []*domain.VPC{
				{ID: uuid.New()},
			},
			wantRunning: 2,
			wantStopped: 1,
			wantVolumes: 2,
			wantVPCs:    1,
		},
		{
			name:        "empty resources",
			instances:   []*domain.Instance{},
			volumes:     []*domain.Volume{},
			vpcs:        []*domain.VPC{},
			wantRunning: 0,
			wantStopped: 0,
			wantVolumes: 0,
			wantVPCs:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instanceRepo, volumeRepo, vpcRepo, _, svc := setupDashboardServiceTest(t)
			defer instanceRepo.AssertExpectations(t)
			defer volumeRepo.AssertExpectations(t)
			defer vpcRepo.AssertExpectations(t)

			instanceRepo.On("List", mock.Anything).Return(tt.instances, nil)
			volumeRepo.On("List", mock.Anything).Return(tt.volumes, nil)
			vpcRepo.On("List", mock.Anything).Return(tt.vpcs, nil)

			summary, err := svc.GetSummary(context.Background())

			assert.NoError(t, err)
			assert.Equal(t, tt.wantRunning, summary.RunningInstances)
			assert.Equal(t, tt.wantStopped, summary.StoppedInstances)
			assert.Equal(t, tt.wantVolumes, summary.TotalVolumes)
			assert.Equal(t, tt.wantVPCs, summary.TotalVPCs)
		})
	}
}

func TestDashboardServiceGetRecentEvents(t *testing.T) {
	_, _, _, eventRepo, svc := setupDashboardServiceTest(t)
	defer eventRepo.AssertExpectations(t)

	events := []*domain.Event{
		{ID: uuid.New(), Action: "INSTANCE_LAUNCH"},
		{ID: uuid.New(), Action: "VOLUME_CREATE"},
	}
	eventRepo.On("List", mock.Anything, 10).Return(events, nil)

	result, err := svc.GetRecentEvents(context.Background(), 10)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestDashboardServiceGetStats(t *testing.T) {
	instanceRepo, volumeRepo, vpcRepo, eventRepo, svc := setupDashboardServiceTest(t)
	defer instanceRepo.AssertExpectations(t)
	defer volumeRepo.AssertExpectations(t)
	defer vpcRepo.AssertExpectations(t)
	defer eventRepo.AssertExpectations(t)

	instanceRepo.On("List", mock.Anything).Return([]*domain.Instance{
		{ID: uuid.New(), Status: domain.StatusRunning},
	}, nil)
	volumeRepo.On("List", mock.Anything).Return([]*domain.Volume{}, nil)
	vpcRepo.On("List", mock.Anything).Return([]*domain.VPC{}, nil)

	events := []*domain.Event{{ID: uuid.New(), Action: "TEST"}}
	eventRepo.On("List", mock.Anything, 10).Return(events, nil)

	stats, err := svc.GetStats(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, 1, stats.Summary.RunningInstances)
	assert.Len(t, stats.RecentEvents, 1)
}
