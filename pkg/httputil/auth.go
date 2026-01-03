package httputil

import (
	"github.com/gin-gonic/gin"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
)

func Auth(identitySvc ports.IdentityService, authSvc ports.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			Error(c, errors.New(errors.Unauthorized, "API key required"))
			c.Abort()
			return
		}

		apiKeyObj, err := identitySvc.ValidateApiKey(c.Request.Context(), apiKey)
		if err != nil {
			Error(c, errors.New(errors.Unauthorized, "invalid API key"))
			c.Abort()
			return
		}

		user, err := authSvc.ValidateUser(c.Request.Context(), apiKeyObj.UserID)
		if err != nil {
			Error(c, errors.New(errors.Unauthorized, "invalid user"))
			c.Abort()
			return
		}

		// Wrap the request context with UserID
		ctx := appcontext.WithUserID(c.Request.Context(), apiKeyObj.UserID)
		ctx = appcontext.WithUserRole(ctx, user.Role)
		c.Request = c.Request.WithContext(ctx)

		c.Set("userID", apiKeyObj.UserID) // Also keep in Gin context for convenience
		c.Set("userRole", user.Role)
		c.Next()
	}
}
