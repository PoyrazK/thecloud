package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/robfig/cron/v3"
)

type CronService struct {
	repo     ports.CronRepository
	eventSvc ports.EventService
	auditSvc ports.AuditService
	parser   cron.Parser
}

func NewCronService(repo ports.CronRepository, eventSvc ports.EventService, auditSvc ports.AuditService) ports.CronService {
	return &CronService{
		repo:     repo,
		eventSvc: eventSvc,
		auditSvc: auditSvc,
		parser:   cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor),
	}
}

func (s *CronService) CreateJob(ctx context.Context, name, schedule, targetURL, targetMethod, targetPayload string) (*domain.CronJob, error) {
	userID := appcontext.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, fmt.Errorf("unauthorized")
	}

	// Validate schedule
	sched, err := s.parser.Parse(schedule)
	if err != nil {
		return nil, fmt.Errorf("invalid cron schedule: %w", err)
	}

	nextRun := sched.Next(time.Now())

	job := &domain.CronJob{
		ID:            uuid.New(),
		UserID:        userID,
		Name:          name,
		Schedule:      schedule,
		TargetURL:     targetURL,
		TargetMethod:  targetMethod,
		TargetPayload: targetPayload,
		Status:        domain.CronStatusActive,
		NextRunAt:     &nextRun,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.repo.CreateJob(ctx, job); err != nil {
		return nil, err
	}

	_ = s.eventSvc.RecordEvent(ctx, "CRON_JOB_CREATED", job.ID.String(), "CRON_JOB", nil)

	_ = s.auditSvc.Log(ctx, job.UserID, "cron.job_create", "cron_job", job.ID.String(), map[string]interface{}{
		"name":     job.Name,
		"schedule": job.Schedule,
	})

	return job, nil
}

func (s *CronService) ListJobs(ctx context.Context) ([]*domain.CronJob, error) {
	userID := appcontext.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, fmt.Errorf("unauthorized")
	}
	return s.repo.ListJobs(ctx, userID)
}

func (s *CronService) GetJob(ctx context.Context, id uuid.UUID) (*domain.CronJob, error) {
	userID := appcontext.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, fmt.Errorf("unauthorized")
	}
	return s.repo.GetJobByID(ctx, id, userID)
}

func (s *CronService) PauseJob(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	job, err := s.repo.GetJobByID(ctx, id, userID)
	if err != nil {
		return err
	}
	job.Status = domain.CronStatusPaused
	job.NextRunAt = nil
	return s.repo.UpdateJob(ctx, job)
}

func (s *CronService) ResumeJob(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	job, err := s.repo.GetJobByID(ctx, id, userID)
	if err != nil {
		return err
	}

	sched, err := s.parser.Parse(job.Schedule)
	if err != nil {
		return fmt.Errorf("invalid schedule in job: %w", err)
	}
	nextRun := sched.Next(time.Now())

	job.Status = domain.CronStatusActive
	job.NextRunAt = &nextRun
	return s.repo.UpdateJob(ctx, job)
}

func (s *CronService) DeleteJob(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	_, err := s.repo.GetJobByID(ctx, id, userID)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteJob(ctx, id); err != nil {
		return err
	}

	_ = s.auditSvc.Log(ctx, userID, "cron.job_delete", "cron_job", id.String(), map[string]interface{}{})

	return nil
}
