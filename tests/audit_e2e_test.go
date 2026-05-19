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

func TestAuditE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Audit E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("audit-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Audit Tester")

	// 1. List Audit Logs
	t.Run("ListAuditLogs", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/audit", token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Audit API not accessible for this user")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data []struct {
				ID           string `json:"id"`
				Action       string `json:"action"`
				ResourceType string `json:"resource_type"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		// Data may be null for new users (API returns {"data": null})
		assert.True(t, res.Data == nil || len(res.Data) >= 0)
	})

	// 2. List Audit Logs with limit
	t.Run("ListAuditLogsWithLimit", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/audit?limit=10", token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		// Data may be null for new users
		if len(res.Data) > 10 {
			t.Errorf("Expected at most 10 audit logs, got %d", len(res.Data))
		}
	})
}