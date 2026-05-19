package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientCreateDeployment(t *testing.T) {
	expectedDep := Deployment{
		ID:           "dep-1",
		Name:         "test-dep",
		Image:        "nginx:latest",
		Replicas:     3,
		CurrentCount: 0,
		Ports:        "80:80",
		Status:       "CREATING",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/containers/deployments", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req struct {
			Name     string `json:"name"`
			Image    string `json:"image"`
			Replicas int    `json:"replicas"`
			Ports    string `json:"ports"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, expectedDep.Name, req.Name)
		assert.Equal(t, expectedDep.Image, req.Image)
		assert.Equal(t, expectedDep.Replicas, req.Replicas)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(expectedDep)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	dep, err := client.CreateDeployment("test-dep", "nginx:latest", 3, "80:80")

	require.NoError(t, err)
	assert.NotNil(t, dep)
	assert.Equal(t, expectedDep.ID, dep.ID)
	assert.Equal(t, expectedDep.Name, dep.Name)
}

func TestClientListDeployments(t *testing.T) {
	expectedDeps := []Deployment{
		{ID: "dep-1", Name: "dep-1"},
		{ID: "dep-2", Name: "dep-2"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/containers/deployments", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Response[[]Deployment]{Data: expectedDeps})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	deps, err := client.ListDeployments()

	require.NoError(t, err)
	assert.Len(t, deps, 2)
}

func TestClientGetDeployment(t *testing.T) {
	id := "dep-123"
	expectedDep := Deployment{
		ID:   id,
		Name: "test-dep",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/containers/deployments/"+id, r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(expectedDep)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	dep, err := client.GetDeployment(id)

	require.NoError(t, err)
	assert.NotNil(t, dep)
	assert.Equal(t, expectedDep.ID, dep.ID)
}

func TestClientScaleDeployment(t *testing.T) {
	id := "dep-123"
	newReplicas := 5

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/containers/deployments/"+id+"/scale", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req struct {
			Replicas int `json:"replicas"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, newReplicas, req.Replicas)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	err := client.ScaleDeployment(id, newReplicas)

	require.NoError(t, err)
}

func TestClientDeleteDeployment(t *testing.T) {
	id := "dep-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/containers/deployments/"+id, r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	err := client.DeleteDeployment(id)

	require.NoError(t, err)
}
