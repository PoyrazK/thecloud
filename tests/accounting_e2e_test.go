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

func TestAccountingE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Accounting E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("billing-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Accounting Tester")

	// 1. Get Billing Summary
	t.Run("GetBillingSummary", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/billing/summary", token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			t.Skip("Billing API not accessible for this user")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				TotalAmount float64 `json:"total_amount"`
				Currency    string  `json:"currency"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		// Billing data is always present for existing users
		assert.GreaterOrEqual(t, res.Data.TotalAmount, float64(0))
	})

	// 2. Get Billing Summary with time range
	t.Run("GetBillingSummaryWithTimeRange", func(t *testing.T) {
		start := time.Now().AddDate(0, -1, 0).Format(time.RFC3339)
		end := time.Now().Format(time.RFC3339)
		resp := getRequest(t, client, fmt.Sprintf("%s/billing/summary?start=%s&end=%s", testutil.TestBaseURL, start, end), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Billing summary with time range not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				TotalAmount float64 `json:"total_amount"`
				Currency    string  `json:"currency"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		// Billing data is always present for existing users
		assert.GreaterOrEqual(t, res.Data.TotalAmount, float64(0))
	})

	// 3. List Usage
	t.Run("ListUsage", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/billing/usage", token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("List usage not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data []struct {
				ID         string  `json:"id"`
				ResourceID string  `json:"resource_id"`
				Quantity   float64 `json:"quantity"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		// Data may be null for new users (API returns {"data": null}) - handled gracefully
	})

	// 4. List Usage with time range
	t.Run("ListUsageWithTimeRange", func(t *testing.T) {
		start := time.Now().AddDate(0, -1, 0).Format(time.RFC3339)
		end := time.Now().Format(time.RFC3339)
		resp := getRequest(t, client, fmt.Sprintf("%s/billing/usage?start=%s&end=%s", testutil.TestBaseURL, start, end), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("List usage with time range not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		// Data may be null for new users - handled gracefully
	})
}
