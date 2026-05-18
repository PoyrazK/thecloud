package workers

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/mock"
)

type mockPipelineRepo struct {
	mock.Mock
}

func (m *mockPipelineRepo) CreatePipeline(ctx context.Context, p *domain.Pipeline) error {
	return m.Called(ctx, p).Error(0)
}
func (m *mockPipelineRepo) GetPipeline(ctx context.Context, id, userID uuid.UUID) (*domain.Pipeline, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Pipeline), args.Error(1)
}
func (m *mockPipelineRepo) GetPipelineByID(ctx context.Context, id uuid.UUID) (*domain.Pipeline, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Pipeline), args.Error(1)
}
func (m *mockPipelineRepo) ListPipelines(ctx context.Context, userID uuid.UUID) ([]*domain.Pipeline, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*domain.Pipeline), args.Error(1)
}
func (m *mockPipelineRepo) UpdatePipeline(ctx context.Context, p *domain.Pipeline) error {
	return m.Called(ctx, p).Error(0)
}
func (m *mockPipelineRepo) DeletePipeline(ctx context.Context, id, userID uuid.UUID) error {
	return m.Called(ctx, id, userID).Error(0)
}
func (m *mockPipelineRepo) CreateBuild(ctx context.Context, b *domain.Build) error {
	return m.Called(ctx, b).Error(0)
}
func (m *mockPipelineRepo) GetBuild(ctx context.Context, id, userID uuid.UUID) (*domain.Build, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Build), args.Error(1)
}
func (m *mockPipelineRepo) ListBuildsByPipeline(ctx context.Context, pipelineID, userID uuid.UUID) ([]*domain.Build, error) {
	args := m.Called(ctx, pipelineID, userID)
	return args.Get(0).([]*domain.Build), args.Error(1)
}
func (m *mockPipelineRepo) UpdateBuild(ctx context.Context, b *domain.Build) error {
	return m.Called(ctx, b).Error(0)
}
func (m *mockPipelineRepo) CreateBuildStep(ctx context.Context, s *domain.BuildStep) error {
	return m.Called(ctx, s).Error(0)
}
func (m *mockPipelineRepo) UpdateBuildStep(ctx context.Context, s *domain.BuildStep) error {
	return m.Called(ctx, s).Error(0)
}
func (m *mockPipelineRepo) ListBuildSteps(ctx context.Context, buildID, userID uuid.UUID) ([]*domain.BuildStep, error) {
	args := m.Called(ctx, buildID, userID)
	return args.Get(0).([]*domain.BuildStep), args.Error(1)
}
func (m *mockPipelineRepo) AppendBuildLog(ctx context.Context, l *domain.BuildLog) error {
	return m.Called(ctx, l).Error(0)
}
func (m *mockPipelineRepo) GetBuildLogs(ctx context.Context, buildID uuid.UUID) ([]*domain.BuildLog, error) {
	args := m.Called(ctx, buildID)
	return args.Get(0).([]*domain.BuildLog), args.Error(1)
}
func (m *mockPipelineRepo) ListBuildLogs(ctx context.Context, buildID, userID uuid.UUID, limit int) ([]*domain.BuildLog, error) {
	args := m.Called(ctx, buildID, userID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.BuildLog), args.Error(1)
}
func (m *mockPipelineRepo) ReserveWebhookDelivery(ctx context.Context, pipelineID uuid.UUID, provider, event, deliveryID string) (bool, error) {
	args := m.Called(ctx, pipelineID, provider, event, deliveryID)
	return args.Bool(0), args.Error(1)
}

type mockComputeBackendExtended struct {
	mock.Mock
}

