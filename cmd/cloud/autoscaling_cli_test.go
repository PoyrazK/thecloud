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
	asgTestAPIKey = "asg-key"
	asgTestID     = "asg-1"
	asgTestName   = "web-scaling"
)

func TestASGCreateJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/autoscaling/groups" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := map[string]interface{}{
			"data": map[string]interface{}{
				"id":            asgTestID,
				"name":          asgTestName,
				"vpc_id":        "vpc-1",
				"image":         "nginx:latest",
				"min_instances": 1,
				"max_instances": 3,
				"desired_count": 2,
				"current_count": 1,
				"status":        "active",
				"created_at":    time.Now().UTC().Format(time.RFC3339),
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = asgTestAPIKey
	opts.JSON = true
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
		opts.JSON = false
	}()

	_ = asgCreateCmd.Flags().Set("name", asgTestName)
	_ = asgCreateCmd.Flags().Set("vpc", "vpc-1")
	_ = asgCreateCmd.Flags().Set("image", "nginx:latest")
	_ = asgCreateCmd.Flags().Set("min", "1")
	_ = asgCreateCmd.Flags().Set("max", "3")
	_ = asgCreateCmd.Flags().Set("desired", "2")

	out := captureStdout(t, func() {
		asgCreateCmd.Run(asgCreateCmd, nil)
	})
	if !strings.Contains(out, asgTestName) {
		t.Fatalf("expected JSON output to include group name, got: %s", out)
	}
}

func TestASGListJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/autoscaling/groups" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":            asgTestID,
					"name":          asgTestName,
					"vpc_id":        "vpc-1",
					"image":         "nginx:latest",
					"min_instances": 1,
					"max_instances": 3,
					"desired_count": 2,
					"current_count": 1,
					"status":        "active",
					"created_at":    time.Now().UTC().Format(time.RFC3339),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = asgTestAPIKey
	opts.JSON = true
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
		opts.JSON = false
	}()

	out := captureStdout(t, func() {
		asgListCmd.Run(asgListCmd, nil)
	})
	if !strings.Contains(out, asgTestID) {
		t.Fatalf("expected JSON output to include group id, got: %s", out)
	}
}

func TestASGDeleteCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Handle list request for ID resolution
		if r.URL.Path == "/autoscaling/groups" && r.Method == http.MethodGet {
			payload := map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"id":            asgTestID,
						"name":          asgTestName,
						"vpc_id":        "vpc-1",
						"image":         "nginx:latest",
						"min_instances": 1,
						"max_instances": 3,
						"desired_count": 2,
						"current_count": 1,
						"status":        "active",
						"created_at":    time.Now().UTC().Format(time.RFC3339),
					},
				},
			}
			_ = json.NewEncoder(w).Encode(payload)
			return
		}
		// Handle delete request
		if r.URL.Path != "/autoscaling/groups/"+asgTestID || r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = asgTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	out := captureStdout(t, func() {
		asgRmCmd.Run(asgRmCmd, []string{asgTestID})
	})
	if !strings.Contains(out, "Scaling Group deleted") {
		t.Fatalf("expected success output, got: %s", out)
	}
}

func TestASGPolicyAddCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/autoscaling/groups" && r.Method == http.MethodGet {
			payload := map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"id":            asgTestID,
						"name":          asgTestName,
						"vpc_id":        "vpc-1",
						"image":         "nginx:latest",
						"min_instances": 1,
						"max_instances": 3,
						"desired_count": 2,
						"current_count": 1,
						"status":        "active",
						"created_at":    time.Now().UTC().Format(time.RFC3339),
					},
				},
			}
			_ = json.NewEncoder(w).Encode(payload)
			return
		}
		if r.URL.Path != "/autoscaling/groups/"+asgTestID+"/policies" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = asgTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	_ = asgPolicyAddCmd.Flags().Set("name", "cpu-80")
	_ = asgPolicyAddCmd.Flags().Set("metric", "cpu")
	_ = asgPolicyAddCmd.Flags().Set("target", "80")
	_ = asgPolicyAddCmd.Flags().Set("scale-out", "1")
	_ = asgPolicyAddCmd.Flags().Set("scale-in", "1")
	_ = asgPolicyAddCmd.Flags().Set("cooldown", "60")

	out := captureStdout(t, func() {
		asgPolicyAddCmd.Run(asgPolicyAddCmd, []string{asgTestID})
	})
	if !strings.Contains(out, "Policy added") {
		t.Fatalf("expected success output, got: %s", out)
	}
}
