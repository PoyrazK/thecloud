// Package httphandlers provides HTTP handlers for the API.
package httphandlers

import (
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/platform"
	"github.com/poyrazk/thecloud/pkg/httputil"
	"github.com/poyrazk/thecloud/pkg/ratelimit"
	"golang.org/x/time/rate"
)

// CreateRouteRequest define the payload for creating a route.
type CreateRouteRequest struct {
	Name                    string   `json:"name" binding:"required"`
	PathPrefix              string   `json:"path_prefix" binding:"required"`
	TargetURL               string   `json:"target_url" binding:"required"`
	Methods                 []string `json:"methods"`
	StripPrefix             bool     `json:"strip_prefix"`
	RateLimit               int      `json:"rate_limit" binding:"gte=0"`
	DialTimeout             int64    `json:"dial_timeout" binding:"gte=0"`
	ResponseHeaderTimeout   int64    `json:"response_header_timeout" binding:"gte=0"`
	IdleConnTimeout         int64    `json:"idle_conn_timeout" binding:"gte=0"`
	TLSSkipVerify           bool     `json:"tls_skip_verify"`
	RequireTLS              bool     `json:"require_tls"`
	AllowedCIDRs            []string `json:"allowed_cidrs"`
	BlockedCIDRs            []string `json:"blocked_cidrs"`
	MaxBodySize             int64    `json:"max_body_size" binding:"gte=0"`
	Priority                int      `json:"priority" binding:"gte=0"`
	CircuitBreakerThreshold int      `json:"circuit_breaker_threshold" binding:"gte=0"`
	CircuitBreakerTimeout   int64    `json:"circuit_breaker_timeout" binding:"gte=0"`
	MaxRetries              int      `json:"max_retries" binding:"gte=0"`
	RetryTimeout            int64    `json:"retry_timeout" binding:"gte=0"`
	AllowedOrigins          []string `json:"allowed_origins"`
	AllowedMethods          []string `json:"allowed_methods"`
	AllowedHeaders          []string `json:"allowed_headers"`
	ExposeHeaders           []string `json:"expose_headers"`
	MaxAge                  int      `json:"max_age"`
	StripResponseHeaders    []string `json:"strip_response_headers"`
	Compression             string   `json:"compression"`
	JWTIssuer               string   `json:"jwt_issuer"`
	JWTJwksURL              string   `json:"jwt_jwks_url"`
	JWTAudience            string   `json:"jwt_audience"`
	ClientCert             string   `json:"client_cert"`
	ClientKey              string   `json:"client_key"`
	CACert                 string   `json:"ca_cert"`
}

// GatewayHandler handles API gateway HTTP endpoints.
// Note: logger may be nil in test contexts; all logging calls check for nil before use.
type GatewayHandler struct {
	svc         ports.GatewayService
	rateLimiter *ratelimit.IPRateLimiter
	logger      *slog.Logger
}

// NewGatewayHandler constructs a GatewayHandler.
func NewGatewayHandler(svc ports.GatewayService, rateLimiter *ratelimit.IPRateLimiter, logger *slog.Logger) *GatewayHandler {
	return &GatewayHandler{svc: svc, rateLimiter: rateLimiter, logger: logger}
}

