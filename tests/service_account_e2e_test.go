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

func TestServiceAccountE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Service Account E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("sa-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Service Account Tester")

	var serviceAccountID string
	saName := fmt.Sprintf("e2e-sa-%d", time.Now().UnixNano()%10000)

	// 1. Create Service Account
	t.Run("CreateServiceAccount", func(t *testing.T) {
		payload := map[string]string{
			"name": saName,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/iam/service-accounts", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Service Account API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				ClientID string `json:"client_id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		serviceAccountID = res.Data.ID
		assert.NotEmpty(t, serviceAccountID)
		assert.Equal(t, saName, res.Data.Name)
		assert.NotEmpty(t, res.Data.ClientID)
	})

	if serviceAccountID == "" {
		t.Fatal("Service Account ID not set - cannot continue tests")
	}

	// 2. Get Service Account Details
	t.Run("GetServiceAccount", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/iam/service-accounts/%s", testutil.TestBaseURL, serviceAccountID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, serviceAccountID, res.Data.ID)
		assert.Equal(t, saName, res.Data.Name)
	})

	// 3. List Service Accounts
	t.Run("ListServiceAccounts", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/iam/service-accounts", token)
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

	// 4. Delete Service Account
	t.Run("DeleteServiceAccount", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/iam/service-accounts/%s", testutil.TestBaseURL, serviceAccountID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	// 5. Verify Service Account is deleted
	t.Run("VerifyServiceAccountDeleted", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/iam/service-accounts/%s", testutil.TestBaseURL, serviceAccountID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}