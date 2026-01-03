package services_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDatabaseRepo
type MockDatabaseRepo struct{ mock.Mock }

func (m *MockDatabaseRepo) Create(ctx context.Context, db *domain.Database) error {
	args := m.Called(ctx, db)
	return args.Error(0)
}
func (m *MockDatabaseRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Database, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Database), args.Error(1)
}
func (m *MockDatabaseRepo) List(ctx context.Context) ([]*domain.Database, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Database), args.Error(1)
}
func (m *MockDatabaseRepo) Update(ctx context.Context, db *domain.Database) error {
	args := m.Called(ctx, db)
	return args.Error(0)
}
func (m *MockDatabaseRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockDockerClient
type MockDockerClient struct{ mock.Mock }

func (m *MockDockerClient) CreateContainer(ctx context.Context, name, image string, ports []string, networkID string, volumeBinds []string, env []string, cmd []string) (string, error) {
	args := m.Called(ctx, name, image, ports, networkID, volumeBinds, env, cmd)
	return args.String(0), args.Error(1)
}
func (m *MockDockerClient) StopContainer(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockDockerClient) RemoveContainer(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockDockerClient) GetLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
func (m *MockDockerClient) GetContainerStats(ctx context.Context, id string) (io.ReadCloser, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
func (m *MockDockerClient) GetContainerPort(ctx context.Context, id string, port string) (int, error) {
	args := m.Called(ctx, id, port)
	return args.Int(0), args.Error(1)
}
func (m *MockDockerClient) CreateNetwork(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}
func (m *MockDockerClient) RemoveNetwork(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockDockerClient) CreateVolume(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}
func (m *MockDockerClient) DeleteVolume(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}
func (m *MockDockerClient) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, error) {
	args := m.Called(ctx, opts)
	return args.String(0), args.Error(1)
}
func (m *MockDockerClient) WaitContainer(ctx context.Context, id string) (int64, error) {
	args := m.Called(ctx, id)
	return int64(args.Int(0)), args.Error(1)
}
func (m *MockDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	args := m.Called(ctx, containerID, cmd)
	return args.String(0), args.Error(1)
}

func TestCreateDatabase_Success(t *testing.T) {
	repo := new(MockDatabaseRepo)
	docker := new(MockDockerClient)
	vpcRepo := new(MockVpcRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewDatabaseService(repo, docker, vpcRepo, eventSvc, logger)

	ctx := context.Background()
	name := "test-db"
	engine := "postgres"
	version := "16"

	docker.On("CreateContainer", ctx, mock.Anything, "postgres:16-alpine", []string{"0:5432"}, "", []string(nil), mock.Anything, []string(nil)).Return("cont-123", nil)
	docker.On("GetContainerPort", ctx, "cont-123", "5432").Return(54321, nil)
	repo.On("Create", ctx, mock.AnythingOfType("*domain.Database")).Return(nil)
	eventSvc.On("RecordEvent", ctx, "DATABASE_CREATE", mock.Anything, "DATABASE", mock.Anything).Return(nil)

	db, err := svc.CreateDatabase(ctx, name, engine, version, nil)

	assert.NoError(t, err)
	assert.NotNil(t, db)
	assert.Equal(t, name, db.Name)
	assert.Equal(t, domain.EnginePostgres, db.Engine)
	assert.Equal(t, 54321, db.Port)
	assert.Equal(t, "cont-123", db.ContainerID)

	repo.AssertExpectations(t)
	docker.AssertExpectations(t)
}

func TestDeleteDatabase_Success(t *testing.T) {
	repo := new(MockDatabaseRepo)
	docker := new(MockDockerClient)
	vpcRepo := new(MockVpcRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewDatabaseService(repo, docker, vpcRepo, eventSvc, logger)

	ctx := context.Background()
	dbID := uuid.New()
	db := &domain.Database{
		ID:          dbID,
		ContainerID: "cont-123",
	}

	repo.On("GetByID", ctx, dbID).Return(db, nil)
	docker.On("RemoveContainer", ctx, "cont-123").Return(nil)
	repo.On("Delete", ctx, dbID).Return(nil)
	eventSvc.On("RecordEvent", ctx, "DATABASE_DELETE", dbID.String(), "DATABASE", mock.Anything).Return(nil)

	err := svc.DeleteDatabase(ctx, dbID)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
	docker.AssertExpectations(t)
}

func TestGetDatabase_ByID(t *testing.T) {
	repo := new(MockDatabaseRepo)
	docker := new(MockDockerClient)
	vpcRepo := new(MockVpcRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewDatabaseService(repo, docker, vpcRepo, eventSvc, logger)

	ctx := context.Background()
	dbID := uuid.New()
	db := &domain.Database{ID: dbID, Name: "my-db"}

	repo.On("GetByID", ctx, dbID).Return(db, nil)

	result, err := svc.GetDatabase(ctx, dbID) // Pass UUID directly

	assert.NoError(t, err)
	assert.Equal(t, dbID, result.ID)
	repo.AssertExpectations(t)
}

func TestListDatabases(t *testing.T) {
	repo := new(MockDatabaseRepo)
	docker := new(MockDockerClient)
	vpcRepo := new(MockVpcRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewDatabaseService(repo, docker, vpcRepo, eventSvc, logger)

	ctx := context.Background()
	dbs := []*domain.Database{{Name: "db1"}, {Name: "db2"}}

	repo.On("List", ctx).Return(dbs, nil)

	result, err := svc.ListDatabases(ctx)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	repo.AssertExpectations(t)
}

func TestGetConnectionString(t *testing.T) {
	repo := new(MockDatabaseRepo)
	docker := new(MockDockerClient)
	vpcRepo := new(MockVpcRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewDatabaseService(repo, docker, vpcRepo, eventSvc, logger)

	ctx := context.Background()
	dbID := uuid.New()
	db := &domain.Database{
		ID:       dbID,
		Name:     "conn-db",
		Engine:   domain.EnginePostgres,
		Port:     5432,
		Username: "admin",
		Password: "secret",
	}

	repo.On("GetByID", ctx, dbID).Return(db, nil)

	connStr, err := svc.GetConnectionString(ctx, dbID) // Correct method name

	assert.NoError(t, err)
	assert.Contains(t, connStr, "postgres://")
	assert.Contains(t, connStr, "admin:secret")
	assert.Contains(t, connStr, "5432")
	repo.AssertExpectations(t)
}

func TestGetDatabaseLogs(t *testing.T) {
	repo := new(MockDatabaseRepo)
	docker := new(MockDockerClient)
	vpcRepo := new(MockVpcRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewDatabaseService(repo, docker, vpcRepo, eventSvc, logger)

	ctx := context.Background()
	dbID := uuid.New()
	db := &domain.Database{ID: dbID, ContainerID: "cont-logs"}

	repo.On("GetByID", ctx, dbID).Return(db, nil)
	docker.On("GetLogs", ctx, "cont-logs").Return(io.NopCloser(strings.NewReader("log line")), nil)

	logs, err := svc.GetDatabaseLogs(ctx, dbID)

	assert.NoError(t, err)
	assert.Contains(t, logs, "log line")
	repo.AssertExpectations(t)
	docker.AssertExpectations(t)
}
