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

func TestClusterE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Cluster E2E test: %v", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("cluster-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Cluster Tester")

	// Create VPC first (cluster needs a VPC)
	vpcID := createTestVPC(t, client, token, fmt.Sprintf("cluster-vpc-%d", time.Now().UnixNano()))
	defer deleteVPC(t, client, token, vpcID)

	var clusterID string
	clusterName := fmt.Sprintf("e2e-cluster-%d", time.Now().UnixNano()%10000)

	// 1. Create Cluster
	t.Run("CreateCluster", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":              clusterName,
			"vpc_id":            vpcID,
			"version":           "v1.29.0",
			"workers":           1,
			"network_isolation": false,
			"ha":                false,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/clusters", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Cluster API not accessible for this user")
		}

		if resp.StatusCode == http.StatusAccepted {
			// Async creation - get the cluster ID from background task
			var res struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
			clusterID = res.Data.ID
		} else {
			require.Equal(t, http.StatusCreated, resp.StatusCode)
			var res struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
			clusterID = res.Data.ID
		}
		assert.NotEmpty(t, clusterID)
	})

	if clusterID == "" {
		t.Fatal("Cluster ID not set - cannot continue tests")
	}

	// 2. Get Cluster Details
	t.Run("GetCluster", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/clusters/%s", testutil.TestBaseURL, clusterID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID      string `json:"id"`
				Name    string `json:"name"`
				Status  string `json:"status"`
				Version string `json:"version"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, clusterName, res.Data.Name)
	})

	// 3. List Clusters
	t.Run("ListClusters", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/clusters", token)
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

	// 4. Get Cluster Kubeconfig
	t.Run("GetClusterKubeconfig", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/clusters/%s/kubeconfig", testutil.TestBaseURL, clusterID), token)
		defer func() { _ = resp.Body.Close() }()

		// May return 200 with kubeconfig or 404 if not ready
		if resp.StatusCode != http.StatusOK {
			t.Skip("Cluster not ready for kubeconfig fetch")
		}

		var res struct {
			Data struct {
				Kubeconfig string `json:"kubeconfig"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.NotEmpty(t, res.Data.Kubeconfig)
	})

	// 5. Get Cluster Health
	t.Run("GetClusterHealth", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/clusters/%s/health", testutil.TestBaseURL, clusterID), token)
		defer func() { _ = resp.Body.Close() }()

		// May return 200 if cluster is running, 400 if not ready, or 404 if endpoint not available
		if resp.StatusCode == http.StatusBadRequest {
			t.Skip("Cluster not in ready state for health check")
		}
		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Cluster health endpoint not available")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 6. Scale Cluster
	t.Run("ScaleCluster", func(t *testing.T) {
		payload := map[string]interface{}{
			"workers": 2,
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/clusters/%s/scale", testutil.TestBaseURL, clusterID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		// May return 200 if scaling succeeds, 400 if cluster not ready, or 404 if endpoint not available
		if resp.StatusCode == http.StatusBadRequest {
			t.Skip("Cluster not ready for scaling")
		}
		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Cluster scale endpoint not available")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 7. Add Node Group
	t.Run("AddNodeGroup", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":          fmt.Sprintf("ng-%d", time.Now().UnixNano()%1000),
			"instance_type": "standard",
			"min_size":      1,
			"max_size":      3,
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/clusters/%s/nodegroups", testutil.TestBaseURL, clusterID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		// May return 200 if successful or 400 if cluster not ready
		if resp.StatusCode == http.StatusBadRequest {
			t.Skip("Cluster not ready for node group operations")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 8. Delete Cluster
	t.Run("DeleteCluster", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/clusters/%s", testutil.TestBaseURL, clusterID), token)
		defer func() { _ = resp.Body.Close() }()

		// May return 202 (accepted) for async deletion or 204 for sync
		if resp.StatusCode == http.StatusAccepted {
			// Wait for deletion to complete
			time.Sleep(5 * time.Second)
		} else {
			assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		}
	})
}
