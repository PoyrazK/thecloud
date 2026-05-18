package services_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFunctionScheduleServiceUnit(t *testing.T) {
	repo := new(MockFunctionScheduleRepo)
	fnRepo := new(MockFunctionRepo)
	eventSvc := new(MockEventService)
	auditSvc := new(MockAuditService)
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil)

	svc := services.NewFunctionScheduleService(repo, fnRepo, rbacSvc, eventSvc, auditSvc, slog.Default())

	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	ctx = appcontext.WithTenantID(ctx, tenantID)
	fnID := uuid.New()

	t.Run("CreateSchedule", func(t *testing.T) {
		fn := &domain.Function{ID: fnID, Name: "test-fn", UserID: userID}
		fnRepo.On("GetByID", mock.Anything, fnID).Return(fn, nil).Once()
		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		eventSvc.On("RecordEvent", mock.Anything, "FUNCTION_SCHEDULE_CREATED", mock.Anything, "FUNCTION_SCHEDULE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "function_schedule.create", "function_schedule", mock.Anything, mock.Anything).Return(nil).Once()

		sched, err := svc.CreateSchedule(ctx, fnID, "nightly", "0 2 * * *", []byte(`{}`))
		require.NoError(t, err)
		assert.NotNil(t, sched)
		assert.Equal(t, "nightly", sched.Name)
		assert.Equal(t, "0 2 * * *", sched.Schedule)
		assert.Equal(t, domain.FunctionScheduleStatusActive, sched.Status)
		repo.AssertExpectations(t)
	})

	t.Run("CreateSchedule_InvalidCron", func(t *testing.T) {
		fn := &domain.Function{ID: fnID, Name: "test-fn", UserID: userID}
		fnRepo.On("GetByID", mock.Anything, fnID).Return(fn, nil).Once()
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionFunctionScheduleCreate, "*").Return(nil).Once()
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionFunctionRead, fn.ID.String()).Return(nil).Once()

		_, err := svc.CreateSchedule(ctx, fnID, "bad", "invalid cron", []byte{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid cron schedule")
	})

	t.Run("CreateSchedule_FunctionNotFound", func(t *testing.T) {
		fnRepo.On("GetByID", mock.Anything, fnID).Return(nil, fmt.Errorf("not found")).Once()
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionFunctionScheduleCreate, "*").Return(nil).Once()

		_, err := svc.CreateSchedule(ctx, fnID, "nightly", "0 2 * * *", []byte{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "function not found")
	})

	t.Run("ListSchedules", func(t *testing.T) {
		expected := []*domain.FunctionSchedule{{ID: uuid.New(), Name: "sched1"}}
		repo.On("List", mock.Anything, userID, tenantID).Return(expected, nil).Once()

		schedules, err := svc.ListSchedules(ctx)
		require.NoError(t, err)
		assert.Len(t, schedules, 1)
		assert.Equal(t, "sched1", schedules[0].Name)
	})

	t.Run("GetSchedule", func(t *testing.T) {
		schedID := uuid.New()
		expected := &domain.FunctionSchedule{ID: schedID, Name: "sched1"}
		repo.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(expected, nil).Once()

		sched, err := svc.GetSchedule(ctx, schedID)
		require.NoError(t, err)
		assert.Equal(t, schedID, sched.ID)
	})

	t.Run("DeleteSchedule", func(t *testing.T) {
		schedID := uuid.New()
		sched := &domain.FunctionSchedule{ID: schedID, UserID: userID}
		repo.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(sched, nil).Once()
		repo.On("Delete", mock.Anything, schedID).Return(nil).Once()
		eventSvc.On("RecordEvent", mock.Anything, "FUNCTION_SCHEDULE_DELETED", schedID.String(), "FUNCTION_SCHEDULE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "function_schedule.delete", "function_schedule", schedID.String(), mock.Anything).Return(nil).Once()

		err := svc.DeleteSchedule(ctx, schedID)
		require.NoError(t, err)
	})

	t.Run("PauseSchedule", func(t *testing.T) {
		schedID := uuid.New()
		sched := &domain.FunctionSchedule{ID: schedID, UserID: userID, Status: domain.FunctionScheduleStatusActive}
		repo.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(sched, nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(s *domain.FunctionSchedule) bool {
			return s.Status == domain.FunctionScheduleStatusPaused && s.NextRunAt == nil
		})).Return(nil).Once()

		err := svc.PauseSchedule(ctx, schedID)
		require.NoError(t, err)
	})

	t.Run("ResumeSchedule", func(t *testing.T) {
		schedID := uuid.New()
		sched := &domain.FunctionSchedule{ID: schedID, UserID: userID, Status: domain.FunctionScheduleStatusPaused, Schedule: "*/5 * * * *"}
		repo.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(sched, nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(s *domain.FunctionSchedule) bool {
			return s.Status == domain.FunctionScheduleStatusActive && s.NextRunAt != nil
		})).Return(nil).Once()

		err := svc.ResumeSchedule(ctx, schedID)
		require.NoError(t, err)
	})

	t.Run("GetScheduleRuns", func(t *testing.T) {
		schedID := uuid.New()
		expectedRuns := []*domain.FunctionScheduleRun{{ID: uuid.New(), Status: "SUCCESS"}}
		repo.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(&domain.FunctionSchedule{ID: schedID, UserID: userID}, nil).Once()
		repo.On("GetScheduleRuns", mock.Anything, schedID, 50).Return(expectedRuns, nil).Once()

		runs, err := svc.GetScheduleRuns(ctx, schedID, 50)
		require.NoError(t, err)
		assert.Len(t, runs, 1)
		assert.Equal(t, "SUCCESS", runs[0].Status)
	})

	t.Run("CreateSchedule_RBACDenied", func(t *testing.T) {
		// Create a fresh service with rbacSvc that returns error on second authorize call
		repo2 := new(MockFunctionScheduleRepo)
		fnRepo2 := new(MockFunctionRepo)
		rbacSvc2 := new(MockRBACService)
		eventSvc2 := new(MockEventService)
		auditSvc2 := new(MockAuditService)

		fn := &domain.Function{ID: fnID, Name: "test-fn", UserID: userID}
		fnRepo2.On("GetByID", mock.Anything, fnID).Return(fn, nil).Once()
		// First call passes, second call fails
		rbacSvc2.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionFunctionScheduleCreate, "*").Return(nil).Once()
		rbacSvc2.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionFunctionRead, fn.ID.String()).Return(fmt.Errorf("access denied")).Once()

		svc2 := services.NewFunctionScheduleService(repo2, fnRepo2, rbacSvc2, eventSvc2, auditSvc2, slog.Default())

		_, err := svc2.CreateSchedule(ctx, fnID, "nightly", "0 2 * * *", []byte{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
	})

	t.Run("DeleteSchedule_NotFound", func(t *testing.T) {
		schedID := uuid.New()
		repo.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(nil, fmt.Errorf("not found")).Once()

		err := svc.DeleteSchedule(ctx, schedID)
		require.Error(t, err)
	})

	t.Run("GetScheduleRuns_NotFound", func(t *testing.T) {
		schedID := uuid.New()
		repo.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(nil, fmt.Errorf("not found")).Once()

		_, err := svc.GetScheduleRuns(ctx, schedID, 50)
		require.Error(t, err)
	})

	t.Run("ResumeSchedule_InvalidCron", func(t *testing.T) {
		schedID := uuid.New()
		sched := &domain.FunctionSchedule{ID: schedID, UserID: userID, Status: domain.FunctionScheduleStatusPaused, Schedule: "invalid"}
		repo.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(sched, nil).Once()

		err := svc.ResumeSchedule(ctx, schedID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid schedule")
	})

	t.Run("CreateSchedule_RepoCreateError", func(t *testing.T) {
		fn := &domain.Function{ID: fnID, Name: "test-fn", UserID: userID}
		fnRepo.On("GetByID", mock.Anything, fnID).Return(fn, nil).Once()
		repo.On("Create", mock.Anything, mock.Anything).Return(fmt.Errorf("db error")).Once()
		auditSvc.On("Log", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		_, err := svc.CreateSchedule(ctx, fnID, "nightly", "0 2 * * *", []byte(`{}`))
		require.Error(t, err)
	})

	t.Run("DeleteSchedule_DeleteError", func(t *testing.T) {
		schedID := uuid.New()
		sched := &domain.FunctionSchedule{ID: schedID, UserID: userID}
		repo.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(sched, nil).Once()
		repo.On("Delete", mock.Anything, schedID).Return(fmt.Errorf("delete failed")).Once()

		err := svc.DeleteSchedule(ctx, schedID)
		require.Error(t, err)
	})

	t.Run("GetScheduleRuns_RepoError", func(t *testing.T) {
		schedID := uuid.New()
		repo.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(&domain.FunctionSchedule{ID: schedID, UserID: userID}, nil).Once()
		repo.On("GetScheduleRuns", mock.Anything, schedID, 50).Return(nil, fmt.Errorf("db error")).Once()

		_, err := svc.GetScheduleRuns(ctx, schedID, 50)
		require.Error(t, err)
	})

	t.Run("ResumeSchedule_UpdateError", func(t *testing.T) {
		repo2 := new(MockFunctionScheduleRepo)
		fnRepo2 := new(MockFunctionRepo)
		rbacSvc2 := new(MockRBACService)
		eventSvc2 := new(MockEventService)
		auditSvc2 := new(MockAuditService)

		schedID := uuid.New()
		sched := &domain.FunctionSchedule{
			ID: schedID, UserID: userID, Status: domain.FunctionScheduleStatusPaused, Schedule: "*/5 * * * *",
		}
		rbacSvc2.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionFunctionScheduleUpdate, schedID.String()).Return(nil).Once()
		repo2.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(sched, nil).Once()
		repo2.On("Update", mock.Anything, mock.Anything).Return(fmt.Errorf("update failed")).Once()

		svc2 := services.NewFunctionScheduleService(repo2, fnRepo2, rbacSvc2, eventSvc2, auditSvc2, slog.Default())
		err := svc2.ResumeSchedule(ctx, schedID)
		require.Error(t, err)
	})

	t.Run("CreateSchedule_EventRecordFailure", func(t *testing.T) {
		fn := &domain.Function{ID: fnID, Name: "test-fn", UserID: userID}
		fnRepo.On("GetByID", mock.Anything, fnID).Return(fn, nil).Once()
		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		eventSvc.On("RecordEvent", mock.Anything, "FUNCTION_SCHEDULE_CREATED", mock.Anything, "FUNCTION_SCHEDULE", mock.Anything).Return(fmt.Errorf("event svc down")).Once()
		auditSvc.On("Log", mock.Anything, userID, "function_schedule.create", "function_schedule", mock.Anything, mock.Anything).Return(nil).Once()

		sched, err := svc.CreateSchedule(ctx, fnID, "nightly", "0 2 * * *", []byte(`{}`))
		require.NoError(t, err)
		assert.NotNil(t, sched)
	})

	t.Run("CreateSchedule_AuditLogFailure", func(t *testing.T) {
		fn := &domain.Function{ID: fnID, Name: "test-fn", UserID: userID}
		fnRepo.On("GetByID", mock.Anything, fnID).Return(fn, nil).Once()
		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		eventSvc.On("RecordEvent", mock.Anything, "FUNCTION_SCHEDULE_CREATED", mock.Anything, "FUNCTION_SCHEDULE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "function_schedule.create", "function_schedule", mock.Anything, mock.Anything).Return(fmt.Errorf("audit svc down")).Once()

		sched, err := svc.CreateSchedule(ctx, fnID, "nightly", "0 2 * * *", []byte(`{}`))
		require.NoError(t, err)
		assert.NotNil(t, sched)
	})

	t.Run("ListSchedules_RBACDenied", func(t *testing.T) {
		repo2 := new(MockFunctionScheduleRepo)
		fnRepo2 := new(MockFunctionRepo)
		rbacSvc2 := new(MockRBACService)
		eventSvc2 := new(MockEventService)
		auditSvc2 := new(MockAuditService)
		rbacSvc2.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionFunctionScheduleRead, "*").Return(fmt.Errorf("access denied")).Once()

		svc2 := services.NewFunctionScheduleService(repo2, fnRepo2, rbacSvc2, eventSvc2, auditSvc2, slog.Default())
		_, err := svc2.ListSchedules(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
	})

	t.Run("ListSchedules_RepoError", func(t *testing.T) {
		repo2 := new(MockFunctionScheduleRepo)
		fnRepo2 := new(MockFunctionRepo)
		rbacSvc2 := new(MockRBACService)
		eventSvc2 := new(MockEventService)
		auditSvc2 := new(MockAuditService)
		rbacSvc2.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil)
		repo2.On("List", mock.Anything, userID, tenantID).Return(nil, fmt.Errorf("db error")).Once()

		svc2 := services.NewFunctionScheduleService(repo2, fnRepo2, rbacSvc2, eventSvc2, auditSvc2, slog.Default())
		_, err := svc2.ListSchedules(ctx)
		require.Error(t, err)
	})

	t.Run("GetSchedule_RBACDenied", func(t *testing.T) {
		schedID := uuid.New()
		repo2 := new(MockFunctionScheduleRepo)
		fnRepo2 := new(MockFunctionRepo)
		rbacSvc2 := new(MockRBACService)
		eventSvc2 := new(MockEventService)
		auditSvc2 := new(MockAuditService)
		rbacSvc2.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionFunctionScheduleRead, schedID.String()).Return(fmt.Errorf("access denied")).Once()

		svc2 := services.NewFunctionScheduleService(repo2, fnRepo2, rbacSvc2, eventSvc2, auditSvc2, slog.Default())
		_, err := svc2.GetSchedule(ctx, schedID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
	})

	t.Run("DeleteSchedule_RBACDenied", func(t *testing.T) {
		schedID := uuid.New()
		repo2 := new(MockFunctionScheduleRepo)
		fnRepo2 := new(MockFunctionRepo)
		rbacSvc2 := new(MockRBACService)
		eventSvc2 := new(MockEventService)
		auditSvc2 := new(MockAuditService)
		rbacSvc2.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionFunctionScheduleDelete, schedID.String()).Return(fmt.Errorf("access denied")).Once()

		svc2 := services.NewFunctionScheduleService(repo2, fnRepo2, rbacSvc2, eventSvc2, auditSvc2, slog.Default())
		err := svc2.DeleteSchedule(ctx, schedID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
	})

	t.Run("PauseSchedule_RBACDenied", func(t *testing.T) {
		schedID := uuid.New()
		repo2 := new(MockFunctionScheduleRepo)
		fnRepo2 := new(MockFunctionRepo)
		rbacSvc2 := new(MockRBACService)
		eventSvc2 := new(MockEventService)
		auditSvc2 := new(MockAuditService)
		rbacSvc2.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionFunctionScheduleUpdate, schedID.String()).Return(fmt.Errorf("access denied")).Once()

		svc2 := services.NewFunctionScheduleService(repo2, fnRepo2, rbacSvc2, eventSvc2, auditSvc2, slog.Default())
		err := svc2.PauseSchedule(ctx, schedID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
	})

	t.Run("PauseSchedule_NotFound", func(t *testing.T) {
		schedID := uuid.New()
		repo2 := new(MockFunctionScheduleRepo)
		fnRepo2 := new(MockFunctionRepo)
		rbacSvc2 := new(MockRBACService)
		eventSvc2 := new(MockEventService)
		auditSvc2 := new(MockAuditService)
		rbacSvc2.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionFunctionScheduleUpdate, schedID.String()).Return(nil).Once()
		repo2.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(nil, fmt.Errorf("not found")).Once()

		svc2 := services.NewFunctionScheduleService(repo2, fnRepo2, rbacSvc2, eventSvc2, auditSvc2, slog.Default())
		err := svc2.PauseSchedule(ctx, schedID)
		require.Error(t, err)
	})

	t.Run("PauseSchedule_UpdateError", func(t *testing.T) {
		schedID := uuid.New()
		repo2 := new(MockFunctionScheduleRepo)
		fnRepo2 := new(MockFunctionRepo)
		rbacSvc2 := new(MockRBACService)
		eventSvc2 := new(MockEventService)
		auditSvc2 := new(MockAuditService)
		sched := &domain.FunctionSchedule{ID: schedID, UserID: userID, Status: domain.FunctionScheduleStatusActive}
		rbacSvc2.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionFunctionScheduleUpdate, schedID.String()).Return(nil).Once()
		repo2.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(sched, nil).Once()
		repo2.On("Update", mock.Anything, mock.MatchedBy(func(s *domain.FunctionSchedule) bool {
			return s.Status == domain.FunctionScheduleStatusPaused && s.NextRunAt == nil
		})).Return(fmt.Errorf("update failed")).Once()

		svc2 := services.NewFunctionScheduleService(repo2, fnRepo2, rbacSvc2, eventSvc2, auditSvc2, slog.Default())
		err := svc2.PauseSchedule(ctx, schedID)
		require.Error(t, err)
	})

	t.Run("ResumeSchedule_RBACDenied", func(t *testing.T) {
		schedID := uuid.New()
		repo2 := new(MockFunctionScheduleRepo)
		fnRepo2 := new(MockFunctionRepo)
		rbacSvc2 := new(MockRBACService)
		eventSvc2 := new(MockEventService)
		auditSvc2 := new(MockAuditService)
		rbacSvc2.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionFunctionScheduleUpdate, schedID.String()).Return(fmt.Errorf("access denied")).Once()

		svc2 := services.NewFunctionScheduleService(repo2, fnRepo2, rbacSvc2, eventSvc2, auditSvc2, slog.Default())
		err := svc2.ResumeSchedule(ctx, schedID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
	})

	t.Run("ResumeSchedule_NotFound", func(t *testing.T) {
		schedID := uuid.New()
		repo2 := new(MockFunctionScheduleRepo)
		fnRepo2 := new(MockFunctionRepo)
		rbacSvc2 := new(MockRBACService)
		eventSvc2 := new(MockEventService)
		auditSvc2 := new(MockAuditService)
		rbacSvc2.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionFunctionScheduleUpdate, schedID.String()).Return(nil).Once()
		repo2.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(nil, fmt.Errorf("not found")).Once()

		svc2 := services.NewFunctionScheduleService(repo2, fnRepo2, rbacSvc2, eventSvc2, auditSvc2, slog.Default())
		err := svc2.ResumeSchedule(ctx, schedID)
		require.Error(t, err)
	})

	t.Run("GetScheduleRuns_RBACDenied", func(t *testing.T) {
		schedID := uuid.New()
		repo2 := new(MockFunctionScheduleRepo)
		fnRepo2 := new(MockFunctionRepo)
		rbacSvc2 := new(MockRBACService)
		eventSvc2 := new(MockEventService)
		auditSvc2 := new(MockAuditService)
		rbacSvc2.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionFunctionScheduleRead, schedID.String()).Return(fmt.Errorf("access denied")).Once()

		svc2 := services.NewFunctionScheduleService(repo2, fnRepo2, rbacSvc2, eventSvc2, auditSvc2, slog.Default())
		_, err := svc2.GetScheduleRuns(ctx, schedID, 50)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
	})

	t.Run("GetScheduleRuns_LimitZero", func(t *testing.T) {
		schedID := uuid.New()
		repo2 := new(MockFunctionScheduleRepo)
		fnRepo2 := new(MockFunctionRepo)
		rbacSvc2 := new(MockRBACService)
		eventSvc2 := new(MockEventService)
		auditSvc2 := new(MockAuditService)
		rbacSvc2.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil)
		repo2.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(&domain.FunctionSchedule{ID: schedID, UserID: userID}, nil).Once()
		repo2.On("GetScheduleRuns", mock.Anything, schedID, 50).Return([]*domain.FunctionScheduleRun{}, nil).Once()

		svc2 := services.NewFunctionScheduleService(repo2, fnRepo2, rbacSvc2, eventSvc2, auditSvc2, slog.Default())
		_, err := svc2.GetScheduleRuns(ctx, schedID, 0)
		require.NoError(t, err)
		repo2.AssertCalled(t, "GetScheduleRuns", mock.Anything, schedID, 50)
	})

	t.Run("GetScheduleRuns_LimitNegative", func(t *testing.T) {
		schedID := uuid.New()
		repo2 := new(MockFunctionScheduleRepo)
		fnRepo2 := new(MockFunctionRepo)
		rbacSvc2 := new(MockRBACService)
		eventSvc2 := new(MockEventService)
		auditSvc2 := new(MockAuditService)
		rbacSvc2.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil)
		repo2.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(&domain.FunctionSchedule{ID: schedID, UserID: userID}, nil).Once()
		repo2.On("GetScheduleRuns", mock.Anything, schedID, 50).Return([]*domain.FunctionScheduleRun{}, nil).Once()

		svc2 := services.NewFunctionScheduleService(repo2, fnRepo2, rbacSvc2, eventSvc2, auditSvc2, slog.Default())
		_, err := svc2.GetScheduleRuns(ctx, schedID, -10)
		require.NoError(t, err)
		repo2.AssertCalled(t, "GetScheduleRuns", mock.Anything, schedID, 50)
	})

	t.Run("GetScheduleRuns_LimitExceeds500", func(t *testing.T) {
		schedID := uuid.New()
		repo2 := new(MockFunctionScheduleRepo)
		fnRepo2 := new(MockFunctionRepo)
		rbacSvc2 := new(MockRBACService)
		eventSvc2 := new(MockEventService)
		auditSvc2 := new(MockAuditService)
		rbacSvc2.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil)
		repo2.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(&domain.FunctionSchedule{ID: schedID, UserID: userID}, nil).Once()
		repo2.On("GetScheduleRuns", mock.Anything, schedID, 500).Return([]*domain.FunctionScheduleRun{}, nil).Once()

		svc2 := services.NewFunctionScheduleService(repo2, fnRepo2, rbacSvc2, eventSvc2, auditSvc2, slog.Default())
		_, err := svc2.GetScheduleRuns(ctx, schedID, 1000)
		require.NoError(t, err)
		repo2.AssertCalled(t, "GetScheduleRuns", mock.Anything, schedID, 500)
	})

	t.Run("GetScheduleRuns_LimitUnder500", func(t *testing.T) {
		schedID := uuid.New()
		repo2 := new(MockFunctionScheduleRepo)
		fnRepo2 := new(MockFunctionRepo)
		rbacSvc2 := new(MockRBACService)
		eventSvc2 := new(MockEventService)
		auditSvc2 := new(MockAuditService)
		rbacSvc2.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil)
		repo2.On("GetByID", mock.Anything, schedID, userID, tenantID).Return(&domain.FunctionSchedule{ID: schedID, UserID: userID}, nil).Once()
		repo2.On("GetScheduleRuns", mock.Anything, schedID, 100).Return([]*domain.FunctionScheduleRun{}, nil).Once()

		svc2 := services.NewFunctionScheduleService(repo2, fnRepo2, rbacSvc2, eventSvc2, auditSvc2, slog.Default())
		_, err := svc2.GetScheduleRuns(ctx, schedID, 100)
		require.NoError(t, err)
		repo2.AssertCalled(t, "GetScheduleRuns", mock.Anything, schedID, 100)
	})
}

