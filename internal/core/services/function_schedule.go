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
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/robfig/cron/v3"
)

// FunctionScheduleService manages scheduled function invocations.
type FunctionScheduleService struct {
	repo     ports.FunctionScheduleRepository
	fnRepo   ports.FunctionRepository
	rbacSvc  ports.RBACService
	eventSvc ports.EventService
	auditSvc ports.AuditService
	logger   *slog.Logger
	parser   cron.Parser
}

// NewFunctionScheduleService constructs a FunctionScheduleService with its dependencies.
func NewFunctionScheduleService(
	repo ports.FunctionScheduleRepository,
	fnRepo ports.FunctionRepository,
	rbacSvc ports.RBACService,
	eventSvc ports.EventService,
	auditSvc ports.AuditService,
	logger *slog.Logger,
) *FunctionScheduleService {
	return &FunctionScheduleService{
		repo:     repo,
		fnRepo:   fnRepo,
		rbacSvc:  rbacSvc,
		eventSvc: eventSvc,
		auditSvc: auditSvc,
		logger:   logger,
		parser:   cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
	}
}

func (s *FunctionScheduleService) CreateSchedule(ctx context.Context, functionID uuid.UUID, name, schedule string, payload []byte) (*domain.FunctionSchedule, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFunctionScheduleCreate, "*"); err != nil {
		return nil, err
	}

	// Verify function exists and user has access
	fn, err := s.fnRepo.GetByID(ctx, functionID)
	if err != nil {
		return nil, errors.New(errors.NotFound, fmt.Sprintf("function not found: %s", functionID))
	}
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFunctionRead, fn.ID.String()); err != nil {
		return nil, err
	}

	// Validate cron expression
	sched, err := s.parser.Parse(schedule)
	if err != nil {
		return nil, errors.New(errors.InvalidInput, fmt.Sprintf("invalid cron schedule: %s", schedule))
	}
	nextRun := sched.Next(time.Now())

	schedObj := &domain.FunctionSchedule{
		ID:         uuid.New(),
		UserID:     userID,
		TenantID:   tenantID,
		FunctionID: functionID,
		Name:       name,
		Schedule:   schedule,
		Payload:    payload,
		Status:     domain.FunctionScheduleStatusActive,
		NextRunAt:  &nextRun,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now(),
	}

	if err := s.repo.Create(ctx, schedObj); err != nil {
		return nil, err
	}

	if err := s.eventSvc.RecordEvent(ctx, "FUNCTION_SCHEDULE_CREATED", schedObj.ID.String(), "FUNCTION_SCHEDULE", nil); err != nil {
		s.logger.Warn("failed to record event", "action", "FUNCTION_SCHEDULE_CREATED", "schedule_id", schedObj.ID, "error", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "function_schedule.create", "function_schedule", schedObj.ID.String(), map[string]interface{}{
		"name":        name,
		"function_id": functionID.String(),
		"schedule":    schedule,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	s.logger.Info("function schedule created", "name", name, "function_id", functionID, "schedule", schedule)

	return schedObj, nil
}

func (s *FunctionScheduleService) ListSchedules(ctx context.Context) ([]*domain.FunctionSchedule, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFunctionScheduleRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.List(ctx, userID, tenantID)
}

func (s *FunctionScheduleService) GetSchedule(ctx context.Context, id uuid.UUID) (*domain.FunctionSchedule, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFunctionScheduleRead, id.String()); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, id, userID, tenantID)
}

func (s *FunctionScheduleService) DeleteSchedule(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFunctionScheduleDelete, id.String()); err != nil {
		return err
	}

	_, err := s.repo.GetByID(ctx, id, userID, tenantID)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	if err := s.eventSvc.RecordEvent(ctx, "FUNCTION_SCHEDULE_DELETED", id.String(), "FUNCTION_SCHEDULE", nil); err != nil {
		s.logger.Warn("failed to record event", "action", "FUNCTION_SCHEDULE_DELETED", "schedule_id", id, "error", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "function_schedule.delete", "function_schedule", id.String(), map[string]interface{}{}); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	return nil
}

func (s *FunctionScheduleService) PauseSchedule(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFunctionScheduleUpdate, id.String()); err != nil {
		return err
	}

	sched, err := s.repo.GetByID(ctx, id, userID, tenantID)
	if err != nil {
		return err
	}

	sched.Status = domain.FunctionScheduleStatusPaused
	sched.NextRunAt = nil
	return s.repo.Update(ctx, sched)
}

func (s *FunctionScheduleService) ResumeSchedule(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFunctionScheduleUpdate, id.String()); err != nil {
		return err
	}

	sched, err := s.repo.GetByID(ctx, id, userID, tenantID)
	if err != nil {
		return err
	}

	schedNext, err := s.parser.Parse(sched.Schedule)
	if err != nil {
		return errors.New(errors.InvalidInput, fmt.Sprintf("invalid schedule for function schedule %s: %s", id, err))
	}
	nextRun := schedNext.Next(time.Now())

	sched.Status = domain.FunctionScheduleStatusActive
	sched.NextRunAt = &nextRun
	return s.repo.Update(ctx, sched)
}

func (s *FunctionScheduleService) GetScheduleRuns(ctx context.Context, id uuid.UUID, limit int) ([]*domain.FunctionScheduleRun, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFunctionScheduleRead, id.String()); err != nil {
		return nil, err
	}

	_, err := s.repo.GetByID(ctx, id, userID, tenantID)
	if err != nil {
		return nil, err
	}

	// Validate and cap limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	return s.repo.GetScheduleRuns(ctx, id, limit)
}
