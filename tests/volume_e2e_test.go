package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/pkg/testutil"
)

func TestVolumeE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Volume E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("volume-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Volume Tester")

	// Create VPC first (volume needs a VPC)
	vpcID := createTestVPC(t, client, token, fmt.Sprintf("vol-e2e-vpc-%d", time.Now().UnixNano()))
	defer deleteVPC(t, client, token, vpcID)

	var volumeID string
	volumeName := fmt.Sprintf("e2e-vol-%d-%s", time.Now().UnixNano()%1000, uuid.New().String())

	// 1. Create Volume
	t.Run("CreateVolume", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":        volumeName,
			"size_gb":     10,
			"vpc_id":      vpcID,
			"volume_type": "standard",
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/volumes", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Volume API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		volumeID = res.Data.ID
		assert.NotEmpty(t, volumeID)
	})

	if volumeID == "" {
		t.Fatal("Volume ID not set - cannot continue tests")
	}

	// 2. Get Volume Details
	t.Run("GetVolume", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/volumes/%s", testutil.TestBaseURL, volumeID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data domain.Volume `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, volumeName, res.Data.Name)
		assert.Equal(t, 10, res.Data.SizeGB)
	})

	// 3. List Volumes
	t.Run("ListVolumes", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/volumes", token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data []domain.Volume `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.NotEmpty(t, res.Data)
	})

	// 4. Attach Volume to Instance (create instance first)
	t.Run("AttachVolume", func(t *testing.T) {
		// Create an instance to attach the volume to
		instanceID := volCreateTestInstance(t, client, token, vpcID)
		defer volDeleteInstance(t, client, token, instanceID)

		// Attach volume
		payload := map[string]string{
			"instance_id": instanceID,
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/volumes/%s/attach", testutil.TestBaseURL, volumeID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify attachment
		attachmentsResp := getRequest(t, client, fmt.Sprintf("%s/volumes/%s/attachments", testutil.TestBaseURL, volumeID), token)
		defer func() { _ = attachmentsResp.Body.Close() }()

		var attachRes struct {
			Data []struct {
				InstanceID string `json:"instance_id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(attachmentsResp.Body).Decode(&attachRes))
		assert.NotEmpty(t, attachRes.Data)
	})

	// 5. Detach Volume
	t.Run("DetachVolume", func(t *testing.T) {
		resp := postRequest(t, client, fmt.Sprintf("%s/volumes/%s/detach", testutil.TestBaseURL, volumeID), token, nil)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 6. Delete Volume
	t.Run("DeleteVolume", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/volumes/%s", testutil.TestBaseURL, volumeID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})
}

// createTestVPC creates a VPC for testing and returns the VPC ID.
func createTestVPC(t *testing.T, client *http.Client, token string, name string) string {
	t.Helper()
	payload := map[string]string{
		"name":       name,
		"cidr_block": "10.1.0.0/16",
	}
	resp := postRequest(t, client, testutil.TestBaseURL+testutil.TestRouteVpcs, token, payload)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var res struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
	return res.Data.ID
}

// deleteVPC deletes a VPC by ID.
func deleteVPC(t *testing.T, client *http.Client, token, vpcID string) {
	t.Helper()
	resp := deleteRequest(t, client, fmt.Sprintf("%s%s/%s", testutil.TestBaseURL, testutil.TestRouteVpcs, vpcID), token)
	defer func() { _ = resp.Body.Close() }()
	// Ignore error - VPC may already be deleted
}

// volCreateTestInstance creates a test instance for volume tests and returns the instance ID.
func volCreateTestInstance(t *testing.T, client *http.Client, token, vpcID string) string {
	t.Helper()
	payload := map[string]string{
		"name":  fmt.Sprintf("e2e-inst-%d", time.Now().UnixNano()%1000),
		"image": "nginx:alpine",
		"ports": "0:80",
	}
	resp := postRequest(t, client, testutil.TestBaseURL+testutil.TestRouteInstances, token, payload)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		t.Skipf("Cannot create test instance: status %d", resp.StatusCode)
	}

	var res struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
	return res.Data.ID
}

// volDeleteInstance deletes an instance by ID.
func volDeleteInstance(t *testing.T, client *http.Client, token, instanceID string) {
	t.Helper()
	resp := deleteRequest(t, client, fmt.Sprintf("%s%s/%s", testutil.TestBaseURL, testutil.TestRouteInstances, instanceID), token)
	defer func() { _ = resp.Body.Close() }()
	// Ignore error - instance may already be deleted
}
