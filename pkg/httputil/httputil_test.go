package httputil

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockIdentityService struct {
	mock.Mock
}

func (m *mockIdentityService) CreateKey(ctx context.Context, userID uuid.UUID, name string) (*domain.APIKey, error) {
	args := m.Called(ctx, userID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *mockIdentityService) ValidateAPIKey(ctx context.Context, key string) (*domain.APIKey, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func TestAuth_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockIdentityService)
	userID := uuid.New()
	svc.On("ValidateAPIKey", mock.Anything, "valid-key").Return(&domain.APIKey{UserID: userID}, nil)

	r := gin.New()
	r.Use(Auth(svc))
	r.GET("/protected", func(c *gin.Context) {
		val, _ := c.Get("userID")
		assert.Equal(t, userID, val)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("X-API-Key", "valid-key")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_MissingKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockIdentityService)

	r := gin.New()
	r.Use(Auth(svc))
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_InvalidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockIdentityService)
	svc.On("ValidateAPIKey", mock.Anything, "invalid-key").Return(nil, fmt.Errorf("invalid"))

	r := gin.New()
	r.Use(Auth(svc))
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("X-API-Key", "invalid-key")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestResponse_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("requestID", "abc")

	data := map[string]string{"foo": "bar"}
	Success(c, http.StatusOK, data)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "foo")
	assert.Contains(t, w.Body.String(), "abc")
}

func TestResponse_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		err        error
		expectCode int
	}{
		{"NotFound", errors.New(errors.NotFound, "not found"), http.StatusNotFound},
		{"InvalidInput", errors.New(errors.InvalidInput, "invalid"), http.StatusBadRequest},
		{"Unauthorized", errors.New(errors.Unauthorized, "unauthorized"), http.StatusUnauthorized},
		{"UnknownError", fmt.Errorf("weird error"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			Error(c, tt.err)
			assert.Equal(t, tt.expectCode, w.Code)
		})
	}
}

func TestRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/id", func(c *gin.Context) {
		id, _ := c.Get("requestID")
		assert.NotEmpty(t, id)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/id", nil)
	r.ServeHTTP(w, req)
	assert.NotEmpty(t, w.Header().Get(HeaderXRequestID))
}

func TestLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	r := gin.New()
	r.Use(Logger(logger))
	r.GET("/log", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/log", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS())
	r.GET("/cors", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	t.Run("GET request", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/cors", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("OPTIONS request", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("OPTIONS", "/cors", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeadersMiddleware())
	r.GET("/secure", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/secure", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
}
