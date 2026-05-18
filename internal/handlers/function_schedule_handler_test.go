package httphandlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockFunctionScheduleService struct {
	mock.Mock
}

func (m *mockFunctionScheduleService) CreateSchedule(ctx context.Context, functionID uuid.UUID, name, schedule string, payload []byte) (*domain.FunctionSchedule, error) {
	args := m.Called(ctx, functionID, name, schedule, payload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.FunctionSchedule)
	return r0, args.Error(1)
}

func (m *mockFunctionScheduleService) ListSchedules(ctx context.Context) ([]*domain.FunctionSchedule, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.FunctionSchedule)
	return r0, args.Error(1)
}

func (m *mockFunctionScheduleService) GetSchedule(ctx context.Context, id uuid.UUID) (*domain.FunctionSchedule, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.FunctionSchedule)
	return r0, args.Error(1)
}

func (m *mockFunctionScheduleService) DeleteSchedule(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockFunctionScheduleService) PauseSchedule(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockFunctionScheduleService) ResumeSchedule(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockFunctionScheduleService) GetScheduleRuns(ctx context.Context, id uuid.UUID, limit int) ([]*domain.FunctionScheduleRun, error) {
	args := m.Called(ctx, id, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.FunctionScheduleRun)
	return r0, args.Error(1)
}

const fnSchedPath = "/function-schedules"

func setupFunctionScheduleHandlerTest() (*mockFunctionScheduleService, *FunctionScheduleHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockFunctionScheduleService)
	handler := NewFunctionScheduleHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestFunctionScheduleHandlerCreateSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.POST(fnSchedPath, handler.Create)

	fnID := uuid.New()
	schedID := uuid.New()

	svc.On("CreateSchedule", mock.Anything, fnID, "test-sched", "*/5 * * * *", mock.Anything).Return(&domain.FunctionSchedule{
		ID: schedID, FunctionID: fnID, Name: "test-sched", Schedule: "*/5 * * * *",
	}, nil).Once()

	body := `{"function_id":"` + fnID.String() + `","name":"test-sched","schedule":"*/5 * * * *"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fnSchedPath, bytes.NewBufferString(body))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerCreateInvalidBody(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.POST(fnSchedPath, handler.Create)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fnSchedPath, bytes.NewBufferString(`{"name":""}`))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerCreateServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.POST(fnSchedPath, handler.Create)

	fnID := uuid.New()
	svc.On("CreateSchedule", mock.Anything, fnID, "test-sched", "*/5 * * * *", mock.Anything).Return(nil, assert.AnError).Once()

	body := `{"function_id":"` + fnID.String() + `","name":"test-sched","schedule":"*/5 * * * *"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fnSchedPath, bytes.NewBufferString(body))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerListSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.GET(fnSchedPath, handler.List)

	svc.On("ListSchedules", mock.Anything).Return([]*domain.FunctionSchedule{}, nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fnSchedPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerListServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.GET(fnSchedPath, handler.List)

	svc.On("ListSchedules", mock.Anything).Return(nil, assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fnSchedPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerGetSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.GET(fnSchedPath+"/:id", handler.Get)

	schedID := uuid.New()
	svc.On("GetSchedule", mock.Anything, schedID).Return(&domain.FunctionSchedule{ID: schedID, Name: "test-sched"}, nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fnSchedPath+"/"+schedID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerGetInvalidUUID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.GET(fnSchedPath+"/:id", handler.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fnSchedPath+"/invalid-uuid", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerGetServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.GET(fnSchedPath+"/:id", handler.Get)

	schedID := uuid.New()
	svc.On("GetSchedule", mock.Anything, schedID).Return(nil, assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fnSchedPath+"/"+schedID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerDeleteSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.DELETE(fnSchedPath+"/:id", handler.Delete)

	schedID := uuid.New()
	svc.On("DeleteSchedule", mock.Anything, schedID).Return(nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, fnSchedPath+"/"+schedID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerDeleteInvalidUUID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.DELETE(fnSchedPath+"/:id", handler.Delete)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, fnSchedPath+"/invalid-uuid", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerDeleteServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.DELETE(fnSchedPath+"/:id", handler.Delete)

	schedID := uuid.New()
	svc.On("DeleteSchedule", mock.Anything, schedID).Return(assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, fnSchedPath+"/"+schedID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerPauseSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.POST(fnSchedPath+"/:id/pause", handler.Pause)

	schedID := uuid.New()
	svc.On("PauseSchedule", mock.Anything, schedID).Return(nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fnSchedPath+"/"+schedID.String()+"/pause", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerPauseInvalidUUID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.POST(fnSchedPath+"/:id/pause", handler.Pause)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fnSchedPath+"/invalid-uuid/pause", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerPauseServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.POST(fnSchedPath+"/:id/pause", handler.Pause)

	schedID := uuid.New()
	svc.On("PauseSchedule", mock.Anything, schedID).Return(assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fnSchedPath+"/"+schedID.String()+"/pause", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerResumeSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.POST(fnSchedPath+"/:id/resume", handler.Resume)

	schedID := uuid.New()
	svc.On("ResumeSchedule", mock.Anything, schedID).Return(nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fnSchedPath+"/"+schedID.String()+"/resume", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerResumeInvalidUUID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.POST(fnSchedPath+"/:id/resume", handler.Resume)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fnSchedPath+"/invalid-uuid/resume", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerResumeServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.POST(fnSchedPath+"/:id/resume", handler.Resume)

	schedID := uuid.New()
	svc.On("ResumeSchedule", mock.Anything, schedID).Return(assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fnSchedPath+"/"+schedID.String()+"/resume", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerGetRunsSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.GET(fnSchedPath+"/:id/runs", handler.GetRuns)

	schedID := uuid.New()
	svc.On("GetScheduleRuns", mock.Anything, schedID, 100).Return([]*domain.FunctionScheduleRun{}, nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fnSchedPath+"/"+schedID.String()+"/runs", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerGetRunsInvalidUUID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.GET(fnSchedPath+"/:id/runs", handler.GetRuns)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fnSchedPath+"/invalid-uuid/runs", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestFunctionScheduleHandlerGetRunsServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupFunctionScheduleHandlerTest()
	r.GET(fnSchedPath+"/:id/runs", handler.GetRuns)

	schedID := uuid.New()
	svc.On("GetScheduleRuns", mock.Anything, schedID, 100).Return(nil, assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fnSchedPath+"/"+schedID.String()+"/runs", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}
