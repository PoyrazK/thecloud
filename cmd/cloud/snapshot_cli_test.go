package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

const (
	snapshotTestAPIKey = "snapshot-key"
	snapshotTestID     = "33333333-3333-3333-3333-333333333333"
)

func TestSnapshotListJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/snapshots" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":          snapshotTestID,
					"volume_id":   uuid.New().String(),
					"volume_name": "data",
					"size_gb":     10,
					"status":      "AVAILABLE",
					"created_at":  time.Now().UTC().Format(time.RFC3339),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = snapshotTestAPIKey
	opts.JSON = true
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
		opts.JSON = false
	}()

	out := captureStdout(t, func() {
		snapshotListCmd.Run(snapshotListCmd, nil)
	})
	if !strings.Contains(out, snapshotTestID) {
		t.Fatalf("expected JSON output to include snapshot id, got: %s", out)
	}
}

func TestSnapshotCreateCmd(t *testing.T) {
	volID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/snapshots" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := map[string]interface{}{
			"data": map[string]interface{}{
				"id":          snapshotTestID,
				"volume_id":   volID.String(),
				"volume_name": "data",
				"size_gb":     10,
				"status":      "CREATING",
				"created_at":  time.Now().UTC().Format(time.RFC3339),
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = snapshotTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	_ = snapshotCreateCmd.Flags().Set("desc", "daily")

	out := captureStdout(t, func() {
		snapshotCreateCmd.Run(snapshotCreateCmd, []string{volID.String()})
	})
	if !strings.Contains(out, "Snapshot creation started") || !strings.Contains(out, snapshotTestID) {
		t.Fatalf("expected success output, got: %s", out)
	}
}

func TestSnapshotRestoreCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/snapshots/"+snapshotTestID+"/restore" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := map[string]interface{}{
			"data": map[string]interface{}{
				"id":         uuid.New().String(),
				"name":       "restore-vol",
				"size_gb":    10,
				"status":     "AVAILABLE",
				"created_at": time.Now().UTC().Format(time.RFC3339),
				"updated_at": time.Now().UTC().Format(time.RFC3339),
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = snapshotTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	_ = snapshotRestoreCmd.Flags().Set("name", "restore-vol")

	out := captureStdout(t, func() {
		snapshotRestoreCmd.Run(snapshotRestoreCmd, []string{snapshotTestID})
	})
	if !strings.Contains(out, "Snapshot restored") || !strings.Contains(out, "restore-vol") {
		t.Fatalf("expected restore output, got: %s", out)
	}
}

func TestSnapshotDeleteCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/snapshots/"+snapshotTestID || r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = snapshotTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	out := captureStdout(t, func() {
		snapshotDeleteCmd.Run(snapshotDeleteCmd, []string{snapshotTestID})
	})
	if !strings.Contains(out, "Snapshot") || !strings.Contains(out, snapshotTestID) {
		t.Fatalf("expected delete output, got: %s", out)
	}
}
