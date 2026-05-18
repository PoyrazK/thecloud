// Package postgres provides PostgreSQL-backed repository implementations.
package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

// PostgresCronRepository provides PostgreSQL-backed cron job persistence.
type PostgresCronRepository struct {
	db DB
}

// NewPostgresCronRepository creates a cron repository using the provided DB.
func NewPostgresCronRepository(db DB) ports.CronRepository {
	return &PostgresCronRepository{db: db}
}

func (r *PostgresCronRepository) CreateJob(ctx context.Context, job *domain.CronJob) error {
	query := `
		INSERT INTO cron_jobs (id, user_id, name, schedule, target_url, target_method, target_payload, status, next_run_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.Exec(ctx, query,
		job.ID,
		job.UserID,
		job.Name,
		job.Schedule,
		job.TargetURL,
		job.TargetMethod,
		job.TargetPayload,
		job.Status,
		job.NextRunAt,
		job.CreatedAt,
		job.UpdatedAt,
	)
	return err
}

func (r *PostgresCronRepository) GetJobByID(ctx context.Context, id, userID uuid.UUID) (*domain.CronJob, error) {
	query := `SELECT id, user_id, name, schedule, target_url, target_method, target_payload, status, last_run_at, next_run_at, created_at, updated_at FROM cron_jobs WHERE id = $1 AND user_id = $2`
	return r.scanCronJob(r.db.QueryRow(ctx, query, id, userID))
}

func (r *PostgresCronRepository) ListJobs(ctx context.Context, userID uuid.UUID) ([]*domain.CronJob, error) {
	query := `SELECT id, user_id, name, schedule, target_url, target_method, target_payload, status, last_run_at, next_run_at, created_at, updated_at FROM cron_jobs WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	return r.scanCronJobs(rows)
}

func (r *PostgresCronRepository) UpdateJob(ctx context.Context, job *domain.CronJob) error {
	query := `
		UPDATE cron_jobs
		SET name = $1, schedule = $2, target_url = $3, target_method = $4, target_payload = $5,
		    status = $6, last_run_at = $7, next_run_at = $8, updated_at = NOW()
		WHERE id = $9
	`
	_, err := r.db.Exec(ctx, query, job.Name, job.Schedule, job.TargetURL, job.TargetMethod, job.TargetPayload,
		job.Status, job.LastRunAt, job.NextRunAt, job.ID)
	return err
}

