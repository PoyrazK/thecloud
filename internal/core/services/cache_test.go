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
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockCacheRepo
type MockCacheRepo struct{ mock.Mock }

func (m *MockCacheRepo) Create(ctx context.Context, c *domain.Cache) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}
func (m *MockCacheRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Cache, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Cache), args.Error(1)
}
func (m *MockCacheRepo) GetByName(ctx context.Context, userID uuid.UUID, name string) (*domain.Cache, error) {
	args := m.Called(ctx, userID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Cache), args.Error(1)
}
func (m *MockCacheRepo) List(ctx context.Context, userID uuid.UUID) ([]*domain.Cache, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*domain.Cache), args.Error(1)
}
func (m *MockCacheRepo) Update(ctx context.Context, c *domain.Cache) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}
func (m *MockCacheRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestCreateCache_Success(t *testing.T) {
	repo := new(MockCacheRepo)
	docker := new(MockDockerClient)
	vpcRepo := new(MockVpcRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewCacheService(repo, docker, vpcRepo, eventSvc, logger)

	ctx := appcontext.WithUserID(context.Background(), uuid.New())
	name := "test-cache"
	version := "7.2"
	memory := 128

	docker.On("CreateContainer", ctx, mock.Anything, "redis:7.2-alpine", []string{"0:6379"}, "", []string(nil), mock.Anything, mock.Anything).Return("cont-123", nil)
	docker.On("GetContainerPort", ctx, "cont-123", "6379").Return(30000, nil)
	repo.On("Create", ctx, mock.AnythingOfType("*domain.Cache")).Return(nil)
	repo.On("Update", ctx, mock.AnythingOfType("*domain.Cache")).Return(nil)
	eventSvc.On("RecordEvent", ctx, "CACHE_CREATE", mock.Anything, "CACHE", mock.Anything).Return(nil)

	cache, err := svc.CreateCache(ctx, name, version, memory, nil)

	assert.NoError(t, err)
	assert.NotNil(t, cache)
	assert.Equal(t, name, cache.Name)
	assert.Equal(t, domain.EngineRedis, cache.Engine)
	assert.Equal(t, 30000, cache.Port)
	assert.Equal(t, "cont-123", cache.ContainerID)
	assert.NotEmpty(t, cache.Password)
	assert.Equal(t, 128, cache.MemoryMB)

	repo.AssertExpectations(t)
	docker.AssertExpectations(t)
}

func TestDeleteCache_Success(t *testing.T) {
	repo := new(MockCacheRepo)
	docker := new(MockDockerClient)
	vpcRepo := new(MockVpcRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewCacheService(repo, docker, vpcRepo, eventSvc, logger)

	ctx := context.Background()
	cacheID := uuid.New()
	cache := &domain.Cache{
		ID:          cacheID,
		ContainerID: "cont-123",
	}

	repo.On("GetByID", ctx, cacheID).Return(cache, nil)
	docker.On("StopContainer", ctx, "cont-123").Return(nil)
	docker.On("RemoveContainer", ctx, "cont-123").Return(nil)
	repo.On("Delete", ctx, cacheID).Return(nil)
	eventSvc.On("RecordEvent", ctx, "CACHE_DELETE", cacheID.String(), "CACHE", mock.Anything).Return(nil)

	err := svc.DeleteCache(ctx, cacheID.String())

	assert.NoError(t, err)
	repo.AssertExpectations(t)
	docker.AssertExpectations(t)
}

func TestFlushCache_Success(t *testing.T) {
	repo := new(MockCacheRepo)
	docker := new(MockDockerClient)
	vpcRepo := new(MockVpcRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewCacheService(repo, docker, vpcRepo, eventSvc, logger)

	ctx := context.Background()
	cacheID := uuid.New()
	cache := &domain.Cache{
		ID:          cacheID,
		Status:      domain.CacheStatusRunning,
		ContainerID: "cont-123",
		Password:    "secret",
	}

	repo.On("GetByID", ctx, cacheID).Return(cache, nil)
	// Expect Exec call: redis-cli -a password FLUSHALL
	docker.On("Exec", ctx, "cont-123", []string{"redis-cli", "-a", "secret", "FLUSHALL"}).Return("OK", nil)

	err := svc.FlushCache(ctx, cacheID.String())

	assert.NoError(t, err)
	docker.AssertExpectations(t)
}

func TestGetCacheStats_Success(t *testing.T) {
	repo := new(MockCacheRepo)
	docker := new(MockDockerClient)
	vpcRepo := new(MockVpcRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewCacheService(repo, docker, vpcRepo, eventSvc, logger)

	ctx := context.Background()
	cacheID := uuid.New()
	containerID := "cont-123"
	cache := &domain.Cache{
		ID:          cacheID,
		Status:      domain.CacheStatusRunning,
		ContainerID: containerID,
		Password:    "secret",
	}

	repo.On("GetByID", ctx, cacheID).Return(cache, nil)

	// Mock Docker Stats
	statsJSON := `{"memory_stats":{"usage":1024,"limit":2048}}`
	docker.On("GetContainerStats", ctx, containerID).Return(io.NopCloser(strings.NewReader(statsJSON)), nil)

	// Mock Exec INFO
	infoOutput := "connected_clients:5\r\ndb0:keys=10,expires=0,avg_ttl=0\r\n"
	docker.On("Exec", ctx, containerID, []string{"redis-cli", "-a", "secret", "INFO"}).Return(infoOutput, nil)

	stats, err := svc.GetCacheStats(ctx, cacheID.String())

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, int64(1024), stats.UsedMemoryBytes)
	assert.Equal(t, int64(2048), stats.MaxMemoryBytes)
	assert.Equal(t, 5, stats.ConnectedClients)
	assert.Equal(t, int64(10), stats.TotalKeys)

	docker.AssertExpectations(t)
}

func TestCreateCache_DockerFailure(t *testing.T) {
	repo := new(MockCacheRepo)
	docker := new(MockDockerClient)
	vpcRepo := new(MockVpcRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewCacheService(repo, docker, vpcRepo, eventSvc, logger)

	ctx := appcontext.WithUserID(context.Background(), uuid.New())
	name := "fail-cache"

	// Expect Repo Create
	repo.On("Create", ctx, mock.Anything).Return(nil)

	// Expect Docker Create to fail
	docker.On("CreateContainer", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", assert.AnError)

	// Expect Rollback Delete
	repo.On("Delete", ctx, mock.Anything).Return(nil)

	cache, err := svc.CreateCache(ctx, name, "7.2", 128, nil)

	assert.Error(t, err)
	assert.Nil(t, cache)
	repo.AssertExpectations(t)
	docker.AssertExpectations(t)
}

func TestGetCache_ByID(t *testing.T) {
	repo := new(MockCacheRepo)
	docker := new(MockDockerClient)
	vpcRepo := new(MockVpcRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewCacheService(repo, docker, vpcRepo, eventSvc, logger)

	ctx := context.Background()
	cacheID := uuid.New()
	cache := &domain.Cache{ID: cacheID, Name: "my-cache"}

	repo.On("GetByID", ctx, cacheID).Return(cache, nil)

	result, err := svc.GetCache(ctx, cacheID.String())

	assert.NoError(t, err)
	assert.Equal(t, cacheID, result.ID)
	repo.AssertExpectations(t)
}

func TestGetCache_ByName(t *testing.T) {
	repo := new(MockCacheRepo)
	docker := new(MockDockerClient)
	vpcRepo := new(MockVpcRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewCacheService(repo, docker, vpcRepo, eventSvc, logger)

	userID := uuid.New()
	ctx := appcontext.WithUserID(context.Background(), userID)
	name := "named-cache"
	cache := &domain.Cache{ID: uuid.New(), Name: name}

	repo.On("GetByName", ctx, userID, name).Return(cache, nil)

	result, err := svc.GetCache(ctx, name)

	assert.NoError(t, err)
	assert.Equal(t, name, result.Name)
	repo.AssertExpectations(t)
}

func TestListCaches(t *testing.T) {
	repo := new(MockCacheRepo)
	docker := new(MockDockerClient)
	vpcRepo := new(MockVpcRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewCacheService(repo, docker, vpcRepo, eventSvc, logger)

	userID := uuid.New()
	ctx := appcontext.WithUserID(context.Background(), userID)

	caches := []*domain.Cache{{Name: "cache1"}, {Name: "cache2"}}
	repo.On("List", ctx, userID).Return(caches, nil)

	result, err := svc.ListCaches(ctx)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	repo.AssertExpectations(t)
}

func TestGetCacheConnectionString(t *testing.T) {
	repo := new(MockCacheRepo)
	docker := new(MockDockerClient)
	vpcRepo := new(MockVpcRepo)
	eventSvc := new(MockEventService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewCacheService(repo, docker, vpcRepo, eventSvc, logger)

	ctx := context.Background()
	cacheID := uuid.New()
	cache := &domain.Cache{
		ID:       cacheID,
		Name:     "conn-cache",
		Port:     6379,
		Password: "secret",
	}

	repo.On("GetByID", ctx, cacheID).Return(cache, nil)

	connStr, err := svc.GetConnectionString(ctx, cacheID.String())

	assert.NoError(t, err)
	assert.Contains(t, connStr, "redis://")
	assert.Contains(t, connStr, "secret")
	assert.Contains(t, connStr, "6379")
	repo.AssertExpectations(t)
}