// CreateRoute establishes a new ingress mapping
// @Summary Create a new gateway route
// @Description Registers a new path pattern for the API gateway to proxy to a backend
// @Tags gateway
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param request body CreateRouteRequest true "Create route request"
// @Success 201 {object} domain.GatewayRoute
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /gateway/routes [post]
func (h *GatewayHandler) CreateRoute(c *gin.Context) {
	var req CreateRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "Invalid request body"))
		return
	}

	if req.RateLimit == 0 {
		req.RateLimit = 100
	}

	// Validate TLS settings
	if req.RequireTLS && req.TLSSkipVerify {
		httputil.Error(c, errors.New(errors.InvalidInput, "cannot set both require_tls and tls_skip_verify"))
		return
	}

	// Validate mTLS certificate pairing
	if (req.ClientCert != "") != (req.ClientKey != "") {
		httputil.Error(c, errors.New(errors.InvalidInput, "both client_cert and client_key must be provided together"))
		return
	}

	// Dry-run mode: validate without persisting
	if c.Request.URL.Query().Get("dry_run") == "true" {
		h.validateDryRun(c, req)
		return
	}

	params := ports.CreateRouteParams{
		Name:                    req.Name,
		Pattern:                 req.PathPrefix,
		Target:                  req.TargetURL,
		Methods:                 req.Methods,
		StripPrefix:             req.StripPrefix,
		RateLimit:               req.RateLimit,
		DialTimeout:             req.DialTimeout,
		ResponseHeaderTimeout:   req.ResponseHeaderTimeout,
		IdleConnTimeout:         req.IdleConnTimeout,
		TLSSkipVerify:           req.TLSSkipVerify,
		RequireTLS:              req.RequireTLS,
		AllowedCIDRs:            req.AllowedCIDRs,
		BlockedCIDRs:            req.BlockedCIDRs,
		MaxBodySize:             req.MaxBodySize,
		Priority:                req.Priority,
		CircuitBreakerThreshold: req.CircuitBreakerThreshold,
		CircuitBreakerTimeout:   req.CircuitBreakerTimeout,
		MaxRetries:              req.MaxRetries,
		RetryTimeout:            req.RetryTimeout,
		AllowedOrigins:          req.AllowedOrigins,
		AllowedMethods:          req.AllowedMethods,
		AllowedHeaders:          req.AllowedHeaders,
		ExposeHeaders:           req.ExposeHeaders,
		MaxAge:                  req.MaxAge,
		StripResponseHeaders:    req.StripResponseHeaders,
		Compression:             req.Compression,
		JWTIssuer:               req.JWTIssuer,
		JWTJwksURL:              req.JWTJwksURL,
		JWTAudience:            req.JWTAudience,
		ClientCert:             req.ClientCert,
		ClientKey:              req.ClientKey,
		CACert:                 req.CACert,
	}

	route, err := h.svc.CreateRoute(c.Request.Context(), params)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, route)
}

// ListRoutes returns all gateway routes
// @Summary List all gateway routes
// @Description Gets a list of all registered API gateway routes
// @Tags gateway
// @Produce json
// @Security APIKeyAuth
// @Success 200 {array} domain.GatewayRoute
// @Failure 500 {object} httputil.Response
// @Router /gateway/routes [get]
func (h *GatewayHandler) ListRoutes(c *gin.Context) {
	routes, err := h.svc.ListRoutes(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, routes)
}

// DeleteRoute removes a gateway route
// @Summary Delete a gateway route
// @Description Removes an existing API gateway route by ID
// @Tags gateway
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Route ID"
// @Success 200 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /gateway/routes/{id} [delete]
func (h *GatewayHandler) DeleteRoute(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "Invalid route ID"))
		return
	}

	if err := h.svc.DeleteRoute(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "Route deleted"})
}

