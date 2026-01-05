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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockContainerService struct {
	mock.Mock
}

func (m *mockContainerService) CreateDeployment(ctx context.Context, name, image string, replicas int, ports string) (*domain.Deployment, error) {
	args := m.Called(ctx, name, image, replicas, ports)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Deployment), args.Error(1)
}

func (m *mockContainerService) ListDeployments(ctx context.Context) ([]*domain.Deployment, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Deployment), args.Error(1)
}

func (m *mockContainerService) GetDeployment(ctx context.Context, id uuid.UUID) (*domain.Deployment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Deployment), args.Error(1)
}

func (m *mockContainerService) ScaleDeployment(ctx context.Context, id uuid.UUID, replicas int) error {
	args := m.Called(ctx, id, replicas)
	return args.Error(0)
}

func (m *mockContainerService) DeleteDeployment(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestContainerHandler_CreateDeployment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockContainerService)
	handler := NewContainerHandler(svc)

	r := gin.New()
	r.POST("/containers/deployments", handler.CreateDeployment)

	dep := &domain.Deployment{ID: uuid.New(), Name: "dep-1"}
	svc.On("CreateDeployment", mock.Anything, "dep-1", "nginx", 3, "80:80").Return(dep, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"name":     "dep-1",
		"image":    "nginx",
		"replicas": 3,
		"ports":    "80:80",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/containers/deployments", bytes.NewBuffer(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestContainerHandler_ListDeployments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockContainerService)
	handler := NewContainerHandler(svc)

	r := gin.New()
	r.GET("/containers/deployments", handler.ListDeployments)

	deps := []*domain.Deployment{{ID: uuid.New(), Name: "dep-1"}}
	svc.On("ListDeployments", mock.Anything).Return(deps, nil)

	req := httptest.NewRequest(http.MethodGet, "/containers/deployments", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestContainerHandler_GetDeployment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockContainerService)
	handler := NewContainerHandler(svc)

	r := gin.New()
	r.GET("/containers/deployments/:id", handler.GetDeployment)

	id := uuid.New()
	dep := &domain.Deployment{ID: id, Name: "dep-1"}
	svc.On("GetDeployment", mock.Anything, id).Return(dep, nil)

	req := httptest.NewRequest(http.MethodGet, "/containers/deployments/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestContainerHandler_ScaleDeployment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockContainerService)
	handler := NewContainerHandler(svc)

	r := gin.New()
	r.POST("/containers/deployments/:id/scale", handler.ScaleDeployment)

	id := uuid.New()
	svc.On("ScaleDeployment", mock.Anything, id, 5).Return(nil)

	body, _ := json.Marshal(map[string]interface{}{"replicas": 5})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/containers/deployments/"+id.String()+"/scale", bytes.NewBuffer(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestContainerHandler_DeleteDeployment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockContainerService)
	handler := NewContainerHandler(svc)

	r := gin.New()
	r.DELETE("/containers/deployments/:id", handler.DeleteDeployment)

	id := uuid.New()
	svc.On("DeleteDeployment", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/containers/deployments/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
