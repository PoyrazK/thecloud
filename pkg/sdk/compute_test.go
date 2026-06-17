package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	computeInstanceID      = "inst-1"
	computeAPIKey          = "test-key"
	computeContentType     = "Content-Type"
	computeApplicationJSON = "application/json"
	computeNewInstance     = "new-instance"
	computeInstancesPath   = "/instances/"
)

func TestClientListInstances(t *testing.T) {
	mockInstances := []Instance{
		{
			ID:     computeInstanceID,
			Name:   "test-instance",
			Status: "running",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/instances", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, computeAPIKey, r.Header.Get("X-API-Key"))

		w.Header().Set(computeContentType, computeApplicationJSON)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Response[[]Instance]{Data: mockInstances})
	}))
	defer server.Close()

	client := NewClient(server.URL, computeAPIKey)
	instances, err := client.ListInstances()

	require.NoError(t, err)
	assert.Len(t, instances, 1)
	assert.Equal(t, computeInstanceID, instances[0].ID)
}

func TestClientGetInstance(t *testing.T) {
	mockInstance := Instance{
		ID:     computeInstanceID,
		Name:   "test-instance",
		Status: "running",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/instances" && r.Method == "GET" {
			w.Header().Set(computeContentType, computeApplicationJSON)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Response[[]Instance]{Data: []Instance{mockInstance}})
			return
		}
		assert.Equal(t, computeInstancesPath+computeInstanceID, r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set(computeContentType, computeApplicationJSON)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Response[Instance]{Data: mockInstance})
	}))
	defer server.Close()

	client := NewClient(server.URL, computeAPIKey)
	instance, err := client.GetInstance(computeInstanceID)

	require.NoError(t, err)
	assert.Equal(t, computeInstanceID, instance.ID)
}

func TestClientLaunchInstance(t *testing.T) {
	mockInstance := Instance{
		ID:     computeInstanceID,
		Name:   computeNewInstance,
		Status: "running",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/instances", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, computeNewInstance, body["name"])
		assert.Equal(t, "nginx", body["image"])

		w.Header().Set(computeContentType, computeApplicationJSON)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Response[Instance]{Data: mockInstance})
	}))
	defer server.Close()

	client := NewClient(server.URL, computeAPIKey)
	instance, err := client.LaunchInstance(computeNewInstance, "nginx", "80:80", "basic-2", "", "", nil, nil, nil, "", nil)

	require.NoError(t, err)
	assert.Equal(t, computeInstanceID, instance.ID)
	assert.Equal(t, computeNewInstance, instance.Name)
}

func TestClientStopInstance(t *testing.T) {
	mockInstance := Instance{
		ID:     computeInstanceID,
		Name:   "test-instance",
		Status: "running",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/instances" && r.Method == "GET" {
			w.Header().Set(computeContentType, computeApplicationJSON)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Response[[]Instance]{Data: []Instance{mockInstance}})
			return
		}
		assert.Equal(t, computeInstancesPath+computeInstanceID+"/stop", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		w.Header().Set(computeContentType, computeApplicationJSON)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, computeAPIKey)
	err := client.StopInstance(computeInstanceID)

	require.NoError(t, err)
}

func TestClientTerminateInstance(t *testing.T) {
	mockInstance := Instance{
		ID:     computeInstanceID,
		Name:   "test-instance",
		Status: "running",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/instances" && r.Method == "GET" {
			w.Header().Set(computeContentType, computeApplicationJSON)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Response[[]Instance]{Data: []Instance{mockInstance}})
			return
		}
		assert.Equal(t, computeInstancesPath+computeInstanceID, r.URL.Path)
		assert.Equal(t, "DELETE", r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, computeAPIKey)
	err := client.TerminateInstance(computeInstanceID)

	require.NoError(t, err)
}

func TestClientGetInstanceLogs(t *testing.T) {
	mockLogs := "hello world\n"
	mockInstance := Instance{
		ID:     computeInstanceID,
		Name:   "test-instance",
		Status: "running",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/instances" && r.Method == "GET" {
			w.Header().Set(computeContentType, computeApplicationJSON)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Response[[]Instance]{Data: []Instance{mockInstance}})
			return
		}
		assert.Equal(t, computeInstancesPath+computeInstanceID+"/logs", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockLogs))
	}))
	defer server.Close()

	client := NewClient(server.URL, computeAPIKey)
	logs, err := client.GetInstanceLogs(computeInstanceID)

	require.NoError(t, err)
	assert.Equal(t, mockLogs, logs)
}

