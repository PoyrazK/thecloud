package ratelimit

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func TestNewIPRateLimiter(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	limiter := NewIPRateLimiter(rate.Limit(1), 1, logger)
	assert.NotNil(t, limiter)
	assert.Equal(t, rate.Limit(1), limiter.rate)
	assert.Equal(t, 1, limiter.burst)
}

func TestIPRateLimiter_GetLimiter(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	limiter := NewIPRateLimiter(rate.Limit(10), 5, logger)

	l1 := limiter.GetLimiter("1.1.1.1")
	l2 := limiter.GetLimiter("1.1.1.1")
	l3 := limiter.GetLimiter("2.2.2.2")

	assert.Equal(t, l1, l2)
	assert.True(t, l1 != l3, "Each key should have its own limiter instance")
}

func TestMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Create a limiter that allows only 1 request per second with a burst of 1
	limiter := NewIPRateLimiter(rate.Limit(1), 1, logger)

	r := gin.New()
	r.Use(Middleware(limiter))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// First request - should be OK
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Second request immediately after - should be rate limited
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
}

func TestMiddleware_WithAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	limiter := NewIPRateLimiter(rate.Limit(10), 10, logger)

	r := gin.New()
	r.Use(Middleware(limiter))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "sk_test_123456789")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
