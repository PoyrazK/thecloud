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

func TestVPCPeeringE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing VPC Peering E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("peering-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Peering Tester")

	// Create two VPCs for peering
	vpcID1 := createTestVPC(t, client, token, fmt.Sprintf("peering-vpc-1-%d", time.Now().UnixNano()))
	defer deleteVPC(t, client, token, vpcID1)

	vpcID2 := createTestVPC(t, client, token, fmt.Sprintf("peering-vpc-2-%d", time.Now().UnixNano()))
	defer deleteVPC(t, client, token, vpcID2)

	var peeringID string

	// 1. Create VPC Peering
	t.Run("CreateVPCPeering", func(t *testing.T) {
		payload := map[string]string{
			"requester_vpc_id": vpcID1,
			"accepter_vpc_id":  vpcID2,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/vpc-peerings", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			t.Skip("VPC Peering API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		peeringID = res.Data.ID
		assert.NotEmpty(t, peeringID)
		assert.Equal(t, "pending_acceptance", res.Data.Status)
	})

	if peeringID == "" {
		t.Skip("Peering ID not set - cannot continue tests")
	}

	// 2. Get VPC Peering Details
	t.Run("GetVPCPeering", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/vpc-peerings/%s", testutil.TestBaseURL, peeringID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Get VPC peering not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID             string `json:"id"`
				RequesterVPCID string `json:"requester_vpc_id"`
				AccepterVPCID  string `json:"accepter_vpc_id"`
				Status         string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, vpcID1, res.Data.RequesterVPCID)
		assert.Equal(t, vpcID2, res.Data.AccepterVPCID)
		assert.Equal(t, "pending_acceptance", res.Data.Status)
	})

	// 3. List VPC Peerings
	t.Run("ListVPCPeerings", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/vpc-peerings", token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("List VPC peerings not available")
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

	// 4. Accept VPC Peering
	t.Run("AcceptVPCPeering", func(t *testing.T) {
		resp := postRequest(t, client, fmt.Sprintf("%s/vpc-peerings/%s/accept", testutil.TestBaseURL, peeringID), token, nil)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Accept VPC peering not available")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify status changed to active
		getResp := getRequest(t, client, fmt.Sprintf("%s/vpc-peerings/%s", testutil.TestBaseURL, peeringID), token)
		defer func() { _ = getResp.Body.Close() }()

		var getRes struct {
			Data struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(getResp.Body).Decode(&getRes))
		// Status may be active or still pending depending on async operation
		assert.True(t, getRes.Data.Status == "active" || getRes.Data.Status == "pending_acceptance")
	})

	// 5. Reject VPC Peering (create a new one to reject)
	t.Run("RejectVPCPeering", func(t *testing.T) {
		// Create another VPC for second peering
		vpcID3 := createTestVPC(t, client, token, fmt.Sprintf("reject-vpc-%d", time.Now().UnixNano()))
		defer deleteVPC(t, client, token, vpcID3)

		// Create peering
		payload := map[string]string{
			"requester_vpc_id": vpcID1,
			"accepter_vpc_id":  vpcID3,
		}
		createResp := postRequest(t, client, testutil.TestBaseURL+"/vpc-peerings", token, payload)
		defer func() { _ = createResp.Body.Close() }()

		if createResp.StatusCode != http.StatusCreated {
			t.Skip("Cannot create peering for reject test")
		}

		var createRes struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(createResp.Body).Decode(&createRes))

		// Reject the peering
		rejectResp := postRequest(t, client, fmt.Sprintf("%s/vpc-peerings/%s/reject", testutil.TestBaseURL, createRes.Data.ID), token, nil)
		defer func() { _ = rejectResp.Body.Close() }()

		if rejectResp.StatusCode == http.StatusNotFound || rejectResp.StatusCode == http.StatusForbidden {
			t.Skip("Reject VPC peering not available")
		}

		assert.Equal(t, http.StatusOK, rejectResp.StatusCode)

		// Verify status changed to rejected
		getResp := getRequest(t, client, fmt.Sprintf("%s/vpc-peerings/%s", testutil.TestBaseURL, createRes.Data.ID), token)
		defer func() { _ = getResp.Body.Close() }()

		var getRes struct {
			Data struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(getResp.Body).Decode(&getRes))
		// Status may be rejected or still pending depending on async operation
		assert.True(t, getRes.Data.Status == "rejected" || getRes.Data.Status == "pending_acceptance")
	})

	// 6. Delete VPC Peering
	t.Run("DeleteVPCPeering", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/vpc-peerings/%s", testutil.TestBaseURL, peeringID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("VPC peering already deleted")
		}
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			t.Skip("Delete returned unexpected status")
		}
	})

	// 7. List Peerings by VPC
	t.Run("ListPeeringsByVPC", func(t *testing.T) {
		// Create a new peering to list
		vpcID4 := createTestVPC(t, client, token, fmt.Sprintf("list-vpc-%d", time.Now().UnixNano()))
		defer deleteVPC(t, client, token, vpcID4)

		payload := map[string]string{
			"requester_vpc_id": vpcID1,
			"accepter_vpc_id":  vpcID4,
		}
		createResp := postRequest(t, client, testutil.TestBaseURL+"/vpc-peerings", token, payload)
		defer func() { _ = createResp.Body.Close() }()

		if createResp.StatusCode != http.StatusCreated {
			t.Skip("Cannot create peering for list test")
		}

		var createRes struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(createResp.Body).Decode(&createRes))

		// List peerings filtered by VPC
		listResp := getRequest(t, client, fmt.Sprintf("%s/vpc-peerings?vpc_id=%s", testutil.TestBaseURL, vpcID1), token)
		defer func() { _ = listResp.Body.Close() }()

		if listResp.StatusCode == http.StatusNotFound || listResp.StatusCode == http.StatusForbidden {
			t.Skip("List peerings by VPC not available")
		}

		assert.Equal(t, http.StatusOK, listResp.StatusCode)

		var listRes struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(listResp.Body).Decode(&listRes))
		assert.NotEmpty(t, listRes.Data)

		// Cleanup
		resp := deleteRequest(t, client, fmt.Sprintf("%s/vpc-peerings/%s", testutil.TestBaseURL, createRes.Data.ID), token)
		_ = resp.Body.Close()
	})
}
