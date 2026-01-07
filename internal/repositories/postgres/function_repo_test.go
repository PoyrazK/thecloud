//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFunctionRepository_Integration(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	repo := NewFunctionRepository(db)
	ctx := setupTestUser(t, db)
	userID := appcontext.UserIDFromContext(ctx)

	// Cleanup
	_, _ = db.Exec(context.Background(), "DELETE FROM invocations")
	_, _ = db.Exec(context.Background(), "DELETE FROM functions")

	var functionID uuid.UUID

	t.Run("CreateFunction", func(t *testing.T) {
		functionID = uuid.New()
		fn := &domain.Function{
			ID:        functionID,
			UserID:    userID,
			Name:      "test-function",
			Runtime:   "python3.9",
			Handler:   "main.handler",
			CodePath:  "/tmp/code.zip",
			Timeout:   30,
			MemoryMB:  256,
			Status:    "active",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err := repo.Create(ctx, fn)
		require.NoError(t, err)
	})

	t.Run("GetByID", func(t *testing.T) {
		fn, err := repo.GetByID(ctx, functionID)
		require.NoError(t, err)
		assert.Equal(t, "test-function", fn.Name)
		assert.Equal(t, "python3.9", fn.Runtime)
		assert.Equal(t, 30, fn.Timeout)
	})

	t.Run("GetByName", func(t *testing.T) {
		fn, err := repo.GetByName(ctx, userID, "test-function")
		require.NoError(t, err)
		assert.Equal(t, functionID, fn.ID)
		assert.Equal(t, "main.handler", fn.Handler)
	})

	t.Run("List", func(t *testing.T) {
		functions, err := repo.List(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, functions, 1)
		assert.Equal(t, "test-function", functions[0].Name)
	})

	t.Run("CreateInvocation", func(t *testing.T) {
		invocationID := uuid.New()
		startTime := time.Now()
		endTime := startTime.Add(100 * time.Millisecond)

		invocation := &domain.Invocation{
			ID:         invocationID,
			FunctionID: functionID,
			Status:     "SUCCESS",
			StartedAt:  startTime,
			EndedAt:    &endTime,
			DurationMs: 100,
			StatusCode: 200,
			Logs:       "Function executed successfully",
		}

		err := repo.CreateInvocation(ctx, invocation)
		require.NoError(t, err)
	})

	t.Run("GetInvocations", func(t *testing.T) {
		invocations, err := repo.GetInvocations(ctx, functionID, 10)
		require.NoError(t, err)
		assert.Len(t, invocations, 1)
		assert.Equal(t, "SUCCESS", invocations[0].Status)
		assert.Equal(t, 100, int(invocations[0].DurationMs))
		assert.Equal(t, 200, invocations[0].StatusCode)
	})

	t.Run("DeleteFunction", func(t *testing.T) {
		err := repo.Delete(ctx, functionID)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, functionID)
		assert.Error(t, err)
	})

	t.Run("GetNonExistentFunction", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.New())
		assert.Error(t, err)
	})
}
