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

func TestLoadbalancerE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Loadbalancer E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("lb-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Loadbalancer Tester")

	// Create VPC for loadbalancer
	vpcID := createTestVPC(t, client, token, fmt.Sprintf("lb-vpc-%d", time.Now().UnixNano()))
	defer deleteVPC(t, client, token, vpcID)

	var lbID string
	lbName := fmt.Sprintf("e2e-lb-%d", time.Now().UnixNano()%10000)

	// 1. Create Loadbalancer
	t.Run("CreateLoadbalancer", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":      lbName,
			"vpc_id":    vpcID,
			"port":      80,
			"algorithm": "round-robin",
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/lb", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Loadbalancer API not accessible for this user")
		}

		// May return 202 Accepted (async) or 201 Created
		if resp.StatusCode == http.StatusAccepted {
			var res struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
			lbID = res.Data.ID
		} else {
			require.Equal(t, http.StatusCreated, resp.StatusCode)
			var res struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
			lbID = res.Data.ID
		}
		assert.NotEmpty(t, lbID)
	})

	if lbID == "" {
		t.Fatal("Loadbalancer ID not set - cannot continue tests")
	}

	// 2. Get Loadbalancer Details
	t.Run("GetLoadbalancer", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/lb/%s", testutil.TestBaseURL, lbID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				Algorithm string `json:"algorithm"`
				Port      int    `json:"port"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, lbID, res.Data.ID)
		assert.Equal(t, lbName, res.Data.Name)
	})

	// 3. List Loadbalancers
	t.Run("ListLoadbalancers", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/lb", token)
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

	// 4. Add Target to Loadbalancer
	t.Run("AddTarget", func(t *testing.T) {
		// Create an instance to add as target
		instanceID := volCreateTestInstance(t, client, token, vpcID)
		defer volDeleteInstance(t, client, token, instanceID)

		payload := map[string]interface{}{
			"instance_id": instanceID,
			"port":        8080,
			"weight":      1,
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/lb/%s/targets", testutil.TestBaseURL, lbID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		// May return 201 Created or 400 if LB not ready
		if resp.StatusCode == http.StatusBadRequest {
			t.Skip("Loadbalancer not ready for target addition")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID         string `json:"id"`
				InstanceID string `json:"instance_id"`
				Port       int    `json:"port"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, instanceID, res.Data.InstanceID)
		assert.Equal(t, 8080, res.Data.Port)
	})

	// 5. List Loadbalancer Targets
	t.Run("ListTargets", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/lb/%s/targets", testutil.TestBaseURL, lbID), token)
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

	// 6. Delete Loadbalancer
	t.Run("DeleteLoadbalancer", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/lb/%s", testutil.TestBaseURL, lbID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 7. Verify Loadbalancer is deleted
	t.Run("VerifyLoadbalancerDeleted", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/lb/%s", testutil.TestBaseURL, lbID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
