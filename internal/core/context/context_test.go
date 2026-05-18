package appcontext_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/stretchr/testify/assert"
)

type testKey string

func TestUserIDContext(t *testing.T) {
	t.Run("Extract from empty context", func(t *testing.T) {
		ctx := context.Background()
		userID := appcontext.UserIDFromContext(ctx)
		assert.Equal(t, uuid.Nil, userID)
	})

	t.Run("Set and extract", func(t *testing.T) {
		ctx := context.Background()
		expectedID := uuid.New()

		ctx = appcontext.WithUserID(ctx, expectedID)
		userID := appcontext.UserIDFromContext(ctx)

		assert.Equal(t, expectedID, userID)
	})

	t.Run("Ignore invalid type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), testKey("user_id"), "not-a-uuid")
		userID := appcontext.UserIDFromContext(ctx)
		assert.Equal(t, uuid.Nil, userID)
	})
}

func TestTenantIDContext(t *testing.T) {
	t.Run("Extract from empty context", func(t *testing.T) {
		ctx := context.Background()
		tenantID := appcontext.TenantIDFromContext(ctx)
		assert.Equal(t, uuid.Nil, tenantID)
	})

	t.Run("Set and extract", func(t *testing.T) {
		ctx := context.Background()
		expectedID := uuid.New()

		ctx = appcontext.WithTenantID(ctx, expectedID)
		tenantID := appcontext.TenantIDFromContext(ctx)

		assert.Equal(t, expectedID, tenantID)
	})

	t.Run("Ignore invalid type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), testKey("tenant_id"), "not-a-uuid")
		tenantID := appcontext.TenantIDFromContext(ctx)
		assert.Equal(t, uuid.Nil, tenantID)
	})
}

func TestSourceIPContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "Extract from empty context",
			ctx:      context.Background(),
			expected: "",
		},
		{
			name:     "Set and extract",
			ctx:      appcontext.WithSourceIP(context.Background(), "192.168.1.100"),
			expected: "192.168.1.100",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := appcontext.SourceIPFromContext(tc.ctx)
			assert.Equal(t, tc.expected, got)
		})
	}
}
