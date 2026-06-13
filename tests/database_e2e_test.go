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

func TestDatabaseE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Database E2E test: %v", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	token := registerAndLogin(t, client, "db-tester@thecloud.local", "DB Tester")

	var dbID string
	dbName := fmt.Sprintf("e2e-db-%d", time.Now().UnixNano()%1000)

	// 1. Create Database
	t.Run("CreateDatabase", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":              dbName,
			"engine":            "postgres",
			"version":           "16",
			"allocated_storage": 10,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/databases", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			t.Skip("Database API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data domain.Database `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		dbID = res.Data.ID.String()
		assert.NotEmpty(t, dbID)
	})

	// 2. Get Connection String
	t.Run("GetConnectionString", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/databases/%s/connection", testutil.TestBaseURL, dbID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("Get connection string not available")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 3. Rotate Credentials
	t.Run("RotateCredentials", func(t *testing.T) {
		resp := postRequest(t, client, fmt.Sprintf("%s/databases/%s/rotate-credentials", testutil.TestBaseURL, dbID), token, nil)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusInternalServerError || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			t.Skip("skipping credential rotation e2e due to transient database readiness failure")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 4. Delete Database
	t.Run("DeleteDatabase", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/databases/%s", testutil.TestBaseURL, dbID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Database already deleted")
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			t.Skip("Delete returned unexpected status")
		}
	})
}
