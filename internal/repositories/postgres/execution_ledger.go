// Package postgres provides PostgreSQL-backed repository implementations.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// PgExecutionLedger implements ports.ExecutionLedger using the job_executions table.
type PgExecutionLedger struct {
	db DB
}

// NewExecutionLedger creates a new Postgres-backed execution ledger.
func NewExecutionLedger(db DB) *PgExecutionLedger {
	return &PgExecutionLedger{db: db}
}

// TryAcquire attempts to claim a job execution. It uses INSERT ... ON CONFLICT
// to atomically check whether the job was already processed:
//
//   - If no row exists, inserts status='running' and returns true.
//   - If a 'completed' row exists, returns false (already done).
//   - If a 'running' row exists and is newer than staleThreshold, returns false
//     (another worker is actively processing).
//   - If a 'running' row exists but is older than staleThreshold, reclaims it
//     by updating started_at and returns true (previous worker likely crashed).
//   - If a 'failed' row exists, reclaims it (allows retry).
func (l *PgExecutionLedger) TryAcquire(ctx context.Context, jobKey string, staleThreshold time.Duration) (bool, error) {
	// Step 1: Try to insert a new row.
	var inserted bool
	err := l.db.QueryRow(ctx, `
		INSERT INTO job_executions (job_key, status, started_at)
		VALUES ($1, 'running', NOW())
		ON CONFLICT (job_key) DO NOTHING
		RETURNING TRUE
	`, jobKey).Scan(&inserted)

	if err == nil && inserted {
		return true, nil // Successfully claimed a brand-new execution.
	}
	// pgx returns ErrNoRows when INSERT ... ON CONFLICT DO NOTHING matches zero rows
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("execution ledger insert %s: %w", jobKey, err)
	}

	// Row already exists. Check its status.
	var status string
	var startedAt time.Time
	err = l.db.QueryRow(ctx, `
		SELECT status, started_at FROM job_executions WHERE job_key = $1
	`, jobKey).Scan(&status, &startedAt)
	if err != nil {
		return false, fmt.Errorf("execution ledger check %s: %w", jobKey, err)
	}

	switch status {
	case "completed":
		// Already done — skip.
		return false, nil
	case "running":
		// Check if the running entry is stale (crashed worker).
		if time.Since(startedAt) < staleThreshold {
			return false, nil // Another worker is still processing.
		}
		// Reclaim the stale entry. Use optimistic locking on started_at to
		// avoid racing with another reclaimer.
		tag, err := l.db.Exec(ctx, `
			UPDATE job_executions
			SET started_at = NOW(), status = 'running'
			WHERE job_key = $1 AND status = 'running' AND started_at = $2
		`, jobKey, startedAt)
		if err != nil {
			return false, fmt.Errorf("execution ledger reclaim %s: %w", jobKey, err)
		}
		return tag.RowsAffected() > 0, nil
	case "failed":
		// Retry a previously failed job.
		tag, err := l.db.Exec(ctx, `
			UPDATE job_executions
			SET started_at = NOW(), status = 'running', completed_at = NULL, result = NULL
			WHERE job_key = $1 AND status = 'failed'
		`, jobKey)
		if err != nil {
			return false, fmt.Errorf("execution ledger retry %s: %w", jobKey, err)
		}
		return tag.RowsAffected() > 0, nil
	default:
		return false, fmt.Errorf("execution ledger unknown status %q for %s", status, jobKey)
	}
}

// MarkComplete marks a job as successfully completed.
func (l *PgExecutionLedger) MarkComplete(ctx context.Context, jobKey string, result string) error {
	tag, err := l.db.Exec(ctx, `
		UPDATE job_executions
		SET status = 'completed', completed_at = NOW(), result = $2
		WHERE job_key = $1 AND status = 'running'
	`, jobKey, result)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("execution ledger mark complete %s: no running row updated", jobKey)
	}
	return nil
}

// MarkFailed marks a job as failed, allowing future retries.
func (l *PgExecutionLedger) MarkFailed(ctx context.Context, jobKey string, reason string) error {
	tag, err := l.db.Exec(ctx, `
		UPDATE job_executions
		SET status = 'failed', completed_at = NOW(), result = $2
		WHERE job_key = $1 AND status = 'running'
	`, jobKey, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("execution ledger mark failed %s: no running row updated", jobKey)
	}
	return nil
}

// GetStatus returns the current status, result and start time of a job.
func (l *PgExecutionLedger) GetStatus(ctx context.Context, jobKey string) (status string, result string, startedAt time.Time, err error) {
	res := l.db.QueryRow(ctx, `
		SELECT status, COALESCE(result, ''), started_at FROM job_executions WHERE job_key = $1
	`, jobKey)

	err = res.Scan(&status, &result, &startedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", time.Time{}, nil
		}
		return "", "", time.Time{}, fmt.Errorf("execution ledger get status %s: %w", jobKey, err)
	}
	return status, result, startedAt, nil
}
