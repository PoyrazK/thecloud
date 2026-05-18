package workers

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockDatabaseRepo struct {
	mock.Mock
}

func (m *mockDatabaseRepo) Create(ctx context.Context, db *domain.Database) error {
	return m.Called(ctx, db).Error(0)
}
func (m *mockDatabaseRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Database, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Database), args.Error(1)
}
func (m *mockDatabaseRepo) List(ctx context.Context) ([]*domain.Database, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Database), args.Error(1)
}
func (m *mockDatabaseRepo) ListReplicas(ctx context.Context, primaryID uuid.UUID) ([]*domain.Database, error) {
	args := m.Called(ctx, primaryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Database), args.Error(1)
}
func (m *mockDatabaseRepo) Update(ctx context.Context, db *domain.Database) error {
	return m.Called(ctx, db).Error(0)
}
func (m *mockDatabaseRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

type mockDatabaseService struct {
	mock.Mock
}

func (m *mockDatabaseService) CreateDatabase(ctx context.Context, req ports.CreateDatabaseRequest) (*domain.Database, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Database), args.Error(1)
}
func (m *mockDatabaseService) CreateReplica(ctx context.Context, primaryID uuid.UUID, name string) (*domain.Database, error) {
	args := m.Called(ctx, primaryID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Database), args.Error(1)
}
func (m *mockDatabaseService) PromoteToPrimary(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockDatabaseService) GetDatabase(ctx context.Context, id uuid.UUID) (*domain.Database, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Database), args.Error(1)
}
func (m *mockDatabaseService) ListDatabases(ctx context.Context) ([]*domain.Database, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Database), args.Error(1)
}
func (m *mockDatabaseService) DeleteDatabase(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockDatabaseService) ModifyDatabase(ctx context.Context, req ports.ModifyDatabaseRequest) (*domain.Database, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Database), args.Error(1)
}
func (m *mockDatabaseService) GetConnectionString(ctx context.Context, id uuid.UUID) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}
func (m *mockDatabaseService) CreateDatabaseSnapshot(ctx context.Context, databaseID uuid.UUID, description string) (*domain.Snapshot, error) {
	args := m.Called(ctx, databaseID, description)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Snapshot), args.Error(1)
}
func (m *mockDatabaseService) ListDatabaseSnapshots(ctx context.Context, databaseID uuid.UUID) ([]*domain.Snapshot, error) {
	args := m.Called(ctx, databaseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Snapshot), args.Error(1)
}
func (m *mockDatabaseService) RestoreDatabase(ctx context.Context, req ports.RestoreDatabaseRequest) (*domain.Database, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Database), args.Error(1)
}
func (m *mockDatabaseService) RotateCredentials(ctx context.Context, id uuid.UUID, idempotencyKey string) error {
	args := m.Called(ctx, id, idempotencyKey)
	return args.Error(0)
}
func (m *mockDatabaseService) StopDatabase(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *mockDatabaseService) StartDatabase(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *mockDatabaseService) ResizeDatabase(ctx context.Context, id uuid.UUID, newSizeGB int) error {
	return m.Called(ctx, id, newSizeGB).Error(0)
}

type mockComputeBackend struct {
	mock.Mock
}