func TestFunctionScheduleWorkerUnit(t *testing.T) {
	t.Run("ProcessSchedules_Empty", func(t *testing.T) {
		repo := new(MockFunctionScheduleRepo)
		fnSvc := new(MockFunctionService)
		repo.On("ClaimNextSchedulesToRun", mock.Anything, 5*time.Minute).Return([]*domain.FunctionSchedule{}, nil).Once()

		worker := services.NewFunctionScheduleWorker(repo, fnSvc)
		worker.ProcessSchedules(context.Background())
		repo.AssertExpectations(t)
	})

	t.Run("ProcessSchedules_InvokeSuccess", func(t *testing.T) {
		repo := new(MockFunctionScheduleRepo)
		fnSvc := new(MockFunctionService)

		schedID := uuid.New()
		fnID := uuid.New()
		invID := uuid.New()

		sched := &domain.FunctionSchedule{
			ID:         schedID,
			FunctionID: fnID,
			Schedule:   "*/5 * * * *",
			Status:     domain.FunctionScheduleStatusActive,
		}

		invocation := &domain.Invocation{
			ID:     invID,
			Status: "SUCCESS",
		}

		repo.On("ClaimNextSchedulesToRun", mock.Anything, 5*time.Minute).Return([]*domain.FunctionSchedule{sched}, nil).Once()
		fnSvc.On("InvokeFunction", mock.Anything, fnID, mock.Anything, true).Return(invocation, nil).Once()
		repo.On("CompleteScheduleRun", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		worker := services.NewFunctionScheduleWorker(repo, fnSvc)
		worker.ProcessSchedules(context.Background())
		repo.AssertExpectations(t)
		fnSvc.AssertExpectations(t)
	})

	t.Run("ProcessSchedules_ClaimError", func(t *testing.T) {
		repo := new(MockFunctionScheduleRepo)
		fnSvc := new(MockFunctionService)
		repo.On("ClaimNextSchedulesToRun", mock.Anything, 5*time.Minute).Return(nil, fmt.Errorf("claim failed")).Once()

		worker := services.NewFunctionScheduleWorker(repo, fnSvc)
		worker.ProcessSchedules(context.Background())
		repo.AssertExpectations(t)
	})

	t.Run("ProcessSchedules_InvokeError", func(t *testing.T) {
		repo := new(MockFunctionScheduleRepo)
		fnSvc := new(MockFunctionService)

		schedID := uuid.New()
		schedFnID := uuid.New()

		sched := &domain.FunctionSchedule{
			ID:         schedID,
			FunctionID: schedFnID,
			Schedule:   "*/5 * * * *",
			Status:     domain.FunctionScheduleStatusActive,
		}

		repo.On("ClaimNextSchedulesToRun", mock.Anything, 5*time.Minute).Return([]*domain.FunctionSchedule{sched}, nil).Once()
		fnSvc.On("InvokeFunction", mock.Anything, schedFnID, mock.Anything, true).Return(nil, fmt.Errorf("invoke failed")).Once()
		repo.On("CompleteScheduleRun", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		worker := services.NewFunctionScheduleWorker(repo, fnSvc)
		worker.ProcessSchedules(context.Background())
		repo.AssertExpectations(t)
		fnSvc.AssertExpectations(t)
	})

	t.Run("ProcessSchedules_InvocationFailed", func(t *testing.T) {
		repo := new(MockFunctionScheduleRepo)
		fnSvc := new(MockFunctionService)

		schedID := uuid.New()
		schedFnID := uuid.New()

		sched := &domain.FunctionSchedule{
			ID:         schedID,
			FunctionID: schedFnID,
			Schedule:   "*/5 * * * *",
			Status:     domain.FunctionScheduleStatusActive,
		}

		invocation := &domain.Invocation{
			ID:     uuid.New(),
			Status: "FAILED",
			Logs:   "error: something went wrong",
		}

		repo.On("ClaimNextSchedulesToRun", mock.Anything, 5*time.Minute).Return([]*domain.FunctionSchedule{sched}, nil).Once()
		fnSvc.On("InvokeFunction", mock.Anything, schedFnID, mock.Anything, true).Return(invocation, nil).Once()
		repo.On("CompleteScheduleRun", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		worker := services.NewFunctionScheduleWorker(repo, fnSvc)
		worker.ProcessSchedules(context.Background())
		repo.AssertExpectations(t)
		fnSvc.AssertExpectations(t)
	})
}
