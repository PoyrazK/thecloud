package httputil

import "github.com/gin-gonic/gin"

// SecurityHeadersMiddleware adds common security headers to responses
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		// Basic CSP - usually needs to be customized for frontend requirements
		c.Header("Content-Security-Policy", "default-src 'self'")

		c.Next()
	}
}
