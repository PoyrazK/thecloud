package services_test

import (
	"context"
	"fmt"
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
	"github.com/stretchr/testify/require"
)

type MockCacheRepository struct {
	mock.Mock
}

func (m *MockCacheRepository) Create(ctx context.Context, cache *domain.Cache) error {
	return m.Called(ctx, cache).Error(0)
}
func (m *MockCacheRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Cache, error) {
	args := m.Called(ctx, id, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Cache)
	return r0, args.Error(1)
}
func (m *MockCacheRepository) GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*domain.Cache, error) {
	args := m.Called(ctx, tenantID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Cache)
	return r0, args.Error(1)
}
func (m *MockCacheRepository) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.Cache, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Cache)
	return r0, args.Error(1)
}
func (m *MockCacheRepository) Update(ctx context.Context, cache *domain.Cache) error {
	return m.Called(ctx, cache).Error(0)
}
func (m *MockCacheRepository) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	return m.Called(ctx, id, tenantID).Error(0)
}

func TestCacheService_Unit(t *testing.T) {
	t.Run("Extended", testCacheServiceUnitExtended)
}

func testCacheServiceUnitExtended(t *testing.T) {
	repo := new(MockCacheRepository)
	compute := new(MockComputeBackend)
	eventSvc := new(MockEventService)
	auditSvc := new(MockAuditService)
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := services.NewCacheService(repo, rbacSvc, compute, nil, eventSvc, auditSvc, slog.Default())
	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	ctx = appcontext.WithTenantID(ctx, tenantID)

	t.Run("CreateCache_Success", func(t *testing.T) {
		compute.On("Type").Return("docker").Maybe()
		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		compute.On("LaunchInstanceWithOptions", mock.Anything, mock.Anything).Return("cid", []string{"30001:6379"}, nil).Once()
		repo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()
		eventSvc.On("RecordEvent", mock.Anything, "CACHE_CREATE", mock.Anything, "CACHE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "cache.create", "cache", mock.Anything, mock.Anything).Return(nil).Once()

		cache, err := svc.CreateCache(ctx, "my-cache", "7.0", 128, nil)
		require.NoError(t, err)
		assert.NotNil(t, cache)
		assert.Equal(t, 30001, cache.Port)
	})

	t.Run("CreateCache_LaunchFailure", func(t *testing.T) {
		compute.On("Type").Return("docker").Maybe()
		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		compute.On("LaunchInstanceWithOptions", mock.Anything, mock.Anything).Return("", nil, fmt.Errorf("launch fail")).Once()
		repo.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		_, err := svc.CreateCache(ctx, "fail", "7.0", 128, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "launch cache container")
	})

	t.Run("GetCache", func(t *testing.T) {
		cacheID := uuid.New()
		repo.On("GetByID", mock.Anything, cacheID, mock.Anything).Return(&domain.Cache{ID: cacheID}, nil).Once()
		res, err := svc.GetCache(ctx, cacheID.String())
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("ListCaches", func(t *testing.T) {
		repo.On("List", mock.Anything, mock.Anything).Return([]*domain.Cache{}, nil).Once()
		res, err := svc.ListCaches(ctx)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("DeleteCache", func(t *testing.T) {
		cacheID := uuid.New()
		cache := &domain.Cache{ID: cacheID, UserID: userID, TenantID: tenantID, Name: "my-cache", ContainerID: "cid"}
		repo.On("GetByID", mock.Anything, cacheID, mock.Anything).Return(cache, nil).Once()
		compute.On("StopInstance", mock.Anything, "cid").Return(nil).Once()
		compute.On("DeleteInstance", mock.Anything, "cid").Return(nil).Once()
		repo.On("Delete", mock.Anything, cacheID, mock.Anything).Return(nil).Once()
		eventSvc.On("RecordEvent", mock.Anything, "CACHE_DELETE", cacheID.String(), "CACHE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "cache.delete", "cache", cacheID.String(), mock.Anything).Return(nil).Once()

		err := svc.DeleteCache(ctx, cacheID.String())
		require.NoError(t, err)
	})

	t.Run("GetConnectionString", func(t *testing.T) {
		cacheID := uuid.New()
		cache := &domain.Cache{
			ID:       cacheID,
			TenantID: tenantID,
			Password: "pass",
			Port:     6379,
		}
		repo.On("GetByID", mock.Anything, cacheID, mock.Anything).Return(cache, nil).Once()

		conn, err := svc.GetConnectionString(ctx, cacheID.String())
		require.NoError(t, err)
		assert.Contains(t, conn, "redis://:pass@localhost:6379")
	})

	t.Run("FlushCache", func(t *testing.T) {
		cacheID := uuid.New()
		cache := &domain.Cache{ID: cacheID, ContainerID: "cid", UserID: userID, TenantID: tenantID, Status: domain.CacheStatusRunning}
		repo.On("GetByID", mock.Anything, cacheID, mock.Anything).Return(cache, nil).Once()
		compute.On("Exec", mock.Anything, "cid", []string{"redis-cli", "FLUSHALL"}).Return("", nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "cache.flush", "cache", cacheID.String(), mock.Anything).Return(nil).Once()

		err := svc.FlushCache(ctx, cacheID.String())
		require.NoError(t, err)
	})

	t.Run("FlushCache_NotRunning", func(t *testing.T) {
		cacheID := uuid.New()
		cache := &domain.Cache{ID: cacheID, TenantID: tenantID, Status: domain.CacheStatusCreating}
		repo.On("GetByID", mock.Anything, cacheID, mock.Anything).Return(cache, nil).Once()

		err := svc.FlushCache(ctx, cacheID.String())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not running")
	})

	t.Run("GetCacheStats", func(t *testing.T) {
		cacheID := uuid.New()
		cache := &domain.Cache{ID: cacheID, ContainerID: "cid", TenantID: tenantID, Status: domain.CacheStatusRunning}
		repo.On("GetByID", mock.Anything, cacheID, mock.Anything).Return(cache, nil).Once()

		statsJSON := `{"memory_stats": {"usage": 1024, "limit": 2048}}`
		compute.On("GetInstanceStats", mock.Anything, "cid").Return(io.NopCloser(strings.NewReader(statsJSON)), nil).Once()
		compute.On("Exec", mock.Anything, "cid", mock.Anything).Return("connected_clients:1\r\ndb0:keys=5,expires=0,avg_ttl=0", nil).Once()

		stats, err := svc.GetCacheStats(ctx, cacheID.String())
		require.NoError(t, err)
		assert.NotNil(t, stats)
		assert.Equal(t, int64(1024), stats.UsedMemoryBytes)
	})

	t.Run("ResizeCache_Success", func(t *testing.T) {
		cacheID := uuid.New()
		cache := &domain.Cache{
			ID:       cacheID,
			TenantID: tenantID,
			UserID:   userID,
			Name:     "my-cache",
			Engine:   "redis",
			Version:  "7.0",
			MemoryMB: 128,
			ContainerID: "old-cid",
			Status:   domain.CacheStatusRunning,
		}
		repo.On("GetByID", mock.Anything, cacheID, mock.Anything).Return(cache, nil).Once()
		compute.On("StopInstance", mock.Anything, "old-cid").Return(nil).Once()
		compute.On("DeleteInstance", mock.Anything, "old-cid").Return(nil).Once()
		compute.On("LaunchInstanceWithOptions", mock.Anything, mock.Anything).Return("new-cid", []string{"30002:6379"}, nil).Once()
		compute.On("GetInstancePort", mock.Anything, "new-cid", 6379).Return(30002, nil).Once()
		repo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "cache.resize", "cache", mock.Anything, mock.Anything).Return(nil).Once()

		err := svc.ResizeCache(ctx, cacheID.String(), 256)
		require.NoError(t, err)
	})

	t.Run("ResizeCache_SameOrSmallerMemory", func(t *testing.T) {
		cacheID := uuid.New()
		cache := &domain.Cache{
			ID:       cacheID,
			TenantID: tenantID,
			UserID:   userID,
			Name:     "my-cache",
			MemoryMB: 256,
			Status:   domain.CacheStatusRunning,
		}
		repo.On("GetByName", mock.Anything, mock.Anything, "my-cache").Return(cache, nil).Maybe()
		compute.On("Type").Return("docker").Maybe()

		err := svc.ResizeCache(ctx, "my-cache", 256)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "larger than current")
	})
}
