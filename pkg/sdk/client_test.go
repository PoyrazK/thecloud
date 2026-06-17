package sdk

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	clientTestAPIKey = "key-123"
	clientErrorURL   = "http://127.0.0.1:0"
)

func TestNewClientSetsAPIKey(t *testing.T) {
	client := NewClient("http://localhost", clientTestAPIKey)
	assert.Equal(t, clientTestAPIKey, client.resty.Header.Get("X-API-Key"))
}

func TestClientGetError(t *testing.T) {
	client := NewClient(clientErrorURL, clientTestAPIKey)
	var res Response[map[string]string]

	err := client.get("/fail", &res)
	assert.Error(t, err)
}

func TestClientGetStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad"))
	}))
	defer server.Close()

	client := NewClient(server.URL, clientTestAPIKey)
	var res Response[map[string]string]

	err := client.get("/fail", &res)
	assert.Error(t, err)
}

func TestClientPostError(t *testing.T) {
	client := NewClient(clientErrorURL, clientTestAPIKey)
	var res Response[map[string]string]

	err := client.post("/fail", map[string]string{"a": "b"}, &res)
	assert.Error(t, err)
}

func TestClientPostStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	client := NewClient(server.URL, clientTestAPIKey)
	var res Response[map[string]string]

	err := client.post("/fail", map[string]string{"a": "b"}, &res)
	assert.Error(t, err)
}

func TestClientDeleteError(t *testing.T) {
	client := NewClient(clientErrorURL, clientTestAPIKey)
	var res Response[map[string]string]

	err := client.delete("/fail", &res)
	assert.Error(t, err)
}

func TestClientDeleteStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("nope"))
	}))
	defer server.Close()

	client := NewClient(server.URL, clientTestAPIKey)
	var res Response[map[string]string]

	err := client.delete("/fail", &res)
	assert.Error(t, err)
}

func TestClientPutError(t *testing.T) {
	client := NewClient(clientErrorURL, clientTestAPIKey)
	var res Response[map[string]string]

	err := client.put("/fail", map[string]string{"a": "b"}, &res)
	assert.Error(t, err)
}

func TestClientPutStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte("conflict"))
	}))
	defer server.Close()

	client := NewClient(server.URL, clientTestAPIKey)
	var res Response[map[string]string]

	err := client.put("/fail", map[string]string{"a": "b"}, &res)
	assert.Error(t, err)
}

func TestClientPatchError(t *testing.T) {
	client := NewClient(clientErrorURL, clientTestAPIKey)
	var res Response[map[string]string]

	err := client.patch("/fail", map[string]string{"a": "b"}, &res)
	assert.Error(t, err)
}

func TestClientPatchStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("teapot"))
	}))
	defer server.Close()

	client := NewClient(server.URL, clientTestAPIKey)
	var res Response[map[string]string]

	err := client.patch("/fail", map[string]string{"a": "b"}, &res)
	assert.Error(t, err)
}

func TestResolveID_ValidUUID(t *testing.T) {
	client := NewClient("http://localhost", clientTestAPIKey)
	items := []interface{}{
		struct{ ID, Name string }{"abc123", "test"},
	}
	id, err := client.resolveID("test", func() ([]interface{}, error) { return items, nil },
		func(v interface{}) string { return v.(struct{ ID, Name string }).ID },
		func(v interface{}) string { return v.(struct{ ID, Name string }).Name },
		"abc123")
	require.NoError(t, err)
	assert.Equal(t, "abc123", id)
}

func TestResolveID_NameMatch(t *testing.T) {
	client := NewClient("http://localhost", clientTestAPIKey)
	items := []interface{}{
		struct{ ID, Name string }{"abc123", "test-name"},
	}
	id, err := client.resolveID("test", func() ([]interface{}, error) { return items, nil },
		func(v interface{}) string { return v.(struct{ ID, Name string }).ID },
		func(v interface{}) string { return v.(struct{ ID, Name string }).Name },
		"test-name")
	require.NoError(t, err)
	assert.Equal(t, "abc123", id)
}

