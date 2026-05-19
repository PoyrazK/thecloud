// Package httputil provides HTTP utilities and middleware.
package httputil

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/services"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
)

// Auth enforces API key authentication and injects user context.
func Auth(svc ports.IdentityService, tenantSvc ports.TenantService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try JWT first
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			// JWT handling would go here - for now, fall through to API key
		}

		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			Error(c, errors.New(errors.Unauthorized, "API key required"))
			c.Abort()
			return
		}

		apiKeyObj, err := svc.ValidateAPIKey(c.Request.Context(), apiKey)
		if err != nil {
			Error(c, errors.New(errors.Unauthorized, "invalid API key"))
			c.Abort()
			return
		}

		// Wrap the request context with UserID
		ctx := appcontext.WithUserID(c.Request.Context(), apiKeyObj.UserID)

		tenantID, err := resolveAndVerifyTenant(ctx, c.GetHeader("X-Tenant-ID"), apiKeyObj.DefaultTenantID, apiKeyObj.UserID, tenantSvc)
		if err != nil {
			Error(c, err)
			c.Abort()
			return
		}

		if tenantID != uuid.Nil {
			ctx = appcontext.WithTenantID(ctx, tenantID)
			c.Set("tenantID", tenantID)
		}

		c.Request = c.Request.WithContext(ctx)

		c.Set("userID", apiKeyObj.UserID) // Also keep in Gin context for convenience
		c.Next()
	}
}

// JWTAuth enforces JWT authentication and injects user context.
// Supports both JWT and API key authentication for flexibility.
func JWTAuth(jwtSvc *services.JWTService, identitySvc ports.IdentityService, tenantSvc ports.TenantService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := jwtSvc.ValidateToken(tokenString)
			if err == nil && claims.UserID != uuid.Nil {
				ctx := appcontext.WithUserID(c.Request.Context(), claims.UserID)
				if claims.TenantID != uuid.Nil {
					ctx = appcontext.WithTenantID(ctx, claims.TenantID)
					c.Set("tenantID", claims.TenantID)
				}
				c.Set("userID", claims.UserID)
				c.Request = c.Request.WithContext(ctx)
				c.Next()
				return
			}
		}

		// Fall back to API key
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			Error(c, errors.New(errors.Unauthorized, "API key or JWT required"))
			c.Abort()
			return
		}

		apiKeyObj, err := identitySvc.ValidateAPIKey(c.Request.Context(), apiKey)
		if err != nil {
			Error(c, errors.New(errors.Unauthorized, "invalid API key"))
			c.Abort()
			return
		}

		ctx := appcontext.WithUserID(c.Request.Context(), apiKeyObj.UserID)

		tenantID, err := resolveAndVerifyTenant(ctx, c.GetHeader("X-Tenant-ID"), apiKeyObj.DefaultTenantID, apiKeyObj.UserID, tenantSvc)
		if err != nil {
			Error(c, err)
			c.Abort()
			return
		}

		if tenantID != uuid.Nil {
			ctx = appcontext.WithTenantID(ctx, tenantID)
			c.Set("tenantID", tenantID)
		}

		c.Request = c.Request.WithContext(ctx)
		c.Set("userID", apiKeyObj.UserID)
		c.Next()
	}
}

// GetTenantID returns the active tenant ID from the request context.
func GetTenantID(c *gin.Context) uuid.UUID {
	val, exists := c.Get("tenantID")
	if !exists {
		return uuid.Nil
	}
	tenantID, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return tenantID
}

// GetUserID returns the authenticated user ID from the request context.
func GetUserID(c *gin.Context) uuid.UUID {
	val, exists := c.Get("userID")
	if !exists {
		return uuid.Nil
	}
	userID, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return userID
}

// Permission enforces RBAC checks for the provided permission.
func Permission(rbac ports.RBACService, permission domain.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		tenantID := GetTenantID(c)
		if userID == uuid.Nil {
			Error(c, errors.New(errors.Unauthorized, "authentication required"))
			c.Abort()
			return
		}

		if err := rbac.Authorize(c.Request.Context(), userID, tenantID, permission, "*"); err != nil {
			Error(c, errors.New(errors.Forbidden, "permission denied"))
			c.Abort()
			return
		}

		c.Next()
	}
}

func resolveAndVerifyTenant(ctx context.Context, tenantIDStr string, defaultTenantID *uuid.UUID, userID uuid.UUID, tenantSvc ports.TenantService) (uuid.UUID, error) {
	var tenantID uuid.UUID

	if tenantIDStr != "" {
		var err error
		tenantID, err = uuid.Parse(tenantIDStr)
		if err != nil {
			return uuid.Nil, errors.New(errors.InvalidInput, "invalid tenant ID header")
		}
	} else if defaultTenantID != nil {
		tenantID = *defaultTenantID
	}

	if tenantID != uuid.Nil {
		// Verify membership
		member, err := tenantSvc.GetMembership(ctx, tenantID, userID)
		if err != nil {
			return uuid.Nil, errors.New(errors.Forbidden, "failed to verify tenant membership")
		}
		if member == nil {
			return uuid.Nil, errors.New(errors.Forbidden, "user is not a member of this tenant")
		}
	}

	return tenantID, nil
}