func (m *mockComputeBackend) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (string, []string, error) {
	args := m.Called(ctx, opts)
	return args.String(0), args.Get(1).([]string), args.Error(2)
}
func (m *mockComputeBackend) StartInstance(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockComputeBackend) StopInstance(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockComputeBackend) DeleteInstance(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockComputeBackend) GetInstanceLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
func (m *mockComputeBackend) GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
func (m *mockComputeBackend) GetInstancePort(ctx context.Context, id string, internalPort string) (int, error) {
	args := m.Called(ctx, id, internalPort)
	return args.Int(0), args.Error(1)
}
func (m *mockComputeBackend) GetInstanceIP(ctx context.Context, id string) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}
func (m *mockComputeBackend) GetConsoleURL(ctx context.Context, id string) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}
func (m *mockComputeBackend) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	args := m.Called(ctx, id, cmd)
	return args.String(0), args.Error(1)
}
func (m *mockComputeBackend) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, []string, error) {
	args := m.Called(ctx, opts)
	return args.String(0), args.Get(1).([]string), args.Error(2)
}
func (m *mockComputeBackend) WaitTask(ctx context.Context, id string) (int64, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(int64), args.Error(1)
}
func (m *mockComputeBackend) CreateNetwork(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}
func (m *mockComputeBackend) DeleteNetwork(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockComputeBackend) AttachVolume(ctx context.Context, id string, volumePath string) (string, string, error) {
	args := m.Called(ctx, id, volumePath)
	return args.String(0), args.String(1), args.Error(2)
}
func (m *mockComputeBackend) DetachVolume(ctx context.Context, id string, volumePath string) (string, error) {
	args := m.Called(ctx, id, volumePath)
	return args.String(0), args.Error(1)
}
func (m *mockComputeBackend) Ping(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}
func (m *mockComputeBackend) PauseInstance(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockComputeBackend) ResumeInstance(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockComputeBackend) ResizeInstance(ctx context.Context, id string, cpu, memory int64) error {
	return m.Called(ctx, id, cpu, memory).Error(0)
}
func (m *mockComputeBackend) Type() string {
	return "mock"
}
func (m *mockComputeBackend) CreateSnapshot(ctx context.Context, id, name string) error {
	return m.Called(ctx, id, name).Error(0)
}
func (m *mockComputeBackend) RestoreSnapshot(ctx context.Context, id, name string) error {
	return m.Called(ctx, id, name).Error(0)
}
func (m *mockComputeBackend) DeleteSnapshot(ctx context.Context, id, name string) error {
	return m.Called(ctx, id, name).Error(0)
}
func (m *mockComputeBackend) ResetCircuitBreaker() {}

