package workers

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/mock"
)

type mockInstanceRepo struct {
	mock.Mock
}

func (m *mockInstanceRepo) Create(ctx context.Context, instance *domain.Instance) error {
	return m.Called(ctx, instance).Error(0)
}
func (m *mockInstanceRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Instance, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstanceRepo) GetByName(ctx context.Context, name string) (*domain.Instance, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstanceRepo) List(ctx context.Context, tagFilter []string) ([]*domain.Instance, error) {
	args := m.Called(ctx, tagFilter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstanceRepo) ListAll(ctx context.Context) ([]*domain.Instance, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstanceRepo) ListBySubnet(ctx context.Context, subnetID uuid.UUID) ([]*domain.Instance, error) {
	args := m.Called(ctx, subnetID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstanceRepo) Update(ctx context.Context, instance *domain.Instance) error {
	return m.Called(ctx, instance).Error(0)
}
func (m *mockInstanceRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

type mockInstanceSvc struct {
	mock.Mock
}

func (m *mockInstanceSvc) LaunchInstance(ctx context.Context, params ports.LaunchParams) (*domain.Instance, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstanceSvc) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (*domain.Instance, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstanceSvc) StartInstance(ctx context.Context, idOrName string) error {
	return m.Called(ctx, idOrName).Error(0)
}
func (m *mockInstanceSvc) StopInstance(ctx context.Context, idOrName string) error {
	return m.Called(ctx, idOrName).Error(0)
}
func (m *mockInstanceSvc) ListInstances(ctx context.Context, tagFilter []string) ([]*domain.Instance, error) {
	args := m.Called(ctx, tagFilter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstanceSvc) GetInstance(ctx context.Context, idOrName string) (*domain.Instance, error) {
	args := m.Called(ctx, idOrName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstanceSvc) GetInstanceLogs(ctx context.Context, idOrName string) (string, error) {
	args := m.Called(ctx, idOrName)
	return args.String(0), args.Error(1)
}
func (m *mockInstanceSvc) GetInstanceStats(ctx context.Context, idOrName string) (*domain.InstanceStats, error) {
	args := m.Called(ctx, idOrName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.InstanceStats)
	return r0, args.Error(1)
}
func (m *mockInstanceSvc) GetConsoleURL(ctx context.Context, idOrName string) (string, error) {
	args := m.Called(ctx, idOrName)
	return args.String(0), args.Error(1)
}
func (m *mockInstanceSvc) TerminateInstance(ctx context.Context, idOrName string) error {
	return m.Called(ctx, idOrName).Error(0)
}
func (m *mockInstanceSvc) Exec(ctx context.Context, idOrName string, cmd []string) (string, error) {
	args := m.Called(ctx, idOrName, cmd)
	return args.String(0), args.Error(1)
}
func (m *mockInstanceSvc) UpdateInstanceMetadata(ctx context.Context, id uuid.UUID, metadata, labels map[string]string) error {
	return m.Called(ctx, id, metadata, labels).Error(0)
}
func (m *mockInstanceSvc) ResizeInstance(ctx context.Context, idOrName, newInstanceType string) (*domain.Instance, error) {
	args := m.Called(ctx, idOrName, newInstanceType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Instance), args.Error(1)
}
func (m *mockInstanceSvc) PauseInstance(ctx context.Context, idOrName string) error {
	return m.Called(ctx, idOrName).Error(0)
}
func (m *mockInstanceSvc) ResumeInstance(ctx context.Context, idOrName string) error {
	return m.Called(ctx, idOrName).Error(0)
}
func (m *mockInstanceSvc) GetTags(ctx context.Context, id uuid.UUID) (map[string]string, error) {
	args := m.Called(ctx, id)
	var r0 map[string]string
	if args.Get(0) != nil {
		r0, _ = args.Get(0).(map[string]string)
	}
	return r0, args.Error(1)
}
func (m *mockInstanceSvc) SetTags(ctx context.Context, id uuid.UUID, labels map[string]string) error {
	return m.Called(ctx, id, labels).Error(0)
}
func (m *mockInstanceSvc) RemoveTag(ctx context.Context, id uuid.UUID, key string) error {
	return m.Called(ctx, id, key).Error(0)
}

func TestHealingWorker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		instances      []*domain.Instance
		listErr        error
		stopErr        error
		startErr       error
		expectedStops  []string
		expectedStarts []string
	}{
		{
			name: "Heal single errored instance",
			instances: []*domain.Instance{
				{ID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), Status: domain.StatusRunning},
				{ID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), Status: domain.StatusError, UserID: uuid.New(), TenantID: uuid.New()},
			},
			expectedStops:  []string{"00000000-0000-0000-0000-000000000002"},
			expectedStarts: []string{"00000000-0000-0000-0000-000000000002"},
		},
		{
			name:      "All instances healthy",
			instances: []*domain.Instance{{ID: uuid.New(), Status: domain.StatusRunning}},
		},
		{
			name:    "Repo list failure",
			listErr: fmt.Errorf("db error"),
		},
		{
			name: "Stop fails but start still attempted",
			instances: []*domain.Instance{
				{ID: uuid.MustParse("00000000-0000-0000-0000-000000000003"), Status: domain.StatusError, UserID: uuid.New(), TenantID: uuid.New()},
			},
			stopErr:        fmt.Errorf("stop failed"),
			expectedStops:  []string{"00000000-0000-0000-0000-000000000003"},
			expectedStarts: []string{"00000000-0000-0000-0000-000000000003"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mockInstanceRepo)
			svc := new(mockInstanceSvc)
			logger := slog.Default()

			worker := NewHealingWorker(svc, repo, logger)
			worker.healingDelay = 1 * time.Millisecond

			repo.On("ListAll", mock.Anything).Return(tt.instances, tt.listErr)

			for _, id := range tt.expectedStops {
				svc.On("StopInstance", mock.Anything, id).Return(tt.stopErr)
			}
			for _, id := range tt.expectedStarts {
				svc.On("StartInstance", mock.Anything, id).Return(tt.startErr)
			}

			worker.healERRORInstances(context.Background())
			worker.reconcileWG.Wait()

			repo.AssertExpectations(t)
			svc.AssertExpectations(t)
		})
	}
}