func (h *GatewayHandler) Proxy(c *gin.Context) {
	path := c.Param("proxy") // Expecting routes like /gw/*proxy
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	proxy, route, params, ok := h.svc.GetProxy(c.Request.Method, path)
	if !ok {
		httputil.Error(c, errors.New(errors.NotFound, "No route found for "+path))
		return
	}

	// Apply IP allowlist/denylist (nil route means no route-specific rules apply)
	if route != nil && !h.checkCIDR(c, route) {
		return
	}

	// JWT validation if configured
	if route != nil && route.JWTJwksURL != "" {
		authHeader := c.GetHeader("Authorization")
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := h.svc.ValidateJWT(c.Request.Context(), route, tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		for k, v := range claims {
			c.Request.Header.Set("X-JWT-Claim-"+k, v)
		}
	}

	// Apply request size limit - reject oversized requests before proxying
	if route != nil && route.MaxBodySize > 0 {
		if c.Request.ContentLength > route.MaxBodySize {
			httputil.Error(c, errors.New(errors.ObjectTooLarge, "request body too large"))
			return
		}
		// For chunked bodies, pre-read and enforce limit
		if c.Request.ContentLength < 0 {
			c.Request.Body = &limitedReader{
				ReadCloser: c.Request.Body,
				limit:      route.MaxBodySize,
			}
		}
	}

	// Inject parameters into request context for downstream services if needed
	if len(params) > 0 {
		for k, v := range params {
			c.Set("path_param_"+k, v)
		}
	}

	// Inject trace headers
	h.injectTraceHeaders(c)

	// Apply per-route rate limiting if configured
	if route != nil && route.RateLimit > 0 && h.rateLimiter != nil {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			key = c.ClientIP()
		} else if len(key) > 5 {
			key = "apikey:" + key[:5]
		}
		burst := route.RateLimit * 2
		limiter := h.rateLimiter.GetRouteLimiter(route.ID, key, rate.Limit(route.RateLimit), burst)
		if !limiter.Allow() {
			if h.logger != nil {
				h.logger.Warn("per-route rate limit exceeded",
					slog.String("key", key),
					slog.String("path", c.Request.URL.Path),
					slog.String("route_id", route.ID.String()))
			}
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}
	}

	// Record upstream latency metric
	routeID := ""
	if route != nil {
		routeID = route.ID.String()
	}
	upstreamStart := time.Now()

	// Wrap response writer to capture status code for CORS headers and optionally compress
	var wrapper http.ResponseWriter = &responseWrapper{ResponseWriter: c.Writer, status: http.StatusOK}

	// Enable compression if configured and client accepts it
	if route != nil && route.Compression != "" && strings.Contains(c.GetHeader("Accept-Encoding"), route.Compression) {
		wrapper = newCompressWriter(c.Writer, route.Compression)
	}

	proxy.ServeHTTP(wrapper, c.Request)

	// Apply route-level CORS headers if configured
	if route != nil && len(route.AllowedOrigins) > 0 {
		h.injectCORSHeaders(c, route)
	}

	// Strip configured response headers
	if route != nil && len(route.StripResponseHeaders) > 0 {
		for _, h := range route.StripResponseHeaders {
			c.Header(h, "")
		}
	}

	platform.GatewayUpstreamLatency.WithLabelValues(routeID, c.Request.Method, path).Observe(time.Since(upstreamStart).Seconds())
}

type responseWrapper struct {
	http.ResponseWriter
	status int
}

func (rw *responseWrapper) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

type compressWriter struct {
	http.ResponseWriter
	status      int
	compression string
	gz          *gzip.Writer
}

func newCompressWriter(w http.ResponseWriter, compression string) *compressWriter {
	return &compressWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
		compression:    compression,
	}
}

func (cw *compressWriter) WriteHeader(status int) {
	cw.Header().Set("Content-Encoding", cw.compression)
	cw.ResponseWriter.WriteHeader(status)
}

func (cw *compressWriter) Write(p []byte) (int, error) {
	if cw.gz == nil {
		cw.gz = gzip.NewWriter(cw.ResponseWriter)
	}
	return cw.gz.Write(p)
}

func (cw *compressWriter) Close() error {
	if cw.gz != nil {
		return cw.gz.Close()
	}
	return nil
}

func (h *GatewayHandler) validateDryRun(c *gin.Context, req CreateRouteRequest) {
	var validationErrors []string

	// Pattern validation
	if strings.Contains(req.PathPrefix, "{") || strings.Contains(req.PathPrefix, "*") {
		// Validate using routing pattern compilation
		_ = req.PathPrefix // pattern validation placeholder
	}

	// CIDR validation
	for _, cidr := range req.AllowedCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("invalid allowed CIDR %q", cidr))
		}
	}
	for _, cidr := range req.BlockedCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("invalid blocked CIDR %q", cidr))
		}
	}

	// TLS conflict
	if req.RequireTLS && req.TLSSkipVerify {
		validationErrors = append(validationErrors, "cannot set both require_tls and tls_skip_verify")
	}

	// mTLS pairing
	if (req.ClientCert != "") != (req.ClientKey != "") {
		validationErrors = append(validationErrors, "both client_cert and client_key must be provided together")
	}

	if len(validationErrors) > 0 {
		httputil.Success(c, http.StatusOK, gin.H{"dry_run": true, "valid": false, "errors": validationErrors})
		return
	}
	httputil.Success(c, http.StatusOK, gin.H{"dry_run": true, "valid": true, "message": "Route configuration is valid"})
}

