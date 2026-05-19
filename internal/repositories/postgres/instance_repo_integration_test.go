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

const integrationInstanceName = "integration-test-inst"

func TestInstanceRepositoryIntegration(t *testing.T) {
	db, _ := SetupDB(t)
	defer db.Close()
	repo := NewInstanceRepository(db)
	ctx := SetupTestUser(t, db)
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	// Cleanup
	// We need to clean up strictly for this user or all?
	// Easier to just not fail on cleanup or do it carefully.
	// "DELETE FROM instances" deletes everything. That might collide with other tests running in parallel
	// but here we are running sequentially usually.
	_, err := db.Exec(context.Background(), "DELETE FROM instances")
	require.NoError(t, err)

	t.Run("Create and Get", func(t *testing.T) {
		id := uuid.New()
		inst := &domain.Instance{
			ID:        id,
			UserID:    userID,
			TenantID:  tenantID,
			Name:      integrationInstanceName,
			Image:     "alpine",
			Status:    domain.StatusStarting,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Version:   1,
		}

		err := repo.Create(ctx, inst)
		require.NoError(t, err)

		fetched, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, inst.Name, fetched.Name)
		assert.Equal(t, inst.Status, fetched.Status)
	})

	t.Run("List", func(t *testing.T) {
		list, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, list)
	})

	t.Run("GetByName", func(t *testing.T) {
		fetched, err := repo.GetByName(ctx, integrationInstanceName)
		require.NoError(t, err)
		assert.Equal(t, integrationInstanceName, fetched.Name)
	})

	t.Run("Update", func(t *testing.T) {
		inst, err := repo.GetByName(ctx, integrationInstanceName)
		require.NoError(t, err)

		inst.Status = domain.StatusRunning
		err = repo.Update(ctx, inst)
		require.NoError(t, err)

		fetched, err := repo.GetByID(ctx, inst.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusRunning, fetched.Status)
		assert.Equal(t, 2, fetched.Version)
	})

	t.Run("Update Conflict", func(t *testing.T) {
		inst, err := repo.GetByName(ctx, integrationInstanceName)
		require.NoError(t, err)

		// Create a stale copy
		staleInst := *inst

		// Update original
		inst.Status = domain.StatusStopped
		err = repo.Update(ctx, inst)
		require.NoError(t, err)

		// Try to update with stale copy
		staleInst.Status = domain.StatusStarting
		err = repo.Update(ctx, &staleInst)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "conflict")
	})

	t.Run("Delete", func(t *testing.T) {
		inst, err := repo.GetByName(ctx, "integration-test-inst")
		require.NoError(t, err)

		err = repo.Delete(ctx, inst.ID)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, inst.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}
