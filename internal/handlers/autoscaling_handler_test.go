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

type mockAutoScalingService struct {
	mock.Mock
}

func (m *mockAutoScalingService) CreateGroup(ctx context.Context, name string, vpcID uuid.UUID, image, ports string, min, max, desired int, lbID *uuid.UUID, idempotencyKey string) (*domain.ScalingGroup, error) {
	args := m.Called(ctx, name, vpcID, image, ports, min, max, desired, lbID, idempotencyKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ScalingGroup), args.Error(1)
}

func (m *mockAutoScalingService) ListGroups(ctx context.Context) ([]*domain.ScalingGroup, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ScalingGroup), args.Error(1)
}

func (m *mockAutoScalingService) GetGroup(ctx context.Context, id uuid.UUID) (*domain.ScalingGroup, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ScalingGroup), args.Error(1)
}

func (m *mockAutoScalingService) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockAutoScalingService) CreatePolicy(ctx context.Context, groupID uuid.UUID, name, metricType string, targetValue float64, scaleOut, scaleIn, cooldown int) (*domain.ScalingPolicy, error) {
	args := m.Called(ctx, groupID, name, metricType, targetValue, scaleOut, scaleIn, cooldown)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ScalingPolicy), args.Error(1)
}

func (m *mockAutoScalingService) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockAutoScalingService) SetDesiredCapacity(ctx context.Context, groupID uuid.UUID, desired int) error {
	args := m.Called(ctx, groupID, desired)
	return args.Error(0)
}

