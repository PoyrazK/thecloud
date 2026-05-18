package httphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	cronPath        = "/cron"
	testJobName     = "job-1"
	testExampleURL  = "http://example.com"
	pausePath       = "/:id/pause"
	resumePath      = "/:id/resume"
	pauseSuffix     = "/pause"
	resumeSuffix    = "/resume"
	cronPathInvalid = "/invalid"
)

type mockCronService struct {
	mock.Mock
}

func (m *mockCronService) CreateJob(ctx context.Context, name, schedule, targetURL, method, payload string) (*domain.CronJob, error) {
	args := m.Called(ctx, name, schedule, targetURL, method, payload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.CronJob)
	return r0, args.Error(1)
}

func (m *mockCronService) ListJobs(ctx context.Context) ([]*domain.CronJob, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.CronJob)
	return r0, args.Error(1)
}

func (m *mockCronService) GetJob(ctx context.Context, id uuid.UUID) (*domain.CronJob, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.CronJob)
	return r0, args.Error(1)
}

func (m *mockCronService) PauseJob(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockCronService) ResumeJob(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *mockCronService) DeleteJob(ctx context.Context, id uuid.UUID) error {
	res := m.Called(ctx, id)
	return res.Error(0)
}

func (m *mockCronService) GetJobRuns(ctx context.Context, id uuid.UUID, limit int) ([]*domain.CronJobRun, error) {
	args := m.Called(ctx, id, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.CronJobRun)
	return r0, args.Error(1)
}

func (m *mockCronService) UpdateJob(ctx context.Context, id uuid.UUID, name, schedule, targetURL, targetMethod, targetPayload string) (*domain.CronJob, error) {
	args := m.Called(ctx, id, name, schedule, targetURL, targetMethod, targetPayload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.CronJob)
	return r0, args.Error(1)
}

