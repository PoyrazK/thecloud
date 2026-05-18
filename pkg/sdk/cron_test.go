package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_CreateCronJob(t *testing.T) {
	expectedJob := CronJob{
		ID:            "cron-1",
		Name:          "test-cron",
		Schedule:      "0 0 * * *",
		TargetURL:     "https://example.com/webhook",
		TargetMethod:  "POST",
		TargetPayload: `{"message": "hello"}`,
		Status:        "active",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/cron/jobs", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req struct {
			Name          string `json:"name"`
			Schedule      string `json:"schedule"`
			TargetURL     string `json:"target_url"`
			TargetMethod  string `json:"target_method"`
			TargetPayload string `json:"target_payload"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, expectedJob.Name, req.Name)
		assert.Equal(t, expectedJob.Schedule, req.Schedule)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(expectedJob)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	job, err := client.CreateCronJob("test-cron", "0 0 * * *", "https://example.com/webhook", "POST", `{"message": "hello"}`)

	require.NoError(t, err)
	assert.NotNil(t, job)
	assert.Equal(t, expectedJob.ID, job.ID)
}

func TestClient_ListCronJobs(t *testing.T) {
	expectedJobs := []CronJob{
		{ID: "cron-1", Name: "job-1"},
		{ID: "cron-2", Name: "job-2"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/cron/jobs", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Response[[]CronJob]{Data: expectedJobs})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	jobs, err := client.ListCronJobs()

	require.NoError(t, err)
	assert.Len(t, jobs, 2)
}

func TestClient_GetCronJob(t *testing.T) {
	id := "cron-123"
	expectedJob := CronJob{
		ID:   id,
		Name: "test-cron",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/cron/jobs/"+id, r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(expectedJob)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	job, err := client.GetCronJob(id)

	require.NoError(t, err)
	assert.NotNil(t, job)
	assert.Equal(t, expectedJob.ID, job.ID)
}

func TestClient_PauseCronJob(t *testing.T) {
	id := "cron-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/cron/jobs/"+id+"/pause", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	err := client.PauseCronJob(id)

	require.NoError(t, err)
}

func TestClient_ResumeCronJob(t *testing.T) {
	id := "cron-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/cron/jobs/"+id+"/resume", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	err := client.ResumeCronJob(id)

	require.NoError(t, err)
}

func TestClient_DeleteCronJob(t *testing.T) {
	id := "cron-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/cron/jobs/"+id, r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	err := client.DeleteCronJob(id)

	require.NoError(t, err)
}
