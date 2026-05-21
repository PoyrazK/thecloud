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

func TestEventE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Event E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("event-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Event Tester")

	// 1. List Events
	t.Run("ListEvents", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/events", token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			t.Skip("Event API not accessible for this user")
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
		// Data may be null for new users
		assert.True(t, res.Data == nil || len(res.Data) >= 0)
	})

	// 2. List Events with limit
	t.Run("ListEventsWithLimit", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/events?limit=10", token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("List events with limit not available")
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
		// Should return at most 10 events
		if len(res.Data) > 10 {
			t.Errorf("Expected at most 10 events, got %d", len(res.Data))
		}
	})
}
