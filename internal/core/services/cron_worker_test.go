package services_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/mock"
)

func TestCronWorker_ProcessJobs(t *testing.T) {
	repo := new(MockCronRepo)
	worker := services.NewCronWorker(repo)

	// Mock target server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	job := &domain.CronJob{
		ID:            uuid.New(),
		Name:          "job-1",
		Schedule:      "* * * * *",
		TargetURL:     server.URL,
		TargetMethod:  "GET",
		TargetPayload: "",
	}

	repo.On("GetNextJobsToRun", mock.Anything).Return([]*domain.CronJob{job}, nil)

	// Expectations for recording run
	repo.On("SaveJobRun", mock.Anything, mock.MatchedBy(func(run *domain.CronJobRun) bool {
		return run.JobID == job.ID && run.Status == "SUCCESS"
	})).Return(nil)

	// Expectations for updating job
	repo.On("UpdateJob", mock.Anything, mock.MatchedBy(func(j *domain.CronJob) bool {
		return j.ID == job.ID && j.LastRunAt != nil && j.NextRunAt != nil
	})).Return(nil)

	// Call the exported method
	worker.ProcessJobs(context.Background())

	// Should wait for goroutines? ProcessJobs launches goroutines.
	// Since w.runJob is called in a goroutine, we need to wait.
	// But w.runJob is not exposed to wait on.
	// However, simple sleep might work for this test, or we mock the "go" keyword? No.
	//
	// The right way is to ensure reproducibility.
	// Verify that the expectations are met eventually.
	// Mock testify AssertExpectations doesn't wait.
	//
	// We can use `Eventually` or just a small sleep since we are testing concurrency.
	time.Sleep(100 * time.Millisecond)

	repo.AssertExpectations(t)
}
