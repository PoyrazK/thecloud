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

type mockLBService struct {
	mock.Mock
}

func (m *mockLBService) Create(ctx context.Context, name string, vpcID uuid.UUID, port int, algorithm, idempotencyKey string) (*domain.LoadBalancer, error) {
	args := m.Called(ctx, name, vpcID, port, algorithm, idempotencyKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.LoadBalancer), args.Error(1)
}

func (m *mockLBService) List(ctx context.Context) ([]*domain.LoadBalancer, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.LoadBalancer), args.Error(1)
}

func (m *mockLBService) Get(ctx context.Context, id uuid.UUID) (*domain.LoadBalancer, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.LoadBalancer), args.Error(1)
}

func (m *mockLBService) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockLBService) AddTarget(ctx context.Context, lbID, instanceID uuid.UUID, port, weight int) error {
	args := m.Called(ctx, lbID, instanceID, port, weight)
	return args.Error(0)
}

func (m *mockLBService) RemoveTarget(ctx context.Context, lbID, instanceID uuid.UUID) error {
	args := m.Called(ctx, lbID, instanceID)
	return args.Error(0)
}

func (m *mockLBService) ListTargets(ctx context.Context, lbID uuid.UUID) ([]*domain.LBTarget, error) {
	args := m.Called(ctx, lbID)
	return args.Get(0).([]*domain.LBTarget), args.Error(1)
}

func setupLBHandlerTest(t *testing.T) (*mockLBService, *LBHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockLBService)
	handler := NewLBHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestLBHandler_Create(t *testing.T) {
	svc, handler, r := setupLBHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST("/lb", handler.Create)

	vpcID := uuid.New()
	lb := &domain.LoadBalancer{ID: uuid.New(), Name: "test-lb"}
	svc.On("Create", mock.Anything, "test-lb", vpcID, 80, "round-robin", "").Return(lb, nil)

	body, err := json.Marshal(map[string]interface{}{
		"name":      "test-lb",
		"vpc_id":    vpcID.String(),
		"port":      80,
		"algorithm": "round-robin",
	})
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/lb", bytes.NewBuffer(body))
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestLBHandler_List(t *testing.T) {
	svc, handler, r := setupLBHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET("/lb", handler.List)

	lbs := []*domain.LoadBalancer{{ID: uuid.New(), Name: "lb1"}}
	svc.On("List", mock.Anything).Return(lbs, nil)

	req := httptest.NewRequest(http.MethodGet, "/lb", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLBHandler_Get(t *testing.T) {
	svc, handler, r := setupLBHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET("/lb/:id", handler.Get)

	id := uuid.New()
	lb := &domain.LoadBalancer{ID: id, Name: "lb1"}
	svc.On("Get", mock.Anything, id).Return(lb, nil)

	req := httptest.NewRequest(http.MethodGet, "/lb/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLBHandler_Delete(t *testing.T) {
	svc, handler, r := setupLBHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE("/lb/:id", handler.Delete)

	id := uuid.New()
	svc.On("Delete", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/lb/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLBHandler_AddTarget(t *testing.T) {
	svc, handler, r := setupLBHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST("/lb/:id/targets", handler.AddTarget)

	lbID := uuid.New()
	instID := uuid.New()
	svc.On("AddTarget", mock.Anything, lbID, instID, 8080, 10).Return(nil)

	body, err := json.Marshal(map[string]interface{}{
		"instance_id": instID.String(),
		"port":        8080,
		"weight":      10,
	})
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/lb/"+lbID.String()+"/targets", bytes.NewBuffer(body))
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestLBHandler_RemoveTarget(t *testing.T) {
	svc, handler, r := setupLBHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE("/lb/:id/targets/:instanceId", handler.RemoveTarget)

	lbID := uuid.New()
	instID := uuid.New()
	svc.On("RemoveTarget", mock.Anything, lbID, instID).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/lb/"+lbID.String()+"/targets/"+instID.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLBHandler_ListTargets(t *testing.T) {
	svc, handler, r := setupLBHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET("/lb/:id/targets", handler.ListTargets)

	lbID := uuid.New()
	targets := []*domain.LBTarget{{InstanceID: uuid.New()}}
	svc.On("ListTargets", mock.Anything, lbID).Return(targets, nil)

	req := httptest.NewRequest(http.MethodGet, "/lb/"+lbID.String()+"/targets", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
