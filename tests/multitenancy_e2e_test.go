//go:build e2e

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const baseURL = "http://localhost:8080"

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	APIKey string `json:"api_key"`
}

type ResponseWrapper struct {
	Data AuthResponse `json:"data"`
}

type Instance struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func TestMultiTenancy_E2E(t *testing.T) {
	// Ensure server is reachable
	require.NoError(t, waitForServer(), "Server must be running at "+baseURL)

	client := &http.Client{Timeout: 5 * time.Second}

	// 1. Register and Login User A
	tokenA := registerAndLogin(t, client, "userA@test.com", "User A")

	// 2. Register and Login User B
	tokenB := registerAndLogin(t, client, "userB@test.com", "User B")

	// 3. User A Creates an Instance
	instA := createInstance(t, client, tokenA, "inst-a")
	assert.NotEmpty(t, instA.ID)

	t.Run("User B cannot see User A's instance in List", func(t *testing.T) {
		listB := listInstances(t, client, tokenB)
		for _, inst := range listB {
			assert.NotEqual(t, instA.ID, inst.ID, "User B should not see User A's instance")
		}
	})

	t.Run("User B cannot Get User A's instance", func(t *testing.T) {
		req, _ := http.NewRequest("GET", fmt.Sprintf("%s/instances/%s", baseURL, instA.ID), nil)
		req.Header.Set("X-API-Key", tokenB)
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Expect 404 (Not Found) or 403 (Forbidden).
		// Since repo filters by user_id, it likely returns Not Found (like it doesn't exist for them).
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("User A can see their instance", func(t *testing.T) {
		req, _ := http.NewRequest("GET", fmt.Sprintf("%s/instances/%s", baseURL, instA.ID), nil)
		req.Header.Set("X-API-Key", tokenA)
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// Cleanup
	deleteInstance(t, client, tokenA, instA.ID)
}

func waitForServer() error {
	for i := 0; i < 5; i++ {
		resp, err := http.Get(baseURL + "/health")
		if err == nil && resp.StatusCode == 200 {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("server not ready")
}

func registerAndLogin(t *testing.T, client *http.Client, email, name string) string {
	// Register
	regReq := RegisterRequest{Email: email, Password: "password123", Name: name}
	body, _ := json.Marshal(regReq)
	resp, err := client.Post(baseURL+"/auth/register", "application/json", bytes.NewBuffer(body))
	if err == nil {
		resp.Body.Close()
	}
	// Ignore error if already registered, proceed to login

	// Login
	loginReq := LoginRequest{Email: email, Password: "password123"}
	body, _ = json.Marshal(loginReq)
	resp, err = client.Post(baseURL+"/auth/login", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Login failed for %s: status %d", email, resp.StatusCode)
	}

	var authResp ResponseWrapper
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&authResp))
	return authResp.Data.APIKey
}

func createInstance(t *testing.T, client *http.Client, token, name string) Instance {
	reqBody := map[string]string{
		"name":  name,
		"image": "alpine",
	}
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", baseURL+"/instances", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", token)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Create instance failed: status %d body %s", resp.StatusCode, body)
	}

	// Response is wrapped in data
	type InstanceWrapper struct {
		Data Instance `json:"data"`
	}
	var instWrapper InstanceWrapper
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&instWrapper))
	return instWrapper.Data
}

func listInstances(t *testing.T, client *http.Client, token string) []Instance {
	req, _ := http.NewRequest("GET", baseURL+"/instances", nil)
	req.Header.Set("X-API-Key", token)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	type ListWrapper struct {
		Data []Instance `json:"data"`
	}
	var listWrapper ListWrapper
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listWrapper))
	return listWrapper.Data
}

func deleteInstance(t *testing.T, client *http.Client, token, id string) {
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/instances/%s", baseURL, id), nil)
	req.Header.Set("X-API-Key", token)
	client.Do(req)
}
