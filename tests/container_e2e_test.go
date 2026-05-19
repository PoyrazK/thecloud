package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/poyrazk/thecloud/pkg/testutil"
)

func TestContainerE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Container E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("container-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Container Tester")

	var deploymentID string
	deploymentName := fmt.Sprintf("e2e-container-%d", time.Now().UnixNano()%10000)

	// 1. Create Container Deployment
	t.Run("CreateDeployment", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":     deploymentName,
			"image":    "nginx:alpine",
			"replicas": 1,
			"ports":    "80:80",
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/containers/deployments", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Container API not accessible for this user")
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
		deploymentID = res.Data.ID
		assert.NotEmpty(t, deploymentID)
		assert.Equal(t, deploymentName, res.Data.Name)
	})

	if deploymentID == "" {
		t.Fatal("Deployment ID not set - cannot continue tests")
	}

	// 2. Get Deployment Details
	t.Run("GetDeployment", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/containers/deployments/%s", testutil.TestBaseURL, deploymentID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, deploymentID, res.Data.ID)
	})

	// 3. List Deployments
	t.Run("ListDeployments", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/containers/deployments", token)
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

	// 4. Scale Deployment
	t.Run("ScaleDeployment", func(t *testing.T) {
		payload := map[string]interface{}{
			"replicas": 2,
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/containers/deployments/%s/scale", testutil.TestBaseURL, deploymentID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 5. Delete Deployment
	t.Run("DeleteDeployment", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/containers/deployments/%s", testutil.TestBaseURL, deploymentID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	// 6. Verify Deployment is deleted
	t.Run("VerifyDeploymentDeleted", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/containers/deployments/%s", testutil.TestBaseURL, deploymentID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}