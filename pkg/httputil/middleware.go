// Package httputil provides HTTP utilities and middleware.
package httputil

import (
	"log/slog"
	"time"

	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/platform"
)

// Metrics records HTTP request counts and latencies.
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method

		platform.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		platform.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
	}
}

// HeaderXRequestID is the header used to track request IDs.
const HeaderXRequestID = "X-Request-ID"

// RequestID injects a request ID into the context and response headers.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(HeaderXRequestID)
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("requestID", requestID)
		c.Header(HeaderXRequestID, requestID)
		c.Next()
	}
}

// Logger logs request details using the provided logger.
func Logger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		requestID, _ := c.Get("requestID")

		logger.Info("request",
			"status", c.Writer.Status(),
			"method", c.Request.Method,
			"path", path,
			"query", query,
			"ip", c.ClientIP(),
			"latency", time.Since(start),
			"requestID", requestID,
		)
	}
}

// CORS enables permissive CORS headers for development.
//
// Browsers reject responses that combine `Access-Control-Allow-Origin: *`
// with `Access-Control-Allow-Credentials: true`. When the request supplies
// an `Origin` header we mirror it back (with `Vary: Origin`) and allow
// credentials; otherwise we fall back to the wildcard with no credentials.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
		} else {
			c.Header("Access-Control-Allow-Origin", "*")
		}
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
		c.Header("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, x-api-key")
		c.Header("Access-Control-Expose-Headers", "Content-Length")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
