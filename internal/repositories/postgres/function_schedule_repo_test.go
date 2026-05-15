package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func functionScheduleTestSchedule() *domain.FunctionSchedule {
	now := time.Now()
	return &domain.FunctionSchedule{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		TenantID:   uuid.New(),
		FunctionID: uuid.New(),
		Name:       "test-schedule",
		Schedule:   "*/5 * * * *",
		Payload:    []byte(`{"key":"value"}`),
		Status:     domain.FunctionScheduleStatusActive,
		NextRunAt:  &now,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func functionScheduleTestRun() *domain.FunctionScheduleRun {
	invID := uuid.New()
	return &domain.FunctionScheduleRun{
		ID:           uuid.New(),
		ScheduleID:   uuid.New(),
		InvocationID: &invID,
		Status:       "completed",
		StatusCode:   200,
		DurationMs:   150,
		ErrorMessage: "",
		StartedAt:    time.Now(),
	}
}

func TestFunctionScheduleRepository_Create(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPostgresFunctionScheduleRepository(mock)
	s := functionScheduleTestSchedule()

	mock.ExpectExec("INSERT INTO function_schedules").
		WithArgs(s.ID, s.UserID, s.TenantID, s.FunctionID, s.Name, s.Schedule, s.Payload, s.Status, s.NextRunAt, s.CreatedAt, s.UpdatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.Create(context.Background(), s)
	require.NoError(t, err)
}

func TestFunctionScheduleRepository_GetByID(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPostgresFunctionScheduleRepository(mock)
	s := functionScheduleTestSchedule()
	payload := []byte(`{}`)

	mock.ExpectQuery("SELECT .+ FROM function_schedules WHERE id = .+AND user_id = .+AND tenant_id = .+").
		WithArgs(s.ID, s.UserID, s.TenantID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "function_id", "name", "schedule", "payload", "status", "last_run_at", "next_run_at", "claimed_until", "created_at", "updated_at"}).
			AddRow(s.ID, s.UserID, s.TenantID, s.FunctionID, s.Name, s.Schedule, payload, string(s.Status), nil, s.NextRunAt, nil, s.CreatedAt, s.UpdatedAt))

	result, err := repo.GetByID(context.Background(), s.ID, s.UserID, s.TenantID)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, s.Name, result.Name)
}

func TestFunctionScheduleRepository_GetByID_NotFound(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPostgresFunctionScheduleRepository(mock)
	id := uuid.New()
	userID := uuid.New()
	tenantID := uuid.New()

	mock.ExpectQuery("SELECT .+ FROM function_schedules WHERE id = .+AND user_id = .+AND tenant_id = .+").
		WithArgs(id, userID, tenantID).
		WillReturnError(context.DeadlineExceeded)

	result, err := repo.GetByID(context.Background(), id, userID, tenantID)
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestFunctionScheduleRepository_List(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPostgresFunctionScheduleRepository(mock)
	s := functionScheduleTestSchedule()
	payload := []byte(`{}`)

	mock.ExpectQuery("SELECT .+ FROM function_schedules WHERE user_id = .+AND tenant_id = .+ORDER BY created_at DESC").
		WithArgs(s.UserID, s.TenantID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "function_id", "name", "schedule", "payload", "status", "last_run_at", "next_run_at", "claimed_until", "created_at", "updated_at"}).
			AddRow(s.ID, s.UserID, s.TenantID, s.FunctionID, s.Name, s.Schedule, payload, string(s.Status), nil, s.NextRunAt, nil, s.CreatedAt, s.UpdatedAt))

	result, err := repo.List(context.Background(), s.UserID, s.TenantID)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestFunctionScheduleRepository_Update(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPostgresFunctionScheduleRepository(mock)
	s := functionScheduleTestSchedule()

	mock.ExpectExec("UPDATE function_schedules SET status = .+, last_run_at = .+, next_run_at = .+, claimed_until = .+, updated_at = NOW\\(\\) WHERE id = .+").
		WithArgs(s.Status, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), s.ID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = repo.Update(context.Background(), s)
	require.NoError(t, err)
}

func TestFunctionScheduleRepository_Delete(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPostgresFunctionScheduleRepository(mock)
	id := uuid.New()

	mock.ExpectExec("DELETE FROM function_schedules WHERE id = .+").
		WithArgs(id).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = repo.Delete(context.Background(), id)
	require.NoError(t, err)
}

func TestFunctionScheduleRepository_GetScheduleRuns(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPostgresFunctionScheduleRepository(mock)
	run := functionScheduleTestRun()
	scheduleID := uuid.New()

	mock.ExpectQuery("SELECT .+ FROM function_schedule_runs WHERE schedule_id = .+ORDER BY started_at DESC LIMIT .+").
		WithArgs(scheduleID, 50).
		WillReturnRows(pgxmock.NewRows([]string{"id", "schedule_id", "invocation_id", "status", "status_code", "duration_ms", "error_message", "started_at"}).
			AddRow(run.ID, run.ScheduleID, run.InvocationID, run.Status, run.StatusCode, run.DurationMs, run.ErrorMessage, run.StartedAt))

	result, err := repo.GetScheduleRuns(context.Background(), scheduleID, 50)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestFunctionScheduleRepository_GetScheduleRuns_Empty(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPostgresFunctionScheduleRepository(mock)
	scheduleID := uuid.New()

	mock.ExpectQuery("SELECT .+ FROM function_schedule_runs WHERE schedule_id = .+ORDER BY started_at DESC LIMIT .+").
		WithArgs(scheduleID, 10).
		WillReturnRows(pgxmock.NewRows([]string{"id", "schedule_id", "invocation_id", "status", "status_code", "duration_ms", "error_message", "started_at"}))

	result, err := repo.GetScheduleRuns(context.Background(), scheduleID, 10)
	require.NoError(t, err)
	assert.Len(t, result, 0)
}
