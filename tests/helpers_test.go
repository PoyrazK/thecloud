package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/poyrazk/thecloud/pkg/testutil"
	"github.com/stretchr/testify/require"
)

const (
	headerTenantID  = "X-Tenant-ID"
	errTenantNotSet = "tenant ID not set for request"
	dbPrefixLen     = 8
)

var (
	dockerCheckOnce sync.Once
	dockerCheckErr  error
)

func waitForServer() error {
	// Check Docker status exactly once to fail fast if infrastructure is broken
	dockerCheckOnce.Do(func() {
		dockerCheckErr = checkDocker()
	})

	if dockerCheckErr != nil {
		return fmt.Errorf("infrastructure error: %w (server not ready at %s)", dockerCheckErr, testutil.TestBaseURL)
	}

	client := &http.Client{Timeout: 1 * time.Second}
	for i := 0; i < 60; i++ {
		resp, err := client.Get(testutil.TestBaseURL + "/health/live")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				// Server is up — reset circuit breakers before tests to ensure clean state
				resetCircuitBreakers()
				return nil
			}
		}

		if i%10 == 0 {
			fmt.Printf("Waiting for server at %s (attempt %d/60)...\n", testutil.TestBaseURL, i+1)
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("server not ready at %s", testutil.TestBaseURL)
}

func checkDocker() error {
	// Simple check to see if docker is responsive
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker is unavailable or paused")
	}
	return nil
}

// resetCircuitBreakers calls the internal admin endpoint to reset the compute
// circuit breaker. This ensures clean state for E2E tests.
func resetCircuitBreakers() {
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("POST", testutil.TestBaseURL+"/internal/admin/reset-circuit-breakers", nil)
	resp, err := client.Do(req)
	if err != nil {
		// Non-fatal — circuit breaker may already be fine
		return
	}
	_ = resp.Body.Close()
}

var (
	tenantIDByToken = map[string]string{}
	tenantMu        sync.RWMutex
)

type ResponseWrapper struct {
	Data interface{} `json:"data"`
}

func registerAndLogin(t *testing.T, client *http.Client, email, name string) string {
	t.Helper()
	// Register
	regReq := map[string]string{
		"email":    email,
		"password": testutil.TestPasswordStrong,
		"name":     name,
	}
	body, _ := json.Marshal(regReq)
	resp, err := client.Post(testutil.TestBaseURL+"/auth/register", testutil.TestContentTypeAppJSON, bytes.NewBuffer(body))
	if err == nil {
		_ = resp.Body.Close()
	}

	// Login
	loginReq := map[string]string{
		"email":    email,
		"password": testutil.TestPasswordStrong,
	}
	body, _ = json.Marshal(loginReq)
	resp, err = client.Post(testutil.TestBaseURL+"/auth/login", testutil.TestContentTypeAppJSON, bytes.NewBuffer(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Login failed for %s: status %d", email, resp.StatusCode)
	}

	var authResp struct {
		Data struct {
			APIKey string `json:"api_key"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&authResp))
	tenantID := createTenant(t, client, authResp.Data.APIKey, name)
	setTenantIDForToken(authResp.Data.APIKey, tenantID)
	switchTenant(t, client, authResp.Data.APIKey, tenantID)
	return authResp.Data.APIKey
}

func createTenant(t *testing.T, client *http.Client, token, name string) string {
	t.Helper()
	payload := map[string]string{
		"name": name,
		"slug": fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", testutil.TestBaseURL+"/tenants", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", testutil.TestContentTypeAppJSON)
	req.Header.Set(testutil.TestHeaderAPIKey, token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Create tenant failed: status %d", resp.StatusCode)
	}

	var tenantResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&tenantResp))
	require.NotEmpty(t, tenantResp.Data.ID)
	return tenantResp.Data.ID
}

func switchTenant(t *testing.T, client *http.Client, token, tenantID string) {
	t.Helper()
	if tenantID == "" {
		t.Fatalf("tenant ID not set before switch")
	}
	req, _ := http.NewRequest("POST", testutil.TestBaseURL+"/tenants/"+tenantID+"/switch", nil)
	req.Header.Set(testutil.TestHeaderAPIKey, token)
	req.Header.Set(headerTenantID, tenantID)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Switch tenant failed: status %d", resp.StatusCode)
	}
}

func postRequest(t *testing.T, client *http.Client, url, token string, payload interface{}) *http.Response {
	t.Helper()
	var body io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewBuffer(b)
	}
	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", testutil.TestContentTypeAppJSON)
	req.Header.Set(testutil.TestHeaderAPIKey, token)
	if requiresTenantHeader(url) {
		tenantID := tenantIDForToken(token)
		if tenantID == "" {
			t.Fatal(errTenantNotSet)
		}
		req.Header.Set(headerTenantID, tenantID)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func getRequest(t *testing.T, client *http.Client, url, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set(testutil.TestHeaderAPIKey, token)
	if requiresTenantHeader(url) {
		tenantID := tenantIDForToken(token)
		if tenantID == "" {
			t.Fatal(errTenantNotSet)
		}
		req.Header.Set(headerTenantID, tenantID)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func deleteRequest(t *testing.T, client *http.Client, url, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set(testutil.TestHeaderAPIKey, token)
	if requiresTenantHeader(url) {
		tenantID := tenantIDForToken(token)
		if tenantID == "" {
			t.Fatal(errTenantNotSet)
		}
		req.Header.Set(headerTenantID, tenantID)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func applyTenantHeader(t *testing.T, req *http.Request, token string) {
	t.Helper()
	if !requiresTenantHeader(req.URL.Path) {
		return
	}
	tenantID := tenantIDForToken(token)
	if tenantID == "" {
		t.Fatal(errTenantNotSet)
	}
	req.Header.Set(headerTenantID, tenantID)
}

func requiresTenantHeader(url string) bool {
	return !strings.Contains(url, "/auth/")
}

func setTenantIDForToken(token, tenantID string) {
	tenantMu.Lock()
	defer tenantMu.Unlock()
	tenantIDByToken[token] = tenantID
}

func tenantIDForToken(token string) string {
	tenantMu.RLock()
	defer tenantMu.RUnlock()
	return tenantIDByToken[token]
}

func safePrefix(id string) string {
	if len(id) < dbPrefixLen {
		return id
	}
	return id[:dbPrefixLen]
}

func closeBody(t *testing.T, resp *http.Response) {
	t.Helper()
	if resp != nil && resp.Body != nil {
		err := resp.Body.Close()
		require.NoError(t, err, "failed to close response body")
	}
}
