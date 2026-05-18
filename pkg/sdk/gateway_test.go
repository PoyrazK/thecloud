package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testRouteID      = "route-1"
	gatewayRoutesURL = "/gateway/routes"
)

func TestGatewayCreateRoute(t *testing.T) {
	expectedRoute := GatewayRoute{
		ID:          testRouteID,
		Name:        "test-route",
		PathPrefix:  "/api/v1",
		TargetURL:   "http://backend:8080",
		StripPrefix: true,
		RateLimit:   100,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, gatewayRoutesURL, r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req struct {
			Name        string   `json:"name"`
			PathPrefix  string   `json:"path_prefix"`
			TargetURL   string   `json:"target_url"`
			Methods     []string `json:"methods"`
			StripPrefix bool     `json:"strip_prefix"`
			RateLimit   int      `json:"rate_limit"`
			Priority    int      `json:"priority"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, expectedRoute.Name, req.Name)
		assert.Equal(t, expectedRoute.PathPrefix, req.PathPrefix)
		assert.Equal(t, ([]string)(nil), req.Methods)
		assert.Equal(t, 0, req.Priority)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(expectedRoute)
	}))
	defer server.Close()

	client := NewClient(server.URL, testAPIKey)
	route, err := client.CreateGatewayRoute("test-route", "/api/v1", "http://backend:8080", nil, true, 100, 0)

	require.NoError(t, err)
	assert.NotNil(t, route)
	assert.Equal(t, expectedRoute.ID, route.ID)
}

func TestGatewayListRoutes(t *testing.T) {
	expectedRoutes := []GatewayRoute{
		{ID: testRouteID, Name: testRouteID},
		{ID: "route-2", Name: "route-2"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, gatewayRoutesURL, r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Response[[]GatewayRoute]{Data: expectedRoutes})
	}))
	defer server.Close()

	client := NewClient(server.URL, testAPIKey)
	routes, err := client.ListGatewayRoutes()

	require.NoError(t, err)
	assert.Len(t, routes, 2)
}

func TestGatewayDeleteRoute(t *testing.T) {
	id := "route-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, gatewayRoutesURL+"/"+id, r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, testAPIKey)
	err := client.DeleteGatewayRoute(id)

	require.NoError(t, err)
}
