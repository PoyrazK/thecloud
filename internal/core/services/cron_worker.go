// Package services implements core business workflows.
package services

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/robfig/cron/v3"
)

const (
	ClaimTimeout = 5 * time.Minute

	// perJobTimeout bounds how long a single cron job's HTTP call may run,
	// independent of the http.Client's idle/read deadlines. Without it a
	// slow target endpoint could pin a worker goroutine until the surrounding
	// process context is cancelled.
	perJobTimeout = 30 * time.Second
)

// CronWorker executes scheduled jobs and records run results.
type CronWorker struct {
	repo   ports.CronRepository
	parser cron.Parser
	client *http.Client
}

// NewCronWorker constructs a CronWorker with default scheduling configuration.
func NewCronWorker(repo ports.CronRepository) *CronWorker {
	return &CronWorker{
		repo:   repo,
		parser: cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (w *CronWorker) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	tickCounter := 0

	log.Println("CloudCron Worker started")

	for {
		select {
		case <-ctx.Done():
			log.Println("CloudCron Worker stopping")
			return
		case <-ticker.C:
			w.ProcessJobs(ctx)
			tickCounter++
			if tickCounter%6 == 0 { // every 60 seconds
				w.reapStaleClaims(ctx)
			}
		}
	}
}

func (w *CronWorker) ProcessJobs(ctx context.Context) {
	jobs, err := w.repo.ClaimNextJobsToRun(ctx, ClaimTimeout)
	if err != nil {
		log.Printf("CronWorker: failed to claim jobs: %v", err)
		return
	}

	for _, job := range jobs {
		w.runJob(ctx, job)
	}
}

func (w *CronWorker) runJob(ctx context.Context, job *domain.CronJob) {
	start := time.Now()

	jobCtx, cancel := context.WithTimeout(ctx, perJobTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(jobCtx, job.TargetMethod, job.TargetURL, bytes.NewBufferString(job.TargetPayload))
	if err != nil {
		w.completeRun(ctx, job, "FAILED", 0, err.Error(), time.Since(start))
		return
	}

	resp, err := w.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		w.completeRun(ctx, job, "FAILED", 0, err.Error(), duration)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	status := "SUCCESS"
	if resp.StatusCode >= 400 {
		status = "FAILED"
	}

	w.completeRun(ctx, job, status, resp.StatusCode, resp.Status, duration)
}

func (w *CronWorker) completeRun(ctx context.Context, job *domain.CronJob, status string, code int, response string, duration time.Duration) {
	run := &domain.CronJobRun{
		ID:         uuid.New(),
		JobID:      job.ID,
		Status:     status,
		StatusCode: code,
		Response:   response,
		DurationMs: duration.Milliseconds(),
		StartedAt:  time.Now().Add(-duration),
	}

	now := time.Now()
	nextRun := now.Add(24 * time.Hour)

	sched, err := w.parser.Parse(job.Schedule)
	if err != nil {
		log.Printf("CronWorker: invalid schedule for job %s: %q: %v; using fallback next run at %s", job.ID, job.Schedule, err, nextRun.Format(time.RFC3339))
	} else {
		nextRun = sched.Next(now)
	}

	if err := w.repo.CompleteJobRun(ctx, run, job, nextRun); err != nil {
		log.Printf("CronWorker: failed to complete job run: %v", err)
	}
}

func (w *CronWorker) reapStaleClaims(ctx context.Context) {
	count, err := w.repo.ReapStaleClaims(ctx)
	if err != nil {
		log.Printf("CronWorker: failed to reap stale claims: %v", err)
	} else if count > 0 {
		log.Printf("CronWorker: reclaimed %d stale claims", count)
	}
}
