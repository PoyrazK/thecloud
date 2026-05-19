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

func TestIdentityE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Identity E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("identity-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Identity Tester")

	var apiKeyID string
	keyName := fmt.Sprintf("e2e-api-key-%d", time.Now().UnixNano()%10000)

	// 1. Create API Key
	t.Run("CreateAPIKey", func(t *testing.T) {
		payload := map[string]string{
			"name": keyName,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/auth/keys", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Identity API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Key  string `json:"key"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		apiKeyID = res.Data.ID
		assert.NotEmpty(t, apiKeyID)
		assert.Equal(t, keyName, res.Data.Name)
		assert.NotEmpty(t, res.Data.Key)
		assert.Contains(t, res.Data.Key, "thecloud_") // API key prefix
	})

	if apiKeyID == "" {
		t.Fatal("API Key ID not set - cannot continue tests")
	}

	// 2. List API Keys
	t.Run("ListAPIKeys", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/auth/keys", token)
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

	// 3. Rotate API Key
	t.Run("RotateAPIKey", func(t *testing.T) {
		resp := postRequest(t, client, fmt.Sprintf("%s/auth/keys/%s/rotate", testutil.TestBaseURL, apiKeyID), token, nil)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID   string `json:"id"`
				Key  string `json:"key"`
				Name string `json:"name"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.NotEmpty(t, res.Data.Key)
		assert.Contains(t, res.Data.Key, "thecloud_")
	})

	// 4. Revoke (Delete) API Key
	t.Run("RevokeAPIKey", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/auth/keys/%s", testutil.TestBaseURL, apiKeyID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	// 5. Verify API Key is revoked
	t.Run("VerifyAPIKeyRevoked", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/auth/keys/%s", testutil.TestBaseURL, apiKeyID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
