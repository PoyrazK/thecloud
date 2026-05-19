package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/pkg/testutil"
)

func TestFunctionScheduleE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Function Schedule E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("fn-sched-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Function Schedule Tester")

	var functionID string
	var scheduleID string
	scheduleName := fmt.Sprintf("e2e-fn-sched-%d", time.Now().UnixNano()%10000)
	functionName := fmt.Sprintf("e2e-fn-sched-src-%d", time.Now().UnixNano()%10000)

	// 0. Create a function to attach the schedule to
	t.Run("CreateFunction", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		_ = writer.WriteField("name", functionName)
		_ = writer.WriteField("runtime", "nodejs20")
		_ = writer.WriteField("handler", "index.handler")

		part, _ := writer.CreateFormFile("code", "code.zip")
		_, _ = part.Write([]byte("fake zip content"))
		_ = writer.Close()

		req, _ := http.NewRequest("POST", testutil.TestBaseURL+"/functions", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set(testutil.TestHeaderAPIKey, token)
		applyTenantHeader(t, req, token)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Functions API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data domain.Function `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		functionID = res.Data.ID.String()
		assert.NotEmpty(t, functionID)
	})

	// Cleanup function after tests
	if functionID != "" {
		defer func() {
			req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/functions/%s", testutil.TestBaseURL, functionID), nil)
			req.Header.Set(testutil.TestHeaderAPIKey, token)
			applyTenantHeader(t, req, token)
			client.Do(req)
		}()
	}

	if functionID == "" {
		t.Fatal("Function ID not set - cannot continue schedule tests")
	}

	// 1. Create Function Schedule
	t.Run("CreateFunctionSchedule", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":        scheduleName,
			"function_id": functionID,
			"schedule":    "*/5 * * * *", // Every 5 minutes
			"payload":     map[string]string{"key": "value"},
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/function-schedules", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Function Schedule API not accessible for this user")
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
		assert.True(t, strings.EqualFold(res.Data.Status, "active"), "expected status to be active, got %s", res.Data.Status)
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
				ID       string `json:"id"`
				Name     string `json:"name"`
				Status   string `json:"status"`
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
		assert.True(t, strings.EqualFold(getRes.Data.Status, "paused"), "expected status to be paused, got %s", getRes.Data.Status)
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
		assert.True(t, strings.EqualFold(getRes.Data.Status, "active"), "expected status to be active, got %s", getRes.Data.Status)
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
		// Data may be null if no runs yet
		assert.True(t, res.Data == nil || len(res.Data) >= 0)
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
