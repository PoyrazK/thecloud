package httphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	cachesPath    = "/caches"
	testCacheName = "cache-1"
)

type mockCacheService struct {
	mock.Mock
}

func (m *mockCacheService) CreateCache(ctx context.Context, name, version string, memoryMB int, vpcID *uuid.UUID) (*domain.Cache, error) {
	args := m.Called(ctx, name, version, memoryMB, vpcID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Cache), args.Error(1)
}

func (m *mockCacheService) ListCaches(ctx context.Context) ([]*domain.Cache, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Cache), args.Error(1)
}

func (m *mockCacheService) GetCache(ctx context.Context, idOrName string) (*domain.Cache, error) {
	args := m.Called(ctx, idOrName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Cache), args.Error(1)
}

func (m *mockCacheService) DeleteCache(ctx context.Context, idOrName string) error {
	args := m.Called(ctx, idOrName)
	return args.Error(0)
}

func (m *mockCacheService) GetConnectionString(ctx context.Context, idOrName string) (string, error) {
	args := m.Called(ctx, idOrName)
	return args.String(0), args.Error(1)
}

func (m *mockCacheService) FlushCache(ctx context.Context, idOrName string) error {
	return m.Called(ctx, idOrName).Error(0)
}

func (m *mockCacheService) GetCacheStats(ctx context.Context, idOrName string) (*ports.CacheStats, error) {
	args := m.Called(ctx, idOrName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.CacheStats), args.Error(1)
}

func setupCacheHandlerTest(t *testing.T) (*mockCacheService, *CacheHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockCacheService)
	handler := NewCacheHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestCacheHandlerCreate(t *testing.T) {
	svc, handler, r := setupCacheHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(cachesPath, handler.Create)

	cache := &domain.Cache{ID: uuid.New(), Name: testCacheName}
	svc.On("CreateCache", mock.Anything, testCacheName, "redis6", 128, (*uuid.UUID)(nil)).Return(cache, nil)

	body, err := json.Marshal(map[string]interface{}{
		"name":      testCacheName,
		"version":   "redis6",
		"memory_mb": 128,
	})
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", cachesPath, bytes.NewBuffer(body))
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCacheHandlerList(t *testing.T) {
	svc, handler, r := setupCacheHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(cachesPath, handler.List)

	caches := []*domain.Cache{{ID: uuid.New(), Name: testCacheName}}
	svc.On("ListCaches", mock.Anything).Return(caches, nil)

	req := httptest.NewRequest(http.MethodGet, cachesPath, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCacheHandlerGet(t *testing.T) {
	svc, handler, r := setupCacheHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(cachesPath+"/:id", handler.Get)

	id := uuid.New().String()
	cache := &domain.Cache{ID: uuid.New(), Name: testCacheName}
	svc.On("GetCache", mock.Anything, id).Return(cache, nil)

	req := httptest.NewRequest(http.MethodGet, cachesPath+"/"+id, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCacheHandlerDelete(t *testing.T) {
	svc, handler, r := setupCacheHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(cachesPath+"/:id", handler.Delete)

	id := uuid.New().String()
	svc.On("DeleteCache", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, cachesPath+"/"+id, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCacheHandlerGetConnectionString(t *testing.T) {
	svc, handler, r := setupCacheHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(cachesPath+"/:id/connection", handler.GetConnectionString)

	id := uuid.New().String()
	svc.On("GetConnectionString", mock.Anything, id).Return("redis://host:6379", nil)

	req := httptest.NewRequest(http.MethodGet, cachesPath+"/"+id+"/connection", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "redis://host:6379")
}

func TestCacheHandlerFlush(t *testing.T) {
	svc, handler, r := setupCacheHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(cachesPath+"/:id/flush", handler.Flush)

	id := uuid.New().String()
	svc.On("FlushCache", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodPost, cachesPath+"/"+id+"/flush", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCacheHandlerGetStats(t *testing.T) {
	svc, handler, r := setupCacheHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(cachesPath+"/:id/stats", handler.GetStats)

	id := uuid.New().String()
	stats := &ports.CacheStats{UsedMemoryBytes: 1024}
	svc.On("GetCacheStats", mock.Anything, id).Return(stats, nil)

	req := httptest.NewRequest(http.MethodGet, cachesPath+"/"+id+"/stats", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCacheHandler_Errors(t *testing.T) {
	svc, handler, r := setupCacheHandlerTest(t)
	r.POST(cachesPath, handler.Create)
	r.GET(cachesPath, handler.List)
	r.GET(cachesPath+"/:id", handler.Get)
	r.DELETE(cachesPath+"/:id", handler.Delete)
	r.GET(cachesPath+"/:id/connection", handler.GetConnectionString)
	r.POST(cachesPath+"/:id/flush", handler.Flush)
	r.GET(cachesPath+"/:id/stats", handler.GetStats)

	id := "test-id"

	t.Run("CreateJSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", cachesPath, bytes.NewBufferString("{invalid}"))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("CreateService", func(t *testing.T) {
		svc.On("CreateCache", mock.Anything, "err", mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
		body, _ := json.Marshal(map[string]interface{}{"name": "err", "version": "v1", "memory_mb": 64})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", cachesPath, bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("List", func(t *testing.T) {
		svc.On("ListCaches", mock.Anything).Return(nil, assert.AnError)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", cachesPath, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Get", func(t *testing.T) {
		svc.On("GetCache", mock.Anything, id).Return(nil, assert.AnError)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", cachesPath+"/"+id, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Delete", func(t *testing.T) {
		svc.On("DeleteCache", mock.Anything, id).Return(assert.AnError)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", cachesPath+"/"+id, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("GetConnectionString", func(t *testing.T) {
		svc.On("GetConnectionString", mock.Anything, id).Return("", assert.AnError)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", cachesPath+"/"+id+"/connection", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Flush", func(t *testing.T) {
		svc.On("FlushCache", mock.Anything, id).Return(assert.AnError)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", cachesPath+"/"+id+"/flush", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("GetStats", func(t *testing.T) {
		svc.On("GetCacheStats", mock.Anything, id).Return(nil, assert.AnError)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", cachesPath+"/"+id+"/stats", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
