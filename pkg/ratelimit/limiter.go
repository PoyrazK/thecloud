package ratelimit

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// IPRateLimiter manages rate limiters for different IPs/clients
type IPRateLimiter struct {
	ips    map[string]*rate.Limiter
	mu     sync.RWMutex
	rate   rate.Limit
	burst  int
	logger *slog.Logger
}

// NewIPRateLimiter creates a new rate limiter manager
func NewIPRateLimiter(r rate.Limit, b int, logger *slog.Logger) *IPRateLimiter {
	i := &IPRateLimiter{
		ips:    make(map[string]*rate.Limiter),
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

// cleanupLoop removes old entries (rudimentary GC)
func (i *IPRateLimiter) cleanupLoop() {
	for {
		time.Sleep(10 * time.Minute)
		i.mu.Lock()
		// Start fresh every cleanup cycle for simplicity
		// A production robust implementation would track last access time
		i.ips = make(map[string]*rate.Limiter)
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
		} else {
			// Mask key for safety in memory
			if len(key) > 5 {
				key = "apikey:" + key[:5]
			}
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
