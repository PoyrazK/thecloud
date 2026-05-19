package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/poyrazk/thecloud/pkg/testutil"
)

func TestLifecycleE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Lifecycle E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("lifecycle-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Lifecycle Tester")

	// First create a storage bucket (lifecycle rules are attached to buckets)
	var bucketName string
	var bucketCreated bool
	t.Run("CreateStorageBucket", func(t *testing.T) {
		bucketName = fmt.Sprintf("lifecycle-bucket-%d", time.Now().UnixNano()%100000)
		payload := map[string]string{
			"name": bucketName,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+testutil.TestRouteStorageBuckets, token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
			t.Skip("Storage bucket API not accessible for this user")
		}

		// May return 201 Created or 409 Conflict if bucket already exists
		if resp.StatusCode == http.StatusConflict {
			bucketName = fmt.Sprintf("lifecycle-bucket-%d", time.Now().UnixNano()%100000+1)
			payload["name"] = bucketName
			resp2 := postRequest(t, client, testutil.TestBaseURL+testutil.TestRouteStorageBuckets, token, payload)
			defer func() { _ = resp2.Body.Close() }()
			if resp2.StatusCode != http.StatusCreated {
				t.Skip("Cannot create storage bucket")
			}
			bucketCreated = true
		} else {
			require.Equal(t, http.StatusCreated, resp.StatusCode)
			bucketCreated = true
		}
	})

	// Cleanup bucket after tests
	if bucketCreated && bucketName != "" {
		defer func() {
			deleteRequest(t, client, fmt.Sprintf("%s%s/%s", testutil.TestBaseURL, testutil.TestRouteStorageBuckets, bucketName), token)
		}()
	}

	// 1. Create Lifecycle Rule
	t.Run("CreateLifecycleRule", func(t *testing.T) {
		payload := map[string]interface{}{
			"prefix":           "logs/",
			"expiration_days": 30,
			"enabled":         true,
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/storage/buckets/%s/lifecycle", testutil.TestBaseURL, bucketName), token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Lifecycle API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID             string `json:"id"`
				Prefix         string `json:"prefix"`
				ExpirationDays int    `json:"expiration_days"`
				Enabled        bool   `json:"enabled"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.NotEmpty(t, res.Data.ID)
		assert.Equal(t, "logs/", res.Data.Prefix)
		assert.Equal(t, 30, res.Data.ExpirationDays)
		assert.True(t, res.Data.Enabled)
	})

	// 2. List Lifecycle Rules
	t.Run("ListLifecycleRules", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/storage/buckets/%s/lifecycle", testutil.TestBaseURL, bucketName), token)
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

	// 3. Delete Lifecycle Rule
	t.Run("DeleteLifecycleRule", func(t *testing.T) {
		// First get the rule ID
		resp := getRequest(t, client, fmt.Sprintf("%s/storage/buckets/%s/lifecycle", testutil.TestBaseURL, bucketName), token)
		defer func() { _ = resp.Body.Close() }()

		var ruleRes struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&ruleRes))

		if len(ruleRes.Data) == 0 {
			t.Skip("No lifecycle rules to delete")
		}

		ruleID := ruleRes.Data[0].ID
		deleteResp := deleteRequest(t, client, fmt.Sprintf("%s/storage/buckets/%s/lifecycle/%s", testutil.TestBaseURL, bucketName, ruleID), token)
		defer func() { _ = deleteResp.Body.Close() }()

		assert.Equal(t, http.StatusNoContent, deleteResp.StatusCode)
	})
}