func TestAutoScalingHandler_CreateGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockAutoScalingService)
	handler := NewAutoScalingHandler(svc)

	r := gin.New()
	r.POST("/autoscaling/groups", handler.CreateGroup)

	vpcID := uuid.New()
	group := &domain.ScalingGroup{ID: uuid.New(), Name: "asg-1"}
	svc.On("CreateGroup", mock.Anything, "asg-1", vpcID, "alpine", "80:80", 1, 5, 2, (*uuid.UUID)(nil), "").Return(group, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"name":          "asg-1",
		"vpc_id":        vpcID.String(),
		"image":         "alpine",
		"ports":         "80:80",
		"min_instances": 1,
		"max_instances": 5,
		"desired_count": 2,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/autoscaling/groups", bytes.NewBuffer(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAutoScalingHandler_ListGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockAutoScalingService)
	handler := NewAutoScalingHandler(svc)

	r := gin.New()
	r.GET("/autoscaling/groups", handler.ListGroups)

	groups := []*domain.ScalingGroup{{ID: uuid.New(), Name: "asg-1"}}
	svc.On("ListGroups", mock.Anything).Return(groups, nil)

	req := httptest.NewRequest(http.MethodGet, "/autoscaling/groups", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAutoScalingHandler_GetGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockAutoScalingService)
	handler := NewAutoScalingHandler(svc)

	r := gin.New()
	r.GET("/autoscaling/groups/:id", handler.GetGroup)

	id := uuid.New()
	group := &domain.ScalingGroup{ID: id, Name: "asg-1"}
	svc.On("GetGroup", mock.Anything, id).Return(group, nil)

	req := httptest.NewRequest(http.MethodGet, "/autoscaling/groups/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAutoScalingHandler_DeleteGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockAutoScalingService)
	handler := NewAutoScalingHandler(svc)

	r := gin.New()
	r.DELETE("/autoscaling/groups/:id", handler.DeleteGroup)

	id := uuid.New()
	svc.On("DeleteGroup", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/autoscaling/groups/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestAutoScalingHandler_CreatePolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockAutoScalingService)
	handler := NewAutoScalingHandler(svc)

	r := gin.New()
	r.POST("/autoscaling/groups/:id/policies", handler.CreatePolicy)

	groupID := uuid.New()
	policy := &domain.ScalingPolicy{ID: uuid.New(), Name: "policy-1"}
	svc.On("CreatePolicy", mock.Anything, groupID, "policy-1", "cpu", 80.0, 1, 1, 60).Return(policy, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"name":           "policy-1",
		"metric_type":    "cpu",
		"target_value":   80.0,
		"scale_out_step": 1,
		"scale_in_step":  1,
		"cooldown_sec":   60,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/autoscaling/groups/"+groupID.String()+"/policies", bytes.NewBuffer(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAutoScalingHandler_DeletePolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockAutoScalingService)
	handler := NewAutoScalingHandler(svc)

	r := gin.New()
	r.DELETE("/autoscaling/policies/:id", handler.DeletePolicy)

	id := uuid.New()
	svc.On("DeletePolicy", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/autoscaling/policies/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestAutoScalingHandler_CreateGroup_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockAutoScalingService)
	handler := NewAutoScalingHandler(svc)
	r := gin.New()
	r.POST("/autoscaling/groups", handler.CreateGroup)

	t.Run("InvalidInput", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/autoscaling/groups", bytes.NewBufferString("invalid json"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc.On("CreateGroup", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError).Once()

		body, _ := json.Marshal(map[string]interface{}{
			"name": "asg-err", "vpc_id": uuid.New().String(), "image": "alpine",
			"ports": "80:80", "min_instances": 1, "max_instances": 5, "desired_count": 2,
		})
		req := httptest.NewRequest(http.MethodPost, "/autoscaling/groups", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestAutoScalingHandler_GetGroup_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockAutoScalingService)
	handler := NewAutoScalingHandler(svc)
	r := gin.New()
	r.GET("/autoscaling/groups/:id", handler.GetGroup)

	t.Run("InvalidID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/autoscaling/groups/invalid-id", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		id := uuid.New()
		svc.On("GetGroup", mock.Anything, id).Return(nil, assert.AnError).Once()
		req := httptest.NewRequest(http.MethodGet, "/autoscaling/groups/"+id.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestAutoScalingHandler_DeleteGroup_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockAutoScalingService)
	handler := NewAutoScalingHandler(svc)
	r := gin.New()
	r.DELETE("/autoscaling/groups/:id", handler.DeleteGroup)

	t.Run("InvalidID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/autoscaling/groups/invalid-id", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		id := uuid.New()
		svc.On("DeleteGroup", mock.Anything, id).Return(assert.AnError).Once()
		req := httptest.NewRequest(http.MethodDelete, "/autoscaling/groups/"+id.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestAutoScalingHandler_ListGroups_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockAutoScalingService)
	handler := NewAutoScalingHandler(svc)
	r := gin.New()
	r.GET("/autoscaling/groups", handler.ListGroups)

	svc.On("ListGroups", mock.Anything).Return(nil, assert.AnError).Once()
	req := httptest.NewRequest(http.MethodGet, "/autoscaling/groups", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAutoScalingHandler_CreatePolicy_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockAutoScalingService)
	handler := NewAutoScalingHandler(svc)
	r := gin.New()
	r.POST("/autoscaling/groups/:id/policies", handler.CreatePolicy)

	t.Run("InvalidGroupID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/autoscaling/groups/invalid-id/policies", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("InvalidInput", func(t *testing.T) {
		id := uuid.New()
		req := httptest.NewRequest(http.MethodPost, "/autoscaling/groups/"+id.String()+"/policies", bytes.NewBufferString("invalid"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		id := uuid.New()
		svc.On("CreatePolicy", mock.Anything, id, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError).Once()

		body, _ := json.Marshal(map[string]interface{}{
			"name": "p1", "metric_type": "cpu", "target_value": 50, "scale_out_step": 1, "scale_in_step": 1, "cooldown_sec": 60,
		})
		req := httptest.NewRequest(http.MethodPost, "/autoscaling/groups/"+id.String()+"/policies", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestAutoScalingHandler_DeletePolicy_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockAutoScalingService)
	handler := NewAutoScalingHandler(svc)
	r := gin.New()
	r.DELETE("/autoscaling/policies/:id", handler.DeletePolicy)

	t.Run("InvalidID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/autoscaling/policies/invalid-id", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		id := uuid.New()
		svc.On("DeletePolicy", mock.Anything, id).Return(assert.AnError).Once()
		req := httptest.NewRequest(http.MethodDelete, "/autoscaling/policies/"+id.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
