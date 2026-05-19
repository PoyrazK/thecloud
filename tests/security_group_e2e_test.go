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

func TestSecurityGroupE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Security Group E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("sg-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Security Group Tester")

	// Create VPC for security group
	vpcID := createTestVPC(t, client, token, fmt.Sprintf("sg-vpc-%d", time.Now().UnixNano()))
	defer deleteVPC(t, client, token, vpcID)

	var sgID string
	sgName := fmt.Sprintf("e2e-sg-%d", time.Now().UnixNano()%10000)

	// 1. Create Security Group
	t.Run("CreateSecurityGroup", func(t *testing.T) {
		payload := map[string]string{
			"vpc_id":     vpcID,
			"name":       sgName,
			"description": "E2E test security group",
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/security-groups", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Security Group API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		sgID = res.Data.ID
		assert.NotEmpty(t, sgID)
		assert.Equal(t, sgName, res.Data.Name)
	})

	if sgID == "" {
		t.Fatal("Security Group ID not set - cannot continue tests")
	}

	// 2. Get Security Group Details
	t.Run("GetSecurityGroup", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/security-groups/%s", testutil.TestBaseURL, sgID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, sgID, res.Data.ID)
		assert.Equal(t, sgName, res.Data.Name)
	})

	// 3. List Security Groups
	t.Run("ListSecurityGroups", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/security-groups?vpc_id=%s", testutil.TestBaseURL, vpcID), token)
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

	// 4. Add Rule to Security Group
	t.Run("AddSecurityGroupRule", func(t *testing.T) {
		payload := map[string]interface{}{
			"direction": "ingress",
			"protocol":  "tcp",
			"port":      8080,
			"cidr":      "0.0.0.0/0",
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/security-groups/%s/rules", testutil.TestBaseURL, sgID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.NotEmpty(t, res.Data.ID)
	})

	// 5. Attach Security Group to Instance
	t.Run("AttachSecurityGroup", func(t *testing.T) {
		// Create an instance to attach the security group to
		instanceID := volCreateTestInstance(t, client, token, vpcID)
		defer volDeleteInstance(t, client, token, instanceID)

		payload := map[string]string{
			"instance_id": instanceID,
			"group_id":   sgID,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/security-groups/attach", token, payload)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 6. Detach Security Group from Instance
	t.Run("DetachSecurityGroup", func(t *testing.T) {
		instanceID := volCreateTestInstance(t, client, token, vpcID)
		defer volDeleteInstance(t, client, token, instanceID)

		// First attach
		attachPayload := map[string]string{
			"instance_id": instanceID,
			"group_id":   sgID,
		}
		attachResp := postRequest(t, client, testutil.TestBaseURL+"/security-groups/attach", token, attachPayload)
		defer func() { _ = attachResp.Body.Close() }()

		// Then detach
		detachPayload := map[string]string{
			"instance_id": instanceID,
			"group_id":   sgID,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/security-groups/detach", token, detachPayload)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 7. Delete Security Group
	t.Run("DeleteSecurityGroup", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/security-groups/%s", testutil.TestBaseURL, sgID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	// 8. Verify Security Group is deleted
	t.Run("VerifySecurityGroupDeleted", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/security-groups/%s", testutil.TestBaseURL, sgID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}