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

func TestNATGatewayE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing NAT Gateway E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("natgw-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "NAT Gateway Tester")

	// Create VPC and Subnet for NAT Gateway
	vpcID := createTestVPC(t, client, token, fmt.Sprintf("natgw-vpc-%d", time.Now().UnixNano()))
	defer deleteVPC(t, client, token, vpcID)

	// Create subnet for NAT gateway
	var subnetID string
	t.Run("CreateSubnet", func(t *testing.T) {
		payload := map[string]string{
			"name":       fmt.Sprintf("natgw-subnet-%d", time.Now().UnixNano()%1000),
			"vpc_id":     vpcID,
			"cidr_block": "10.1.1.0/24",
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
		assert.NotEmpty(t, subnetID)
	})

	if subnetID == "" {
		t.Fatal("Subnet ID not set - cannot continue NAT Gateway tests")
	}

	// Create Elastic IP for NAT Gateway
	var eipID string
	t.Run("CreateElasticIP", func(t *testing.T) {
		payload := map[string]string{
			"name": fmt.Sprintf("natgw-eip-%d", time.Now().UnixNano()%1000),
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/elastic-ips", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
			t.Skip("Elastic IP API not accessible for this user")
		}

		if resp.StatusCode == http.StatusCreated {
			var res struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
			eipID = res.Data.ID
		} else {
			// EIP might use different endpoint or status code
			var res struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			if json.NewDecoder(resp.Body).Decode(&res) == nil {
				eipID = res.Data.ID
			}
		}
	})

	var natGatewayID string

	// 1. Create NAT Gateway
	t.Run("CreateNATGateway", func(t *testing.T) {
		if subnetID == "" || eipID == "" {
			t.Skip("Cannot create NAT gateway - missing subnet or EIP")
		}

		payload := map[string]string{
			"subnet_id": subnetID,
			"eip_id":    eipID,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/nat-gateways", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("NAT Gateway API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		natGatewayID = res.Data.ID
		assert.NotEmpty(t, natGatewayID)
	})

	if natGatewayID == "" {
		// Cleanup and skip
		deleteSubnet(t, client, token, subnetID)
		if eipID != "" {
			deleteElasticIP(t, client, token, eipID)
		}
		t.Fatal("NAT Gateway ID not set - cannot continue tests")
	}

	// 2. Get NAT Gateway Details
	t.Run("GetNATGateway", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/nat-gateways/%s", testutil.TestBaseURL, natGatewayID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID       string `json:"id"`
				Status   string `json:"status"`
				SubnetID string `json:"subnet_id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, natGatewayID, res.Data.ID)
	})

	// 3. List NAT Gateways
	t.Run("ListNATGateways", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/nat-gateways?vpc_id=%s", testutil.TestBaseURL, vpcID), token)
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

	// 4. Delete NAT Gateway
	t.Run("DeleteNATGateway", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/nat-gateways/%s", testutil.TestBaseURL, natGatewayID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	// Cleanup
	deleteSubnet(t, client, token, subnetID)
	if eipID != "" {
		deleteElasticIP(t, client, token, eipID)
	}
}

// deleteSubnet deletes a subnet by ID.
func deleteSubnet(t *testing.T, client *http.Client, token, subnetID string) {
	t.Helper()
	resp := deleteRequest(t, client, fmt.Sprintf("%s/subnets/%s", testutil.TestBaseURL, subnetID), token)
	defer func() { _ = resp.Body.Close() }()
	// Ignore error - subnet may already be deleted
}

// deleteElasticIP deletes an elastic IP by ID.
func deleteElasticIP(t *testing.T, client *http.Client, token, eipID string) {
	t.Helper()
	resp := deleteRequest(t, client, fmt.Sprintf("%s/elastic-ips/%s", testutil.TestBaseURL, eipID), token)
	defer func() { _ = resp.Body.Close() }()
	// Ignore error - EIP may already be deleted
}
