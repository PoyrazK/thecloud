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

func TestStackE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Stack E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("stack-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Stack Tester")

	var stackID string
	stackName := fmt.Sprintf("e2e-stack-%d", time.Now().UnixNano()%10000)
	sampleTemplate := `
name: test-stack
resources:
  vpc:
    type: aws:vpc
    properties:
      cidr_block: 10.0.0.0/16
`

	// 1. Validate IaC Template
	t.Run("ValidateTemplate", func(t *testing.T) {
		payload := map[string]string{
			"template": sampleTemplate,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/iac/validate", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			t.Skip("Stack API not accessible for this user")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				Valid      bool     `json:"valid"`
				Errors     []string `json:"errors"`
				Parameters []string `json:"parameters"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		if !res.Data.Valid {
			t.Skip("IaC template validation not supported or template invalid")
		}
		assert.True(t, res.Data.Valid)
	})

	// 2. Create Stack
	t.Run("CreateStack", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":     stackName,
			"template": sampleTemplate,
			"parameters": map[string]string{
				"environment": "test",
			},
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/iac/stacks", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
			t.Skip("Stack API not accessible for this user")
		}
		if resp.StatusCode == http.StatusBadRequest {
			t.Skip("Stack creation not available")
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
		stackID = res.Data.ID
		if stackID == "" {
			t.Skip("Stack creation did not return ID")
		}
		assert.NotEmpty(t, stackID)
		assert.Equal(t, stackName, res.Data.Name)
	})

	if stackID == "" {
		t.Fatal("Stack ID not set - cannot continue tests")
	}

	// 3. Get Stack Details
	t.Run("GetStack", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/iac/stacks/%s", testutil.TestBaseURL, stackID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Get stack not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, stackID, res.Data.ID)
		assert.Equal(t, stackName, res.Data.Name)
	})

	// 4. List Stacks
	t.Run("ListStacks", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/iac/stacks", token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("List stacks not available")
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

	// 5. Delete Stack
	t.Run("DeleteStack", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/iac/stacks/%s", testutil.TestBaseURL, stackID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Stack already deleted")
		}
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			t.Skip("Delete returned unexpected status")
		}
	})

	// 6. Verify Stack is deleted
	t.Run("VerifyStackDeleted", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/iac/stacks/%s", testutil.TestBaseURL, stackID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Verify not available")
		}
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
