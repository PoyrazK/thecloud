package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListOperations_WrappedResponse verifies that list operations correctly
// unmarshal the API's wrapped response format {"data": [...], "meta": {...}}
func TestListOperations_WrappedResponse(t *testing.T) {
	// Test data
	deployments := []Deployment{
		{ID: "dep-1", Name: "web", Image: "nginx", Replicas: 2},
		{ID: "dep-2", Name: "api", Image: "go-api", Replicas: 1},
	}
	cronJobs := []CronJob{
		{ID: "cron-1", Name: "backup", Schedule: "0 0 * * *"},
		{ID: "cron-2", Name: "cleanup", Schedule: "0 1 * * *"},
	}
	gatewayRoutes := []GatewayRoute{
		{ID: "route-1", Name: "api-route", PathPrefix: "/api"},
		{ID: "route-2", Name: "web-route", PathPrefix: "/web"},
	}
	snapshots := []*domain.Snapshot{
		{ID: uuid.New(), Description: "snap-1"},
		{ID: uuid.New(), Description: "snap-2"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/containers/deployments":
			_ = json.NewEncoder(w).Encode(Response[[]Deployment]{Data: deployments})
		case "/cron/jobs":
			_ = json.NewEncoder(w).Encode(Response[[]CronJob]{Data: cronJobs})
		case "/gateway/routes":
			_ = json.NewEncoder(w).Encode(Response[[]GatewayRoute]{Data: gatewayRoutes})
		case "/snapshots":
			_ = json.NewEncoder(w).Encode(Response[[]*domain.Snapshot]{Data: snapshots})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, testAPIKey)

	t.Run("ListDeployments", func(t *testing.T) {
		deps, err := client.ListDeployments()
		require.NoError(t, err)
		assert.Len(t, deps, 2)
		assert.Equal(t, "dep-1", deps[0].ID)
		assert.Equal(t, "web", deps[0].Name)
	})

	t.Run("ListCronJobs", func(t *testing.T) {
		jobs, err := client.ListCronJobs()
		require.NoError(t, err)
		assert.Len(t, jobs, 2)
		assert.Equal(t, "cron-1", jobs[0].ID)
	})

	t.Run("ListGatewayRoutes", func(t *testing.T) {
		routes, err := client.ListGatewayRoutes()
		require.NoError(t, err)
		assert.Len(t, routes, 2)
		assert.Equal(t, "route-1", routes[0].ID)
	})

	t.Run("ListSnapshots", func(t *testing.T) {
		snaps, err := client.ListSnapshots()
		require.NoError(t, err)
		assert.Len(t, snaps, 2)
	})
}

// TestListOperations_EmptyResponse verifies that list operations handle empty responses
func TestListOperations_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return empty wrapped response
		_ = json.NewEncoder(w).Encode(Response[[]Deployment]{Data: []Deployment{}})
	}))
	defer server.Close()

	client := NewClient(server.URL, testAPIKey)

	deps, err := client.ListDeployments()
	require.NoError(t, err)
	assert.Empty(t, deps)
}

// TestListOperations_WithMeta verifies that list operations correctly ignore meta field
func TestListOperations_WithMeta(t *testing.T) {
	deployments := []Deployment{{ID: "dep-1", Name: "test"}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Response includes meta field - should be ignored by SDK
		resp := struct {
			Data interface{} `json:"data"`
			Meta struct {
				RequestID string `json:"request_id"`
			} `json:"meta"`
		}{
			Data: deployments,
			Meta: struct {
				RequestID string `json:"request_id"`
			}{RequestID: "req-123"},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, testAPIKey)

	deps, err := client.ListDeployments()
	require.NoError(t, err)
	assert.Len(t, deps, 1)
	assert.Equal(t, "dep-1", deps[0].ID)
}
