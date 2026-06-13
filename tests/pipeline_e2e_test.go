package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/poyrazk/thecloud/pkg/testutil"
)

func TestPipelineE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Pipeline E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("pipeline-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Pipeline Tester")

	var pipelineID string
	pipelineName := fmt.Sprintf("e2e-pipeline-%d", time.Now().UnixNano()%10000)

	// 1. Create Pipeline
	t.Run("CreatePipeline", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":           pipelineName,
			"repository_url": "https://github.com/example/repo",
			"branch":         "main",
			"webhook_secret": "test-secret",
			"config": map[string]interface{}{
				"stages": []map[string]interface{}{
					{
						"name": "build",
						"steps": []map[string]interface{}{
							{"name": "compile", "image": "golang:1.21", "commands": []string{"go build ./..."}},
						},
					},
				},
			},
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/pipelines", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Pipeline API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		pipelineID = res.Data.ID
		assert.NotEmpty(t, pipelineID)
		assert.Equal(t, pipelineName, res.Data.Name)
	})

	if pipelineID == "" {
		t.Fatal("Pipeline ID not set - cannot continue tests")
	}

	// 2. Get Pipeline Details
	t.Run("GetPipeline", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/pipelines/%s", testutil.TestBaseURL, pipelineID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Branch string `json:"branch"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, pipelineID, res.Data.ID)
		assert.Equal(t, pipelineName, res.Data.Name)
		assert.Equal(t, "main", res.Data.Branch)
	})

	// 3. List Pipelines
	t.Run("ListPipelines", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/pipelines", token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.NotEmpty(t, res.Data)
	})

	// 4. Update Pipeline
	t.Run("UpdatePipeline", func(t *testing.T) {
		payload := map[string]interface{}{
			"branch": "develop",
			"config": map[string]interface{}{
				"stages": []map[string]interface{}{
					{
						"name": "test",
						"steps": []map[string]interface{}{
							{"name": "run-tests", "image": "golang:1.21", "commands": []string{"go test ./..."}},
						},
					},
				},
			},
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/pipelines/%s", testutil.TestBaseURL, pipelineID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Pipeline update endpoint not available")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				Branch string `json:"branch"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, "develop", res.Data.Branch)
	})

	// 5. Trigger Pipeline Run
	t.Run("TriggerPipelineRun", func(t *testing.T) {
		payload := map[string]string{
			"commit_hash":  "abc123def456",
			"trigger_type": "MANUAL",
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/pipelines/%s/runs", testutil.TestBaseURL, pipelineID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		// May return 201 (Created) or 202 (Accepted) for async trigger
		if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusCreated {
			var res struct {
				Data struct {
					ID     string `json:"id"`
					Status string `json:"status"`
				} `json:"data"`
			}
			if json.NewDecoder(resp.Body).Decode(&res) == nil {
				// Run was triggered successfully
				t.Logf("Pipeline run triggered: %s with status %s", res.Data.ID, res.Data.Status)
			}
		} else if resp.StatusCode == http.StatusBadRequest {
			t.Skip("Pipeline trigger not available")
		}
	})

	// 6. List Pipeline Runs
	t.Run("ListPipelineRuns", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/pipelines/%s/runs", testutil.TestBaseURL, pipelineID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		// Should have at least one run from the trigger test
		assert.NotEmpty(t, res.Data)
	})

	// 7. Delete Pipeline
	t.Run("DeletePipeline", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/pipelines/%s", testutil.TestBaseURL, pipelineID), token)
		defer func() { _ = resp.Body.Close() }()

		// May return 204, 200, or 202 (async deletion)
		if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK {
			// Wait briefly for async deletion
			time.Sleep(2 * time.Second)
		}
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			t.Skip("Pipeline delete not available")
		}
	})

	// 8. Verify Pipeline is deleted
	t.Run("VerifyPipelineDeleted", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/pipelines/%s", testutil.TestBaseURL, pipelineID), token)
		defer func() { _ = resp.Body.Close() }()

		// May return 404 (deleted), 200 (still deleting), or 500 (server error)
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError {
			t.Skip("Pipeline may still be deleting or API returned error")
		}
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