func TestClientGetInstanceLogsErrorStatus(t *testing.T) {
	mockInstance := Instance{
		ID:     computeInstanceID,
		Name:   "test-instance",
		Status: "running",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/instances" && r.Method == "GET" {
			w.Header().Set(computeContentType, computeApplicationJSON)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Response[[]Instance]{Data: []Instance{mockInstance}})
			return
		}
		assert.Equal(t, computeInstancesPath+computeInstanceID+"/logs", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	client := NewClient(server.URL, computeAPIKey)
	_, err := client.GetInstanceLogs(computeInstanceID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api error")
}

func TestClientGetInstanceLogsRequestError(t *testing.T) {
	client := NewClient("http://127.0.0.1:0", computeAPIKey)
	_, err := client.GetInstanceLogs(computeInstanceID)

	require.Error(t, err)
}

func TestClientGetInstanceStats(t *testing.T) {
	mockStats := InstanceStats{
		CPUPercentage:    15.5,
		MemoryUsageBytes: 1024 * 1024 * 10,
	}
	mockInstance := Instance{
		ID:     computeInstanceID,
		Name:   "test-instance",
		Status: "running",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/instances" && r.Method == "GET" {
			w.Header().Set(computeContentType, computeApplicationJSON)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Response[[]Instance]{Data: []Instance{mockInstance}})
			return
		}
		assert.Equal(t, computeInstancesPath+computeInstanceID+"/stats", r.URL.Path)
		w.Header().Set(computeContentType, computeApplicationJSON)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Response[InstanceStats]{Data: mockStats})
	}))
	defer server.Close()

	client := NewClient(server.URL, computeAPIKey)
	stats, err := client.GetInstanceStats(computeInstanceID)

	require.NoError(t, err)
	assert.InDelta(t, 15.5, stats.CPUPercentage, 0.01)
}

func TestClientComputeErrors(t *testing.T) {
	mockInstance := Instance{
		ID:     computeInstanceID,
		Name:   "test-instance",
		Status: "running",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/instances" && r.Method == "GET" {
			w.Header().Set(computeContentType, computeApplicationJSON)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Response[[]Instance]{Data: []Instance{mockInstance}})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	client := NewClient(server.URL, computeAPIKey)
	_, err := client.GetInstance(computeInstanceID)
	require.Error(t, err)

	_, err = client.GetInstanceStats(computeInstanceID)
	require.Error(t, err)
}

func TestClientLaunchInstanceError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	client := NewClient(server.URL, computeAPIKey)
	_, err := client.LaunchInstance("name", "img", "80", "basic-2", "", "", nil, nil, nil, "", nil)
	require.Error(t, err)
}

func TestClientAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(computeContentType, computeApplicationJSON)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"type": "bad_request", "message": "invalid input"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, computeAPIKey)
	_, err := client.ListInstances()

	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.InvalidInput), "expected InvalidInput error, got: %v", err)
	assert.Contains(t, err.Error(), "invalid input")
}

func TestClientResizeInstance(t *testing.T) {
	mockInstance := Instance{
		ID:     computeInstanceID,
		Name:   "test-instance",
		Status: "running",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/instances" && r.Method == "GET" {
			w.Header().Set(computeContentType, computeApplicationJSON)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Response[[]Instance]{Data: []Instance{mockInstance}})
			return
		}
		assert.Equal(t, computeInstancesPath+computeInstanceID+"/resize", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req map[string]string
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		assert.Equal(t, "basic-4", req["instance_type"])

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, computeAPIKey)
	err := client.ResizeInstance(computeInstanceID, "basic-4")
	require.NoError(t, err)
}

func TestClientResizeInstanceError(t *testing.T) {
	mockInstance := Instance{
		ID:     computeInstanceID,
		Name:   "test-instance",
		Status: "running",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/instances" && r.Method == "GET" {
			w.Header().Set(computeContentType, computeApplicationJSON)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(Response[[]Instance]{Data: []Instance{mockInstance}})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	client := NewClient(server.URL, computeAPIKey)
	err := client.ResizeInstance(computeInstanceID, "basic-4")
	require.Error(t, err)
}
