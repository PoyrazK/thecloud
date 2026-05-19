package httphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
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

func (m *mockGatewayService) ValidateJWT(ctx context.Context, route *domain.GatewayRoute, tokenString string) (map[string]string, error) {
	args := m.Called(ctx, route, tokenString)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(map[string]string)
	return r0, args.Error(1)
}

func setupGatewayHandlerTest(_ *testing.T) (*mockGatewayService, *GatewayHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockGatewayService)
	handler := NewGatewayHandler(svc, nil, nil)
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

func TestGatewayHandlerProxyJWTEmptyToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(mockGatewayService)
	handler := NewGatewayHandler(mockSvc, nil, nil)
	r := gin.New()
	r.Any(gwProxyPath, handler.Proxy)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, wreq *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	targetURL, _ := url.Parse(ts.URL)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	route := &domain.GatewayRoute{
		ID:           uuid.New(),
		Name:         "jwt-test",
		JWTJwksURL:   "https://auth.example.com/.well-known/jwks.json",
		AllowedIPNets: []*net.IPNet{},
	}
	mockSvc.On("GetProxy", "GET", "/api").Return(proxy, route, map[string]string{}, true).Once()

	// Request without Authorization header
	req, err := http.NewRequest("GET", gwAPITestPath, nil)
	require.NoError(t, err)
	req.RemoteAddr = "10.0.0.1:12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestGatewayHandlerProxyJWTMissingBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(mockGatewayService)
	handler := NewGatewayHandler(mockSvc, nil, nil)
	r := gin.New()
	r.Any(gwProxyPath, handler.Proxy)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, wreq *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	targetURL, _ := url.Parse(ts.URL)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	route := &domain.GatewayRoute{
		ID:           uuid.New(),
		Name:         "jwt-test",
		JWTJwksURL:   "https://auth.example.com/.well-known/jwks.json",
		AllowedIPNets: []*net.IPNet{},
	}
	mockSvc.On("GetProxy", "GET", "/api").Return(proxy, route, map[string]string{}, true).Once()

	// Request with Authorization header but no Bearer prefix
	req, err := http.NewRequest("GET", gwAPITestPath, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Basic dXNlcm5hbWU6cGFzc3dvcmQ=")
	req.RemoteAddr = "10.0.0.1:12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestGatewayHandlerProxyJWTValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(mockGatewayService)
	handler := NewGatewayHandler(mockSvc, nil, nil)
	r := gin.New()
	r.Any(gwProxyPath, handler.Proxy)

	// Use a capturing proxy that records upstream headers
	var upstreamReqHeaders http.Header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, wreq *http.Request) {
		upstreamReqHeaders = wreq.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	targetURL, _ := url.Parse(ts.URL)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	route := &domain.GatewayRoute{
		ID:            uuid.New(),
		Name:          "jwt-test",
		JWTJwksURL:    "https://auth.example.com/.well-known/jwks.json",
		AllowedIPNets: []*net.IPNet{},
	}
	mockSvc.On("GetProxy", "GET", "/api").Return(proxy, route, map[string]string{}, true).Once()
	mockSvc.On("ValidateJWT", mock.Anything, route, "valid-token").Return(
		map[string]string{"sub": "user123", "iss": "test-issuer"}, nil).Once()

	req, err := http.NewRequest("GET", gwAPITestPath, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer valid-token")
	req.RemoteAddr = "10.0.0.1:12345"
	w := &closeNotifierRecorder{httptest.NewRecorder()}
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user123", upstreamReqHeaders.Get("X-JWT-Claim-sub"))
	assert.Equal(t, "test-issuer", upstreamReqHeaders.Get("X-JWT-Claim-iss"))
	mockSvc.AssertExpectations(t)
}

func TestGatewayHandlerProxyJWTInvalidTokenServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(mockGatewayService)
	handler := NewGatewayHandler(mockSvc, nil, nil)
	r := gin.New()
	r.Any(gwProxyPath, handler.Proxy)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, wreq *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	targetURL, _ := url.Parse(ts.URL)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	route := &domain.GatewayRoute{
		ID:            uuid.New(),
		Name:          "jwt-test",
		JWTJwksURL:    "https://auth.example.com/.well-known/jwks.json",
		AllowedIPNets: []*net.IPNet{},
	}
	mockSvc.On("GetProxy", "GET", "/api").Return(proxy, route, map[string]string{}, true).Once()
	mockSvc.On("ValidateJWT", mock.Anything, route, "malformed-token").Return(
		nil, fmt.Errorf("invalid token")).Once()

	req, err := http.NewRequest("GET", gwAPITestPath, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer malformed-token")
	req.RemoteAddr = "10.0.0.1:12345"
	w := &closeNotifierRecorder{httptest.NewRecorder()}
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestGatewayHandlerProxyJWTClaimsPropagation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(mockGatewayService)
	handler := NewGatewayHandler(mockSvc, nil, nil)
	r := gin.New()
	r.Any(gwProxyPath, handler.Proxy)

	var upstreamReqHeaders http.Header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, wreq *http.Request) {
		upstreamReqHeaders = wreq.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	targetURL, _ := url.Parse(ts.URL)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	route := &domain.GatewayRoute{
		ID:            uuid.New(),
		Name:          "jwt-test",
		JWTJwksURL:    "https://auth.example.com/.well-known/jwks.json",
		AllowedIPNets: []*net.IPNet{},
	}
	mockSvc.On("GetProxy", "GET", "/api").Return(proxy, route, map[string]string{}, true).Once()
	mockSvc.On("ValidateJWT", mock.Anything, route, "multi-claim-token").Return(
		map[string]string{
			"sub":   "user1",
			"role":  "admin",
			"email": "u@example.com",
		}, nil).Once()

	req, err := http.NewRequest("GET", gwAPITestPath, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer multi-claim-token")
	req.RemoteAddr = "10.0.0.1:12345"
	w := &closeNotifierRecorder{httptest.NewRecorder()}
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user1", upstreamReqHeaders.Get("X-JWT-Claim-sub"))
	assert.Equal(t, "admin", upstreamReqHeaders.Get("X-JWT-Claim-role"))
	assert.Equal(t, "u@example.com", upstreamReqHeaders.Get("X-JWT-Claim-email"))
	mockSvc.AssertExpectations(t)
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

func TestGatewayHandlerInjectCORSHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockGatewayService)
	handler := NewGatewayHandler(svc, nil, nil)
	r := gin.New()
	r.Any(gwProxyPath, handler.Proxy)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, wreq *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	targetURL, _ := url.Parse(ts.URL)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	tests := []struct {
		name               string
		origin             string
		allowedOrigins     []string
		allowedMethods     []string
		allowedHeaders     []string
		exposeHeaders      []string
		maxAge             int
		expectCORS         bool
		expectMethods      bool
		expectAllowMethods string
	}{
		{
			name:           "wildcard origin - allowed",
			origin:         "http://example.com",
			allowedOrigins: []string{"*"},
			expectCORS:     true,
		},
		{
			name:           "exact origin match",
			origin:         "http://example.com",
			allowedOrigins: []string{"http://example.com", "http://test.com"},
			expectCORS:     true,
		},
		{
			name:           "origin not in allowlist",
			origin:         "http://evil.com",
			allowedOrigins: []string{"http://example.com"},
			expectCORS:     false,
		},
		{
			name:              "with methods and headers",
			origin:            "http://example.com",
			allowedOrigins:    []string{"http://example.com"},
			allowedMethods:     []string{"GET", "POST"},
			allowedHeaders:     []string{"Authorization", "Content-Type"},
			exposeHeaders:      []string{"X-Custom-Header"},
			maxAge:             3600,
			expectCORS:         true,
			expectMethods:      true,
			expectAllowMethods: "GET, POST",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			route := &domain.GatewayRoute{
				ID:              uuid.New(),
				Name:            "cors-test",
				Compression:     "",
				AllowedOrigins:  tc.allowedOrigins,
				AllowedMethods:  tc.allowedMethods,
				AllowedHeaders:  tc.allowedHeaders,
				ExposeHeaders:   tc.exposeHeaders,
				MaxAge:          tc.maxAge,
				AllowedIPNets:   []*net.IPNet{},
			}
			svc.On("GetProxy", "GET", "/api").Return(proxy, route, map[string]string{}, true).Once()

			req, err := http.NewRequest("GET", gwAPITestPath, nil)
			require.NoError(t, err)
			req.Header.Set("Origin", tc.origin)
			req.RemoteAddr = "10.0.0.1:12345"
			w := &closeNotifierRecorder{httptest.NewRecorder()}
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			if tc.expectCORS {
				assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Origin"))
				if tc.expectMethods {
					assert.Equal(t, tc.expectAllowMethods, w.Header().Get("Access-Control-Allow-Methods"))
					assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
					assert.Equal(t, "X-Custom-Header", w.Header().Get("Access-Control-Expose-Headers"))
					assert.Equal(t, "3600", w.Header().Get("Access-Control-Max-Age"))
				}
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}
			svc.AssertExpectations(t)
		})
	}
}