func (m *mockComputeBackendExtended) Type() string { return "mock" }
func (m *mockComputeBackendExtended) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (string, []string, error) {
	args := m.Called(ctx, opts)
	return args.String(0), args.Get(1).([]string), args.Error(2)
}
func (m *mockComputeBackendExtended) StartInstance(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockComputeBackendExtended) StopInstance(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockComputeBackendExtended) DeleteInstance(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockComputeBackendExtended) GetInstanceLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
func (m *mockComputeBackendExtended) GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error) {
	return nil, nil
}
func (m *mockComputeBackendExtended) GetInstancePort(ctx context.Context, id, port string) (int, error) {
	args := m.Called(ctx, id, port)
	return args.Int(0), args.Error(1)
}
func (m *mockComputeBackendExtended) GetInstanceIP(ctx context.Context, id string) (string, error) {
	ret := m.Called(ctx, id)
	return ret.String(0), ret.Error(1)
}
func (m *mockComputeBackendExtended) GetConsoleURL(ctx context.Context, id string) (string, error) {
	return "", nil
}
func (m *mockComputeBackendExtended) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	args := m.Called(ctx, id, cmd)
	return args.String(0), args.Error(1)
}
func (m *mockComputeBackendExtended) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, []string, error) {
	args := m.Called(ctx, opts)
	return args.String(0), args.Get(1).([]string), args.Error(2)
}
func (m *mockComputeBackendExtended) WaitTask(ctx context.Context, id string) (int64, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(int64), args.Error(1)
}
func (m *mockComputeBackendExtended) CreateNetwork(ctx context.Context, name string) (string, error) {
	return "", nil
}
func (m *mockComputeBackendExtended) DeleteNetwork(ctx context.Context, id string) error {
	return nil
}
func (m *mockComputeBackendExtended) AttachVolume(ctx context.Context, id, volumePath string) (string, string, error) {
	args := m.Called(ctx, id, volumePath)
	return args.String(0), args.String(1), args.Error(2)
}
func (m *mockComputeBackendExtended) DetachVolume(ctx context.Context, id, volumePath string) (string, error) {
	args := m.Called(ctx, id, volumePath)
	return args.String(0), args.Error(1)
}
func (m *mockComputeBackendExtended) Ping(ctx context.Context) error {
	return nil
}
func (m *mockComputeBackendExtended) ResizeInstance(ctx context.Context, id string, cpu, memory int64) error {
	return m.Called(ctx, id, cpu, memory).Error(0)
}
func (m *mockComputeBackendExtended) PauseInstance(ctx context.Context, id string) error {
	return nil
}
func (m *mockComputeBackendExtended) ResumeInstance(ctx context.Context, id string) error {
	return nil
}
func (m *mockComputeBackendExtended) CreateSnapshot(ctx context.Context, id, name string) error {
	return m.Called(ctx, id, name).Error(0)
}
func (m *mockComputeBackendExtended) RestoreSnapshot(ctx context.Context, id, name string) error {
	return m.Called(ctx, id, name).Error(0)
}
func (m *mockComputeBackendExtended) DeleteSnapshot(ctx context.Context, id, name string) error {
	return m.Called(ctx, id, name).Error(0)
}
func (m *mockComputeBackendExtended) ResetCircuitBreaker() {}

