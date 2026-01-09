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

const (
	asgPath        = "/autoscaling/groups"
	policyPath     = "/autoscaling/policies"
	testAsgName    = "asg-1"
	testPolicyName = "policy-1"
	invalidIDPath  = "/invalid-id"
	policiesSuffix = "/policies"
	port8080       = "80:80"
	imageAlpine    = "alpine"
	metricCPU      = "cpu"
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
	return m.Called(ctx, id).Error(0)
}

func (m *mockAutoScalingService) CreatePolicy(ctx context.Context, groupID uuid.UUID, name, metricType string, targetValue float64, scaleOut, scaleIn, cooldown int) (*domain.ScalingPolicy, error) {
	args := m.Called(ctx, groupID, name, metricType, targetValue, scaleOut, scaleIn, cooldown)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ScalingPolicy), args.Error(1)
}

func (m *mockAutoScalingService) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *mockAutoScalingService) SetDesiredCapacity(ctx context.Context, groupID uuid.UUID, desired int) error {
	args := m.Called(ctx, groupID, desired)
	return args.Error(0)
}

func setupAutoScalingHandlerTest(t *testing.T) (*mockAutoScalingService, *AutoScalingHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockAutoScalingService)
	handler := NewAutoScalingHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestAutoScalingHandlerCreateGroup(t *testing.T) {
	svc, handler, r := setupAutoScalingHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(asgPath, handler.CreateGroup)

	vpcID := uuid.New()
	group := &domain.ScalingGroup{ID: uuid.New(), Name: testAsgName}
	svc.On("CreateGroup", mock.Anything, testAsgName, vpcID, imageAlpine, port8080, 1, 5, 2, (*uuid.UUID)(nil), "").Return(group, nil)

	body, err := json.Marshal(map[string]interface{}{
		"name":          testAsgName,
		"vpc_id":        vpcID.String(),
		"image":         imageAlpine,
		"ports":         port8080,
		"min_instances": 1,
		"max_instances": 5,
		"desired_count": 2,
	})
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", asgPath, bytes.NewBuffer(body))
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAutoScalingHandlerListGroups(t *testing.T) {
	svc, handler, r := setupAutoScalingHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(asgPath, handler.ListGroups)

	groups := []*domain.ScalingGroup{{ID: uuid.New(), Name: testAsgName}}
	svc.On("ListGroups", mock.Anything).Return(groups, nil)

	req, err := http.NewRequest(http.MethodGet, asgPath, nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAutoScalingHandlerGetGroup(t *testing.T) {
	svc, handler, r := setupAutoScalingHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(asgPath+"/:id", handler.GetGroup)

	id := uuid.New()
	group := &domain.ScalingGroup{ID: id, Name: testAsgName}
	svc.On("GetGroup", mock.Anything, id).Return(group, nil)

	req, err := http.NewRequest(http.MethodGet, asgPath+"/"+id.String(), nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAutoScalingHandlerDeleteGroup(t *testing.T) {
	svc, handler, r := setupAutoScalingHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(asgPath+"/:id", handler.DeleteGroup)

	id := uuid.New()
	svc.On("DeleteGroup", mock.Anything, id).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, asgPath+"/"+id.String(), nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestAutoScalingHandlerCreatePolicy(t *testing.T) {
	svc, handler, r := setupAutoScalingHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(asgPath+"/:id"+policiesSuffix, handler.CreatePolicy)

	groupID := uuid.New()
	policy := &domain.ScalingPolicy{ID: uuid.New(), Name: testPolicyName}
	svc.On("CreatePolicy", mock.Anything, groupID, testPolicyName, metricCPU, 80.0, 1, 1, 60).Return(policy, nil)

	body, err := json.Marshal(map[string]interface{}{
		"name":           testPolicyName,
		"metric_type":    metricCPU,
		"target_value":   80.0,
		"scale_out_step": 1,
		"scale_in_step":  1,
		"cooldown_sec":   60,
	})
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", asgPath+"/"+groupID.String()+policiesSuffix, bytes.NewBuffer(body))
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAutoScalingHandlerDeletePolicy(t *testing.T) {
	svc, handler, r := setupAutoScalingHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(policyPath+"/:id", handler.DeletePolicy)

	id := uuid.New()
	svc.On("DeletePolicy", mock.Anything, id).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, policyPath+"/"+id.String(), nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestAutoScalingHandlerCreateGroupErrors(t *testing.T) {
	svc, handler, r := setupAutoScalingHandlerTest(t)
	defer svc.AssertExpectations(t)
	r.POST(asgPath, handler.CreateGroup)

	t.Run("InvalidInput", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, asgPath, bytes.NewBufferString("invalid json"))
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc.On("CreateGroup", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError).Once()

		body, err := json.Marshal(map[string]interface{}{
			"name": "asg-err", "vpc_id": uuid.New().String(), "image": imageAlpine,
			"ports": port8080, "min_instances": 1, "max_instances": 5, "desired_count": 2,
		})
		assert.NoError(t, err)
		req, err := http.NewRequest(http.MethodPost, asgPath, bytes.NewBuffer(body))
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestAutoScalingHandlerGetGroupErrors(t *testing.T) {
	svc, handler, r := setupAutoScalingHandlerTest(t)
	defer svc.AssertExpectations(t)
	r.GET(asgPath+"/:id", handler.GetGroup)

	t.Run("InvalidID", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, asgPath+invalidIDPath, nil)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		id := uuid.New()
		svc.On("GetGroup", mock.Anything, id).Return(nil, assert.AnError).Once()
		req, err := http.NewRequest(http.MethodGet, asgPath+"/"+id.String(), nil)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestAutoScalingHandlerDeleteGroupErrors(t *testing.T) {
	svc, handler, r := setupAutoScalingHandlerTest(t)
	defer svc.AssertExpectations(t)
	r.DELETE(asgPath+"/:id", handler.DeleteGroup)

	t.Run("InvalidID", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, asgPath+invalidIDPath, nil)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		id := uuid.New()
		svc.On("DeleteGroup", mock.Anything, id).Return(assert.AnError).Once()
		req, err := http.NewRequest(http.MethodDelete, asgPath+"/"+id.String(), nil)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestAutoScalingHandlerListGroupsError(t *testing.T) {
	svc, handler, r := setupAutoScalingHandlerTest(t)
	defer svc.AssertExpectations(t)
	r.GET(asgPath, handler.ListGroups)

	svc.On("ListGroups", mock.Anything).Return(nil, assert.AnError).Once()
	req, err := http.NewRequest(http.MethodGet, asgPath, nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAutoScalingHandlerCreatePolicyErrors(t *testing.T) {
	svc, handler, r := setupAutoScalingHandlerTest(t)
	defer svc.AssertExpectations(t)
	r.POST(asgPath+"/:id"+policiesSuffix, handler.CreatePolicy)

	t.Run("InvalidGroupID", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, asgPath+invalidIDPath+policiesSuffix, nil)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("InvalidInput", func(t *testing.T) {
		id := uuid.New()
		req, err := http.NewRequest(http.MethodPost, asgPath+"/"+id.String()+policiesSuffix, bytes.NewBufferString("invalid"))
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		id := uuid.New()
		svc.On("CreatePolicy", mock.Anything, id, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError).Once()

		body, err := json.Marshal(map[string]interface{}{
			"name": "p1", "metric_type": metricCPU, "target_value": 50, "scale_out_step": 1, "scale_in_step": 1, "cooldown_sec": 60,
		})
		assert.NoError(t, err)
		req, err := http.NewRequest(http.MethodPost, asgPath+"/"+id.String()+policiesSuffix, bytes.NewBuffer(body))
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestAutoScalingHandlerDeletePolicyErrors(t *testing.T) {
	svc, handler, r := setupAutoScalingHandlerTest(t)
	defer svc.AssertExpectations(t)
	r.DELETE(policyPath+"/:id", handler.DeletePolicy)

	t.Run("InvalidID", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, policyPath+invalidIDPath, nil)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		id := uuid.New()
		svc.On("DeletePolicy", mock.Anything, id).Return(assert.AnError).Once()
		req, err := http.NewRequest(http.MethodDelete, policyPath+"/"+id.String(), nil)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