func TestGatewayHandlerInjectCORSHeadersPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockGatewayService)
	handler := NewGatewayHandler(svc, nil, nil)
	r := gin.New()
	r.Any(gwProxyPath, handler.Proxy)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, wreq *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	targetURL, _ := url.Parse(ts.URL)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	route := &domain.GatewayRoute{
		ID:              uuid.New(),
		Name:            "cors-preflight",
		AllowedOrigins:  []string{"http://example.com"},
		AllowedMethods:  []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:  []string{"Authorization", "Content-Type"},
		ExposeHeaders:   []string{"X-Custom-Header"},
		MaxAge:          3600,
		AllowedIPNets:   []*net.IPNet{},
	}
	svc.On("GetProxy", "OPTIONS", "/api").Return(proxy, route, map[string]string{}, true).Once()

	req, err := http.NewRequest("OPTIONS", gwAPITestPath, nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
	req.RemoteAddr = "10.0.0.1:12345"
	w := &closeNotifierRecorder{httptest.NewRecorder()}
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "http://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "GET, POST, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
	assert.Equal(t, "X-Custom-Header", w.Header().Get("Access-Control-Expose-Headers"))
	assert.Equal(t, "3600", w.Header().Get("Access-Control-Max-Age"))
	svc.AssertExpectations(t)
}

