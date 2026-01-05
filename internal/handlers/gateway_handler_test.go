package httphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockGatewayService struct {
	mock.Mock
}

func (m *mockGatewayService) CreateRoute(ctx context.Context, name, prefix, target string, strip bool, rateLimit int) (*domain.GatewayRoute, error) {
	args := m.Called(ctx, name, prefix, target, strip, rateLimit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.GatewayRoute), args.Error(1)
}

func (m *mockGatewayService) ListRoutes(ctx context.Context) ([]*domain.GatewayRoute, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.GatewayRoute), args.Error(1)
}

func (m *mockGatewayService) DeleteRoute(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockGatewayService) RefreshRoutes(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockGatewayService) GetProxy(path string) (*httputil.ReverseProxy, bool) {
	args := m.Called(path)
	if args.Get(0) == nil {
		return nil, args.Bool(1)
	}
	return args.Get(0).(*httputil.ReverseProxy), args.Bool(1)
}

func TestGatewayHandler_CreateRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockGatewayService)
	handler := NewGatewayHandler(svc)

	r := gin.New()
	r.POST("/gateway/routes", handler.CreateRoute)

	route := &domain.GatewayRoute{ID: uuid.New(), Name: "route-1"}
	svc.On("CreateRoute", mock.Anything, "route-1", "/api/v1", "http://example.com", false, 100).Return(route, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"name":        "route-1",
		"path_prefix": "/api/v1",
		"target_url":  "http://example.com",
		"rate_limit":  100,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/gateway/routes", bytes.NewBuffer(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestGatewayHandler_ListRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockGatewayService)
	handler := NewGatewayHandler(svc)

	r := gin.New()
	r.GET("/gateway/routes", handler.ListRoutes)

	routes := []*domain.GatewayRoute{{ID: uuid.New(), Name: "route-1"}}
	svc.On("ListRoutes", mock.Anything).Return(routes, nil)

	req := httptest.NewRequest(http.MethodGet, "/gateway/routes", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGatewayHandler_DeleteRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockGatewayService)
	handler := NewGatewayHandler(svc)

	r := gin.New()
	r.DELETE("/gateway/routes/:id", handler.DeleteRoute)

	id := uuid.New()
	svc.On("DeleteRoute", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/gateway/routes/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGatewayHandler_Proxy_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockGatewayService)
	handler := NewGatewayHandler(svc)

	r := gin.New()
	r.Any("/gw/*proxy", handler.Proxy)

	svc.On("GetProxy", "/unknown").Return(nil, false)

	req := httptest.NewRequest(http.MethodGet, "/gw/unknown", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
