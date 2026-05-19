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

func TestSSHKeyE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing SSH Key E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("sshkey-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "SSH Key Tester")

	// Sample valid SSH public key for testing
	testPublicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC+tm9kZW9tb3NlLXByb2plY3Qta2V5LWFsdGVybmF0aXZlLW9ubHktc2lnbmluZy1wcm9qZWN0LWtleS1hbHRlcm5hdGl2ZS1vbmx5LXNpZ25pbmctcHJvamVjdC1rZXktYWx0ZXJuYXRpdmUtb25seS1zaWduaW5nLXByb2plY3Qta2V5LWFsdGVybmF0aXZlLW9ubHktc2lnbmluZy1wcm9qZWN0LWtleS1hbHRlcm5hdGl2ZS1vbmx5LXNpZ25pbmctcHJvamVjdC1rZXktYWx0ZXJuYXRpdmUta2V5LW1vY2sta2V5LWZvci10ZXN0aW5nLXB1cnBvc2VzLW9ubHktdGVzdC1rZXktZm9yLXRoZWNsb3VkLWxhYi1pbnRlcm5hbC10ZXN0aW5nLW9ubHktdGVzdC1rZXktcHVycG9zZXMtb25seS1pbnRlcm5hbC10ZXN0aW5nLW9ubHkgdGVzdEB0ZXN0LmNvbQ=="

	var sshKeyID string
	keyName := fmt.Sprintf("e2e-ssh-key-%d", time.Now().UnixNano()%10000)

	// 1. Create SSH Key
	t.Run("CreateSSHKey", func(t *testing.T) {
		payload := map[string]string{
			"name":       keyName,
			"public_key": testPublicKey,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/ssh-keys", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("SSH Key API not accessible for this user")
		}

		if resp.StatusCode == http.StatusBadRequest {
			t.Skip("SSH Key API rejected the public key format")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Fingerprint string `json:"fingerprint"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		sshKeyID = res.Data.ID
		assert.NotEmpty(t, sshKeyID)
		assert.Equal(t, keyName, res.Data.Name)
		assert.NotEmpty(t, res.Data.Fingerprint)
	})

	if sshKeyID == "" {
		t.Fatal("SSH Key ID not set - cannot continue tests")
	}

	// 2. Get SSH Key Details
	t.Run("GetSSHKey", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/ssh-keys/%s", testutil.TestBaseURL, sshKeyID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, sshKeyID, res.Data.ID)
		assert.Equal(t, keyName, res.Data.Name)
	})

	// 3. List SSH Keys
	t.Run("ListSSHKeys", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/ssh-keys", token)
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

	// 4. Create SSH Key with duplicate name (should fail)
	t.Run("CreateSSHKeyDuplicate", func(t *testing.T) {
		payload := map[string]string{
			"name":       keyName,
			"public_key": testPublicKey,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/ssh-keys", token, payload)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	// 5. Delete SSH Key
	t.Run("DeleteSSHKey", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/ssh-keys/%s", testutil.TestBaseURL, sshKeyID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	// 6. Verify SSH Key is deleted
	t.Run("VerifySSHKeyDeleted", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/ssh-keys/%s", testutil.TestBaseURL, sshKeyID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	// 7. Create SSH Key with invalid key format (should fail)
	t.Run("CreateSSHKeyInvalid", func(t *testing.T) {
		payload := map[string]string{
			"name":       "invalid-key",
			"public_key": "not-a-valid-ssh-key",
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/ssh-keys", token, payload)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
