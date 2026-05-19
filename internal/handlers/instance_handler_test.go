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
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testInstanceName = "test-inst"
	instancesPath    = "/instances"
	contentType      = "Content-Type"
	applicationJSON  = "application/json"
	complexTestName  = "test-complex"
)

type instanceServiceMock struct {
	mock.Mock
}

func (m *instanceServiceMock) LaunchInstance(ctx context.Context, params ports.LaunchParams) (*domain.Instance, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}

func (m *instanceServiceMock) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (*domain.Instance, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}

func (m *instanceServiceMock) Provision(ctx context.Context, job domain.ProvisionJob) error {
	return m.Called(ctx, job).Error(0)
}

func (m *instanceServiceMock) StartInstance(ctx context.Context, idOrName string) error {
	return m.Called(ctx, idOrName).Error(0)
}

func (m *instanceServiceMock) StopInstance(ctx context.Context, idOrName string) error {
	return m.Called(ctx, idOrName).Error(0)
}
func (m *instanceServiceMock) PauseInstance(ctx context.Context, idOrName string) error {
	return m.Called(ctx, idOrName).Error(0)
}
func (m *instanceServiceMock) ResumeInstance(ctx context.Context, idOrName string) error {
	return m.Called(ctx, idOrName).Error(0)
}

func (m *instanceServiceMock) ListInstances(ctx context.Context, tagFilter []string) ([]*domain.Instance, error) {
	args := m.Called(ctx, tagFilter)
	r0, _ := args.Get(0).([]*domain.Instance)
	return r0, args.Error(1)
}

func (m *instanceServiceMock) GetInstance(ctx context.Context, idOrName string) (*domain.Instance, error) {
	args := m.Called(ctx, idOrName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}

func (m *instanceServiceMock) GetInstanceLogs(ctx context.Context, idOrName string) (string, error) {
	args := m.Called(ctx, idOrName)
	logs := args.String(0)
	return logs, args.Error(1)
}

func (m *instanceServiceMock) GetInstanceStats(ctx context.Context, idOrName string) (*domain.InstanceStats, error) {
	args := m.Called(ctx, idOrName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.InstanceStats)
	return r0, args.Error(1)
}

func (m *instanceServiceMock) GetConsoleURL(ctx context.Context, idOrName string) (string, error) {
	args := m.Called(ctx, idOrName)
	return args.String(0), args.Error(1)
}

func (m *instanceServiceMock) TerminateInstance(ctx context.Context, idOrName string) error {
	// TerminateInstance permanently deletes an instance
	args := m.Called(ctx, idOrName)
	return args.Error(0)
}

func (m *instanceServiceMock) Exec(ctx context.Context, idOrName string, cmd []string) (string, error) {
	args := m.Called(ctx, idOrName, cmd)
	return args.String(0), args.Error(1)
}

func (m *instanceServiceMock) UpdateInstanceMetadata(ctx context.Context, id uuid.UUID, metadata, labels map[string]string) error {
	return m.Called(ctx, id, metadata, labels).Error(0)
}

func (m *instanceServiceMock) ResizeInstance(ctx context.Context, idOrName, newInstanceType string) (*domain.Instance, error) {
	args := m.Called(ctx, idOrName, newInstanceType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Instance), args.Error(1)
}

func (m *instanceServiceMock) GetTags(ctx context.Context, id uuid.UUID) (map[string]string, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(map[string]string)
	return r0, args.Error(1)
}

func (m *instanceServiceMock) SetTags(ctx context.Context, id uuid.UUID, labels map[string]string) error {
	return m.Called(ctx, id, labels).Error(0)
}

func (m *instanceServiceMock) RemoveTag(ctx context.Context, id uuid.UUID, key string) error {
	return m.Called(ctx, id, key).Error(0)
}

func setupInstanceHandlerTest(_ *testing.T) (*instanceServiceMock, *InstanceHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(instanceServiceMock)
	handler := NewInstanceHandler(mockSvc)
	r := gin.New()
	return mockSvc, handler, r
}

func TestInstanceHandlerLaunchRejectsEmptyImage(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST(instancesPath, handler.Launch)

	body := `{"name":"` + testInstanceName + `","image":"   "}`
	req := httptest.NewRequest(http.MethodPost, instancesPath, strings.NewReader(body))
	req.Header.Set(contentType, applicationJSON)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var wrapper struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wrapper))
	assert.Equal(t, "INVALID_INPUT", wrapper.Error.Type)
	mockSvc.AssertNotCalled(t, "LaunchInstance", mock.Anything, mock.Anything)
}

