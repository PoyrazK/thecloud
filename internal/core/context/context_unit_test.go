package appcontext

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemUserID(t *testing.T) {
	id, err := SystemUserID()
	require.NoError(t, err)
	assert.Equal(t, systemUserIDStr, id.String())
}

func TestInternalCall(t *testing.T) {
	ctx := context.Background()
	assert.False(t, IsInternalCall(ctx))

	ctx = WithInternalCall(ctx)
	assert.True(t, IsInternalCall(ctx))
}

func TestUserIDFromContext(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, uuid.Nil, UserIDFromContext(ctx))

	userID := uuid.New()
	ctx = WithUserID(ctx, userID)
	assert.Equal(t, userID, UserIDFromContext(ctx))
}

func TestTenantIDFromContext(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, uuid.Nil, TenantIDFromContext(ctx))

	tenantID := uuid.New()
	ctx = WithTenantID(ctx, tenantID)
	assert.Equal(t, tenantID, TenantIDFromContext(ctx))
}

func TestSourceIPFromContext_InvalidType(t *testing.T) {
	// Use the real sourceIPKey to test that invalid type handling works
	ctx := context.WithValue(context.Background(), sourceIPKey, 12345)
	got := SourceIPFromContext(ctx)
	assert.Equal(t, "", got, "should return empty string when value is not a string")
}
