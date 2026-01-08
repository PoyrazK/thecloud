package postgres

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.up.sql
var migrationsFS embed.FS

// RunMigrations applies all embedded up migrations.
// In a real production system, this should track applied migrations in a table.
// For The Cloud, we'll use IF NOT EXISTS in SQL to make them idempotent-ish,
// or just run them and ignore "already exists" errors for simplicity in this MVP.
func RunMigrations(ctx context.Context, db *pgxpool.Pool) error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations: %w", err)
	}

	// Sort by validation (filename)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", entry.Name(), err)
		}

		// Execute migration
		// We ignore errors here assuming idempotency or manual intervention for MVP
		// A better approach would be checking a schema_migrations table
		_, err = db.Exec(ctx, string(content))
		if err != nil {
			// Log but don't fail, as tables might already exist
			// Ideally we should check specific error codes
			fmt.Printf("Migration %s result: %v\n", entry.Name(), err)
		} else {
			fmt.Printf("Applied migration: %s\n", entry.Name())
		}
	}

	return nil
}
