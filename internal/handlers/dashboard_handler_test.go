package httphandlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type dashboardServiceMock struct {
	mock.Mock
}

func (m *dashboardServiceMock) GetSummary(ctx context.Context) (*domain.ResourceSummary, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ResourceSummary), args.Error(1)
}

func (m *dashboardServiceMock) GetRecentEvents(ctx context.Context, limit int) ([]*domain.Event, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Event), args.Error(1)
}

func (m *dashboardServiceMock) GetStats(ctx context.Context) (*domain.DashboardStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DashboardStats), args.Error(1)
}

func setupDashboardHandlerTest(t *testing.T) (*dashboardServiceMock, *DashboardHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(dashboardServiceMock)
	handler := NewDashboardHandler(mockSvc)
	r := gin.New()
	return mockSvc, handler, r
}

func TestDashboardHandler_GetSummary(t *testing.T) {
	mockSvc, handler, r := setupDashboardHandlerTest(t)
	defer mockSvc.AssertExpectations(t)

	r.GET("/summary", handler.GetSummary)

	summary := &domain.ResourceSummary{TotalInstances: 5}
	mockSvc.On("GetSummary", mock.Anything).Return(summary, nil)

	req, err := http.NewRequest("GET", "/summary", nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var wrapper struct {
		Data domain.ResourceSummary `json:"data"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &wrapper))
	assert.Equal(t, 5, wrapper.Data.TotalInstances)
}

func TestDashboardHandler_GetRecentEvents(t *testing.T) {
	mockSvc, handler, r := setupDashboardHandlerTest(t)
	defer mockSvc.AssertExpectations(t)

	r.GET("/events", handler.GetRecentEvents)

	events := []*domain.Event{{ID: uuid.New(), Action: "TEST"}}
	mockSvc.On("GetRecentEvents", mock.Anything, 10).Return(events, nil)

	req, err := http.NewRequest("GET", "/events?limit=10", nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var wrapper struct {
		Data []*domain.Event `json:"data"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &wrapper))
	assert.Len(t, wrapper.Data, 1)
}

func TestDashboardHandler_GetStats(t *testing.T) {
	mockSvc, handler, r := setupDashboardHandlerTest(t)
	defer mockSvc.AssertExpectations(t)

	r.GET("/stats", handler.GetStats)

	stats := &domain.DashboardStats{
		CPUHistory: []domain.MetricPoint{{Value: 10.1}},
	}
	mockSvc.On("GetStats", mock.Anything).Return(stats, nil)

	req, err := http.NewRequest("GET", "/stats", nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var wrapper struct {
		Data domain.DashboardStats `json:"data"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &wrapper))
	assert.Len(t, wrapper.Data.CPUHistory, 1)
	assert.Equal(t, 10.1, wrapper.Data.CPUHistory[0].Value)
}

func TestDashboardHandler_StreamEvents(t *testing.T) {
	mockSvc, handler, r := setupDashboardHandlerTest(t)
	defer mockSvc.AssertExpectations(t)

	r.GET("/stream", handler.StreamEvents)

	summary := &domain.ResourceSummary{TotalInstances: 10}
	mockSvc.On("GetSummary", mock.Anything).Return(summary, nil)

	req, err := http.NewRequest("GET", "/stream", nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)

	go r.ServeHTTP(w, req)

	time.Sleep(100 * time.Millisecond)
	cancel()

	assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
	assert.Contains(t, w.Body.String(), "event:summary")
}
