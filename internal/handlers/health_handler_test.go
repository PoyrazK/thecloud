package httphandlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockHealthService struct {
	mock.Mock
}

func (m *mockHealthService) Check(ctx context.Context) ports.HealthCheckResult {
	args := m.Called(ctx)
	return args.Get(0).(ports.HealthCheckResult)
}

func TestHealthHandler_Live(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHealthHandler(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/health/live", nil)

	h.Live(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}

func TestHealthHandler_Ready(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Healthy", func(t *testing.T) {
		svc := new(mockHealthService)
		h := NewHealthHandler(svc)

		res := ports.HealthCheckResult{
			Status: "HEALTHY",
			Checks: map[string]string{"db": "ok"},
			Time:   time.Now(),
		}
		svc.On("Check", mock.Anything).Return(res)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/health/ready", nil)

		h.Ready(c)

		assert.Equal(t, http.StatusOK, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("Degraded", func(t *testing.T) {
		svc := new(mockHealthService)
		h := NewHealthHandler(svc)

		res := ports.HealthCheckResult{
			Status: "DEGRADED",
			Checks: map[string]string{"db": "slow"},
			Time:   time.Now(),
		}
		svc.On("Check", mock.Anything).Return(res)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/health/ready", nil)

		h.Ready(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		svc.AssertExpectations(t)
	})
}
