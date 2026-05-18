package httphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/ratelimit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type closeNotifierRecorder struct {
	*httptest.ResponseRecorder
}

func (c *closeNotifierRecorder) CloseNotify() <-chan bool {
	return make(chan bool)
}

const (
	routesPath    = "/gateway/routes"
	testRouteName = "route-1"
	gwProxyPath   = "/gw/*proxy"
	gwAPITestPath = "/gw/api"
	gwPathInvalid = "/invalid"
)

type mockGatewayService struct {
	mock.Mock
}

func (m *mockGatewayService) CreateRoute(ctx context.Context, params ports.CreateRouteParams) (*domain.GatewayRoute, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.GatewayRoute)
	return r0, args.Error(1)
}

func (m *mockGatewayService) GetProxy(method, path string) (*httputil.ReverseProxy, *domain.GatewayRoute, map[string]string, bool) {
	args := m.Called(method, path)
	if args.Get(0) == nil {
		return nil, nil, nil, args.Bool(3)
	}
	var route *domain.GatewayRoute
	if r := args.Get(1); r != nil {
		route = r.(*domain.GatewayRoute)
	}
	var params map[string]string
	if p := args.Get(2); p != nil {
		params = p.(map[string]string)
	}
	r0, _ := args.Get(0).(*httputil.ReverseProxy)
	return r0, route, params, args.Bool(3)
}

func (m *mockGatewayService) ListRoutes(ctx context.Context) ([]*domain.GatewayRoute, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.GatewayRoute)
	return r0, args.Error(1)
}

func (m *mockGatewayService) DeleteRoute(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockGatewayService) RefreshRoutes(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func setupGatewayHandlerTest(_ *testing.T) (*mockGatewayService, *GatewayHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockGatewayService)
	rateLimiter := ratelimit.NewIPRateLimiter(100, 200, nil)
	handler := NewGatewayHandler(svc, rateLimiter, nil)
	r := gin.New()
	return svc, handler, r
}

