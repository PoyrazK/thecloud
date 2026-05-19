package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	secretsTestAPIKey = "secrets-key"
	secretsTestID     = "secret-1"
	secretsTestName   = "api-token"
)

func TestSecretsListJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/secrets" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":          secretsTestID,
					"name":        secretsTestName,
					"description": "token",
					"created_at":  time.Now().UTC().Format(time.RFC3339),
					"updated_at":  time.Now().UTC().Format(time.RFC3339),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = secretsTestAPIKey
	opts.JSON = true
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
		opts.JSON = false
	}()

	out := captureStdout(t, func() {
		secretsListCmd.Run(secretsListCmd, nil)
	})
	if !strings.Contains(out, secretsTestID) {
		t.Fatalf("expected JSON output to include secret id, got: %s", out)
	}
}

func TestSecretsCreateCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/secrets" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := map[string]interface{}{
			"data": map[string]interface{}{
				"id":          secretsTestID,
				"name":        secretsTestName,
				"description": "token",
				"created_at":  time.Now().UTC().Format(time.RFC3339),
				"updated_at":  time.Now().UTC().Format(time.RFC3339),
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = secretsTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	_ = secretsCreateCmd.Flags().Set("description", "token")

	out := captureStdout(t, func() {
		secretsCreateCmd.Run(secretsCreateCmd, []string{secretsTestName, "secret-value"})
	})
	if !strings.Contains(out, "Secret") || !strings.Contains(out, secretsTestName) {
		t.Fatalf("expected success output, got: %s", out)
	}
}
