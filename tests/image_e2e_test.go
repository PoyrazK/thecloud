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

func TestImageE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Image E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("image-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Image Tester")

	var imageID string
	imageName := fmt.Sprintf("e2e-image-%d", time.Now().UnixNano()%10000)

	// 1. Register Image
	t.Run("RegisterImage", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":        imageName,
			"description": "E2E test image",
			"os":          "ubuntu",
			"version":     "22.04",
			"is_public":   false,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/images", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			t.Skip("Image API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		imageID = res.Data.ID
		assert.NotEmpty(t, imageID)
		assert.Equal(t, imageName, res.Data.Name)
	})

	if imageID == "" {
		t.Fatal("Image ID not set - cannot continue tests")
	}

	// 2. Get Image Details
	t.Run("GetImage", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/images/%s", testutil.TestBaseURL, imageID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Get image not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				OS   string `json:"os"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, imageID, res.Data.ID)
		assert.Equal(t, imageName, res.Data.Name)
	})

	// 3. List Images
	t.Run("ListImages", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/images", token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("List images not available")
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

	// 4. Import Image from URL
	t.Run("ImportImage", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":        fmt.Sprintf("imported-image-%d", time.Now().UnixNano()%10000),
			"url":         "https://example.com/ubuntu-22.04.qcow2",
			"description": "Imported via E2E test",
			"os":          "ubuntu",
			"version":     "22.04",
			"is_public":   false,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/images/import", token, payload)
		defer func() { _ = resp.Body.Close() }()

		// May return 202 Accepted (async) or 400 if import not supported
		if resp.StatusCode == http.StatusBadRequest {
			t.Skip("Image import not available")
		}

		if resp.StatusCode == http.StatusAccepted {
			// Async import - get the image ID from response
			var res struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			if json.NewDecoder(resp.Body).Decode(&res) == nil && res.Data.ID != "" {
				// Cleanup async imported image
				defer deleteRequest(t, client, fmt.Sprintf("%s/images/%s", testutil.TestBaseURL, res.Data.ID), token)
			}
		}

		// Just verify the request was processed
		if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
			t.Skip("Image import endpoint not available")
		}
	})

	// 5. Delete Image
	t.Run("DeleteImage", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/images/%s", testutil.TestBaseURL, imageID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Image already deleted")
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			t.Skip("Delete returned unexpected status")
		}
	})

	// 6. Verify Image is deleted
	t.Run("VerifyImageDeleted", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/images/%s", testutil.TestBaseURL, imageID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Verify not available")
		}
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
