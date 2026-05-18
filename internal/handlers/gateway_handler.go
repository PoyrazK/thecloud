// Package httphandlers provides HTTP handlers for the API.
package httphandlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
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
}

// GatewayHandler handles API gateway HTTP endpoints.
// Note: logger may be nil in test contexts; all logging calls check for nil before use.
type GatewayHandler struct {
	svc    ports.GatewayService
	logger *slog.Logger
}

// NewGatewayHandler constructs a GatewayHandler.
func NewGatewayHandler(svc ports.GatewayService, logger *slog.Logger) *GatewayHandler {
	return &GatewayHandler{svc: svc, logger: logger}
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

	proxy.ServeHTTP(c.Writer, c.Request)
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
