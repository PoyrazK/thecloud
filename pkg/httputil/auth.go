// Package httputil provides HTTP utilities and middleware.
package httputil

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
)

// Auth enforces API key or Bearer token authentication and injects context.
func Auth(svc ports.IdentityService, tenantSvc ports.TenantService) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		sourceIP := c.ClientIP()
		if sourceIP == "" {
			sourceIP = c.Request.RemoteAddr
		}
		ctx = appcontext.WithSourceIP(ctx, sourceIP)

		// Check for Bearer token first
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := svc.ValidateAccessToken(ctx, token)
			if err != nil {
				Error(c, errors.New(errors.Unauthorized, "invalid access token"))
				c.Abort()
				return
			}

			// Service account context
			ctx = appcontext.WithServiceAccountID(ctx, claims.ServiceAccountID)
			ctx = appcontext.WithTenantID(ctx, claims.TenantID)
			c.Set("tenantID", claims.TenantID)
			c.Set("serviceAccountID", claims.ServiceAccountID)
			c.Set("serviceAccountRole", claims.Role)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		// Fall back to API key
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			Error(c, errors.New(errors.Unauthorized, "API key or Bearer token required"))
			c.Abort()
			return
		}

		apiKeyObj, err := svc.ValidateAPIKey(ctx, apiKey)
		if err != nil {
			Error(c, errors.New(errors.Unauthorized, "invalid API key"))
			c.Abort()
			return
		}

		// User context
		ctx = appcontext.WithUserID(ctx, apiKeyObj.UserID)

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

// GetServiceAccountID returns the authenticated service account ID from the request context.
func GetServiceAccountID(c *gin.Context) uuid.UUID {
	val, exists := c.Get("serviceAccountID")
	if !exists {
		return uuid.Nil
	}
	saID, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return saID
}

// Permission enforces RBAC checks for the provided permission.
func Permission(rbac ports.RBACService, permission domain.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		saID := GetServiceAccountID(c)
		tenantID := GetTenantID(c)

		if userID == uuid.Nil && saID == uuid.Nil {
			Error(c, errors.New(errors.Unauthorized, "authentication required"))
			c.Abort()
			return
		}

		var err error
		if saID != uuid.Nil {
			err = rbac.AuthorizeServiceAccount(c.Request.Context(), saID, tenantID, permission, "*")
		} else {
			err = rbac.Authorize(c.Request.Context(), userID, tenantID, permission, "*")
		}

		if err != nil {
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