func TestDatabaseFailoverWorker(t *testing.T) {
	t.Parallel()

	primaryID := uuid.New()
	replicaID := uuid.New()

	primary := &domain.Database{
		ID:     primaryID,
		Name:   "primary",
		Role:   domain.RolePrimary,
		Status: domain.DatabaseStatusRunning,
		Port:   1234, // Default unhealthy port
	}

	replica := &domain.Database{
		ID:          replicaID,
		Name:        "replica",
		Role:        domain.RoleReplica,
		PrimaryID:   &primaryID,
		Status:      domain.DatabaseStatusRunning,
		Engine:      domain.EnginePostgres,
		ContainerID: "container-1",
	}

	anyCtx := mock.MatchedBy(func(ctx context.Context) bool { return true })

	t.Run("Failover triggered on unhealthy primary", func(t *testing.T) {
		repo := new(mockDatabaseRepo)
		svc := new(mockDatabaseService)
		compute := new(mockComputeBackend)
		logger := slog.Default()

		worker := NewDatabaseFailoverWorker(svc, repo, compute, logger)

		repo.On("List", anyCtx).Return([]*domain.Database{primary}, nil)
		repo.On("ListReplicas", anyCtx, primaryID).Return([]*domain.Database{replica}, nil)
		compute.On("Exec", anyCtx, "container-1", mock.Anything).Return("1\n", nil) // low lag
		svc.On("PromoteToPrimary", anyCtx, replicaID).Return(nil)

		worker.checkDatabases(context.Background())

		repo.AssertExpectations(t)
		svc.AssertExpectations(t)
		compute.AssertExpectations(t)
	})

	t.Run("No failover if primary healthy", func(t *testing.T) {
		repo := new(mockDatabaseRepo)
		svc := new(mockDatabaseService)
		compute := new(mockComputeBackend)
		logger := slog.Default()

		healthyPrimary := *primary
		healthyPrimary.Port = 0 // port 0 is always healthy in our worker for simulation

		worker := NewDatabaseFailoverWorker(svc, repo, compute, logger)

		repo.On("List", mock.Anything).Return([]*domain.Database{&healthyPrimary}, nil)

		worker.checkDatabases(context.Background())

		repo.AssertExpectations(t)
		svc.AssertNotCalled(t, "PromoteToPrimary", mock.Anything, mock.Anything)
	})

	t.Run("No failover if no replicas", func(t *testing.T) {
		repo := new(mockDatabaseRepo)
		svc := new(mockDatabaseService)
		compute := new(mockComputeBackend)
		worker := NewDatabaseFailoverWorker(svc, repo, compute, slog.Default())

		repo.On("List", anyCtx).Return([]*domain.Database{primary}, nil)
		repo.On("ListReplicas", anyCtx, primaryID).Return([]*domain.Database{}, nil)

		worker.checkDatabases(context.Background())

		repo.AssertExpectations(t)
		svc.AssertNotCalled(t, "PromoteToPrimary", anyCtx, mock.Anything)
	})

	t.Run("Repo list error handled", func(t *testing.T) {
		repo := new(mockDatabaseRepo)
		svc := new(mockDatabaseService)
		compute := new(mockComputeBackend)
		worker := NewDatabaseFailoverWorker(svc, repo, compute, slog.Default())

		repo.On("List", anyCtx).Return([]*domain.Database(nil), fmt.Errorf("db error"))

		worker.checkDatabases(context.Background())

		repo.AssertExpectations(t)
	})

	t.Run("Selects replica with lowest lag", func(t *testing.T) {
		repo := new(mockDatabaseRepo)
		svc := new(mockDatabaseService)
		compute := new(mockComputeBackend)
		logger := slog.Default()

		replica1 := &domain.Database{
			ID:          uuid.New(),
			Name:        "replica-1",
			Role:        domain.RoleReplica,
			PrimaryID:   &primaryID,
			Status:      domain.DatabaseStatusRunning,
			Engine:      domain.EnginePostgres,
			ContainerID: "container-1",
		}
		replica2 := &domain.Database{
			ID:          uuid.New(),
			Name:        "replica-2",
			Role:        domain.RoleReplica,
			PrimaryID:   &primaryID,
			Status:      domain.DatabaseStatusRunning,
			Engine:      domain.EnginePostgres,
			ContainerID: "container-2",
		}

		worker := NewDatabaseFailoverWorker(svc, repo, compute, logger)

		repo.On("List", anyCtx).Return([]*domain.Database{primary}, nil)
		repo.On("ListReplicas", anyCtx, primaryID).Return([]*domain.Database{replica1, replica2}, nil)
		// Replica 1 has high lag (10 seconds - unhealthy)
		compute.On("Exec", anyCtx, "container-1", mock.Anything).Return("10\n", nil)
		// Replica 2 has low lag (1 second - healthy)
		compute.On("Exec", anyCtx, "container-2", mock.Anything).Return("1\n", nil)
		svc.On("PromoteToPrimary", anyCtx, replica2.ID).Return(nil)

		worker.checkDatabases(context.Background())

		repo.AssertExpectations(t)
		svc.AssertExpectations(t)
		compute.AssertExpectations(t)
	})

	t.Run("Non-PostgreSQL engine uses TCP health check", func(t *testing.T) {
		repo := new(mockDatabaseRepo)
		svc := new(mockDatabaseService)
		compute := new(mockComputeBackend)
		logger := slog.Default()

		mysqlReplica := &domain.Database{
			ID:          uuid.New(),
			Name:        "mysql-replica",
			Role:        domain.RoleReplica,
			PrimaryID:   &primaryID,
			Status:      domain.DatabaseStatusRunning,
			Engine:      domain.EngineMySQL,
			Port:        0, // healthy (port 0 is always healthy)
			ContainerID: "container-mysql",
		}

		worker := NewDatabaseFailoverWorker(svc, repo, compute, logger)
		repo.On("List", anyCtx).Return([]*domain.Database{primary}, nil)
		repo.On("ListReplicas", anyCtx, primaryID).Return([]*domain.Database{mysqlReplica}, nil)
		// Should NOT call compute.Exec for MySQL - uses TCP isHealthy instead
		svc.On("PromoteToPrimary", anyCtx, mysqlReplica.ID).Return(nil)

		worker.checkDatabases(context.Background())

		repo.AssertExpectations(t)
		svc.AssertExpectations(t)
		compute.AssertNotCalled(t, "Exec", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Replication check Exec error handled gracefully", func(t *testing.T) {
		repo := new(mockDatabaseRepo)
		svc := new(mockDatabaseService)
		compute := new(mockComputeBackend)
		logger := slog.Default()

		replica := &domain.Database{
			ID:          uuid.New(),
			Name:        "replica",
			Role:        domain.RoleReplica,
			PrimaryID:   &primaryID,
			Status:      domain.DatabaseStatusRunning,
			Engine:      domain.EnginePostgres,
			ContainerID: "container-1",
		}

		worker := NewDatabaseFailoverWorker(svc, repo, compute, logger)
		repo.On("List", anyCtx).Return([]*domain.Database{primary}, nil)
		repo.On("ListReplicas", anyCtx, primaryID).Return([]*domain.Database{replica}, nil)
		compute.On("Exec", anyCtx, "container-1", mock.Anything).Return("", fmt.Errorf("exec failed"))
		// PromoteToPrimary should NOT be called since replica is unhealthy
		svc.On("PromoteToPrimary", anyCtx, mock.Anything).Return(nil).Maybe()

		worker.checkDatabases(context.Background())

		repo.AssertExpectations(t)
		compute.AssertExpectations(t)
		// Verify PromoteToPrimary was not called successfully
		svc.AssertNotCalled(t, "PromoteToPrimary", anyCtx, mock.Anything)
	})

	t.Run("Non-numeric lag output handled gracefully", func(t *testing.T) {
		repo := new(mockDatabaseRepo)
		svc := new(mockDatabaseService)
		compute := new(mockComputeBackend)
		logger := slog.Default()

		replica := &domain.Database{
			ID:          uuid.New(),
			Name:        "replica",
			Role:        domain.RoleReplica,
			PrimaryID:   &primaryID,
			Status:      domain.DatabaseStatusRunning,
			Engine:      domain.EnginePostgres,
			ContainerID: "container-1",
		}

		worker := NewDatabaseFailoverWorker(svc, repo, compute, logger)
		repo.On("List", anyCtx).Return([]*domain.Database{primary}, nil)
		repo.On("ListReplicas", anyCtx, primaryID).Return([]*domain.Database{replica}, nil)
		compute.On("Exec", anyCtx, "container-1", mock.Anything).Return("invalid\n", nil)
		svc.On("PromoteToPrimary", anyCtx, mock.Anything).Return(nil).Maybe()

		worker.checkDatabases(context.Background())

		repo.AssertExpectations(t)
		compute.AssertExpectations(t)
		svc.AssertNotCalled(t, "PromoteToPrimary", anyCtx, mock.Anything)
	})

	t.Run("Failover aborted when all replicas unhealthy", func(t *testing.T) {
		repo := new(mockDatabaseRepo)
		svc := new(mockDatabaseService)
		compute := new(mockComputeBackend)
		logger := slog.Default()

		replica := &domain.Database{
			ID:          uuid.New(),
			Name:        "laggy-replica",
			Role:        domain.RoleReplica,
			PrimaryID:   &primaryID,
			Status:      domain.DatabaseStatusRunning,
			Engine:      domain.EnginePostgres,
			ContainerID: "container-1",
		}

		worker := NewDatabaseFailoverWorker(svc, repo, compute, logger)
		repo.On("List", anyCtx).Return([]*domain.Database{primary}, nil)
		repo.On("ListReplicas", anyCtx, primaryID).Return([]*domain.Database{replica}, nil)
		// Replica has high lag (10 seconds > maxAcceptableLagSeconds of 5)
		compute.On("Exec", anyCtx, "container-1", mock.Anything).Return("10\n", nil)

		worker.checkDatabases(context.Background())

		repo.AssertExpectations(t)
		compute.AssertExpectations(t)
		// PromoteToPrimary should NOT be called - no healthy replica
		svc.AssertNotCalled(t, "PromoteToPrimary", anyCtx, mock.Anything)
	})

	t.Run("isHealthy returns false when TCP dial fails", func(t *testing.T) {
		repo := new(mockDatabaseRepo)
		svc := new(mockDatabaseService)
		compute := new(mockComputeBackend)
		logger := slog.Default()
		worker := NewDatabaseFailoverWorker(svc, repo, compute, logger)

		// Allocate an ephemeral port, capture it, then close the listener
		// so the port becomes unused — the dial will fail because nothing is listening
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		port := ln.Addr().(*net.TCPAddr).Port
		ln.Close()

		replica := &domain.Database{
			ID:   uuid.New(),
			Port: port,
		}
		assert.False(t, worker.isHealthy(context.Background(), replica))
	})

	t.Run("checkDatabases skips databases that are not running", func(t *testing.T) {
		repo := new(mockDatabaseRepo)
		svc := new(mockDatabaseService)
		compute := new(mockComputeBackend)
		logger := slog.Default()

		stoppedDb := &domain.Database{
			ID:     uuid.New(),
			Role:   domain.RolePrimary,
			Status: domain.DatabaseStatusStopped,
			Port:   1234,
		}

		worker := NewDatabaseFailoverWorker(svc, repo, compute, logger)
		repo.On("List", anyCtx).Return([]*domain.Database{stoppedDb}, nil)
		worker.checkDatabases(context.Background())
		repo.AssertExpectations(t)
	})

	t.Run("checkDatabases skips replica role databases", func(t *testing.T) {
		repo := new(mockDatabaseRepo)
		svc := new(mockDatabaseService)
		compute := new(mockComputeBackend)
		logger := slog.Default()

		replicaDb := &domain.Database{
			ID:     uuid.New(),
			Role:   domain.RoleReplica,
			Status: domain.DatabaseStatusRunning,
			Port:   1234,
		}

		worker := NewDatabaseFailoverWorker(svc, repo, compute, logger)
		repo.On("List", anyCtx).Return([]*domain.Database{replicaDb}, nil)
		worker.checkDatabases(context.Background())
		repo.AssertExpectations(t)
	})

	t.Run("handleFailover handles ListReplicas error", func(t *testing.T) {
		repo := new(mockDatabaseRepo)
		svc := new(mockDatabaseService)
		compute := new(mockComputeBackend)
		logger := slog.Default()

		worker := NewDatabaseFailoverWorker(svc, repo, compute, logger)
		repo.On("ListReplicas", anyCtx, primaryID).Return(nil, fmt.Errorf("replica list error"))
		worker.handleFailover(context.Background(), primary)
		repo.AssertExpectations(t)
		svc.AssertNotCalled(t, "PromoteToPrimary", anyCtx, mock.Anything)
	})

	t.Run("handleFailover handles PromoteToPrimary error", func(t *testing.T) {
		repo := new(mockDatabaseRepo)
		svc := new(mockDatabaseService)
		compute := new(mockComputeBackend)
		logger := slog.Default()

		replica := &domain.Database{
			ID:          uuid.New(),
			Name:        "replica",
			Role:        domain.RoleReplica,
			PrimaryID:   &primaryID,
			Status:      domain.DatabaseStatusRunning,
			Engine:      domain.EnginePostgres,
			ContainerID: "container-1",
		}

		worker := NewDatabaseFailoverWorker(svc, repo, compute, logger)
		repo.On("ListReplicas", anyCtx, primaryID).Return([]*domain.Database{replica}, nil)
		compute.On("Exec", anyCtx, "container-1", mock.Anything).Return("1\n", nil)
		svc.On("PromoteToPrimary", anyCtx, replica.ID).Return(fmt.Errorf("promotion failed"))
		worker.handleFailover(context.Background(), primary)
		repo.AssertExpectations(t)
		compute.AssertExpectations(t)
		svc.AssertExpectations(t)
	})
}
