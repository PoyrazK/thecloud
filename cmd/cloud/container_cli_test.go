package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	containerTestAPIKey = "container-key"
	containerTestID     = "dep-1"
	containerTestName   = "web"
)

func TestCreateDeploymentCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/containers/deployments" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := map[string]interface{}{
			"id":            containerTestID,
			"name":          containerTestName,
			"image":         "nginx:latest",
			"replicas":      1,
			"current_count": 1,
			"ports":         "80:80",
			"status":        "running",
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = containerTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	_ = createDeploymentCmd.Flags().Set("replicas", "1")
	_ = createDeploymentCmd.Flags().Set("ports", "80:80")

	out := captureStdout(t, func() {
		createDeploymentCmd.Run(createDeploymentCmd, []string{containerTestName, "nginx:latest"})
	})
	if !strings.Contains(out, "Deployment created") || !strings.Contains(out, containerTestName) {
		t.Fatalf("expected success output, got: %s", out)
	}
}

func TestListDeploymentsCmd(t *testing.T) {
	type listDeploymentsResponse struct {
		Data []map[string]interface{} `json:"data"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/containers/deployments" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":            containerTestID,
					"name":          containerTestName,
					"image":         "nginx:latest",
					"replicas":      2,
					"current_count": 2,
					"ports":         "80:80",
					"status":        "running",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(listDeploymentsResponse{Data: payload["data"].([]map[string]interface{})})
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = containerTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	out := captureStdout(t, func() {
		listDeploymentsCmd.Run(listDeploymentsCmd, nil)
	})
	if !strings.Contains(out, containerTestName) {
		t.Fatalf("expected list output to include deployment name, got: %s", out)
	}
}

func TestScaleDeploymentInvalidReplicas(t *testing.T) {
	out := captureStdout(t, func() {
		scaleDeploymentCmd.Run(scaleDeploymentCmd, []string{containerTestID, "invalid"})
	})
	if !strings.Contains(out, "invalid replica count") {
		t.Fatalf("expected invalid replica count error, got: %s", out)
	}
}

func TestDeleteDeploymentCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/containers/deployments/"+containerTestID || r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = containerTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	out := captureStdout(t, func() {
		deleteDeploymentCmd.Run(deleteDeploymentCmd, []string{containerTestID})
	})
	if !strings.Contains(out, "Deletion initiated") {
		t.Fatalf("expected deletion output, got: %s", out)
	}
}
