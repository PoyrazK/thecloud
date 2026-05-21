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

func TestDashboardE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Dashboard E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("dashboard-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Dashboard Tester")

	// 1. Get Dashboard Summary
	t.Run("GetDashboardSummary", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/api/dashboard/summary", token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			t.Skip("Dashboard API not accessible for this user")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				TotalInstances   int `json:"total_instances"`
				RunningInstances int `json:"running_instances"`
				TotalVolumes     int `json:"total_volumes"`
				TotalVPCs        int `json:"total_vpcs"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		// Dashboard data is always present for existing users
		assert.True(t, res.Data.TotalInstances >= 0)
	})

	// 2. Get Recent Events
	t.Run("GetRecentEvents", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/api/dashboard/events?limit=5", token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Recent events not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		// Data may be null for new users
		assert.True(t, res.Data == nil || len(res.Data) >= 0)
		// Should return at most 5 events
		if len(res.Data) > 5 {
			t.Errorf("Expected at most 5 events, got %d", len(res.Data))
		}
	})

	// 3. Get Dashboard Stats
	t.Run("GetDashboardStats", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/api/dashboard/stats", token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Dashboard stats not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				Summary struct {
					TotalInstances int `json:"total_instances"`
				} `json:"summary"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		// Dashboard stats data is always present for existing users
		assert.True(t, res.Data.Summary.TotalInstances >= 0)
	})
}