func TestInstanceHandlerLaunchRejectsInvalidJSON(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST(instancesPath, handler.Launch)

	req := httptest.NewRequest(http.MethodPost, instancesPath, strings.NewReader("{invalid"))
	req.Header.Set(contentType, applicationJSON)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockSvc.AssertNotCalled(t, "LaunchInstance", mock.Anything, mock.Anything)
}

func TestInstanceHandlerLaunchRejectsInvalidNameCharacters(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST(instancesPath, handler.Launch)

	body := `{"name":"bad$name","image":"alpine"}`
	req := httptest.NewRequest(http.MethodPost, instancesPath, strings.NewReader(body))
	req.Header.Set(contentType, applicationJSON)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockSvc.AssertNotCalled(t, "LaunchInstance", mock.Anything, mock.Anything)
}

func TestInstanceHandlerLaunch(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST(instancesPath, handler.Launch)

	inst := &domain.Instance{ID: uuid.New(), Name: testInstanceName}
	mockSvc.On("LaunchInstance", mock.Anything, ports.LaunchParams{
		Name:    testInstanceName,
		Image:   "alpine",
		Volumes: []domain.VolumeAttachment{},
	}).Return(inst, nil)

	body := `{"name":"` + testInstanceName + `","image":"alpine"}`
	req := httptest.NewRequest(http.MethodPost, instancesPath, strings.NewReader(body))
	req.Header.Set(contentType, applicationJSON)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestInstanceHandlerLaunchRejectsInvalidMountPath(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST(instancesPath, handler.Launch)

	body := `{"name":"` + testInstanceName + `","image":"alpine","volumes":[{"volume_id":"vol-1","mount_path":"mnt/data"}]}`
	req := httptest.NewRequest(http.MethodPost, instancesPath, strings.NewReader(body))
	req.Header.Set(contentType, applicationJSON)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockSvc.AssertNotCalled(t, "LaunchInstance", mock.Anything, mock.Anything)
}

func TestInstanceHandlerList(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.GET(instancesPath, handler.List)

	instances := []*domain.Instance{{ID: uuid.New(), Name: testInstanceName}}
	mockSvc.On("ListInstances", mock.Anything, []string(nil)).Return(instances, nil)

	req := httptest.NewRequest(http.MethodGet, instancesPath, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInstanceHandlerGet(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.GET(instancesPath+"/:id", handler.Get)

	id := uuid.New().String()
	inst := &domain.Instance{ID: uuid.MustParse(id), Name: testInstanceName}
	mockSvc.On("GetInstance", mock.Anything, id).Return(inst, nil)

	req := httptest.NewRequest(http.MethodGet, instancesPath+"/"+id, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInstanceHandlerStop(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST(instancesPath+"/:id/stop", handler.Stop)

	id := uuid.New().String()
	mockSvc.On("StopInstance", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodPost, instancesPath+"/"+id+"/stop", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInstanceHandlerTerminate(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.DELETE(instancesPath+"/:id", handler.Terminate)

	id := uuid.New().String()
	mockSvc.On("TerminateInstance", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, instancesPath+"/"+id, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInstanceHandlerStopNotFound(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST(instancesPath+"/:id/stop", handler.Stop)

	id := uuid.New().String()
	mockSvc.On("StopInstance", mock.Anything, id).Return(errors.New(errors.NotFound, "not found"))

	req := httptest.NewRequest(http.MethodPost, instancesPath+"/"+id+"/stop", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestInstanceHandlerTerminateNotFound(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.DELETE(instancesPath+"/:id", handler.Terminate)

	id := uuid.New().String()
	mockSvc.On("TerminateInstance", mock.Anything, id).Return(errors.New(errors.NotFound, "not found"))

	req := httptest.NewRequest(http.MethodDelete, instancesPath+"/"+id, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestInstanceHandlerPause(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST(instancesPath+"/:id/pause", handler.Pause)

	id := uuid.New().String()
	mockSvc.On("PauseInstance", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodPost, instancesPath+"/"+id+"/pause", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInstanceHandlerPauseNotFound(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST(instancesPath+"/:id/pause", handler.Pause)

	id := uuid.New().String()
	mockSvc.On("PauseInstance", mock.Anything, id).Return(errors.New(errors.NotFound, "not found"))

	req := httptest.NewRequest(http.MethodPost, instancesPath+"/"+id+"/pause", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestInstanceHandlerPauseConflict(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST(instancesPath+"/:id/pause", handler.Pause)

	id := uuid.New().String()
	mockSvc.On("PauseInstance", mock.Anything, id).Return(errors.New(errors.Conflict, "instance not in RUNNING state"))

	req := httptest.NewRequest(http.MethodPost, instancesPath+"/"+id+"/pause", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestInstanceHandlerResume(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST(instancesPath+"/:id/resume", handler.Resume)

	id := uuid.New().String()
	mockSvc.On("ResumeInstance", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodPost, instancesPath+"/"+id+"/resume", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInstanceHandlerResumeNotFound(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST(instancesPath+"/:id/resume", handler.Resume)

	id := uuid.New().String()
	mockSvc.On("ResumeInstance", mock.Anything, id).Return(errors.New(errors.NotFound, "not found"))

	req := httptest.NewRequest(http.MethodPost, instancesPath+"/"+id+"/resume", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestInstanceHandlerResumeConflict(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST(instancesPath+"/:id/resume", handler.Resume)

	id := uuid.New().String()
	mockSvc.On("ResumeInstance", mock.Anything, id).Return(errors.New(errors.Conflict, "instance not in PAUSED state"))

	req := httptest.NewRequest(http.MethodPost, instancesPath+"/"+id+"/resume", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestInstanceHandlerGetLogs(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.GET(instancesPath+"/:id/logs", handler.GetLogs)

	id := uuid.New().String()
	mockSvc.On("GetInstanceLogs", mock.Anything, id).Return("logs content", nil)

	req := httptest.NewRequest(http.MethodGet, instancesPath+"/"+id+"/logs", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "logs content", w.Body.String())
}

func ptrUint64(v uint64) *uint64 { return &v }

func TestInstanceHandlerGetStats(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.GET(instancesPath+"/:id/stats", handler.GetStats)

	id := uuid.New().String()
	stats := &domain.InstanceStats{
		CPUPercentage:      10.5,
		MemoryUsageBytes:   128,
		MemoryLimitBytes:   256,
		MemoryPercentage:   50.0,
		NetworkRxBytes:     ptrUint64(1024),
		NetworkTxBytes:     ptrUint64(512),
		DiskReadBytes:      ptrUint64(4096),
		DiskWriteBytes:     ptrUint64(2048),
		CPUTimeNanoseconds: ptrUint64(3000000000),
	}
	mockSvc.On("GetInstanceStats", mock.Anything, id).Return(stats, nil)

	req := httptest.NewRequest(http.MethodGet, instancesPath+"/"+id+"/stats", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify all new stats fields are present in the JSON response
	// httputil.Success wraps data in {"data": {...}}
	var wrapper struct {
		Data domain.InstanceStats `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &wrapper)
	require.NoError(t, err)

	assert.InDelta(t, 10.5, wrapper.Data.CPUPercentage, 0.01)
	assert.InDelta(t, 128, wrapper.Data.MemoryUsageBytes, 0.01)
	assert.InDelta(t, 256, wrapper.Data.MemoryLimitBytes, 0.01)
	assert.InDelta(t, 50.0, wrapper.Data.MemoryPercentage, 0.01)
	assert.Equal(t, uint64(1024), *wrapper.Data.NetworkRxBytes)
	assert.Equal(t, uint64(512), *wrapper.Data.NetworkTxBytes)
	assert.Equal(t, uint64(4096), *wrapper.Data.DiskReadBytes)
	assert.Equal(t, uint64(2048), *wrapper.Data.DiskWriteBytes)
	assert.Equal(t, uint64(3000000000), *wrapper.Data.CPUTimeNanoseconds)
}

func TestInstanceHandlerLaunchWithVolumesAndVPC(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST(instancesPath, handler.Launch)

	vpcID := uuid.New()
	volID := "vol-123"

	inst := &domain.Instance{ID: uuid.New(), Name: complexTestName, VpcID: &vpcID}

	expectedVolumes := []domain.VolumeAttachment{
		{VolumeIDOrName: volID, MountPath: "/mnt/data"},
	}

	mockSvc.On("LaunchInstance", mock.Anything, ports.LaunchParams{
		Name:    complexTestName,
		Image:   "ubuntu",
		Ports:   "80:80",
		VpcID:   &vpcID,
		Volumes: expectedVolumes,
	}).Return(inst, nil)

	body := map[string]interface{}{
		"name":   complexTestName,
		"image":  "ubuntu",
		"ports":  "80:80",
		"vpc_id": vpcID.String(),
		"volumes": []map[string]string{
			{"volume_id": volID, "mount_path": "/mnt/data"},
		},
	}
	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, instancesPath, bytes.NewBuffer(jsonBody))
	req.Header.Set(contentType, applicationJSON)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestInstanceHandlerLaunchRejectsInvalidVPCID(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.POST(instancesPath, handler.Launch)

	body := `{"name":"` + testInstanceName + `","image":"ubuntu","vpc_id":"not-a-uuid"}`
	req := httptest.NewRequest(http.MethodPost, instancesPath, strings.NewReader(body))
	req.Header.Set(contentType, applicationJSON)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockSvc.AssertNotCalled(t, "LaunchInstance", mock.Anything, mock.Anything)
}

func TestInstanceHandlerGetConsole(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.GET(instancesPath+"/:id/console", handler.GetConsole)

	id := uuid.New().String()
	url := "vnc://localhost:5900"
	mockSvc.On("GetConsoleURL", mock.Anything, id).Return(url, nil)

	req := httptest.NewRequest(http.MethodGet, instancesPath+"/"+id+"/console", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data map[string]string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, url, resp.Data["url"])
}

func TestInstanceHandlerGetConsoleError(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.GET(instancesPath+"/:id/console", handler.GetConsole)

	id := uuid.New().String()
	mockSvc.On("GetConsoleURL", mock.Anything, id).Return("", errors.New(errors.Internal, "console error"))

	req := httptest.NewRequest(http.MethodGet, instancesPath+"/"+id+"/console", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestInstanceHandlerUpdateMetadata(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupInstanceHandlerTest(t)
	defer mockSvc.AssertExpectations(t)
	r.PUT(instancesPath+"/:id/metadata", handler.UpdateMetadata)

	id := uuid.New()
	metadata := map[string]string{"key": "val"}

	mockSvc.On("UpdateInstanceMetadata", mock.Anything, id, metadata, mock.Anything).Return(nil)

	body := map[string]interface{}{
		"metadata": metadata,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, instancesPath+"/"+id.String()+"/metadata", bytes.NewBuffer(jsonBody))
	req.Header.Set(contentType, applicationJSON)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInstanceHandlerResizeInstance(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockSvc, handler, r := setupInstanceHandlerTest(t)
		defer mockSvc.AssertExpectations(t)
		r.POST(instancesPath+"/:id/resize", handler.ResizeInstance)

		id := uuid.New()
		mockSvc.On("ResizeInstance", mock.Anything, id.String(), "basic-4").Return(&domain.Instance{InstanceType: "basic-4", Status: domain.StatusRunning}, nil).Once()

		body := `{"instance_type":"basic-4"}`
		req := httptest.NewRequest(http.MethodPost, instancesPath+"/"+id.String()+"/resize", strings.NewReader(body))
		req.Header.Set(contentType, applicationJSON)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"message":"instance resized"`)
		assert.Contains(t, w.Body.String(), `"data"`)
	})

	t.Run("InvalidID", func(t *testing.T) {
		mockSvc, handler, r := setupInstanceHandlerTest(t)
		defer mockSvc.AssertExpectations(t)
		r.POST(instancesPath+"/:id/resize", handler.ResizeInstance)

		// Handler accepts name-or-uuid, passes raw string to service
		mockSvc.On("ResizeInstance", mock.Anything, "my-instance-name", "basic-4").Return(&domain.Instance{InstanceType: "basic-4", Status: domain.StatusRunning}, nil).Once()

		body := `{"instance_type":"basic-4"}`
		req := httptest.NewRequest(http.MethodPost, instancesPath+"/my-instance-name/resize", strings.NewReader(body))
		req.Header.Set(contentType, applicationJSON)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InvalidBody", func(t *testing.T) {
		mockSvc, handler, r := setupInstanceHandlerTest(t)
		defer mockSvc.AssertExpectations(t)
		r.POST(instancesPath+"/:id/resize", handler.ResizeInstance)

		id := uuid.New()
		req := httptest.NewRequest(http.MethodPost, instancesPath+"/"+id.String()+"/resize", strings.NewReader(`{invalid json}`))
		req.Header.Set(contentType, applicationJSON)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockSvc.AssertNotCalled(t, "ResizeInstance", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("EmptyInstanceType", func(t *testing.T) {
		mockSvc, handler, r := setupInstanceHandlerTest(t)
		defer mockSvc.AssertExpectations(t)
		r.POST(instancesPath+"/:id/resize", handler.ResizeInstance)

		id := uuid.New()
		req := httptest.NewRequest(http.MethodPost, instancesPath+"/"+id.String()+"/resize", strings.NewReader(`{"instance_type":""}`))
		req.Header.Set(contentType, applicationJSON)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockSvc.AssertNotCalled(t, "ResizeInstance", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("NotFound", func(t *testing.T) {
		mockSvc, handler, r := setupInstanceHandlerTest(t)
		defer mockSvc.AssertExpectations(t)
		r.POST(instancesPath+"/:id/resize", handler.ResizeInstance)

		id := uuid.New()
		mockSvc.On("ResizeInstance", mock.Anything, id.String(), "basic-4").Return(nil, errors.New(errors.NotFound, "instance not found")).Once()

		body := `{"instance_type":"basic-4"}`
		req := httptest.NewRequest(http.MethodPost, instancesPath+"/"+id.String()+"/resize", strings.NewReader(body))
		req.Header.Set(contentType, applicationJSON)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("QuotaExceeded", func(t *testing.T) {
		mockSvc, handler, r := setupInstanceHandlerTest(t)
		defer mockSvc.AssertExpectations(t)
		r.POST(instancesPath+"/:id/resize", handler.ResizeInstance)

		id := uuid.New()
		mockSvc.On("ResizeInstance", mock.Anything, id.String(), "basic-4").Return(nil, errors.New(errors.QuotaExceeded, "quota exceeded for resources")).Once()

		body := `{"instance_type":"basic-4"}`
		req := httptest.NewRequest(http.MethodPost, instancesPath+"/"+id.String()+"/resize", strings.NewReader(body))
		req.Header.Set(contentType, applicationJSON)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
	})
}
