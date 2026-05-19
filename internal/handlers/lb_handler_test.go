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
	"github.com/stretchr/testify/require"
)

const (
	lbPath         = "/lb"
	testLBName     = "test-lb"
	algoRoundRobin = "round-robin"
)

type mockLBService struct {
	mock.Mock
}

func (m *mockLBService) Create(ctx context.Context, name string, vpcID uuid.UUID, port int, algorithm, idempotencyKey string) (*domain.LoadBalancer, error) {
	args := m.Called(ctx, name, vpcID, port, algorithm, idempotencyKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.LoadBalancer)
	return r0, args.Error(1)
}

func (m *mockLBService) List(ctx context.Context) ([]*domain.LoadBalancer, error) {
	args := m.Called(ctx)
	r0, _ := args.Get(0).([]*domain.LoadBalancer)
	return r0, args.Error(1)
}

func (m *mockLBService) Get(ctx context.Context, idOrName string) (*domain.LoadBalancer, error) {
	args := m.Called(ctx, idOrName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.LoadBalancer)
	return r0, args.Error(1)
}

func (m *mockLBService) Delete(ctx context.Context, idOrName string) error {
	args := m.Called(ctx, idOrName)
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
	r0, _ := args.Get(0).([]*domain.LBTarget)
	return r0, args.Error(1)
}

func setupLBHandlerTest(_ *testing.T) (*mockLBService, *LBHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockLBService)
	handler := NewLBHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestLBHandlerCreate(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupLBHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(lbPath, handler.Create)

	vpcID := uuid.New()
	lb := &domain.LoadBalancer{ID: uuid.New(), Name: testLBName}

	t.Run("Success", func(t *testing.T) {
		svc.On("Create", mock.Anything, testLBName, vpcID, 80, algoRoundRobin, "").Return(lb, nil)
		body, err := json.Marshal(map[string]interface{}{
			"name":      testLBName,
			"vpc_id":    vpcID.String(),
			"port":      80,
			"algorithm": algoRoundRobin,
		})
		require.NoError(t, err)
		w := httptest.NewRecorder()
		req, err := http.NewRequest("POST", lbPath, bytes.NewBuffer(body))
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusAccepted, w.Code)
	})

	t.Run("PortZero_Rejected", func(t *testing.T) {
		body, err := json.Marshal(map[string]interface{}{
			"name":   testLBName,
			"vpc_id": vpcID.String(),
			"port":   0,
		})
		require.NoError(t, err)
		w := httptest.NewRecorder()
		req, err := http.NewRequest("POST", lbPath, bytes.NewBuffer(body))
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("PortNegative_Rejected", func(t *testing.T) {
		body, err := json.Marshal(map[string]interface{}{
			"name":   testLBName,
			"vpc_id": vpcID.String(),
			"port":   -1,
		})
		require.NoError(t, err)
		w := httptest.NewRecorder()
		req, err := http.NewRequest("POST", lbPath, bytes.NewBuffer(body))
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("PortExceedsMax_Rejected", func(t *testing.T) {
		body, err := json.Marshal(map[string]interface{}{
			"name":   testLBName,
			"vpc_id": vpcID.String(),
			"port":   65536,
		})
		require.NoError(t, err)
		w := httptest.NewRecorder()
		req, err := http.NewRequest("POST", lbPath, bytes.NewBuffer(body))
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("PortAtMax_IsValid", func(t *testing.T) {
		svc.On("Create", mock.Anything, testLBName, vpcID, 65535, algoRoundRobin, "").Return(lb, nil)
		body, err := json.Marshal(map[string]interface{}{
			"name":      testLBName,
			"vpc_id":    vpcID.String(),
			"port":      65535,
			"algorithm": algoRoundRobin,
		})
		require.NoError(t, err)
		w := httptest.NewRecorder()
		req, err := http.NewRequest("POST", lbPath, bytes.NewBuffer(body))
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusAccepted, w.Code)
	})
}

func TestLBHandlerList(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupLBHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(lbPath, handler.List)

	lbs := []*domain.LoadBalancer{{ID: uuid.New(), Name: "lb1"}}
	svc.On("List", mock.Anything).Return(lbs, nil)

	req := httptest.NewRequest(http.MethodGet, lbPath, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLBHandlerGet(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupLBHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(lbPath+"/:id", handler.Get)

	id := uuid.New()
	lb := &domain.LoadBalancer{ID: id, Name: "lb1"}
	svc.On("Get", mock.Anything, id.String()).Return(lb, nil)

	req := httptest.NewRequest(http.MethodGet, lbPath+"/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLBHandlerDelete(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupLBHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(lbPath+"/:id", handler.Delete)

	id := uuid.New()
	svc.On("Delete", mock.Anything, id.String()).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, lbPath+"/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLBHandlerAddTarget(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupLBHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(lbPath+"/:id/targets", handler.AddTarget)

	lbID := uuid.New()
	instID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		svc.On("AddTarget", mock.Anything, lbID, instID, 8080, 10).Return(nil)
		body, err := json.Marshal(map[string]interface{}{
			"instance_id": instID.String(),
			"port":        8080,
			"weight":      10,
		})
		require.NoError(t, err)
		w := httptest.NewRecorder()
		req, err := http.NewRequest("POST", lbPath+"/"+lbID.String()+"/targets", bytes.NewBuffer(body))
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("PortZero_Rejected", func(t *testing.T) {
		body, err := json.Marshal(map[string]interface{}{
			"instance_id": instID.String(),
			"port":        0,
			"weight":      10,
		})
		require.NoError(t, err)
		w := httptest.NewRecorder()
		req, err := http.NewRequest("POST", lbPath+"/"+lbID.String()+"/targets", bytes.NewBuffer(body))
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("PortNegative_Rejected", func(t *testing.T) {
		body, err := json.Marshal(map[string]interface{}{
			"instance_id": instID.String(),
			"port":        -1,
			"weight":      10,
		})
		require.NoError(t, err)
		w := httptest.NewRecorder()
		req, err := http.NewRequest("POST", lbPath+"/"+lbID.String()+"/targets", bytes.NewBuffer(body))
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("PortExceedsMax_Rejected", func(t *testing.T) {
		body, err := json.Marshal(map[string]interface{}{
			"instance_id": instID.String(),
			"port":        65536,
			"weight":      10,
		})
		require.NoError(t, err)
		w := httptest.NewRecorder()
		req, err := http.NewRequest("POST", lbPath+"/"+lbID.String()+"/targets", bytes.NewBuffer(body))
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestLBHandlerRemoveTarget(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupLBHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(lbPath+"/:id/targets/:instanceId", handler.RemoveTarget)

	lbID := uuid.New()
	instID := uuid.New()
	svc.On("RemoveTarget", mock.Anything, lbID, instID).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, lbPath+"/"+lbID.String()+"/targets/"+instID.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLBHandlerListTargets(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupLBHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(lbPath+"/:id/targets", handler.ListTargets)

	lbID := uuid.New()
	targets := []*domain.LBTarget{{InstanceID: uuid.New()}}
	svc.On("ListTargets", mock.Anything, lbID).Return(targets, nil)

	req := httptest.NewRequest(http.MethodGet, lbPath+"/"+lbID.String()+"/targets", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