func (r *PostgresCronRepository) DeleteJob(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM cron_jobs WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

func (r *PostgresCronRepository) GetJobRuns(ctx context.Context, jobID uuid.UUID, limit int) ([]*domain.CronJobRun, error) {
	query := `SELECT id, job_id, status, status_code, response, duration_ms, started_at FROM cron_job_runs WHERE job_id = $1 ORDER BY started_at DESC LIMIT $2`
	rows, err := r.db.Query(ctx, query, jobID, limit)
	if err != nil {
		return nil, err
	}
	return r.scanCronJobRuns(rows)
}

func (r *PostgresCronRepository) ClaimNextJobsToRun(ctx context.Context, lockTimeout time.Duration) ([]*domain.CronJob, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now()
	claimExpiry := now.Add(lockTimeout)
	farFuture := now.Add(365 * 24 * time.Hour)

	selectQuery := `
		SELECT id, user_id, name, schedule, target_url, target_method, target_payload,
		       status, last_run_at, next_run_at, tenant_id, created_at, updated_at
		FROM cron_jobs
		WHERE status = 'ACTIVE'
		  AND next_run_at <= NOW()
		  AND (claimed_until IS NULL OR claimed_until <= NOW())
		ORDER BY next_run_at ASC
		LIMIT 10
		FOR UPDATE SKIP LOCKED
	`
	rows, err := tx.Query(ctx, selectQuery)
	if err != nil {
		return nil, err
	}

	jobs, err := r.scanCronJobsWithTenant(rows)
	if err != nil {
		return nil, err
	}

	if len(jobs) == 0 {
		return nil, tx.Commit(ctx)
	}

	ids := make([]uuid.UUID, len(jobs))
	for i, job := range jobs {
		ids[i] = job.ID
	}

	updateQuery := `
		UPDATE cron_jobs
		SET next_run_at = $1, claimed_until = $2, updated_at = NOW()
		WHERE id = ANY($3)
	`
	_, err = tx.Exec(ctx, updateQuery, farFuture, claimExpiry, ids)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	for _, job := range jobs {
		job.NextRunAt = &farFuture
		job.ClaimedUntil = &claimExpiry
	}
	return jobs, nil
}

func (r *PostgresCronRepository) CompleteJobRun(ctx context.Context, run *domain.CronJobRun, job *domain.CronJob, nextRunAt time.Time) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	insertQuery := `
		INSERT INTO cron_job_runs (id, job_id, status, status_code, response, duration_ms, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = tx.Exec(ctx, insertQuery, run.ID, run.JobID, run.Status, run.StatusCode, run.Response, run.DurationMs, run.StartedAt)
	if err != nil {
		return err
	}

	updateQuery := `
		UPDATE cron_jobs
		SET last_run_at = $1, next_run_at = $2, claimed_until = NULL, updated_at = $3
		WHERE id = $4
	`
	_, err = tx.Exec(ctx, updateQuery, run.StartedAt, nextRunAt, time.Now(), job.ID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *PostgresCronRepository) ReapStaleClaims(ctx context.Context) (int, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
		UPDATE cron_jobs
		SET claimed_until = NULL,
		    next_run_at = NOW(),
		    updated_at = NOW()
		WHERE claimed_until IS NOT NULL
		  AND claimed_until < NOW()
		  AND next_run_at > NOW() + INTERVAL '1 day'
		RETURNING id
	`
	rows, err := tx.Query(ctx, query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return count, tx.Commit(ctx)
}

func (r *PostgresCronRepository) scanCronJob(row pgx.Row) (*domain.CronJob, error) {
	var job domain.CronJob
	var status string
	err := row.Scan(
		&job.ID,
		&job.UserID,
		&job.Name,
		&job.Schedule,
		&job.TargetURL,
		&job.TargetMethod,
		&job.TargetPayload,
		&status,
		&job.LastRunAt,
		&job.NextRunAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	job.Status = domain.CronStatus(status)
	return &job, nil
}

func (r *PostgresCronRepository) scanCronJobWithTenant(row pgx.Row) (*domain.CronJob, error) {
	var job domain.CronJob
	var status string
	err := row.Scan(
		&job.ID,
		&job.UserID,
		&job.Name,
		&job.Schedule,
		&job.TargetURL,
		&job.TargetMethod,
		&job.TargetPayload,
		&status,
		&job.LastRunAt,
		&job.NextRunAt,
		&job.TenantID,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	job.Status = domain.CronStatus(status)
	return &job, nil
}

func (r *PostgresCronRepository) scanCronJobs(rows pgx.Rows) ([]*domain.CronJob, error) {
	defer rows.Close()
	var jobs []*domain.CronJob
	for rows.Next() {
		job, err := r.scanCronJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (r *PostgresCronRepository) scanCronJobsWithTenant(rows pgx.Rows) ([]*domain.CronJob, error) {
	defer rows.Close()
	var jobs []*domain.CronJob
	for rows.Next() {
		job, err := r.scanCronJobWithTenant(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (r *PostgresCronRepository) scanCronJobRuns(rows pgx.Rows) ([]*domain.CronJobRun, error) {
	defer rows.Close()
	var runs []*domain.CronJobRun
	for rows.Next() {
		var run domain.CronJobRun
		if err := rows.Scan(&run.ID, &run.JobID, &run.Status, &run.StatusCode, &run.Response, &run.DurationMs, &run.StartedAt); err != nil {
			return nil, err
		}
		runs = append(runs, &run)
	}
	return runs, rows.Err()
}

func (r *PostgresCronRepository) SaveJobRun(ctx context.Context, run *domain.CronJobRun) error {
	query := `INSERT INTO cron_job_runs (id, job_id, status, status_code, response, duration_ms, started_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.db.Exec(ctx, query, run.ID, run.JobID, run.Status, run.StatusCode, run.Response, run.DurationMs, run.StartedAt)
	return err
}
