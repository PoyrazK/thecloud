//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFunctionScheduleRepository_Integration(t *testing.T) {
	db, _ := SetupDB(t)
	defer db.Close()
	repo := NewPostgresFunctionScheduleRepository(db)
	ctx := SetupTestUser(t, db)

	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), "DELETE FROM function_schedule_runs")
		_, _ = db.Exec(context.Background(), "DELETE FROM function_schedules")
	})

	t.Run("Create", func(t *testing.T) {
		s := &domain.FunctionSchedule{
			ID:         uuid.New(),
			UserID:     appcontext.UserIDFromContext(ctx),
			TenantID:   appcontext.TenantIDFromContext(ctx),
			FunctionID: uuid.New(),
			Name:       "test-schedule",
			Schedule:   "*/5 * * * *",
			Payload:    []byte(`{"key":"value"}`),
			Status:     domain.FunctionScheduleStatusActive,
			NextRunAt:  func() *time.Time { t := time.Now().Add(10 * time.Second); return &t }(),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		err := repo.Create(ctx, s)
		require.NoError(t, err)

		result, err := repo.GetByID(ctx, s.ID, s.UserID, s.TenantID)
		require.NoError(t, err)
		assert.Equal(t, s.Name, result.Name)
	})

	t.Run("GetByID_NotFound", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.New(), uuid.New(), uuid.New())
		require.Error(t, err)
	})

	t.Run("List", func(t *testing.T) {
		list, err := repo.List(ctx, appcontext.UserIDFromContext(ctx), appcontext.TenantIDFromContext(ctx))
		require.NoError(t, err)
		assert.Len(t, list, 1)
	})

	t.Run("ClaimNextSchedulesToRun_NoSchedules", func(t *testing.T) {
		schedules, err := repo.ClaimNextSchedulesToRun(ctx, 30*time.Second)
		require.NoError(t, err)
		assert.Len(t, schedules, 0)
	})

	t.Run("ClaimNextSchedulesToRun_OneSchedule", func(t *testing.T) {
		s := &domain.FunctionSchedule{
			ID:         uuid.New(),
			UserID:     appcontext.UserIDFromContext(ctx),
			TenantID:   appcontext.TenantIDFromContext(ctx),
			FunctionID: uuid.New(),
			Name:       "test-claim",
			Schedule:   "*/5 * * * *",
			Payload:    []byte(`{}`),
			Status:     domain.FunctionScheduleStatusActive,
			NextRunAt:  func() *time.Time { t := time.Now().Add(-time.Minute); return &t }(),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		require.NoError(t, repo.Create(ctx, s))

		schedules, err := repo.ClaimNextSchedulesToRun(ctx, 30*time.Second)
		require.NoError(t, err)
		require.Len(t, schedules, 1)
		assert.Equal(t, s.ID, schedules[0].ID)
		assert.NotNil(t, schedules[0].ClaimedUntil)
	})

	t.Run("CompleteScheduleRun", func(t *testing.T) {
		s := &domain.FunctionSchedule{
			ID:         uuid.New(),
			UserID:     appcontext.UserIDFromContext(ctx),
			TenantID:   appcontext.TenantIDFromContext(ctx),
			FunctionID: uuid.New(),
			Name:       "test-complete",
			Schedule:   "*/5 * * * *",
			Payload:    []byte(`{}`),
			Status:     domain.FunctionScheduleStatusActive,
			NextRunAt:  func() *time.Time { t := time.Now().Add(-time.Minute); return &t }(),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		require.NoError(t, repo.Create(ctx, s))

		schedules, err := repo.ClaimNextSchedulesToRun(ctx, 30*time.Second)
		require.NoError(t, err)
		require.Len(t, schedules, 1)

		claimed := schedules[0]
		run := &domain.FunctionScheduleRun{
			ID:           uuid.New(),
			ScheduleID:   claimed.ID,
			InvocationID: func() *uuid.UUID { u := uuid.New(); return &u }(),
			Status:       "completed",
			StatusCode:   200,
			DurationMs:   150,
			StartedAt:    time.Now(),
		}
		nextRunAt := time.Now().Add(5 * time.Minute)
		require.NoError(t, repo.CompleteScheduleRun(ctx, run, claimed, nextRunAt))

		runs, err := repo.GetScheduleRuns(ctx, claimed.ID, 10)
		require.NoError(t, err)
		assert.Len(t, runs, 1)
		assert.Equal(t, "completed", runs[0].Status)
	})

	t.Run("ReapStaleClaims", func(t *testing.T) {
		s := &domain.FunctionSchedule{
			ID:         uuid.New(),
			UserID:     appcontext.UserIDFromContext(ctx),
			TenantID:   appcontext.TenantIDFromContext(ctx),
			FunctionID: uuid.New(),
			Name:       "test-reap",
			Schedule:   "*/5 * * * *",
			Payload:    []byte(`{}`),
			Status:     domain.FunctionScheduleStatusActive,
			NextRunAt:  func() *time.Time { t := time.Now().Add(-time.Minute); return &t }(),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		require.NoError(t, repo.Create(ctx, s))

		count, err := repo.ReapStaleClaims(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})
}
