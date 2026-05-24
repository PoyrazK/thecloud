package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const gatewayRoutesPath = "/gateway/routes"
const httpbinAnything = "https://httpbin.org/anything"

func waitForRoute(t *testing.T, client *http.Client, url string, token string) *http.Response {
	t.Helper()
	var resp *http.Response
	var err error
	for i := 0; i < 10; i++ {
		req, _ := http.NewRequest("GET", url, nil)
		if token != "" {
			req.Header.Set(testutil.TestHeaderAPIKey, token)
		}
		resp, err = client.Do(req)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	require.NoError(t, err, "Timed out waiting for route: "+url)
	require.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300, "Route returned non-success status: %d for URL: %s", resp.StatusCode, url)
	return resp
}

func TestGatewayE2E(t *testing.T) {
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Gateway E2E test: %v", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	token := registerAndLogin(t, client, "gateway-tester@thecloud.local", "Gateway Tester")

	// Use a unique suffix for route names to avoid collisions in E2E environment
	ts := time.Now().UnixNano() % 100000

	t.Run("CreateAndListPatternRoute", func(t *testing.T) {
		// 1. Create a pattern-based route
		// We'll use httpbin.org to verify the proxying works
		pattern := fmt.Sprintf("/httpbin-%d/{method}", ts)
		routeName := fmt.Sprintf("httpbin-pattern-%d", ts)
		targetURL := "https://httpbin.org"

		payload := map[string]interface{}{
			"name":         routeName,
			"path_prefix":  pattern, // API currently uses path_prefix field for the pattern
			"target_url":   targetURL,
			"strip_prefix": true,
			"rate_limit":   100,
		}

		resp := postRequest(t, client, testutil.TestBaseURL+gatewayRoutesPath, token, payload)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		defer func() { _ = resp.Body.Close() }()

		var res struct {
			Data domain.GatewayRoute `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, "pattern", res.Data.PatternType)
		assert.Equal(t, pattern, res.Data.PathPattern)

		// 2. List routes and verify it's there
		listResp := getRequest(t, client, testutil.TestBaseURL+gatewayRoutesPath, token)
		require.Equal(t, http.StatusOK, listResp.StatusCode)
		defer func() { _ = listResp.Body.Close() }()

		var listRes struct {
			Data []domain.GatewayRoute `json:"data"`
		}
		require.NoError(t, json.NewDecoder(listResp.Body).Decode(&listRes))

		found := false
		for _, r := range listRes.Data {
			if r.Name == routeName {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("VerifyPatternProxying", func(t *testing.T) {
		// Test GET request through gateway
		url := fmt.Sprintf("%s/gw/httpbin-%d/get", testutil.TestBaseURL, ts)
		resp := waitForRoute(t, client, url, token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var httpbinResp struct {
			URL string `json:"url"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&httpbinResp))
		assert.Contains(t, httpbinResp.URL, "/get")
	})

	t.Run("VerifyRegexPatternProxying", func(t *testing.T) {
		pattern := fmt.Sprintf("/status-%d/{code:[0-9]+}", ts)
		routeName := fmt.Sprintf("status-code-%d", ts)
		targetURL := "https://httpbin.org" // Use base URL

		payload := map[string]interface{}{
			"name":         routeName,
			"path_prefix":  pattern,
			"target_url":   targetURL,
			"strip_prefix": false,
		}
		// Redefine for clarity
		pattern = fmt.Sprintf("/gw-status-%d/{code:[0-9]+}", ts)
		payload["path_prefix"] = pattern
		payload["target_url"] = "https://httpbin.org/status"
		payload["strip_prefix"] = true

		resp := postRequest(t, client, testutil.TestBaseURL+gatewayRoutesPath, token, payload)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		_ = resp.Body.Close()

		// This should match and return 201
		url := fmt.Sprintf("%s/gw/gw-status-%d/201", testutil.TestBaseURL, ts)
		respMatch := waitForRoute(t, client, url, "")
		assert.Equal(t, http.StatusCreated, respMatch.StatusCode)
		_ = respMatch.Body.Close()

		// This should NOT match (letters instead of numbers)
		req, _ := http.NewRequest("GET", fmt.Sprintf("%s/gw/status-%d/abc", testutil.TestBaseURL, ts), nil)
		resp, err := client.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		_ = resp.Body.Close()
	})

	t.Run("VerifyWildcardProxying", func(t *testing.T) {
		pattern := fmt.Sprintf("/wild-%d/*", ts)
		routeName := fmt.Sprintf("wildcard-route-%d", ts)
		targetURL := httpbinAnything

		payload := map[string]interface{}{
			"name":         routeName,
			"path_prefix":  pattern,
			"target_url":   targetURL,
			"strip_prefix": true,
			"rate_limit":   100,
		}

		resp := postRequest(t, client, testutil.TestBaseURL+gatewayRoutesPath, token, payload)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		_ = resp.Body.Close()

		url := fmt.Sprintf("%s/gw/wild-%d/foo/bar", testutil.TestBaseURL, ts)
		retryResp := waitForRoute(t, client, url, "")
		defer func() { _ = retryResp.Body.Close() }()

		assert.Equal(t, http.StatusOK, retryResp.StatusCode)

		var httpbinResp struct {
			URL string `json:"url"`
		}
		require.NoError(t, json.NewDecoder(retryResp.Body).Decode(&httpbinResp))
		assert.Contains(t, httpbinResp.URL, "/anything/foo/bar")
	})

	t.Run("VerifyMultiParamProxying", func(t *testing.T) {
		pattern := fmt.Sprintf("/orgs-%d/{org}/projects/{project}", ts)
		routeName := fmt.Sprintf("multi-param-%d", ts)
		targetURL := httpbinAnything

		payload := map[string]interface{}{
			"name":         routeName,
			"path_prefix":  pattern,
			"target_url":   targetURL,
			"strip_prefix": true,
			"rate_limit":   100,
		}

		resp := postRequest(t, client, testutil.TestBaseURL+gatewayRoutesPath, token, payload)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		_ = resp.Body.Close()

		url := fmt.Sprintf("%s/gw/orgs-%d/google/projects/chrome", testutil.TestBaseURL, ts)
		retryResp := waitForRoute(t, client, url, token)
		defer func() { _ = retryResp.Body.Close() }()

		assert.Equal(t, http.StatusOK, retryResp.StatusCode)

		var httpbinResp struct {
			URL string `json:"url"`
		}
		require.NoError(t, json.NewDecoder(retryResp.Body).Decode(&httpbinResp))
		assert.Contains(t, httpbinResp.URL, "/google/projects/chrome")
	})

	t.Run("VerifyPriorityMatching", func(t *testing.T) {
		// 1. General pattern route (Low priority)
		patternGen := fmt.Sprintf("/users-%d/{id}", ts)
		payloadGen := map[string]interface{}{
			"name":         fmt.Sprintf("user-gen-%d", ts),
			"path_prefix":  patternGen,
			"target_url":   httpbinAnything + "/general",
			"strip_prefix": true,
			"priority":     1,
		}
		resGen := postRequest(t, client, testutil.TestBaseURL+gatewayRoutesPath, token, payloadGen)
		_ = resGen.Body.Close()

		// 2. Specific exact route (High priority implied by length or explicit)
		patternSpec := fmt.Sprintf("/users-%d/me", ts)
		payloadSpec := map[string]interface{}{
			"name":         fmt.Sprintf("user-spec-%d", ts),
			"path_prefix":  patternSpec,
			"target_url":   httpbinAnything + "/specific",
			"strip_prefix": true,
			"priority":     10,
		}
		resSpec := postRequest(t, client, testutil.TestBaseURL+gatewayRoutesPath, token, payloadSpec)
		_ = resSpec.Body.Close()

		url := fmt.Sprintf("%s/gw/users-%d/me", testutil.TestBaseURL, ts)
		// We expect it to eventually hit "specific"
		var finalURL string
		for i := 0; i < 10; i++ {
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set(testutil.TestHeaderAPIKey, token)
			resp, err := client.Do(req)
			if resp != nil {
				defer func() { _ = resp.Body.Close() }()
			}
			if err == nil && resp.StatusCode == http.StatusOK {
				var res struct {
					URL string `json:"url"`
				}
				_ = json.NewDecoder(resp.Body).Decode(&res)
				if finalURL = res.URL; finalURL != "" && (finalURL == httpbinAnything+"/specific" || finalURL == httpbinAnything+"/general") {
					if finalURL == httpbinAnything+"/specific" {
						break
					}
				}
			}
			time.Sleep(1 * time.Second)
		}
		assert.Contains(t, finalURL, "/specific")
	})

	t.Run("VerifyExtensionMatching", func(t *testing.T) {
		pattern := fmt.Sprintf("/static-%d/*.{ext}", ts)
		routeName := fmt.Sprintf("extension-route-%d", ts)
		targetURL := httpbinAnything

		payload := map[string]interface{}{
			"name":         routeName,
			"path_prefix":  pattern,
			"target_url":   targetURL,
			"strip_prefix": true,
		}

		resExt := postRequest(t, client, testutil.TestBaseURL+gatewayRoutesPath, token, payload)
		_ = resExt.Body.Close()

		url := fmt.Sprintf("%s/gw/static-%d/image.png", testutil.TestBaseURL, ts)
		retryResp := waitForRoute(t, client, url, token)
		defer func() { _ = retryResp.Body.Close() }()

		assert.Equal(t, http.StatusOK, retryResp.StatusCode)

		var httpbinResp struct {
			URL string `json:"url"`
		}
		require.NoError(t, json.NewDecoder(retryResp.Body).Decode(&httpbinResp))
		assert.Contains(t, httpbinResp.URL, "/image.png")
	})

	t.Run("VerifyMethodMatching", func(t *testing.T) {
		pattern := fmt.Sprintf("/method-test-%d", ts)

		// 1. GET only route
		payloadGet := map[string]interface{}{
			"name":        fmt.Sprintf("get-route-%d", ts),
			"path_prefix": pattern + "/get",
			"target_url":  httpbinAnything + "/get-only",
			"methods":     []string{"GET"},
		}
		resMethodGet := postRequest(t, client, testutil.TestBaseURL+gatewayRoutesPath, token, payloadGet)
		require.Equal(t, http.StatusCreated, resMethodGet.StatusCode, "GET route creation failed")
		_ = resMethodGet.Body.Close()

		// 2. POST only route
		payloadPost := map[string]interface{}{
			"name":        fmt.Sprintf("post-route-%d", ts),
			"path_prefix": pattern + "/post",
			"target_url":  httpbinAnything + "/post-only",
			"methods":     []string{"POST"},
		}
		resMethodPost := postRequest(t, client, testutil.TestBaseURL+gatewayRoutesPath, token, payloadPost)
		require.Equal(t, http.StatusCreated, resMethodPost.StatusCode, "POST route creation failed")
		_ = resMethodPost.Body.Close()

		// Test GET request
		urlGet := testutil.TestBaseURL + "/gw" + pattern + "/get"
		respGet := waitForRoute(t, client, urlGet, token)
		defer func() { _ = respGet.Body.Close() }()
		require.Equal(t, http.StatusOK, respGet.StatusCode, "GET request failed after retries")

		bodyBytesGet, err := io.ReadAll(respGet.Body)
		require.NoError(t, err)
		var resGet struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(bodyBytesGet, &resGet); err != nil {
			t.Fatalf("Failed to decode GET response (Status: %d): %v. Body: %s", respGet.StatusCode, err, string(bodyBytesGet))
		}
		assert.Contains(t, resGet.URL, "/get-only")

		// Test POST request
		urlPost := testutil.TestBaseURL + "/gw" + pattern + "/post"
		reqPost, _ := http.NewRequest("POST", urlPost, nil)
		reqPost.Header.Set(testutil.TestHeaderAPIKey, token)

		// Use a simple retry for POST since waitForRoute is GET only
		var respPost *http.Response
		for i := 0; i < 5; i++ {
			respPost, err = client.Do(reqPost)
			if err == nil && respPost.StatusCode == http.StatusOK {
				break
			}
			if respPost != nil {
				_ = respPost.Body.Close()
			}
			time.Sleep(1 * time.Second)
		}

		require.NoError(t, err)
		defer func() { _ = respPost.Body.Close() }()
		require.Equal(t, http.StatusOK, respPost.StatusCode, "POST request failed after retries")

		bodyBytes, err := io.ReadAll(respPost.Body)
		require.NoError(t, err)
		var resPost struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(bodyBytes, &resPost); err != nil {
			t.Fatalf("Failed to decode POST response (Status: %d): %v. Body: %s", respPost.StatusCode, err, string(bodyBytes))
		}
		assert.Contains(t, resPost.URL, "/post-only")
	})
}
