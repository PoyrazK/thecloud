// Package services implements core business workflows.
package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/robfig/cron/v3"
)

// CronService manages scheduled jobs and persistence.
type CronService struct {
	repo     ports.CronRepository
	rbacSvc  ports.RBACService
	eventSvc ports.EventService
	auditSvc ports.AuditService
	logger   *slog.Logger
	parser   cron.Parser
}

// NewCronService constructs a CronService with its dependencies.
func NewCronService(repo ports.CronRepository, rbacSvc ports.RBACService, eventSvc ports.EventService, auditSvc ports.AuditService, logger *slog.Logger) ports.CronService {
	return &CronService{
		repo:     repo,
		rbacSvc:  rbacSvc,
		eventSvc: eventSvc,
		auditSvc: auditSvc,
		logger:   logger,
		parser:   cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor),
	}
}

func (s *CronService) CreateJob(ctx context.Context, name, schedule, targetURL, targetMethod, targetPayload string) (*domain.CronJob, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionCronCreate, "*"); err != nil {
		return nil, err
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

	if err := s.eventSvc.RecordEvent(ctx, "CRON_JOB_CREATED", job.ID.String(), "CRON_JOB", nil); err != nil {
		s.logger.Warn("failed to record event", "action", "CRON_JOB_CREATED", "job_id", job.ID, "error", err)
	}

	if err := s.auditSvc.Log(ctx, job.UserID, "cron.job_create", "cron_job", job.ID.String(), map[string]interface{}{
		"name":     job.Name,
		"schedule": job.Schedule,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "cron.job_create", "job_id", job.ID, "error", err)
	}

	return job, nil
}

func (s *CronService) ListJobs(ctx context.Context) ([]*domain.CronJob, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionCronRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.ListJobs(ctx, userID)
}

func (s *CronService) GetJob(ctx context.Context, id uuid.UUID) (*domain.CronJob, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionCronRead, id.String()); err != nil {
		return nil, err
	}

	return s.repo.GetJobByID(ctx, id, userID)
}

func (s *CronService) GetJobRuns(ctx context.Context, id uuid.UUID, limit int) ([]*domain.CronJobRun, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionCronRead, id.String()); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 50
	}

	return s.repo.GetJobRuns(ctx, id, limit)
}

func (s *CronService) PauseJob(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionCronUpdate, id.String()); err != nil {
		return err
	}

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
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionCronUpdate, id.String()); err != nil {
		return err
	}

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
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionCronDelete, id.String()); err != nil {
		return err
	}

	_, err := s.repo.GetJobByID(ctx, id, userID)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteJob(ctx, id); err != nil {
		return err
	}

	if err := s.auditSvc.Log(ctx, userID, "cron.job_delete", "cron_job", id.String(), map[string]interface{}{}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "cron.job_delete", "job_id", id, "error", err)
	}

	return nil
}

func (s *CronService) UpdateJob(ctx context.Context, id uuid.UUID, name, schedule, targetURL, targetMethod, targetPayload string) (*domain.CronJob, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionCronUpdate, id.String()); err != nil {
		return nil, err
	}

	job, err := s.repo.GetJobByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	if name != "" {
		job.Name = name
	}
	if schedule != "" {
		sched, err := s.parser.Parse(schedule)
		if err != nil {
			return nil, fmt.Errorf("invalid cron schedule: %w", err)
		}
		job.Schedule = schedule
		job.NextRunAt = pointerTime(sched.Next(time.Now()))
	}
	if targetURL != "" {
		job.TargetURL = targetURL
	}
	if targetMethod != "" {
		job.TargetMethod = targetMethod
	}
	if targetPayload != "" {
		job.TargetPayload = targetPayload
	}

	if err := s.repo.UpdateJob(ctx, job); err != nil {
		return nil, err
	}

	if err := s.auditSvc.Log(ctx, job.UserID, "cron.job_update", "cron_job", job.ID.String(), map[string]interface{}{
		"name": job.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "cron.job_update", "job_id", job.ID, "error", err)
	}

	return job, nil
}

func pointerTime(t time.Time) *time.Time {
	return &t
}
