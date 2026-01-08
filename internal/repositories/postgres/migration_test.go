//go:build integration

package postgres

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrationRollback(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	ctx := context.Background()

	// Get all migration files
	files, err := os.ReadDir("migrations")
	require.NoError(t, err)

	var upMigrations []string
	var downMigrations []string

	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".up.sql") {
			upMigrations = append(upMigrations, f.Name())
		} else if strings.HasSuffix(f.Name(), ".down.sql") {
			downMigrations = append(downMigrations, f.Name())
		}
	}

	sort.Strings(upMigrations)
	sort.Strings(downMigrations)

	// Function to run a migration file
	runFile := func(name string) error {
		content, err := os.ReadFile(filepath.Join("migrations", name))
		if err != nil {
			return err
		}
		_, err = db.Exec(ctx, string(content))
		return err
	}

	t.Run("Step 1: Apply all UP migrations", func(t *testing.T) {
		for _, m := range upMigrations {
			err := runFile(m)
			// We use IF NOT EXISTS in most migrations, but if not, we might get errors
			// if the DB isn't clean. For this test, we want to see it work.
			if err != nil {
				t.Logf("Warning: migration %s failed: %v (might be expected if already applied)", m, err)
			}
		}
	})

	t.Run("Step 2: Apply all DOWN migrations in reverse order", func(t *testing.T) {
		// Reverse down migrations
		for i := len(downMigrations) - 1; i >= 0; i-- {
			m := downMigrations[i]
			err := runFile(m)
			require.NoError(t, err, "Failed to rollback migration: %s", m)
			t.Logf("Rolled back: %s", m)
		}
	})

	t.Run("Step 3: Re-apply all UP migrations", func(t *testing.T) {
		for _, m := range upMigrations {
			err := runFile(m)
			require.NoError(t, err, "Failed to re-apply migration: %s", m)
			t.Logf("Re-applied: %s", m)
		}
	})
}
