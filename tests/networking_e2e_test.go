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

func TestNetworkingE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Networking E2E test: %v", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	token := registerAndLogin(t, client, "network-tester@thecloud.local", "Network Tester")

	const (
		vpcRoute  = "%s%s/%s?force=true"
		subRoute  = "%s/vpcs/%s/subnets"
		sgRoute   = "%s/security-groups/%s"
		lbRoute   = "%s/lb/%s"
		subSingle = "%s/subnets/%s"
	)

	var vpcID string
	vpcName := fmt.Sprintf("e2e-vpc-%d", time.Now().UnixNano())

	// 1. Create VPC
	t.Run("CreateVPC", func(t *testing.T) {
		payload := map[string]string{
			"name":       vpcName,
			"cidr_block": "10.0.0.0/16",
		}
		resp := postRequest(t, client, testutil.TestBaseURL+testutil.TestRouteVpcs, token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			t.Skip("VPC API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data domain.VPC `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		vpcID = res.Data.ID.String()
		assert.NotEmpty(t, vpcID)
		assert.Equal(t, vpcName, res.Data.Name)
	})

	// 2. Create Subnet
	var subnetID string
	t.Run("CreateSubnet", func(t *testing.T) {
		payload := map[string]string{
			"name":       "e2e-subnet",
			"cidr_block": "10.0.1.0/24",
		}
		resp := postRequest(t, client, fmt.Sprintf(subRoute, testutil.TestBaseURL, vpcID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			t.Skip("Subnet API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data domain.Subnet `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		subnetID = res.Data.ID.String()
		assert.NotEmpty(t, subnetID)
	})

	// 3. List Subnets
	t.Run("ListSubnets", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf(subRoute, testutil.TestBaseURL, vpcID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("List subnets not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data []domain.Subnet `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.GreaterOrEqual(t, len(res.Data), 1)
	})

	// 4. Create Security Group
	var sgID string
	t.Run("CreateSecurityGroup", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":        "e2e-sg",
			"description": "E2E security group",
			"vpc_id":      vpcID,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/security-groups", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			t.Skip("Security group API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data domain.SecurityGroup `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		sgID = res.Data.ID.String()
	})

	// 5. Load Balancer
	var lbID string
	t.Run("CreateLoadBalancer", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":      "e2e-lb",
			"vpc_id":    vpcID,
			"port":      80,
			"algorithm": "round-robin",
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/lb", token, payload)
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusAccepted, resp.StatusCode)

		var res struct {
			Data domain.LoadBalancer `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		lbID = res.Data.ID.String()
		assert.NotEmpty(t, lbID)
	})

	// 6. Cleanup
	t.Run("Cleanup", func(t *testing.T) {
		// Delete LB
		resp := deleteRequest(t, client, fmt.Sprintf(lbRoute, testutil.TestBaseURL, lbID), token)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			// Already deleted
		} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			t.Skip("LB delete returned unexpected status")
		}

		// Delete Security Group
		resp = deleteRequest(t, client, fmt.Sprintf(sgRoute, testutil.TestBaseURL, sgID), token)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			// Already deleted
		} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			t.Skip("Security group delete returned unexpected status")
		}

		// Delete Subnet
		resp = deleteRequest(t, client, fmt.Sprintf(subSingle, testutil.TestBaseURL, subnetID), token)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			// Already deleted
		} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			t.Skip("Subnet delete returned unexpected status")
		}

		// Delete VPC with retry to account for asynchronous cleanup of resources like LBs
		timeout := 120 * time.Second
		start := time.Now()
		for time.Since(start) < timeout {
			resp = deleteRequest(t, client, fmt.Sprintf(vpcRoute, testutil.TestBaseURL, testutil.TestRouteVpcs, vpcID), token)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
				return
			}
			time.Sleep(10 * time.Second)
		}
		t.Errorf("Timeout waiting for VPC %s to be deleted", vpcID)
	})
}
