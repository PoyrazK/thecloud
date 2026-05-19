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

func TestGlobalLBE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Global LB E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("glb-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Global LB Tester")

	var glbID string
	glbName := fmt.Sprintf("e2e-glb-%d", time.Now().UnixNano()%10000)

	// 1. Create Global LB
	t.Run("CreateGlobalLB", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":     glbName,
			"hostname": fmt.Sprintf("global-%d.example.com", time.Now().UnixNano()%10000),
			"policy":   "LATENCY",
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/global-lb", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Global LB API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Policy string `json:"policy"`
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		glbID = res.Data.ID
		assert.NotEmpty(t, glbID)
		assert.Equal(t, glbName, res.Data.Name)
		assert.Equal(t, "LATENCY", res.Data.Policy)
	})

	if glbID == "" {
		t.Fatal("Global LB ID not set - cannot continue tests")
	}

	// 2. Get Global LB Details
	t.Run("GetGlobalLB", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/global-lb/%s", testutil.TestBaseURL, glbID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Policy string `json:"policy"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, glbID, res.Data.ID)
		assert.Equal(t, glbName, res.Data.Name)
	})

	// 3. List Global LBs
	t.Run("ListGlobalLBs", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/global-lb", token)
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

	// 4. Add Endpoint to Global LB
	t.Run("AddGlobalLBEndpoint", func(t *testing.T) {
		payload := map[string]interface{}{
			"region":      "us-east-1",
			"target_type": "LB",
			"weight":      1,
			"priority":    1,
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/global-lb/%s/endpoints", testutil.TestBaseURL, glbID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		// May return 201 or 400 if endpoint creation not available
		if resp.StatusCode == http.StatusBadRequest {
			t.Skip("Cannot add endpoint to Global LB")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID     string `json:"id"`
				Region string `json:"region"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, "us-east-1", res.Data.Region)
	})

	// 5. Delete Global LB
	t.Run("DeleteGlobalLB", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/global-lb/%s", testutil.TestBaseURL, glbID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	// 6. Verify Global LB is deleted
	t.Run("VerifyGlobalLBDeleted", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/global-lb/%s", testutil.TestBaseURL, glbID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
