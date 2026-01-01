//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/poyraz/cloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDB(t *testing.T) *pgxpool.Pool {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://cloud:cloud@localhost:5433/miniaws"
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)

	err = db.Ping(ctx)
	if err != nil {
		t.Skip("Skipping integration test: database not available")
	}

	return db
}

func TestInstanceRepository_Integration(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	repo := NewInstanceRepository(db)
	ctx := context.Background()

	// Cleanup
	_, err := db.Exec(ctx, "DELETE FROM instances")
	require.NoError(t, err)

	t.Run("Create and Get", func(t *testing.T) {
		id := uuid.New()
		inst := &domain.Instance{
			ID:        id,
			Name:      "integration-test-inst",
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
		list, err := repo.List(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, list)
	})
}