func TestResolveID_NotFound(t *testing.T) {
	client := NewClient("http://localhost", clientTestAPIKey)
	items := []interface{}{
		struct{ ID, Name string }{"abc123", "test"},
	}
	_, err := client.resolveID("test", func() ([]interface{}, error) { return items, nil },
		func(v interface{}) string { return v.(struct{ ID, Name string }).ID },
		func(v interface{}) string { return v.(struct{ ID, Name string }).Name },
		"nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestResolveID_Ambiguous(t *testing.T) {
	client := NewClient("http://localhost", clientTestAPIKey)
	items := []interface{}{
		struct{ ID, Name string }{"abc123", "test-a"},
		struct{ ID, Name string }{"abc456", "test-b"},
	}
	_, err := client.resolveID("test", func() ([]interface{}, error) { return items, nil },
		func(v interface{}) string { return v.(struct{ ID, Name string }).ID },
		func(v interface{}) string { return v.(struct{ ID, Name string }).Name },
		"abc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")
}

func TestResolveID_ListError(t *testing.T) {
	client := NewClient("http://localhost", clientTestAPIKey)
	_, err := client.resolveID("test", func() ([]interface{}, error) { return nil, assert.AnError },
		func(v interface{}) string { return "" },
		func(v interface{}) string { return "" },
		"abc")
	require.Error(t, err)
}

func TestParseAPIError_BucketNotFound(t *testing.T) {
	jsonBody := []byte(`{"error": {"type": "BUCKET_NOT_FOUND", "message": "bucket not found"}}`)
	err := parseAPIError(jsonBody)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.BucketNotFound), "expected BucketNotFound, got: %v", err)
	assert.Contains(t, err.Error(), "bucket not found")
}

func TestParseAPIError_NotFound(t *testing.T) {
	jsonBody := []byte(`{"error": {"type": "NOT_FOUND", "message": "resource not found"}}`)
	err := parseAPIError(jsonBody)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.NotFound), "expected NotFound, got: %v", err)
	assert.Contains(t, err.Error(), "resource not found")
}

func TestParseAPIError_InvalidInput(t *testing.T) {
	jsonBody := []byte(`{"error": {"type": "INVALID_INPUT", "message": "invalid input provided"}}`)
	err := parseAPIError(jsonBody)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.InvalidInput), "expected InvalidInput, got: %v", err)
	assert.Contains(t, err.Error(), "invalid input provided")
}

func TestParseAPIError_Forbidden(t *testing.T) {
	jsonBody := []byte(`{"error": {"type": "FORBIDDEN", "message": "access denied"}}`)
	err := parseAPIError(jsonBody)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.Forbidden), "expected Forbidden, got: %v", err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestParseAPIError_LowercaseType(t *testing.T) {
	jsonBody := []byte(`{"error": {"type": "not_found", "message": "item not found"}}`)
	err := parseAPIError(jsonBody)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.NotFound), "expected NotFound (lowercase), got: %v", err)
}

func TestParseAPIError_UnknownType(t *testing.T) {
	jsonBody := []byte(`{"error": {"type": "SOME_UNKNOWN_ERROR", "message": "something went wrong"}}`)
	err := parseAPIError(jsonBody)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.Internal), "expected Internal for unknown type, got: %v", err)
	assert.Contains(t, err.Error(), "something went wrong")
}

func TestParseAPIError_InvalidJSON(t *testing.T) {
	jsonBody := []byte(`not valid json`)
	err := parseAPIError(jsonBody)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.Internal), "expected Internal for invalid JSON, got: %v", err)
}

func TestParseAPIError_MissingErrorField(t *testing.T) {
	jsonBody := []byte(`{"data": "something"}`)
	err := parseAPIError(jsonBody)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.Internal), "expected Internal for missing error field, got: %v", err)
}

func TestClientGet_ParsesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": {"type": "BUCKET_NOT_FOUND", "message": "bucket not found"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, clientTestAPIKey)
	var res Response[any]
	err := client.get("/buckets/test", &res)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.BucketNotFound), "expected BucketNotFound, got: %v", err)
}

func TestClientPost_ParsesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"type": "INVALID_INPUT", "message": "missing required field"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, clientTestAPIKey)
	var res Response[any]
	err := client.post("/test", map[string]string{"a": "b"}, &res)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.InvalidInput), "expected InvalidInput, got: %v", err)
}

func TestClientDelete_ParsesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error": {"type": "FORBIDDEN", "message": "cannot delete"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, clientTestAPIKey)
	var res Response[any]
	err := client.delete("/test/123", &res)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.Forbidden), "expected Forbidden, got: %v", err)
}
