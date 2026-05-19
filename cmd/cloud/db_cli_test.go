package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/poyrazk/thecloud/pkg/sdk"
)

const (
	dbTestAPIKey = "db-key"
	dbTestID     = "db-1"
	dbTestName   = "appdb"
)

func TestDBListJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/databases" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":         dbTestID,
					"name":       dbTestName,
					"engine":     "postgres",
					"version":    "16",
					"status":     "available",
					"port":       5432,
					"username":   "admin",
					"created_at": time.Now().UTC().Format(time.RFC3339),
					"updated_at": time.Now().UTC().Format(time.RFC3339),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = dbTestAPIKey
	opts.JSON = true
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
		opts.JSON = false
	}()

	out := captureStdout(t, func() {
		dbListCmd.Run(dbListCmd, nil)
	})
	if !strings.Contains(out, dbTestID) {
		t.Fatalf("expected JSON output to include database id, got: %s", out)
	}
}

func TestDBCreateJSONMasksPassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/databases" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := map[string]interface{}{
			"data": map[string]interface{}{
				"id":         dbTestID,
				"name":       dbTestName,
				"engine":     "postgres",
				"version":    "16",
				"status":     "available",
				"port":       5432,
				"username":   "admin",
				"password":   "secret",
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
	opts.APIKey = dbTestAPIKey
	opts.JSON = true
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
		opts.JSON = false
	}()

	_ = dbCreateCmd.Flags().Set("name", dbTestName)
	_ = dbCreateCmd.Flags().Set("engine", "postgres")
	_ = dbCreateCmd.Flags().Set("version", "16")

	out := captureStdout(t, func() {
		dbCreateCmd.Run(dbCreateCmd, nil)
	})
	if strings.Contains(out, "secret") {
		t.Fatalf("expected password to be masked, got: %s", out)
	}
	if !strings.Contains(out, "********") {
		t.Fatalf("expected masked password, got: %s", out)
	}
}

func TestDBConnectionCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/databases/"+dbTestID+"/connection" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := map[string]interface{}{
			"data": map[string]string{"connection_string": "postgres://user:pass@localhost:5432/db"},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = dbTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	out := captureStdout(t, func() {
		dbConnCmd.Run(dbConnCmd, []string{dbTestID})
	})
	if !strings.Contains(out, "Connection String") {
		t.Fatalf("expected connection string output, got: %s", out)
	}
}

func TestDBCreateSizeValidation(t *testing.T) {
	tests := []struct {
		name        string
		size        int
		wantErr     bool
		errContains string
	}{
		{name: "size too small", size: 5, wantErr: true, errContains: "--size must be at least 10GB"},
		{name: "size at minimum", size: 10, wantErr: false},
		{name: "size above minimum", size: 100, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotReq sdk.CreateDatabaseInput
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path != "/databases" || r.Method != http.MethodPost {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				payload := map[string]interface{}{
					"data": map[string]interface{}{
						"id":         dbTestID,
						"name":       dbTestName,
						"engine":     "postgres",
						"version":    "16",
						"status":     "available",
						"port":       5432,
						"username":   "admin",
						"password":   "secret",
						"created_at": time.Now().UTC().Format(time.RFC3339),
						"updated_at": time.Now().UTC().Format(time.RFC3339),
					},
				}
				if err := json.NewEncoder(w).Encode(payload); err != nil {
					t.Fatalf("failed to encode response: %v", err)
				}
			}))
			defer server.Close()

			oldURL := opts.APIURL
			oldKey := opts.APIKey
			opts.APIURL = server.URL
			opts.APIKey = dbTestAPIKey
			defer func() {
				opts.APIURL = oldURL
				opts.APIKey = oldKey
			}()

			_ = dbCreateCmd.Flags().Set("name", dbTestName)
			_ = dbCreateCmd.Flags().Set("engine", "postgres")
			_ = dbCreateCmd.Flags().Set("version", "16")
			_ = dbCreateCmd.Flags().Set("size", fmt.Sprintf("%d", tt.size))

			out := captureStdout(t, func() {
				dbCreateCmd.Run(dbCreateCmd, nil)
			})

			if tt.wantErr {
				if !strings.Contains(out, tt.errContains) {
					t.Fatalf("expected error containing %q, got: %s", tt.errContains, out)
				}
			} else {
				if !strings.Contains(out, "[SUCCESS]") {
					t.Fatalf("expected success, got: %s", out)
				}
				if gotReq.AllocatedStorage != tt.size {
					t.Fatalf("expected AllocatedStorage %d, got %d", tt.size, gotReq.AllocatedStorage)
				}
			}

			_ = dbCreateCmd.Flags().Set("size", "10")
		})
	}
}
