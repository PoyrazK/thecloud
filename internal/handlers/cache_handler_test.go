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
	args := m.Called(ctx, idOrName)
	return args.Error(0)
}

func (m *mockCacheService) GetCacheStats(ctx context.Context, idOrName string) (*ports.CacheStats, error) {
	args := m.Called(ctx, idOrName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.CacheStats), args.Error(1)
}

func TestCacheHandler_Create(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockCacheService)
	handler := NewCacheHandler(svc)

	r := gin.New()
	r.POST("/caches", handler.Create)

	cache := &domain.Cache{ID: uuid.New(), Name: "cache-1"}
	svc.On("CreateCache", mock.Anything, "cache-1", "redis6", 128, (*uuid.UUID)(nil)).Return(cache, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"name":      "cache-1",
		"version":   "redis6",
		"memory_mb": 128,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/caches", bytes.NewBuffer(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCacheHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockCacheService)
	handler := NewCacheHandler(svc)

	r := gin.New()
	r.GET("/caches", handler.List)

	caches := []*domain.Cache{{ID: uuid.New(), Name: "cache-1"}}
	svc.On("ListCaches", mock.Anything).Return(caches, nil)

	req := httptest.NewRequest(http.MethodGet, "/caches", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCacheHandler_Get(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockCacheService)
	handler := NewCacheHandler(svc)

	r := gin.New()
	r.GET("/caches/:id", handler.Get)

	id := uuid.New().String()
	cache := &domain.Cache{ID: uuid.New(), Name: "cache-1"}
	svc.On("GetCache", mock.Anything, id).Return(cache, nil)

	req := httptest.NewRequest(http.MethodGet, "/caches/"+id, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCacheHandler_Delete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockCacheService)
	handler := NewCacheHandler(svc)

	r := gin.New()
	r.DELETE("/caches/:id", handler.Delete)

	id := uuid.New().String()
	svc.On("DeleteCache", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/caches/"+id, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCacheHandler_GetConnectionString(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockCacheService)
	handler := NewCacheHandler(svc)

	r := gin.New()
	r.GET("/caches/:id/connection", handler.GetConnectionString)

	id := uuid.New().String()
	svc.On("GetConnectionString", mock.Anything, id).Return("redis://host:6379", nil)

	req := httptest.NewRequest(http.MethodGet, "/caches/"+id+"/connection", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "redis://host:6379")
}

func TestCacheHandler_Flush(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockCacheService)
	handler := NewCacheHandler(svc)

	r := gin.New()
	r.POST("/caches/:id/flush", handler.Flush)

	id := uuid.New().String()
	svc.On("FlushCache", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/caches/"+id+"/flush", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCacheHandler_GetStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockCacheService)
	handler := NewCacheHandler(svc)

	r := gin.New()
	r.GET("/caches/:id/stats", handler.GetStats)

	id := uuid.New().String()
	stats := &ports.CacheStats{UsedMemoryBytes: 1024}
	svc.On("GetCacheStats", mock.Anything, id).Return(stats, nil)

	req := httptest.NewRequest(http.MethodGet, "/caches/"+id+"/stats", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
