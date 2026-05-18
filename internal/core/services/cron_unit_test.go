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

func TestCronServiceUnit(t *testing.T) {
	repo := new(MockCronRepository)
	eventSvc := new(MockEventService)
	auditSvc := new(MockAuditService)
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := services.NewCronService(repo, rbacSvc, eventSvc, auditSvc, slog.Default())

	ctx := context.Background()
	userID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)

	t.Run("CreateJob", func(t *testing.T) {
		repo.On("CreateJob", mock.Anything, mock.Anything).Return(nil).Once()
		eventSvc.On("RecordEvent", mock.Anything, "CRON_JOB_CREATED", mock.Anything, "CRON_JOB", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "cron.job_create", "cron_job", mock.Anything, mock.Anything).Return(nil).Once()

		job, err := svc.CreateJob(ctx, "test-job", "* * * * *", "http://example.com", "GET", "")
		require.NoError(t, err)
		assert.NotNil(t, job)
		assert.Equal(t, "test-job", job.Name)
		repo.AssertExpectations(t)
	})

	t.Run("CreateJob_InvalidSchedule", func(t *testing.T) {
		_, err := svc.CreateJob(ctx, "fail", "invalid", "http://url", "GET", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid cron schedule")
	})

	t.Run("ListJobs", func(t *testing.T) {
		expectedJobs := []*domain.CronJob{{ID: uuid.New(), Name: "job1"}}
		repo.On("ListJobs", mock.Anything, userID).Return(expectedJobs, nil).Once()

		jobs, err := svc.ListJobs(ctx)
		require.NoError(t, err)
		assert.Len(t, jobs, 1)
		assert.Equal(t, "job1", jobs[0].Name)
	})

	t.Run("GetJob", func(t *testing.T) {
		jobID := uuid.New()
		expectedJob := &domain.CronJob{ID: jobID, Name: "job1"}
		repo.On("GetJobByID", mock.Anything, jobID, userID).Return(expectedJob, nil).Once()

		job, err := svc.GetJob(ctx, jobID)
		require.NoError(t, err)
		assert.Equal(t, jobID, job.ID)
	})

	t.Run("PauseJob", func(t *testing.T) {
		jobID := uuid.New()
		job := &domain.CronJob{ID: jobID, UserID: userID, Status: domain.CronStatusActive}
		repo.On("GetJobByID", mock.Anything, jobID, userID).Return(job, nil).Once()
		repo.On("UpdateJob", mock.Anything, mock.MatchedBy(func(j *domain.CronJob) bool {
			return j.Status == domain.CronStatusPaused
		})).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "cron.job_pause", "cron_job", jobID.String(), mock.Anything).Return(nil).Once()

		err := svc.PauseJob(ctx, jobID)
		require.NoError(t, err)
	})

	t.Run("PauseJob_NotFound", func(t *testing.T) {
		jobID := uuid.New()
		repo.On("GetJobByID", mock.Anything, jobID, userID).Return(nil, fmt.Errorf("not found")).Once()

		err := svc.PauseJob(ctx, jobID)
		require.Error(t, err)
	})

	t.Run("ResumeJob", func(t *testing.T) {
		jobID := uuid.New()
		job := &domain.CronJob{ID: jobID, UserID: userID, Status: domain.CronStatusPaused, Schedule: "* * * * *"}
		repo.On("GetJobByID", mock.Anything, jobID, userID).Return(job, nil).Once()
		repo.On("UpdateJob", mock.Anything, mock.MatchedBy(func(j *domain.CronJob) bool {
			return j.Status == domain.CronStatusActive
		})).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "cron.job_resume", "cron_job", jobID.String(), mock.Anything).Return(nil).Once()

		err := svc.ResumeJob(ctx, jobID)
		require.NoError(t, err)
	})

	t.Run("DeleteJob", func(t *testing.T) {
		jobID := uuid.New()
		job := &domain.CronJob{ID: jobID, UserID: userID}
		repo.On("GetJobByID", mock.Anything, jobID, userID).Return(job, nil).Once()
		repo.On("DeleteJob", mock.Anything, jobID).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "cron.job_delete", "cron_job", jobID.String(), mock.Anything).Return(nil).Once()

		err := svc.DeleteJob(ctx, jobID)
		require.NoError(t, err)
	})

	t.Run("GetJobRuns", func(t *testing.T) {
		jobID := uuid.New()
		expectedRuns := []*domain.CronJobRun{
			{ID: uuid.New(), JobID: jobID, Status: "SUCCESS", StatusCode: 200, DurationMs: 150, StartedAt: time.Now()},
			{ID: uuid.New(), JobID: jobID, Status: "FAILED", StatusCode: 500, DurationMs: 50, StartedAt: time.Now()},
		}
		repo.On("GetJobRuns", mock.Anything, jobID, 50).Return(expectedRuns, nil).Once()

		runs, err := svc.GetJobRuns(ctx, jobID, 50)
		require.NoError(t, err)
		assert.Len(t, runs, 2)
		assert.Equal(t, "SUCCESS", runs[0].Status)
	})

	t.Run("GetJobRuns_DefaultLimit", func(t *testing.T) {
		jobID := uuid.New()
		repo.On("GetJobRuns", mock.Anything, jobID, 50).Return([]*domain.CronJobRun{}, nil).Once()

		runs, err := svc.GetJobRuns(ctx, jobID, 0) // limit <= 0 should default to 50
		require.NoError(t, err)
		assert.Empty(t, runs)
	})

	t.Run("UpdateJob", func(t *testing.T) {
		jobID := uuid.New()
		job := &domain.CronJob{ID: jobID, UserID: userID, Name: "old-name", Schedule: "0 * * * *", TargetURL: "http://old.example.com"}
		repo.On("GetJobByID", mock.Anything, jobID, userID).Return(job, nil).Once()
		repo.On("UpdateJob", mock.Anything, mock.MatchedBy(func(j *domain.CronJob) bool {
			return j.Name == "new-name" && j.Schedule == "0 0 * * *" && j.TargetURL == "http://new.example.com"
		})).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "cron.job_update", "cron_job", jobID.String(), mock.Anything).Return(nil).Once()

		result, err := svc.UpdateJob(ctx, jobID, "new-name", "0 0 * * *", "http://new.example.com", "", "")
		require.NoError(t, err)
		assert.Equal(t, "new-name", result.Name)
	})

	t.Run("UpdateJob_InvalidSchedule", func(t *testing.T) {
		jobID := uuid.New()
		job := &domain.CronJob{ID: jobID, UserID: userID, Schedule: "* * * * *"}
		repo.On("GetJobByID", mock.Anything, jobID, userID).Return(job, nil).Once()

		_, err := svc.UpdateJob(ctx, jobID, "", "invalid-schedule", "", "", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid cron schedule")
	})

	t.Run("UpdateJob_NotFound", func(t *testing.T) {
		jobID := uuid.New()
		repo.On("GetJobByID", mock.Anything, jobID, userID).Return(nil, fmt.Errorf("not found")).Once()

		_, err := svc.UpdateJob(ctx, jobID, "name", "* * * * *", "http://example.com", "", "")
		require.Error(t, err)
	})
}
