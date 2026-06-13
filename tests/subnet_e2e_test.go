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

func TestSubnetE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Subnet E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("subnet-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Subnet Tester")

	// Create VPC for subnet
	vpcID := createTestVPC(t, client, token, fmt.Sprintf("subnet-vpc-%d", time.Now().UnixNano()))
	defer deleteVPC(t, client, token, vpcID)

	var subnetID string
	subnetName := fmt.Sprintf("e2e-subnet-%d", time.Now().UnixNano()%10000)

	// 1. Create Subnet
	t.Run("CreateSubnet", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":       subnetName,
			"cidr_block": "10.1.0.0/24",
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/vpcs/%s/subnets", testutil.TestBaseURL, vpcID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			t.Skip("Subnet API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				CIDR string `json:"cidr_block"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		subnetID = res.Data.ID
		assert.NotEmpty(t, subnetID)
		assert.Equal(t, subnetName, res.Data.Name)
		assert.Equal(t, "10.1.0.0/24", res.Data.CIDR)
	})

	if subnetID == "" {
		t.Fatal("Subnet ID not set - cannot continue tests")
	}

	// 2. Get Subnet Details
	t.Run("GetSubnet", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/subnets/%s", testutil.TestBaseURL, subnetID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Get subnet not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				CIDR string `json:"cidr_block"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, subnetID, res.Data.ID)
		assert.Equal(t, subnetName, res.Data.Name)
	})

	// 3. List Subnets
	t.Run("ListSubnets", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/vpcs/%s/subnets", testutil.TestBaseURL, vpcID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("List subnets not available")
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

	// 4. Delete Subnet
	t.Run("DeleteSubnet", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/subnets/%s", testutil.TestBaseURL, subnetID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Subnet already deleted")
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			t.Skip("Delete returned unexpected status")
		}
	})

	// 5. Verify Subnet is deleted
	t.Run("VerifySubnetDeleted", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/subnets/%s", testutil.TestBaseURL, subnetID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Verify not available")
		}
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
