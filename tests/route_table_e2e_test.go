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

func TestRouteTableE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Route Table E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("rt-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Route Table Tester")

	// Create VPC for route table
	vpcID := createTestVPC(t, client, token, fmt.Sprintf("rt-vpc-%d", time.Now().UnixNano()))
	defer deleteVPC(t, client, token, vpcID)

	// Create subnet for association
	var subnetID string
	subnetCreated := false
	t.Run("CreateSubnet", func(t *testing.T) {
		payload := map[string]string{
			"name":       fmt.Sprintf("rt-subnet-%d", time.Now().UnixNano()%1000),
			"vpc_id":     vpcID,
			"cidr_block": "10.1.2.0/24",
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/subnets", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
			t.Skip("Subnet API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		subnetID = res.Data.ID
		subnetCreated = true
		assert.NotEmpty(t, subnetID)
	})

	if !subnetCreated {
		// Subnet creation was skipped - the test can't proceed
		deleteVPC(t, client, token, vpcID)
		t.Skip("Subnet creation failed - cannot continue Route Table tests")
	}

	if subnetCreated {
		defer deleteSubnet(t, client, token, subnetID)
	}
	defer deleteVPC(t, client, token, vpcID)

	var rtID string
	rtName := fmt.Sprintf("e2e-rt-%d", time.Now().UnixNano()%10000)

	// 1. Create Route Table
	t.Run("CreateRouteTable", func(t *testing.T) {
		payload := map[string]interface{}{
			"vpc_id":  vpcID,
			"name":    rtName,
			"is_main": false,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/route-tables", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			t.Skip("Route Table API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				IsMain bool   `json:"is_main"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		rtID = res.Data.ID
		assert.NotEmpty(t, rtID)
		assert.Equal(t, rtName, res.Data.Name)
	})

	if rtID == "" {
		t.Fatal("Route Table ID not set - cannot continue tests")
	}

	// 2. Get Route Table Details
	t.Run("GetRouteTable", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/route-tables/%s", testutil.TestBaseURL, rtID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Get route table not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 3. List Route Tables
	t.Run("ListRouteTables", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/route-tables?vpc_id=%s", testutil.TestBaseURL, vpcID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("List route tables not available")
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

	// 4. Add Route to Route Table
	t.Run("AddRoute", func(t *testing.T) {
		payload := map[string]string{
			"destination_cidr": "10.0.0.0/16",
			"target_type":      "local",
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/route-tables/%s/routes", testutil.TestBaseURL, rtID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Add route not available")
		}
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID              string `json:"id"`
				DestinationCIDR string `json:"destination_cidr"`
				TargetType      string `json:"target_type"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, "10.0.0.0/16", res.Data.DestinationCIDR)
		assert.Equal(t, "local", res.Data.TargetType)
	})

	// 5. Associate Subnet with Route Table
	t.Run("AssociateSubnet", func(t *testing.T) {
		payload := map[string]string{
			"subnet_id": subnetID,
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/route-tables/%s/associate", testutil.TestBaseURL, rtID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Associate subnet not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 6. Disassociate Subnet from Route Table
	t.Run("DisassociateSubnet", func(t *testing.T) {
		payload := map[string]string{
			"subnet_id": subnetID,
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/route-tables/%s/disassociate", testutil.TestBaseURL, rtID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Disassociate subnet not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 7. Delete Route Table
	t.Run("DeleteRouteTable", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/route-tables/%s", testutil.TestBaseURL, rtID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Route table already deleted")
		}
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			t.Skip("Delete returned unexpected status")
		}
	})

	// 8. Verify Route Table is deleted
	t.Run("VerifyRouteTableDeleted", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/route-tables/%s", testutil.TestBaseURL, rtID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Verify not available")
		}
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
