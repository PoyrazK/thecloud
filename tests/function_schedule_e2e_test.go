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

func TestFunctionScheduleE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Function Schedule E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("fn-sched-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Function Schedule Tester")

	var scheduleID string
	scheduleName := fmt.Sprintf("e2e-fn-sched-%d", time.Now().UnixNano()%10000)
	functionID := "00000000-0000-0000-0000-000000000001" // Placeholder - function must exist

	// 1. Create Function Schedule
	t.Run("CreateFunctionSchedule", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":       scheduleName,
			"function_id": functionID,
			"schedule":   "*/5 * * * *", // Every 5 minutes
			"payload":    map[string]string{"key": "value"},
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/function-schedules", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Function Schedule API not accessible for this user")
		}

		if resp.StatusCode == http.StatusBadRequest {
			t.Skip("Function ID does not exist or schedule creation not available")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		scheduleID = res.Data.ID
		assert.NotEmpty(t, scheduleID)
		assert.Equal(t, scheduleName, res.Data.Name)
		assert.Equal(t, "active", res.Data.Status)
	})

	if scheduleID == "" {
		t.Fatal("Function Schedule ID not set - cannot continue tests")
	}

	// 2. Get Function Schedule Details
	t.Run("GetFunctionSchedule", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/function-schedules/%s", testutil.TestBaseURL, scheduleID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Status string `json:"status"`
				Schedule string `json:"schedule"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, scheduleID, res.Data.ID)
		assert.Equal(t, scheduleName, res.Data.Name)
	})

	// 3. List Function Schedules
	t.Run("ListFunctionSchedules", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/function-schedules", token)
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

	// 4. Pause Function Schedule
	t.Run("PauseFunctionSchedule", func(t *testing.T) {
		resp := postRequest(t, client, fmt.Sprintf("%s/function-schedules/%s/pause", testutil.TestBaseURL, scheduleID), token, nil)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify status changed to paused
		getResp := getRequest(t, client, fmt.Sprintf("%s/function-schedules/%s", testutil.TestBaseURL, scheduleID), token)
		defer func() { _ = getResp.Body.Close() }()

		var getRes struct {
			Data struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(getResp.Body).Decode(&getRes))
		assert.Equal(t, "paused", getRes.Data.Status)
	})

	// 5. Resume Function Schedule
	t.Run("ResumeFunctionSchedule", func(t *testing.T) {
		resp := postRequest(t, client, fmt.Sprintf("%s/function-schedules/%s/resume", testutil.TestBaseURL, scheduleID), token, nil)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify status changed back to active
		getResp := getRequest(t, client, fmt.Sprintf("%s/function-schedules/%s", testutil.TestBaseURL, scheduleID), token)
		defer func() { _ = getResp.Body.Close() }()

		var getRes struct {
			Data struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(getResp.Body).Decode(&getRes))
		assert.Equal(t, "active", getRes.Data.Status)
	})

	// 6. Get Schedule Runs
	t.Run("GetScheduleRuns", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/function-schedules/%s/runs", testutil.TestBaseURL, scheduleID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		// May be empty if no runs yet
		assert.NotNil(t, res.Data)
	})

	// 7. Delete Function Schedule
	t.Run("DeleteFunctionSchedule", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/function-schedules/%s", testutil.TestBaseURL, scheduleID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	// 8. Verify Function Schedule is deleted
	t.Run("VerifyFunctionScheduleDeleted", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/function-schedules/%s", testutil.TestBaseURL, scheduleID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}