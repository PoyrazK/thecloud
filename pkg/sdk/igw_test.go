package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientCreateIGW(t *testing.T) {
	t.Parallel()
	expectedIGW := InternetGateway{
		ID:     "igw-123",
		Status: IGWStatusDetached,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/internet-gateways", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Response[InternetGateway]{Data: expectedIGW})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	igw, err := client.CreateIGW()

	require.NoError(t, err)
	assert.NotNil(t, igw)
	assert.Equal(t, expectedIGW.ID, igw.ID)
	assert.Equal(t, expectedIGW.Status, igw.Status)
}

func TestClientListIGWs(t *testing.T) {
	t.Parallel()
	expectedIGWs := []InternetGateway{
		{ID: "igw-1", Status: IGWStatusDetached},
		{ID: "igw-2", Status: IGWStatusAttached},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/internet-gateways", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Response[[]InternetGateway]{Data: expectedIGWs})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	igws, err := client.ListIGWs()

	require.NoError(t, err)
	assert.Len(t, igws, 2)
	assert.Equal(t, expectedIGWs[0].ID, igws[0].ID)
}

func TestClientGetIGW(t *testing.T) {
	t.Parallel()
	id := "igw-123"
	expectedIGW := InternetGateway{
		ID:     id,
		Status: IGWStatusAttached,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/internet-gateways/"+id, r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Response[InternetGateway]{Data: expectedIGW})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	igw, err := client.GetIGW(id)

	require.NoError(t, err)
	assert.NotNil(t, igw)
	assert.Equal(t, expectedIGW.ID, igw.ID)
}

func TestClientDeleteIGW(t *testing.T) {
	t.Parallel()
	id := "igw-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/internet-gateways/"+id, r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	err := client.DeleteIGW(id)

	require.NoError(t, err)
}

func TestClientAttachIGW(t *testing.T) {
	t.Parallel()
	igwID := "igw-123"
	vpcID := "vpc-456"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/internet-gateways/"+igwID+"/attach", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req map[string]string
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, vpcID, req["vpc_id"])

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	err := client.AttachIGW(igwID, vpcID)

	require.NoError(t, err)
}

func TestClientDetachIGW(t *testing.T) {
	t.Parallel()
	igwID := "igw-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/internet-gateways/"+igwID+"/detach", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	err := client.DetachIGW(igwID)

	require.NoError(t, err)
}

func TestClientCreateIGWError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	_, err := client.CreateIGW()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api error")
}

func TestClientListIGWsError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	_, err := client.ListIGWs()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api error")
}

func TestClientGetIGWError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	_, err := client.GetIGW("nonexistent")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api error")
}

func TestClientDeleteIGWError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	err := client.DeleteIGW("igw-123")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api error")
}
