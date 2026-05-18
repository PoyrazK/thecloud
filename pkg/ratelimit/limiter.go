// Package ratelimit provides rate limiting middleware and utilities.
package ratelimit

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// IPRateLimiter manages rate limiters for different IPs/clients
type IPRateLimiter struct {
	ips    map[string]*rate.Limiter
	routes map[uuid.UUID]map[string]*rate.Limiter // per-route limiters
	mu     sync.RWMutex
	rate   rate.Limit
	burst  int
	logger *slog.Logger
}

// NewIPRateLimiter creates a new rate limiter manager
func NewIPRateLimiter(r rate.Limit, b int, logger *slog.Logger) *IPRateLimiter {
	i := &IPRateLimiter{
		ips:    make(map[string]*rate.Limiter),
		routes: make(map[uuid.UUID]map[string]*rate.Limiter),
		rate:   r,
		burst:  b,
		logger: logger,
	}

	// Periodic cleanup of old entries
	go i.cleanupLoop()

	return i
}

// GetLimiter returns a rate limiter for the given key (IP or API Key)
func (i *IPRateLimiter) GetLimiter(key string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.ips[key]
	if !exists {
		limiter = rate.NewLimiter(i.rate, i.burst)
		i.ips[key] = limiter
	}

	return limiter
}

// GetRouteLimiter returns a rate limiter for a specific route and client key.
// This enables per-route rate limiting while maintaining per-client tracking.
// The r and burst parameters specify the per-route rate limits.
func (i *IPRateLimiter) GetRouteLimiter(routeID uuid.UUID, key string, r rate.Limit, burst int) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.routes[routeID] == nil {
		i.routes[routeID] = make(map[string]*rate.Limiter)
	}

	limiter, exists := i.routes[routeID][key]
	if !exists {
		limiter = rate.NewLimiter(r, burst)
		i.routes[routeID][key] = limiter
		return limiter
	}

	// Recreate limiter if rate or burst changed (SetLimit only updates rate)
	if limiter.Limit() != r || limiter.Burst() != burst {
		limiter = rate.NewLimiter(r, burst)
		i.routes[routeID][key] = limiter
	}
	return limiter
}

// TestSetLimiter sets a fixed limiter for a route+key pair, for package testing only.
func (i *IPRateLimiter) TestSetLimiter(routeID uuid.UUID, key string, limiter *rate.Limiter) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.routes[routeID] == nil {
		i.routes[routeID] = make(map[string]*rate.Limiter)
	}
	i.routes[routeID][key] = limiter
}

// cleanupLoop removes old entries periodically. This is a simplified GC strategy
// that discards all limiters every 10 minutes rather than tracking per-entry
// access times. Sufficient for moderate traffic; consider tracking last-access
// timestamps for high-traffic production deployments.
func (i *IPRateLimiter) cleanupLoop() {
	for {
		time.Sleep(10 * time.Minute)
		i.mu.Lock()
		// Start fresh every cleanup cycle for simplicity
		// A production robust implementation would track last access time
		i.ips = make(map[string]*rate.Limiter)
		i.routes = make(map[uuid.UUID]map[string]*rate.Limiter)
		i.mu.Unlock()
	}
}

// Middleware creates a Gin middleware for rate limiting
func Middleware(limiter *IPRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prefer API Key if available, fallback to IP
		key := c.GetHeader("X-API-Key")
		if key == "" {
			key = c.ClientIP()
		} else if len(key) > 5 {
			// Mask key for safety in memory
			key = "apikey:" + key[:5]
		}

		l := limiter.GetLimiter(key)
		if !l.Allow() {
			limiter.logger.Warn("rate limit exceeded",
				slog.String("key", key),
				slog.String("path", c.Request.URL.Path))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}

		c.Next()
	}
}
