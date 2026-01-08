package httphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type instanceServiceMock struct {
	mock.Mock
}

func (m *instanceServiceMock) LaunchInstance(ctx context.Context, name, image, ports string, vpcID, subnetID *uuid.UUID, volumes []domain.VolumeAttachment) (*domain.Instance, error) {
	args := m.Called(ctx, name, image, ports, vpcID, subnetID, volumes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Instance), args.Error(1)
}

func (m *instanceServiceMock) StopInstance(ctx context.Context, idOrName string) error {
	args := m.Called(ctx, idOrName)
	return args.Error(0)
}

func (m *instanceServiceMock) ListInstances(ctx context.Context) ([]*domain.Instance, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Instance), args.Error(1)
}

func (m *instanceServiceMock) GetInstance(ctx context.Context, idOrName string) (*domain.Instance, error) {
	args := m.Called(ctx, idOrName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Instance), args.Error(1)
}

func (m *instanceServiceMock) GetInstanceLogs(ctx context.Context, idOrName string) (string, error) {
	args := m.Called(ctx, idOrName)
	return args.String(0), args.Error(1)
}

func (m *instanceServiceMock) GetInstanceStats(ctx context.Context, idOrName string) (*domain.InstanceStats, error) {
	args := m.Called(ctx, idOrName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.InstanceStats), args.Error(1)
}

func (m *instanceServiceMock) TerminateInstance(ctx context.Context, idOrName string) error {
	args := m.Called(ctx, idOrName)
	return args.Error(0)
}

func setupInstanceHandlerTest(t *testing.T) (*instanceServiceMock, *InstanceHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(instanceServiceMock)
	handler := NewInstanceHandler(mockSvc)
	r := gin.New()
	return mockSvc, handler, r
}

func TestInstanceHandler_LaunchRejectsEmptyImage(t *testing.T) {
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST("/instances", handler.Launch)

	body := `{"name":"test-inst","image":"   "}`
	req := httptest.NewRequest(http.MethodPost, "/instances", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var wrapper struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &wrapper))
	assert.Equal(t, "INVALID_INPUT", wrapper.Error.Type)
	mockSvc.AssertNotCalled(t, "LaunchInstance", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestInstanceHandler_Launch(t *testing.T) {
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST("/instances", handler.Launch)

	inst := &domain.Instance{ID: uuid.New(), Name: "test-inst"}
	mockSvc.On("LaunchInstance", mock.Anything, "test-inst", "alpine", "", (*uuid.UUID)(nil), (*uuid.UUID)(nil), []domain.VolumeAttachment(nil)).Return(inst, nil)

	body := `{"name":"test-inst","image":"alpine"}`
	req := httptest.NewRequest(http.MethodPost, "/instances", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestInstanceHandler_List(t *testing.T) {
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.GET("/instances", handler.List)

	instances := []*domain.Instance{{ID: uuid.New(), Name: "test-inst"}}
	mockSvc.On("ListInstances", mock.Anything).Return(instances, nil)

	req := httptest.NewRequest(http.MethodGet, "/instances", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInstanceHandler_Get(t *testing.T) {
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.GET("/instances/:id", handler.Get)

	id := uuid.New().String()
	inst := &domain.Instance{ID: uuid.MustParse(id), Name: "test-inst"}
	mockSvc.On("GetInstance", mock.Anything, id).Return(inst, nil)

	req := httptest.NewRequest(http.MethodGet, "/instances/"+id, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInstanceHandler_Stop(t *testing.T) {
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST("/instances/:id/stop", handler.Stop)

	id := uuid.New().String()
	mockSvc.On("StopInstance", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/instances/"+id+"/stop", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInstanceHandler_Terminate(t *testing.T) {
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.DELETE("/instances/:id", handler.Terminate)

	id := uuid.New().String()
	mockSvc.On("TerminateInstance", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/instances/"+id, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInstanceHandler_GetLogs(t *testing.T) {
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.GET("/instances/:id/logs", handler.GetLogs)

	id := uuid.New().String()
	mockSvc.On("GetInstanceLogs", mock.Anything, id).Return("logs content", nil)

	req := httptest.NewRequest(http.MethodGet, "/instances/"+id+"/logs", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "logs content", w.Body.String())
}

func TestInstanceHandler_GetStats(t *testing.T) {
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.GET("/instances/:id/stats", handler.GetStats)

	id := uuid.New().String()
	stats := &domain.InstanceStats{CPUPercentage: 10.5, MemoryUsageBytes: 128}
	mockSvc.On("GetInstanceStats", mock.Anything, id).Return(stats, nil)

	req := httptest.NewRequest(http.MethodGet, "/instances/"+id+"/stats", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInstanceHandler_Launch_WithVolumesAndVPC(t *testing.T) {
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST("/instances", handler.Launch)

	vpcID := uuid.New()
	volID := "vol-123"

	inst := &domain.Instance{ID: uuid.New(), Name: "test-complex", VpcID: &vpcID}

	expectedVolumes := []domain.VolumeAttachment{
		{VolumeIDOrName: volID, MountPath: "/mnt/data"},
	}

	mockSvc.On("LaunchInstance", mock.Anything, "test-complex", "ubuntu", "80:80", &vpcID, (*uuid.UUID)(nil), expectedVolumes).Return(inst, nil)

	body := map[string]interface{}{
		"name":   "test-complex",
		"image":  "ubuntu",
		"ports":  "80:80",
		"vpc_id": vpcID.String(),
		"volumes": []map[string]string{
			{"volume_id": volID, "mount_path": "/mnt/data"},
		},
	}
	jsonBody, err := json.Marshal(body)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/instances", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}
