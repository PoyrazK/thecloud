package appcontext

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const (
	userIDKey contextKey = "user_id"
)

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
