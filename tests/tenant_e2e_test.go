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

func TestTenantE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Tenant E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("tenant-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Tenant Tester")

	var tenantID string
	tenantName := fmt.Sprintf("e2e-tenant-%d", time.Now().UnixNano()%10000)
	tenantSlug := fmt.Sprintf("e2e-tenant-%d", time.Now().UnixNano()%10000)

	// 1. Create Tenant
	t.Run("CreateTenant", func(t *testing.T) {
		payload := map[string]string{
			"name": tenantName,
			"slug": tenantSlug,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/tenants", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Tenant API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Slug string `json:"slug"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		tenantID = res.Data.ID
		assert.NotEmpty(t, tenantID)
		assert.Equal(t, tenantName, res.Data.Name)
		assert.Equal(t, tenantSlug, res.Data.Slug)
	})

	if tenantID == "" {
		t.Fatal("Tenant ID not set - cannot continue tests")
	}

	// 2. List Tenants
	t.Run("ListTenants", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/tenants", token)
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

	// 3. Switch Tenant
	t.Run("SwitchTenant", func(t *testing.T) {
		payload := map[string]string{
			"tenant_id": tenantID,
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/tenants/%s/switch", testutil.TestBaseURL, tenantID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, tenantID, res.Data.ID)
	})

	// 4. Invite Member to Tenant
	t.Run("InviteTenantMember", func(t *testing.T) {
		payload := map[string]string{
			"email": fmt.Sprintf("member-%d@thecloud.local", time.Now().UnixNano()%10000),
			"role":  "member",
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/tenants/%s/members", testutil.TestBaseURL, tenantID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		// May return 201 Created or 400 if inviting already existing member
		if resp.StatusCode == http.StatusBadRequest {
			t.Skip("Member invite not available or member already exists")
		}

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})
}
