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

func TestAdminE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Admin E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("admin-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Admin Tester")

	// 1. Reset Circuit Breakers (admin operation)
	t.Run("ResetCircuitBreakers", func(t *testing.T) {
		resp := postRequest(t, client, testutil.TestBaseURL+"/internal/admin/reset-circuit-breakers", token, nil)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Admin API not accessible for this user")
		}

		// May return 200 OK or 404 if internal routes not exposed
		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Internal admin endpoint not exposed")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Reset bool `json:"reset"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.True(t, res.Reset)
	})
}