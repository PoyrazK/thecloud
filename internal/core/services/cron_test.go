package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCronService_CreateJob(t *testing.T) {
	repo := new(MockCronRepo)
	eventSvc := new(MockEventService)
	svc := services.NewCronService(repo, eventSvc)

	userID := uuid.New()
	ctx := appcontext.WithUserID(context.Background(), userID)

	repo.On("CreateJob", ctx, mock.AnythingOfType("*domain.CronJob")).Return(nil)
	eventSvc.On("RecordEvent", ctx, "CRON_JOB_CREATED", mock.Anything, "CRON_JOB", mock.Anything).Return(nil)

	job, err := svc.CreateJob(ctx, "daily-task", "0 0 * * *", "http://api/run", "POST", "")

	assert.NoError(t, err)
	assert.NotNil(t, job)
	assert.Equal(t, "daily-task", job.Name)
	assert.Equal(t, domain.CronStatusActive, job.Status)
	assert.NotNil(t, job.NextRunAt)
	repo.AssertExpectations(t)
}

func TestCronService_PauseResume(t *testing.T) {
	repo := new(MockCronRepo)
	svc := services.NewCronService(repo, nil)

	userID := uuid.New()
	jobID := uuid.New()
	ctx := appcontext.WithUserID(context.Background(), userID)

	job := &domain.CronJob{ID: jobID, UserID: userID, Status: domain.CronStatusActive, Schedule: "0 0 * * *"}
	repo.On("GetJobByID", ctx, jobID, userID).Return(job, nil)
	repo.On("UpdateJob", ctx, mock.MatchedBy(func(j *domain.CronJob) bool {
		return j.Status == domain.CronStatusPaused
	})).Return(nil)

	err := svc.PauseJob(ctx, jobID)
	assert.NoError(t, err)

	repo.On("UpdateJob", ctx, mock.MatchedBy(func(j *domain.CronJob) bool {
		return j.Status == domain.CronStatusActive && j.NextRunAt != nil
	})).Return(nil)

	err = svc.ResumeJob(ctx, jobID)
	assert.NoError(t, err)
}
