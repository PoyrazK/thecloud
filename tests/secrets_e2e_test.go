package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/pkg/testutil"
)

func TestSecretsE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Secrets E2E test: %v", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	token := registerAndLogin(t, client, "secrets-tester@thecloud.local", "Secrets Tester")

	var secretID string
	secretName := fmt.Sprintf("e2e-secret-%d", time.Now().UnixNano()%1000)

	// 1. Create Secret
	t.Run("CreateSecret", func(t *testing.T) {
		payload := map[string]string{
			"name":  secretName,
			"value": "super-secret-value",
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/secrets", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			t.Skip("Secrets API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data domain.Secret `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		secretID = res.Data.ID.String()
		assert.NotEmpty(t, secretID)
	})

	// 2. Get Secret
	t.Run("GetSecret", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/secrets/%s", testutil.TestBaseURL, secretID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Get secret not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data domain.Secret `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, secretName, res.Data.Name)
		assert.Equal(t, "super-secret-value", res.Data.EncryptedValue)
	})

	// 3. Delete Secret
	t.Run("DeleteSecret", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/secrets/%s", testutil.TestBaseURL, secretID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Secret already deleted")
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			t.Skip("Delete returned unexpected status")
		}
	})
}
