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

func TestCronHandler_CreateJob(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockCronService)
	handler := NewCronHandler(svc)

	r := gin.New()
	r.POST("/cron", handler.CreateJob)

	job := &domain.CronJob{ID: uuid.New(), Name: "job-1"}
	svc.On("CreateJob", mock.Anything, "job-1", "* * * * *", "http://example.com", "POST", "payload").Return(job, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"name":           "job-1",
		"schedule":       "* * * * *",
		"target_url":     "http://example.com",
		"target_method":  "POST",
		"target_payload": "payload",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/cron", bytes.NewBuffer(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCronHandler_ListJobs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockCronService)
	handler := NewCronHandler(svc)

	r := gin.New()
	r.GET("/cron", handler.ListJobs)

	jobs := []*domain.CronJob{{ID: uuid.New(), Name: "job-1"}}
	svc.On("ListJobs", mock.Anything).Return(jobs, nil)

	req := httptest.NewRequest(http.MethodGet, "/cron", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCronHandler_GetJob(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockCronService)
	handler := NewCronHandler(svc)

	r := gin.New()
	r.GET("/cron/:id", handler.GetJob)

	id := uuid.New()
	job := &domain.CronJob{ID: id, Name: "job-1"}
	svc.On("GetJob", mock.Anything, id).Return(job, nil)

	req := httptest.NewRequest(http.MethodGet, "/cron/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCronHandler_PauseJob(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockCronService)
	handler := NewCronHandler(svc)

	r := gin.New()
	r.POST("/cron/:id/pause", handler.PauseJob)

	id := uuid.New()
	svc.On("PauseJob", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/cron/"+id.String()+"/pause", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCronHandler_ResumeJob(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockCronService)
	handler := NewCronHandler(svc)

	r := gin.New()
	r.POST("/cron/:id/resume", handler.ResumeJob)

	id := uuid.New()
	svc.On("ResumeJob", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/cron/"+id.String()+"/resume", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCronHandler_DeleteJob(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockCronService)
	handler := NewCronHandler(svc)

	r := gin.New()
	r.DELETE("/cron/:id", handler.DeleteJob)

	id := uuid.New()
	svc.On("DeleteJob", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/cron/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
