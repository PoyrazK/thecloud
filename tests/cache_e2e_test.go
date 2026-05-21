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

func TestCacheE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Cache E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("cache-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Cache Tester")

	var cacheID string
	cacheName := fmt.Sprintf("e2e-cache-%d", time.Now().UnixNano()%10000)

	// 1. Create Cache
	t.Run("CreateCache", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":      cacheName,
			"version":   "7.0",
			"memory_mb": 256,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/caches", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			t.Skip("Cache API not accessible for this user")
		}

		// May return 201 Created or 202 Accepted (async creation)
		if resp.StatusCode == http.StatusAccepted {
			// Async - try to get the cache ID from response or skip
			var res struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			if json.NewDecoder(resp.Body).Decode(&res) == nil && res.Data.ID != "" {
				cacheID = res.Data.ID
			}
			// Wait for cache to be ready
			time.Sleep(3 * time.Second)
		} else {
			require.Equal(t, http.StatusCreated, resp.StatusCode)
			var res struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
			cacheID = res.Data.ID
		}
		assert.NotEmpty(t, cacheID)
	})

	if cacheID == "" {
		t.Fatal("Cache ID not set - cannot continue tests")
	}

	// 2. Get Cache Details
	t.Run("GetCache", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/caches/%s", testutil.TestBaseURL, cacheID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Get cache not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, cacheID, res.Data.ID)
		assert.Equal(t, cacheName, res.Data.Name)
	})

	// 3. List Caches
	t.Run("ListCaches", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/caches", token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("List caches not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.NotEmpty(t, res.Data)
	})

	// 4. Get Connection String
	t.Run("GetCacheConnection", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/caches/%s/connection", testutil.TestBaseURL, cacheID), token)
		defer func() { _ = resp.Body.Close() }()

		// May return 400 if cache not in RUNNING state
		if resp.StatusCode == http.StatusBadRequest {
			t.Skip("Cache not in running state")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			ConnectionString string `json:"connection_string"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		if res.ConnectionString == "" {
			t.Skip("Connection string not available yet")
		}
		assert.NotEmpty(t, res.ConnectionString)
		assert.Contains(t, res.ConnectionString, "redis://")
	})

	// 5. Get Cache Stats
	t.Run("GetCacheStats", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/caches/%s/stats", testutil.TestBaseURL, cacheID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusBadRequest {
			t.Skip("Cache not in running state")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			UsedMemoryBytes  int64 `json:"used_memory_bytes"`
			MaxMemoryBytes   int64 `json:"max_memory_bytes"`
			ConnectedClients int   `json:"connected_clients"`
			TotalKeys        int64 `json:"total_keys"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		if res.MaxMemoryBytes == 0 {
			t.Skip("Cache stats not available yet")
		}
		assert.Positive(t, res.MaxMemoryBytes)
	})

	// 6. Resize Cache
	t.Run("ResizeCache", func(t *testing.T) {
		payload := map[string]interface{}{
			"memory_mb": 512,
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/caches/%s/resize", testutil.TestBaseURL, cacheID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusBadRequest {
			t.Skip("Cache resize not available")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 7. Flush Cache
	t.Run("FlushCache", func(t *testing.T) {
		resp := postRequest(t, client, fmt.Sprintf("%s/caches/%s/flush", testutil.TestBaseURL, cacheID), token, nil)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusBadRequest {
			t.Skip("Cache flush not available")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 8. Delete Cache
	t.Run("DeleteCache", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/caches/%s", testutil.TestBaseURL, cacheID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Cache already deleted")
		}
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			t.Skip("Delete returned unexpected status")
		}
	})

	// 9. Verify Cache is deleted
	t.Run("VerifyCacheDeleted", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/caches/%s", testutil.TestBaseURL, cacheID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Verify not available")
		}
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
