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

func TestHealthE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Health E2E test: %v", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("health-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Health Tester")

	// 1. Health Check
	t.Run("HealthCheck", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/health", token)
		defer func() { _ = resp.Body.Close() }()

		// Health endpoint may be accessible without auth
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			t.Skip("Health endpoint not accessible")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Status string `json:"status"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.NotEmpty(t, res.Status)
	})

	// 2. Ready Check
	t.Run("ReadyCheck", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/ready", token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			t.Skip("Ready endpoint not accessible")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