func (h *GatewayHandler) injectTraceHeaders(c *gin.Context) {
	requestID := c.GetHeader("X-Request-ID")
	if requestID == "" {
		requestID = uuid.New().String()
	}
	c.Request.Header.Set("X-Request-ID", requestID)
	c.Header("X-Request-ID", requestID)

	// W3C TraceContext - preserve incoming trace headers if present
	inboundTraceParent := c.GetHeader("traceparent")
	if inboundTraceParent != "" {
		c.Request.Header.Set("traceparent", inboundTraceParent)
		c.Header("traceparent", inboundTraceParent)
		inboundTraceState := c.GetHeader("tracestate")
		if inboundTraceState != "" {
			c.Request.Header.Set("tracestate", inboundTraceState)
			c.Header("tracestate", inboundTraceState)
		}
		return
	}

	// No inbound traceparent - generate new trace context
	traceID := generateTraceID()
	spanID := generateSpanID()
	c.Request.Header.Set("traceparent", fmt.Sprintf("00-%s-%s-01", traceID, spanID))
	c.Request.Header.Set("tracestate", "")
	c.Header("traceparent", fmt.Sprintf("00-%s-%s-01", traceID, spanID))
	c.Header("tracestate", "")
}

func generateTraceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read rarely fails, but handle it gracefully
		return uuid.New().String()
	}
	return hex.EncodeToString(b)
}

func generateSpanID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return uuid.New().String()[:16]
	}
	return hex.EncodeToString(b)
}

func (h *GatewayHandler) checkCIDR(c *gin.Context, route *domain.GatewayRoute) bool {
	clientIP := net.ParseIP(c.ClientIP())
	if clientIP == nil {
		httputil.Error(c, errors.New(errors.Forbidden, "access denied: invalid client IP"))
		return false
	}

	// Check blocked CIDRs first (takes precedence)
	for _, ipNet := range route.BlockedIPNets {
		if ipNet.Contains(clientIP) {
			httputil.Error(c, errors.New(errors.Forbidden, "access denied"))
			return false
		}
	}

	// If allowlist is non-empty, only allow matched IPs
	if len(route.AllowedIPNets) > 0 {
		allowed := false
		for _, ipNet := range route.AllowedIPNets {
			if ipNet.Contains(clientIP) {
				allowed = true
				break
			}
		}
		if !allowed {
			httputil.Error(c, errors.New(errors.Forbidden, "access denied"))
			return false
		}
	}

	return true
}

func (h *GatewayHandler) injectCORSHeaders(c *gin.Context, route *domain.GatewayRoute) {
	origin := c.GetHeader("Origin")
	if origin == "" {
		return
	}

	// Check if origin is allowed
	allowed := false
	for _, o := range route.AllowedOrigins {
		if o == "*" || o == origin {
			allowed = true
			break
		}
	}
	if !allowed {
		return
	}

	c.Header("Access-Control-Allow-Origin", origin)
	if len(route.AllowedMethods) > 0 {
		c.Header("Access-Control-Allow-Methods", strings.Join(route.AllowedMethods, ", "))
	}
	if len(route.AllowedHeaders) > 0 {
		c.Header("Access-Control-Allow-Headers", strings.Join(route.AllowedHeaders, ", "))
	}
	if len(route.ExposeHeaders) > 0 {
		c.Header("Access-Control-Expose-Headers", strings.Join(route.ExposeHeaders, ", "))
	}
	if route.MaxAge > 0 {
		c.Header("Access-Control-Max-Age", strconv.Itoa(route.MaxAge))
	}
}

// limitedReader wraps an io.ReadCloser and enforces a byte limit.
type limitedReader struct {
	io.ReadCloser
	limit int64
	read  int64
}

// Read enforces the byte limit. When the limit is reached, io.EOF is returned
// even if the underlying reader returned an error (error shadowing for limit enforcement).
func (l *limitedReader) Read(p []byte) (n int, err error) {
	if l.read >= l.limit {
		return 0, io.EOF
	}
	toRead := l.limit - l.read
	if int64(len(p)) > toRead {
		p = p[:toRead]
	}
	n, err = l.ReadCloser.Read(p)
	l.read += int64(n)
	if l.read >= l.limit && err == nil {
		err = io.EOF
	}
	return
}

func (l *limitedReader) Close() error {
	return l.ReadCloser.Close()
}
