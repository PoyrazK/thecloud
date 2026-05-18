// Package appcontext provides context utilities for the application.
package appcontext

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const (
	userIDKey           contextKey = "user_id"
	tenantIDKey         contextKey = "tenant_id"
	internalKey         contextKey = "is_internal"
	sourceIPKey         contextKey = "source_ip"
	serviceAccountIDKey contextKey = "service_account_id"

	systemUserIDStr = "00000000-0000-0000-0000-000000000001"
)

// SystemUserID returns the reserved UUID for system-level background tasks.
func SystemUserID() (uuid.UUID, error) {
	return uuid.Parse(systemUserIDStr)
}

// WithInternalCall returns a new context marked as an internal system call.
func WithInternalCall(ctx context.Context) context.Context {
	return context.WithValue(ctx, internalKey, true)
}

// IsInternalCall returns true if the context is marked as an internal system call.
func IsInternalCall(ctx context.Context) bool {
	internal, ok := ctx.Value(internalKey).(bool)
	return ok && internal
}

// WithUserID returns a new context with the given userID.
func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserIDFromContext returns the userID from the context, or uuid.Nil if not found.
func UserIDFromContext(ctx context.Context) uuid.UUID {
	userID, ok := ctx.Value(userIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return userID
}

// WithTenantID returns a new context with the given tenantID.
func WithTenantID(ctx context.Context, tenantID uuid.UUID) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// TenantIDFromContext returns the tenantID from the context, or uuid.Nil if not found.
func TenantIDFromContext(ctx context.Context) uuid.UUID {
	tenantID, ok := ctx.Value(tenantIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return tenantID
}

// WithSourceIP returns a new context with the client source IP address.
func WithSourceIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, sourceIPKey, ip)
}

// SourceIPFromContext returns the source IP from the context, or empty string if not found.
func SourceIPFromContext(ctx context.Context) string {
	ip, ok := ctx.Value(sourceIPKey).(string)
	if !ok {
		return ""
	}
	return ip
}

// WithServiceAccountID returns a new context with the given service account ID.
func WithServiceAccountID(ctx context.Context, saID uuid.UUID) context.Context {
	return context.WithValue(ctx, serviceAccountIDKey, saID)
}

// ServiceAccountIDFromContext returns the service account ID from the context, or uuid.Nil if not found.
func ServiceAccountIDFromContext(ctx context.Context) uuid.UUID {
	saID, ok := ctx.Value(serviceAccountIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return saID
}
