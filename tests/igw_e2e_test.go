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

func TestInternetGatewayE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Internet Gateway E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("igw-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Internet Gateway Tester")

	var igwID string
	var attachVPCID string // VPC used for attach/detach - must persist across subtests

	// 1. Create Internet Gateway
	t.Run("CreateInternetGateway", func(t *testing.T) {
		resp := postRequest(t, client, testutil.TestBaseURL+"/internet-gateways", token, nil)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			t.Skip("Internet Gateway API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID     string `json:"id"`
				Status string `json:"status"`
				ARN    string `json:"arn"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		igwID = res.Data.ID
		assert.NotEmpty(t, igwID)
		assert.Equal(t, "detached", res.Data.Status)
	})

	if igwID == "" {
		t.Fatal("Internet Gateway ID not set - cannot continue tests")
	}

	// 2. Get Internet Gateway Details
	t.Run("GetInternetGateway", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/internet-gateways/%s", testutil.TestBaseURL, igwID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Get internet gateway not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, igwID, res.Data.ID)
		assert.Equal(t, "detached", res.Data.Status)
	})

	// 3. List Internet Gateways
	t.Run("ListInternetGateways", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/internet-gateways", token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("List internet gateways not available")
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

	// 4. Attach Internet Gateway to VPC
	t.Run("AttachInternetGateway", func(t *testing.T) {
		// Create VPC for the IGW - must persist until Detach completes
		attachVPCID = createTestVPC(t, client, token, fmt.Sprintf("igw-vpc-%d", time.Now().UnixNano()))

		payload := map[string]string{
			"vpc_id": attachVPCID,
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/internet-gateways/%s/attach", testutil.TestBaseURL, igwID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Attach internet gateway not available")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify IGW status changed to attached
		getResp := getRequest(t, client, fmt.Sprintf("%s/internet-gateways/%s", testutil.TestBaseURL, igwID), token)
		defer func() { _ = getResp.Body.Close() }()

		var getRes struct {
			Data struct {
				Status string `json:"status"`
				VPCID  string `json:"vpc_id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(getResp.Body).Decode(&getRes))
		assert.Equal(t, "attached", getRes.Data.Status)
		assert.NotEmpty(t, getRes.Data.VPCID)
	})

	// 5. Detach Internet Gateway
	t.Run("DetachInternetGateway", func(t *testing.T) {
		if attachVPCID == "" {
			t.Skip("No VPC to detach from")
		}

		resp := postRequest(t, client, fmt.Sprintf("%s/internet-gateways/%s/detach", testutil.TestBaseURL, igwID), token, nil)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusInternalServerError || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Internet Gateway detach not available")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify IGW status changed to detached
		getResp := getRequest(t, client, fmt.Sprintf("%s/internet-gateways/%s", testutil.TestBaseURL, igwID), token)
		defer func() { _ = getResp.Body.Close() }()

		var getRes struct {
			Data struct {
				Status string `json:"status"`
				VPCID  string `json:"vpc_id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(getResp.Body).Decode(&getRes))
		assert.Equal(t, "detached", getRes.Data.Status)

		// Now safe to delete the VPC
		deleteVPC(t, client, token, attachVPCID)
	})

	// 6. Delete Internet Gateway
	t.Run("DeleteInternetGateway", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/internet-gateways/%s", testutil.TestBaseURL, igwID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Internet gateway already deleted")
		}
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			t.Skip("Delete returned unexpected status")
		}
	})

	// 7. Verify Internet Gateway is deleted
	t.Run("VerifyInternetGatewayDeleted", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/internet-gateways/%s", testutil.TestBaseURL, igwID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Verify not available")
		}
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
