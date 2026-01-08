package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient_ListVPCs(t *testing.T) {
	mockVPCs := []VPC{
		{
			ID:   "vpc-1",
			Name: "test-vpc",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vpcs", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Response[[]VPC]{Data: mockVPCs})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	vpcs, err := client.ListVPCs()

	assert.NoError(t, err)
	assert.Len(t, vpcs, 1)
	assert.Equal(t, "vpc-1", vpcs[0].ID)
}

func TestClient_CreateVPC(t *testing.T) {
	mockVPC := VPC{
		ID:   "vpc-1",
		Name: "new-vpc",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vpcs", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "new-vpc", body["name"])
		assert.Equal(t, "10.0.0.0/16", body["cidr_block"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Response[VPC]{Data: mockVPC})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	vpc, err := client.CreateVPC("new-vpc", "10.0.0.0/16")

	assert.NoError(t, err)
	assert.Equal(t, "vpc-1", vpc.ID)
	assert.Equal(t, "new-vpc", vpc.Name)
}

func TestClient_GetVPC(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vpcs/vpc-1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Response[VPC]{Data: VPC{ID: "vpc-1"}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	vpc, err := client.GetVPC("vpc-1")

	assert.NoError(t, err)
	assert.Equal(t, "vpc-1", vpc.ID)
}

func TestClient_DeleteVPC(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vpcs/vpc-1", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	err := client.DeleteVPC("vpc-1")

	assert.NoError(t, err)
}