func TestGatewayHandlerCreateRoute(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupGatewayHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(routesPath, handler.CreateRoute)

	route := &domain.GatewayRoute{ID: uuid.New(), Name: testRouteName}
	expectedParams := ports.CreateRouteParams{
		Name:        testRouteName,
		Pattern:     "/api/v1",
		Target:      "http://example.com",
		Methods:     nil,
		StripPrefix: false,
		RateLimit:   100,
		Priority:    0,
	}
	svc.On("CreateRoute", mock.Anything, expectedParams).Return(route, nil)

	body, err := json.Marshal(map[string]interface{}{
		"name":        testRouteName,
		"path_prefix": "/api/v1",
		"target_url":  "http://example.com",
		"rate_limit":  100,
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", routesPath, bytes.NewBuffer(body))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestGatewayHandlerListRoutes(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupGatewayHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(routesPath, handler.ListRoutes)

	routes := []*domain.GatewayRoute{{ID: uuid.New(), Name: testRouteName}}
	svc.On("ListRoutes", mock.Anything).Return(routes, nil)

	req, err := http.NewRequest(http.MethodGet, routesPath, nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGatewayHandlerDeleteRoute(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupGatewayHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(routesPath+"/:id", handler.DeleteRoute)

	id := uuid.New()
	svc.On("DeleteRoute", mock.Anything, id).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, routesPath+"/"+id.String(), nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGatewayHandlerProxyNotFound(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupGatewayHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.Any(gwProxyPath, handler.Proxy)

	svc.On("GetProxy", "GET", "/unknown").Return(nil, nil, nil, false)

	req, err := http.NewRequest(http.MethodGet, "/gw/unknown", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGatewayHandlerProxySuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupGatewayHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.Any(gwProxyPath, handler.Proxy)

	// Mock ReverseProxy
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("proxied"))
	}))
	defer ts.Close()
	targetURL, _ := url.Parse(ts.URL)

	// Use real proxy targeting test server
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	// NewSingleHostReverseProxy doesn't set a Director that strips the prefix /gw by default if we just proxy.
	// But GatewayHandler typically strips prefix before calling proxy or expects proxy to handle it.
	// Gateway Handler implementation: c.Request.URL.Path = c.Param("proxy")? or just calls ServeHTTP.
	// If GatewayHandler calls `proxy.ServeHTTP(w, c.Request)`, the request path "/gw/api" is sent to target.
	// Test server expects any path.
	svc.On("GetProxy", "GET", "/api").Return(proxy, (*domain.GatewayRoute)(nil), map[string]string{}, true)

	req, err := http.NewRequest(http.MethodGet, gwAPITestPath, nil)
	require.NoError(t, err)
	w := &closeNotifierRecorder{httptest.NewRecorder()}
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGatewayHandlerProxyWithoutSlash(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupGatewayHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.Any(gwProxyPath, handler.Proxy)

	// Mock ReverseProxy
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	targetURL, _ := url.Parse(ts.URL)

	svc.On("GetProxy", "GET", "/api").Return(httputil.NewSingleHostReverseProxy(targetURL), (*domain.GatewayRoute)(nil), map[string]string{}, true)

	req, err := http.NewRequest(http.MethodGet, gwAPITestPath, nil)
	require.NoError(t, err)
	w := &closeNotifierRecorder{httptest.NewRecorder()}
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGatewayHandlerProxyWithSlash(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupGatewayHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.Any(gwProxyPath, handler.Proxy)

	// Mock ReverseProxy
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	targetURL, _ := url.Parse(ts.URL)

	svc.On("GetProxy", "GET", "//api").Return(httputil.NewSingleHostReverseProxy(targetURL), (*domain.GatewayRoute)(nil), map[string]string{}, true)

	req, err := http.NewRequest(http.MethodGet, "/gw//api", nil)
	require.NoError(t, err)
	w := &closeNotifierRecorder{httptest.NewRecorder()}
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGatewayHandlerCreateError(t *testing.T) {
	t.Parallel()
	t.Run("InvalidJSON", func(t *testing.T) {
		_, handler, r := setupGatewayHandlerTest(t)
		r.POST(routesPath, handler.CreateRoute)
		req, _ := http.NewRequest("POST", routesPath, bytes.NewBufferString("invalid"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc, handler, r := setupGatewayHandlerTest(t)
		r.POST(routesPath, handler.CreateRoute)
		svc.On("CreateRoute", mock.Anything, mock.Anything).
			Return(nil, errors.New(errors.Internal, "error"))
		body, _ := json.Marshal(map[string]interface{}{"name": "n", "path_prefix": "/p", "target_url": "u"})
		req, _ := http.NewRequest("POST", routesPath, bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestGatewayHandlerListError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupGatewayHandlerTest(t)
	r.GET(routesPath, handler.ListRoutes)
	svc.On("ListRoutes", mock.Anything).Return(nil, errors.New(errors.Internal, "error"))
	req, _ := http.NewRequest(http.MethodGet, routesPath, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestGatewayHandlerProxyBodySizeLimit(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupGatewayHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.Any(gwProxyPath, handler.Proxy)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("proxy should not be called for oversized body")
	}))
	defer ts.Close()
	targetURL, _ := url.Parse(ts.URL)

	svc.On("GetProxy", "GET", "/api").Return(
		httputil.NewSingleHostReverseProxy(targetURL),
		&domain.GatewayRoute{MaxBodySize: 10, AllowedCIDRs: []string{"10.0.0.0/8"}},
		map[string]string{},
		true,
	)

	req, _ := http.NewRequest("GET", gwAPITestPath, nil)
	req.ContentLength = 100 // Exceeds MaxBodySize of 10
	req.RemoteAddr = "10.0.0.1:12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestGatewayHandlerProxyParamWithoutSlash(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockGatewayService)
	handler := NewGatewayHandler(mockSvc, nil, nil)
	gin.SetMode(gin.TestMode)

	// Manually create context to pass parameter without slash
	w := &closeNotifierRecorder{httptest.NewRecorder()}
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "proxy", Value: "api"}}
	c.Request = httptest.NewRequest("GET", gwAPITestPath, nil)

	// Mock ReverseProxy
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	targetURL, _ := url.Parse(ts.URL)

	// Expect GetProxy to be called with "/api" (slash added)
	mockSvc.On("GetProxy", "GET", "/api").Return(httputil.NewSingleHostReverseProxy(targetURL), (*domain.GatewayRoute)(nil), map[string]string{}, true)

	handler.Proxy(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestGatewayHandlerProxyRateLimitExceeded(t *testing.T) {
	// TODO: Test requires mock-friendly rate limiter to be deterministic.
	// The real limiter refills tokens over time (rate=10 → 1 token/100ms),
	// so by the time the proxied request runs, tokens may have refilled.
	// Will be addressed when a mock-able RateLimiter interface is introduced.
}

func TestGatewayHandlerDeleteError(t *testing.T) {
	t.Parallel()
	t.Run("InvalidID", func(t *testing.T) {
		_, handler, r := setupGatewayHandlerTest(t)
		r.DELETE(routesPath+"/:id", handler.DeleteRoute)
		req, _ := http.NewRequest(http.MethodDelete, routesPath+gwPathInvalid, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc, handler, r := setupGatewayHandlerTest(t)
		r.DELETE(routesPath+"/:id", handler.DeleteRoute)
		id := uuid.New()
		svc.On("DeleteRoute", mock.Anything, id).Return(errors.New(errors.Internal, "error"))
		req, _ := http.NewRequest(http.MethodDelete, routesPath+"/"+id.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		svc.AssertExpectations(t)
	})
}