func TestGatewayHandlerProxyCompression(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockGatewayService)
	handler := NewGatewayHandler(svc, nil, nil)
	r := gin.New()
	r.Any(gwProxyPath, handler.Proxy)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, wreq *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("proxied response"))
	}))
	defer ts.Close()
	targetURL, _ := url.Parse(ts.URL)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	tests := []struct {
		name           string
		compression    string
		acceptEncoding string
		expectEncoding string
	}{
		{
			name:           "gzip compression enabled",
			compression:    "gzip",
			acceptEncoding: "gzip",
			expectEncoding: "gzip",
		},
		{
			name:           "no compression - route disabled",
			compression:    "",
			acceptEncoding: "gzip",
			expectEncoding: "",
		},
		{
			name:           "client does not accept gzip",
			compression:    "gzip",
			acceptEncoding: "",
			expectEncoding: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			route := &domain.GatewayRoute{
				ID:            uuid.New(),
				Name:          "compression-test",
				Compression:   tc.compression,
				AllowedIPNets: []*net.IPNet{},
			}
			svc.On("GetProxy", "GET", "/api").Return(proxy, route, map[string]string{}, true).Once()

			req, err := http.NewRequest("GET", gwAPITestPath, nil)
			require.NoError(t, err)
			req.Header.Set("Accept-Encoding", tc.acceptEncoding)
			req.RemoteAddr = "10.0.0.1:12345"
			w := &closeNotifierRecorder{httptest.NewRecorder()}
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tc.expectEncoding, w.Header().Get("Content-Encoding"))
			svc.AssertExpectations(t)
		})
	}
}