func setupCronHandlerTest(_ *testing.T) (*mockCronService, *CronHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockCronService)
	handler := NewCronHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestCronHandlerCreateJob(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupCronHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(cronPath, handler.CreateJob)

	job := &domain.CronJob{ID: uuid.New(), Name: testJobName}
	svc.On("CreateJob", mock.Anything, testJobName, "* * * * *", testExampleURL, "POST", "payload").Return(job, nil)

	body, err := json.Marshal(map[string]interface{}{
		"name":           testJobName,
		"schedule":       "* * * * *",
		"target_url":     testExampleURL,
		"target_method":  "POST",
		"target_payload": "payload",
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", cronPath, bytes.NewBuffer(body))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCronHandlerListJobs(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupCronHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(cronPath, handler.ListJobs)

	jobs := []*domain.CronJob{{ID: uuid.New(), Name: testJobName}}
	svc.On("ListJobs", mock.Anything).Return(jobs, nil)

	req, err := http.NewRequest(http.MethodGet, cronPath, nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCronHandlerGetJob(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupCronHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(cronPath+"/:id", handler.GetJob)

	id := uuid.New()
	job := &domain.CronJob{ID: id, Name: testJobName}
	svc.On("GetJob", mock.Anything, id).Return(job, nil)

	req, err := http.NewRequest(http.MethodGet, cronPath+"/"+id.String(), nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCronHandlerPauseJob(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupCronHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(cronPath+pausePath, handler.PauseJob)

	id := uuid.New()
	svc.On("PauseJob", mock.Anything, id).Return(nil)

	req, err := http.NewRequest(http.MethodPost, cronPath+"/"+id.String()+"/pause", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCronHandlerResumeJob(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupCronHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(cronPath+resumePath, handler.ResumeJob)

	id := uuid.New()
	svc.On("ResumeJob", mock.Anything, id).Return(nil)

	req, err := http.NewRequest(http.MethodPost, cronPath+"/"+id.String()+"/resume", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCronHandlerDeleteJob(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupCronHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(cronPath+"/:id", handler.DeleteJob)

	id := uuid.New()
	svc.On("DeleteJob", mock.Anything, id).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, cronPath+"/"+id.String(), nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCronHandlerCreateError(t *testing.T) {
	t.Parallel()
	t.Run("InvalidJSON", func(t *testing.T) {
		_, handler, r := setupCronHandlerTest(t)
		r.POST(cronPath, handler.CreateJob)
		req, _ := http.NewRequest("POST", cronPath, bytes.NewBufferString("invalid"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc, handler, r := setupCronHandlerTest(t)
		r.POST(cronPath, handler.CreateJob)
		svc.On("CreateJob", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, errors.New(errors.Internal, "error"))
		body, _ := json.Marshal(map[string]interface{}{"name": "n", "schedule": "s", "target_url": "u"})
		req, _ := http.NewRequest("POST", cronPath, bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestCronHandlerListError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupCronHandlerTest(t)
	r.GET(cronPath, handler.ListJobs)
	svc.On("ListJobs", mock.Anything).Return(nil, errors.New(errors.Internal, "error"))
	req, _ := http.NewRequest(http.MethodGet, cronPath, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestCronHandlerGetError(t *testing.T) {
	t.Parallel()
	t.Run("InvalidID", func(t *testing.T) {
		_, handler, r := setupCronHandlerTest(t)
		r.GET(cronPath+"/:id", handler.GetJob)
		req, _ := http.NewRequest(http.MethodGet, cronPath+cronPathInvalid, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc, handler, r := setupCronHandlerTest(t)
		r.GET(cronPath+"/:id", handler.GetJob)
		id := uuid.New()
		svc.On("GetJob", mock.Anything, id).Return(nil, errors.New(errors.NotFound, errNotFound))
		req, _ := http.NewRequest(http.MethodGet, cronPath+"/"+id.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestCronHandlerPauseError(t *testing.T) {
	t.Parallel()
	t.Run("InvalidID", func(t *testing.T) {
		_, handler, r := setupCronHandlerTest(t)
		r.POST(cronPath+pausePath, handler.PauseJob)
		req, _ := http.NewRequest(http.MethodPost, cronPath+cronPathInvalid+pauseSuffix, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc, handler, r := setupCronHandlerTest(t)
		r.POST(cronPath+pausePath, handler.PauseJob)
		id := uuid.New()
		svc.On("PauseJob", mock.Anything, id).Return(errors.New(errors.Internal, "error"))
		req, _ := http.NewRequest(http.MethodPost, cronPath+"/"+id.String()+pauseSuffix, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestCronHandlerResumeError(t *testing.T) {
	t.Parallel()
	t.Run("InvalidID", func(t *testing.T) {
		_, handler, r := setupCronHandlerTest(t)
		r.POST(cronPath+resumePath, handler.ResumeJob)
		req, _ := http.NewRequest(http.MethodPost, cronPath+cronPathInvalid+resumeSuffix, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc, handler, r := setupCronHandlerTest(t)
		r.POST(cronPath+resumePath, handler.ResumeJob)
		id := uuid.New()
		svc.On("ResumeJob", mock.Anything, id).Return(errors.New(errors.Internal, "error"))
		req, _ := http.NewRequest(http.MethodPost, cronPath+"/"+id.String()+resumeSuffix, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestCronHandlerDeleteError(t *testing.T) {
	t.Parallel()
	t.Run("InvalidID", func(t *testing.T) {
		_, handler, r := setupCronHandlerTest(t)
		r.DELETE(cronPath+"/:id", handler.DeleteJob)
		req, _ := http.NewRequest(http.MethodDelete, cronPath+cronPathInvalid, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc, handler, r := setupCronHandlerTest(t)
		r.DELETE(cronPath+"/:id", handler.DeleteJob)
		id := uuid.New()
		svc.On("DeleteJob", mock.Anything, id).Return(errors.New(errors.Internal, "error"))
		req, _ := http.NewRequest(http.MethodDelete, cronPath+"/"+id.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestCronHandlerGetJobRuns(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupCronHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(cronPath+"/:id/runs", handler.GetJobRuns)

	id := uuid.New()
	runs := []*domain.CronJobRun{
		{ID: uuid.New(), JobID: id, Status: "SUCCESS", StatusCode: 200, DurationMs: 150, StartedAt: time.Now()},
	}
	svc.On("GetJobRuns", mock.Anything, id, 50).Return(runs, nil)

	req, err := http.NewRequest(http.MethodGet, cronPath+"/"+id.String()+"/runs?limit=50", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCronHandlerGetJobRunsError(t *testing.T) {
	t.Parallel()
	t.Run("InvalidID", func(t *testing.T) {
		_, handler, r := setupCronHandlerTest(t)
		r.GET(cronPath+"/:id/runs", handler.GetJobRuns)
		req, _ := http.NewRequest(http.MethodGet, cronPath+cronPathInvalid+"/runs", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc, handler, r := setupCronHandlerTest(t)
		r.GET(cronPath+"/:id/runs", handler.GetJobRuns)
		id := uuid.New()
		svc.On("GetJobRuns", mock.Anything, id, 50).Return(nil, errors.New(errors.Internal, "error"))
		req, _ := http.NewRequest(http.MethodGet, cronPath+"/"+id.String()+"/runs", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestCronHandlerUpdateJob(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupCronHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.PUT(cronPath+"/:id", handler.UpdateJob)

	id := uuid.New()
	job := &domain.CronJob{ID: id, Name: "updated-job", Schedule: "0 0 * * *", TargetURL: "http://updated.example.com"}
	svc.On("UpdateJob", mock.Anything, id, "updated-job", "0 0 * * *", "http://updated.example.com", "GET", "").Return(job, nil)

	body, err := json.Marshal(map[string]interface{}{
		"name":          "updated-job",
		"schedule":      "0 0 * * *",
		"target_url":    "http://updated.example.com",
		"target_method": "GET",
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPut, cronPath+"/"+id.String(), bytes.NewBuffer(body))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCronHandlerUpdateJobError(t *testing.T) {
	t.Parallel()
	t.Run("InvalidJSON", func(t *testing.T) {
		_, handler, r := setupCronHandlerTest(t)
		r.PUT(cronPath+"/:id", handler.UpdateJob)
		req, _ := http.NewRequest(http.MethodPut, cronPath+"/"+uuid.New().String(), bytes.NewBufferString("invalid"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("InvalidID", func(t *testing.T) {
		_, handler, r := setupCronHandlerTest(t)
		r.PUT(cronPath+"/:id", handler.UpdateJob)
		req, _ := http.NewRequest(http.MethodPut, cronPath+cronPathInvalid, bytes.NewBufferString("{}"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc, handler, r := setupCronHandlerTest(t)
		r.PUT(cronPath+"/:id", handler.UpdateJob)
		id := uuid.New()
		svc.On("UpdateJob", mock.Anything, id, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, errors.New(errors.Internal, "error"))
		body, _ := json.Marshal(map[string]interface{}{
			"name":       "updated-job",
			"schedule":   "0 0 * * *",
			"target_url": "http://updated.example.com",
		})
		req, _ := http.NewRequest(http.MethodPut, cronPath+"/"+id.String(), bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		svc.AssertExpectations(t)
	})
}
