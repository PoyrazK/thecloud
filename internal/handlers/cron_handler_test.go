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
	cronPath    = "/cron"
	testJobName = "job-1"
)

type mockCronService struct {
	mock.Mock
}

func (m *mockCronService) CreateJob(ctx context.Context, name, schedule, targetURL, method, payload string) (*domain.CronJob, error) {
	args := m.Called(ctx, name, schedule, targetURL, method, payload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CronJob), args.Error(1)
}

func (m *mockCronService) ListJobs(ctx context.Context) ([]*domain.CronJob, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CronJob), args.Error(1)
}

func (m *mockCronService) GetJob(ctx context.Context, id uuid.UUID) (*domain.CronJob, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CronJob), args.Error(1)
}

func (m *mockCronService) PauseJob(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockCronService) ResumeJob(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockCronService) DeleteJob(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func setupCronHandlerTest(t *testing.T) (*mockCronService, *CronHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockCronService)
	handler := NewCronHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestCronHandlerCreateJob(t *testing.T) {
	svc, handler, r := setupCronHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(cronPath, handler.CreateJob)

	job := &domain.CronJob{ID: uuid.New(), Name: testJobName}
	svc.On("CreateJob", mock.Anything, testJobName, "* * * * *", "http://example.com", "POST", "payload").Return(job, nil)

	body, err := json.Marshal(map[string]interface{}{
		"name":           testJobName,
		"schedule":       "* * * * *",
		"target_url":     "http://example.com",
		"target_method":  "POST",
		"target_payload": "payload",
	})
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", cronPath, bytes.NewBuffer(body))
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCronHandlerListJobs(t *testing.T) {
	svc, handler, r := setupCronHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(cronPath, handler.ListJobs)

	jobs := []*domain.CronJob{{ID: uuid.New(), Name: testJobName}}
	svc.On("ListJobs", mock.Anything).Return(jobs, nil)

	req, err := http.NewRequest(http.MethodGet, cronPath, nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCronHandlerGetJob(t *testing.T) {
	svc, handler, r := setupCronHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(cronPath+"/:id", handler.GetJob)

	id := uuid.New()
	job := &domain.CronJob{ID: id, Name: testJobName}
	svc.On("GetJob", mock.Anything, id).Return(job, nil)

	req, err := http.NewRequest(http.MethodGet, cronPath+"/"+id.String(), nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCronHandlerPauseJob(t *testing.T) {
	svc, handler, r := setupCronHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(cronPath+"/:id/pause", handler.PauseJob)

	id := uuid.New()
	svc.On("PauseJob", mock.Anything, id).Return(nil)

	req, err := http.NewRequest(http.MethodPost, cronPath+"/"+id.String()+"/pause", nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCronHandlerResumeJob(t *testing.T) {
	svc, handler, r := setupCronHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(cronPath+"/:id/resume", handler.ResumeJob)

	id := uuid.New()
	svc.On("ResumeJob", mock.Anything, id).Return(nil)

	req, err := http.NewRequest(http.MethodPost, cronPath+"/"+id.String()+"/resume", nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCronHandlerDeleteJob(t *testing.T) {
	svc, handler, r := setupCronHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(cronPath+"/:id", handler.DeleteJob)

	id := uuid.New()
	svc.On("DeleteJob", mock.Anything, id).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, cronPath+"/"+id.String(), nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