func TestGatewayHandlerInjectTraceHeadersWithInbound(t *testing.T) {
	t.Parallel()
	svc := new(mockGatewayService)
	handler := NewGatewayHandler(svc, nil, nil)
	r := gin.New()
	r.Any(gwProxyPath, handler.Proxy)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, wreq *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	targetURL, _ := url.Parse(ts.URL)

	route := &domain.GatewayRoute{ID: uuid.New(), AllowedIPNets: []*net.IPNet{}}
	svc.On("GetProxy", "GET", "/api").Return(
		httputil.NewSingleHostReverseProxy(targetURL), route, map[string]string{}, true).Once()

	req, _ := http.NewRequest("GET", gwAPITestPath, nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	req.Header.Set("tracestate", "congo=t61rcWkgMzE")
	req.RemoteAddr = "10.0.0.1:12345"
	w := &closeNotifierRecorder{httptest.NewRecorder()}
	r.ServeHTTP(w, req)

	// Inbound traceparent should be preserved (not replaced with generated)
	assert.Equal(t, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", w.Header().Get("traceparent"))
	assert.Equal(t, "congo=t61rcWkgMzE", w.Header().Get("tracestate"))
	svc.AssertExpectations(t)
}

func TestGatewayHandlerProxyCompressionGzipFlushed(t *testing.T) {
	t.Parallel()
	svc := new(mockGatewayService)
	handler := NewGatewayHandler(svc, nil, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Any(gwProxyPath, handler.Proxy)

	var gzipFlushed bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, wreq *http.Request) {
		// Read body to confirm gzip was flushed
		body := make([]byte, 100)
		n, _ := wreq.Body.Read(body)
		gzipFlushed = n > 0
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	targetURL, _ := url.Parse(ts.URL)

	route := &domain.GatewayRoute{ID: uuid.New(), Compression: "gzip", AllowedIPNets: []*net.IPNet{}}
	svc.On("GetProxy", "GET", "/api").Return(
		httputil.NewSingleHostReverseProxy(targetURL), route, map[string]string{}, true).Once()

	req, _ := http.NewRequest("GET", gwAPITestPath, strings.NewReader("hello world this is some test data"))
	req.Header.Set("Accept-Encoding", "gzip")
	req.RemoteAddr = "10.0.0.1:12345"
	w := &closeNotifierRecorder{httptest.NewRecorder()}
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
	assert.True(t, gzipFlushed, "gzip data should be flushed to upstream")
	svc.AssertExpectations(t)
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

func TestGatewayHandlerCreateRouteDryRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		body          map[string]interface{}
		wantValid     bool
		wantErrCount  int
		errContains   []string
	}{
		{
			name: "valid CIDR - passes",
			body: map[string]interface{}{
				"name":        "dry-run-test",
				"path_prefix": "/api/v1",
				"target_url":  "http://example.com",
				"allowed_cidrs": []string{"10.0.0.0/8"},
			},
			wantValid: true,
		},
		{
			name: "invalid CIDR - fails",
			body: map[string]interface{}{
				"name":        "dry-run-test",
				"path_prefix": "/api/v1",
				"target_url":  "http://example.com",
				"allowed_cidrs": []string{"not-a-cidr"},
			},
			wantValid:    false,
			errContains:  []string{"invalid allowed CIDR"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, handler, r := setupGatewayHandlerTest(t)
			r.POST(routesPath, handler.CreateRoute)

			body, err := json.Marshal(tc.body)
			require.NoError(t, err)
			req, err := http.NewRequest("POST", routesPath+"?dry_run=true", bytes.NewBuffer(body))
			require.NoError(t, err)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			var resp map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)
			data, ok := resp["data"].(map[string]interface{})
			require.True(t, ok, "response data should be a map")
			assert.Equal(t, true, data["dry_run"])
			assert.Equal(t, tc.wantValid, data["valid"])
			if !tc.wantValid && len(tc.errContains) > 0 {
				errs, ok := data["errors"].([]interface{})
				require.True(t, ok)
				for _, exp := range tc.errContains {
					found := false
					for _, e := range errs {
						if strings.Contains(e.(string), exp) {
							found = true
							break
						}
					}
					assert.True(t, found, "expected error containing %q", exp)
				}
			}
		})
	}
}