func TestHealingWorker_ContextCancelled_BeforeTimerFires(t *testing.T) {
	repo := new(mockInstanceRepo)
	svc := new(mockInstanceSvc)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	worker := NewHealingWorker(svc, repo, logger)
	worker.healingDelay = 100 * time.Millisecond // Short but won't fire before ctx cancel

	erroredInstance := &domain.Instance{
		ID:       uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Status:   domain.StatusError,
		UserID:   uuid.New(),
		TenantID: uuid.New(),
	}

	repo.On("ListAll", mock.Anything).Return([]*domain.Instance{erroredInstance}, nil)
	// StopInstance and StartInstance must NOT be called — ctx cancelled before timer fires
	svc.On("StopInstance", mock.Anything, "00000000-0000-0000-0000-000000000001").Return(nil).Maybe()
	svc.On("StartInstance", mock.Anything, "00000000-0000-0000-0000-000000000001").Return(nil).Maybe()

	ctx, cancel := context.WithCancel(context.Background())
	worker.healERRORInstances(ctx)

	// Cancel immediately — ctx.Done() should fire before the 100ms timer
	cancel()
	worker.reconcileWG.Wait()

	// Verify the goroutine hit ctx.Done() and bailed out before calling StopInstance or StartInstance
	svc.AssertNotCalled(t, "StopInstance", mock.Anything, "00000000-0000-0000-0000-000000000001")
	svc.AssertNotCalled(t, "StartInstance", mock.Anything, "00000000-0000-0000-0000-000000000001")
	repo.AssertExpectations(t)
}

func TestHealingWorker_Run(t *testing.T) {
	repo := new(mockInstanceRepo)
	svc := new(mockInstanceSvc)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	worker := NewHealingWorker(svc, repo, logger)
	worker.tickInterval = 1 * time.Millisecond

	repo.On("ListAll", mock.Anything).Return([]*domain.Instance{}, nil).Maybe()

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)

	go worker.Run(ctx, &wg)

	time.Sleep(5 * time.Millisecond)
	cancel()
	wg.Wait()

	repo.AssertExpectations(t)
}
