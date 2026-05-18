package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/poyrazk/thecloud/pkg/sdk"
)

const (
	cronTestAPIKey   = "cron-key"
	cronTestJobID    = "cron-1"
	cronTestJobName  = "nightly"
	cronTestSchedule = "0 0 * * *"
	cronTestURL      = "https://example.com/hook"
)

func TestCreateCronCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/cron/jobs" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		job := map[string]interface{}{
			"id":       cronTestJobID,
			"name":     cronTestJobName,
			"schedule": cronTestSchedule,
			"url":      cronTestURL,
			"status":   "active",
			"next_run": "2024-01-01T00:00:00Z",
		}
		_ = json.NewEncoder(w).Encode(job)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = cronTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	_ = createCronCmd.Flags().Set("method", "POST")
	_ = createCronCmd.Flags().Set("payload", "{}")

	out := captureStdout(t, func() {
		createCronCmd.Run(createCronCmd, []string{cronTestJobName, cronTestSchedule, cronTestURL})
	})
	if !strings.Contains(out, "Cron job created") || !strings.Contains(out, cronTestJobID) {
		t.Fatalf("expected success output, got: %s", out)
	}
}

func TestPauseCronCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/cron/jobs/"+cronTestJobID+"/pause" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = cronTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	out := captureStdout(t, func() {
		pauseCronCmd.Run(pauseCronCmd, []string{cronTestJobID})
	})
	if !strings.Contains(out, "Job paused") {
		t.Fatalf("expected success output, got: %s", out)
	}
}

func TestDeleteCronCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/cron/jobs/"+cronTestJobID || r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = cronTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	out := captureStdout(t, func() {
		deleteCronCmd.Run(deleteCronCmd, []string{cronTestJobID})
	})
	if !strings.Contains(out, "Job deleted") {
		t.Fatalf("expected success output, got: %s", out)
	}
}

func TestResolveCronJobIDByName(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cron/jobs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sdk.Response[[]sdk.CronJob]{
			Data: []sdk.CronJob{
				{ID: "uuid-456", Name: "nightly-backup", Status: "active"},
			},
		})
	}))
	defer server.Close()

	client := sdk.NewClient(server.URL, "test-key")
	resolved := resolveCronJobID("nightly-backup", client)
	if resolved != "uuid-456" {
		t.Fatalf("expected uuid-456, got %s", resolved)
	}
}

func TestResolveCronJobIDByUUID(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound) // Should not be called
	}))
	defer server.Close()

	client := sdk.NewClient(server.URL, "test-key")
	id := "abc123-def456"
	resolved := resolveCronJobID(id, client)
	if resolved != id {
		t.Fatalf("expected %s, got %s", id, resolved)
	}
}
