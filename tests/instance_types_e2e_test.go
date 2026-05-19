package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/pkg/testutil"
)

func TestInstanceTypesE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Instance Types E2E test: %v", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	token := registerAndLogin(t, client, "instancetype-tester@thecloud.local", "InstanceType Tester")

	var selectedType domain.InstanceType
	var instanceID string
	instanceName := fmt.Sprintf("e2e-inst-type-%d-%s", time.Now().UnixNano()%1000, uuid.New().String())

	// 1. List Instance Types
	t.Run("ListInstanceTypes", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/instance-types", token)
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data []domain.InstanceType `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		require.NotEmpty(t, res.Data)

		selectedType = res.Data[0]
		assert.NotEmpty(t, selectedType.ID)
	})

	// 2. Launch Instance with selected type
	t.Run("LaunchInstanceWithType", func(t *testing.T) {
		payload := map[string]string{
			"name":          instanceName,
			"image":         "nginx:alpine",
			"ports":         "0:80",
			"instance_type": selectedType.ID,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+testutil.TestRouteInstances, token, payload)
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusAccepted, resp.StatusCode)

		var res struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		instanceID = res.Data.ID
		assert.NotEmpty(t, instanceID)
	})

	// 3. Verify Instance Details
	t.Run("VerifyInstanceDetails", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf(testutil.TestRouteFormat, testutil.TestBaseURL, testutil.TestRouteInstances, instanceID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data domain.Instance `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, instanceName, res.Data.Name)
		assert.Equal(t, selectedType.ID, res.Data.InstanceType)
	})

	// 4. Terminate Instance
	const (
		maxRetries   = 3
		retryDelayMs = 500 // milliseconds between retry attempts
	)
	t.Run("TerminateInstance", func(t *testing.T) {
		var resp *http.Response
		for attempt := 0; attempt < maxRetries; attempt++ {
			// resp is always closed before the next iteration or early return
			if attempt > 0 {
				time.Sleep(time.Duration(attempt*retryDelayMs) * time.Millisecond)
			}
			resp = deleteRequest(t, client, fmt.Sprintf(testutil.TestRouteFormat, testutil.TestBaseURL, testutil.TestRouteInstances, instanceID), token)
			if resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return
			}
			if resp.StatusCode != http.StatusInternalServerError {
				break
			}
			// 500 — check if instance was already deleted by API restart cleanup
			checkResp := getRequest(t, client, fmt.Sprintf(testutil.TestRouteFormat, testutil.TestBaseURL, testutil.TestRouteInstances, instanceID), token)
			checkResp.Body.Close()
			if checkResp.StatusCode == http.StatusNotFound {
				t.Log("Instance already deleted (API restart cleanup during chaos tests)")
				resp.Body.Close()
				return
			}
			resp.Body.Close()
		}
		if resp != nil {
			t.Logf("Instance still exists after %d attempts", maxRetries)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		}
	})
}
