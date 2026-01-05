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

type CronWorker struct {
	repo   ports.CronRepository
	parser cron.Parser
	client *http.Client
}

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

	log.Println("CloudCron Worker started")

	for {
		select {
		case <-ctx.Done():
			log.Println("CloudCron Worker stopping")
			return
		case <-ticker.C:
			w.ProcessJobs(ctx)
		}
	}
}

func (w *CronWorker) ProcessJobs(ctx context.Context) {
	jobs, err := w.repo.GetNextJobsToRun(ctx)
	if err != nil {
		log.Printf("CronWorker: failed to fetch jobs: %v", err)
		return
	}

	for _, job := range jobs {
		go w.runJob(context.Background(), job)
	}
}

func (w *CronWorker) runJob(ctx context.Context, job *domain.CronJob) {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, job.TargetMethod, job.TargetURL, bytes.NewBufferString(job.TargetPayload))
	if err != nil {
		w.recordRun(ctx, job, "FAILED", 0, err.Error(), time.Since(start))
		return
	}

	resp, err := w.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		w.recordRun(ctx, job, "FAILED", 0, err.Error(), duration)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	status := "SUCCESS"
	if resp.StatusCode >= 400 {
		status = "FAILED"
	}

	w.recordRun(ctx, job, status, resp.StatusCode, resp.Status, duration)
}

func (w *CronWorker) recordRun(ctx context.Context, job *domain.CronJob, status string, code int, response string, duration time.Duration) {
	run := &domain.CronJobRun{
		ID:         uuid.New(),
		JobID:      job.ID,
		Status:     status,
		StatusCode: code,
		Response:   response,
		DurationMs: duration.Milliseconds(),
		StartedAt:  time.Now().Add(-duration),
	}

	if err := w.repo.SaveJobRun(ctx, run); err != nil {
		log.Printf("CronWorker: failed to save job run: %v", err)
	}

	// Update job state
	sched, _ := w.parser.Parse(job.Schedule)
	now := time.Now()
	nextRun := sched.Next(now)

	job.LastRunAt = &now
	job.NextRunAt = &nextRun

	if err := w.repo.UpdateJob(ctx, job); err != nil {
		log.Printf("CronWorker: failed to update job state: %v", err)
	}
}