func TestPipelineWorker_processJob(t *testing.T) {
	repo := new(mockPipelineRepo)
	compute := new(mockComputeBackendExtended)
	taskQueue := new(MockTaskQueue)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	worker := NewPipelineWorker(repo, taskQueue, nil, compute, logger)

	buildID := uuid.New()
	pipelineID := uuid.New()
	userID := uuid.New()
	job := domain.BuildJob{BuildID: buildID, PipelineID: pipelineID, UserID: userID}
	msg := &ports.DurableMessage{ID: "1-0", Queue: pipelineQueueName}

	t.Run("Success", func(t *testing.T) {
		build := &domain.Build{ID: buildID, PipelineID: pipelineID, UserID: userID}
		pipeline := &domain.Pipeline{
			ID: pipelineID,
			Config: domain.PipelineConfig{
				Stages: []domain.PipelineStage{
					{
						Name: "Test",
						Steps: []domain.PipelineStep{
							{Name: "step1", Image: "alpine", Commands: []string{"echo hi"}},
						},
					},
				},
			},
		}

		repo.On("GetBuild", mock.Anything, buildID, userID).Return(build, nil).Once()
		repo.On("GetPipeline", mock.Anything, pipelineID, userID).Return(pipeline, nil).Once()
		repo.On("UpdateBuild", mock.Anything, mock.MatchedBy(func(b *domain.Build) bool {
			return b.Status == domain.BuildStatusRunning
		})).Return(nil).Once()
		repo.On("CreateBuildStep", mock.Anything, mock.Anything).Return(nil).Once()

		compute.On("RunTask", mock.Anything, mock.MatchedBy(func(opts ports.RunTaskOptions) bool {
			return opts.Image == "alpine" && strings.Contains(strings.Join(opts.Command, " "), "echo hi")
		})).Return("task-1", []string{}, nil).Once()
		compute.On("WaitTask", mock.Anything, "task-1").Return(int64(0), nil).Once()
		compute.On("GetInstanceLogs", mock.Anything, "task-1").Return(io.NopCloser(strings.NewReader("logs")), nil).Once()
		compute.On("DeleteInstance", mock.Anything, "task-1").Return(nil).Once()

		repo.On("AppendBuildLog", mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("UpdateBuildStep", mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("UpdateBuild", mock.Anything, mock.MatchedBy(func(b *domain.Build) bool {
			return b.Status == domain.BuildStatusSucceeded
		})).Return(nil).Once()
		taskQueue.On("Ack", mock.Anything, pipelineQueueName, pipelineGroup, msg.ID).Return(nil).Once()

		worker.processJob(context.Background(), msg, job)
		repo.AssertExpectations(t)
		compute.AssertExpectations(t)
		taskQueue.AssertExpectations(t)
	})

	t.Run("DeleteInstanceCleanupFailure_IsLogged", func(t *testing.T) {
		build := &domain.Build{ID: buildID, PipelineID: pipelineID, UserID: userID}
		pipeline := &domain.Pipeline{
			ID: pipelineID,
			Config: domain.PipelineConfig{
				Stages: []domain.PipelineStage{
					{
						Name: "Test",
						Steps: []domain.PipelineStep{
							{Name: "step1", Image: "alpine", Commands: []string{"echo hi"}},
						},
					},
				},
			},
		}

		repo.On("GetBuild", mock.Anything, buildID, userID).Return(build, nil).Once()
		repo.On("GetPipeline", mock.Anything, pipelineID, userID).Return(pipeline, nil).Once()
		repo.On("UpdateBuild", mock.Anything, mock.MatchedBy(func(b *domain.Build) bool {
			return b.Status == domain.BuildStatusRunning
		})).Return(nil).Once()
		repo.On("CreateBuildStep", mock.Anything, mock.Anything).Return(nil).Once()

		compute.On("RunTask", mock.Anything, mock.MatchedBy(func(opts ports.RunTaskOptions) bool {
			return opts.Image == "alpine" && strings.Contains(strings.Join(opts.Command, " "), "echo hi")
		})).Return("task-1", []string{}, nil).Once()
		compute.On("WaitTask", mock.Anything, "task-1").Return(int64(0), nil).Once()
		compute.On("GetInstanceLogs", mock.Anything, "task-1").Return(io.NopCloser(strings.NewReader("logs")), nil).Once()
		compute.On("DeleteInstance", mock.Anything, "task-1").Return(context.DeadlineExceeded).Once()

		repo.On("AppendBuildLog", mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("UpdateBuildStep", mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("UpdateBuild", mock.Anything, mock.MatchedBy(func(b *domain.Build) bool {
			return b.Status == domain.BuildStatusSucceeded
		})).Return(nil).Once()
		taskQueue.On("Ack", mock.Anything, pipelineQueueName, pipelineGroup, msg.ID).Return(nil).Once()

		// Should not panic — cleanup error is logged, not propagated
		worker.processJob(context.Background(), msg, job)
		compute.AssertExpectations(t)
	})
}
