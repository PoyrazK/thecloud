package sdk

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	storageTestBucket      = "my-bucket"
	storageTestKey         = "file.txt"
	storageTestContent     = "hello world"
	storageContentType     = "Content-Type"
	storageApplicationJSON = "application/json"
	storageAPIKey          = "test-api-key"
	storagePathPrefix      = "/storage/"
	storageBucketName      = "bucket-1"
	storageBucketsPath     = "/storage/buckets/"
	storageRuleID          = "rule-1"
)

func TestClientListObjects(t *testing.T) {
	bucket := storageTestBucket
	expectedObjects := []Object{
		{ID: "obj-1", Key: "file1.txt", Bucket: bucket, CreatedAt: time.Now()},
		{ID: "obj-2", Key: "file2.txt", Bucket: bucket, CreatedAt: time.Now()},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, storagePathPrefix+bucket, r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set(storageContentType, storageApplicationJSON)
		resp := Response[[]Object]{Data: expectedObjects}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	objects, err := client.ListObjects(bucket)

	require.NoError(t, err)
	assert.Len(t, objects, 2)
	assert.Equal(t, expectedObjects[0].Key, objects[0].Key)
}

func TestClientUploadObject(t *testing.T) {
	bucket := storageTestBucket
	key := storageTestKey
	content := storageTestContent

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, storagePathPrefix+bucket+"/"+key, r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Equal(t, content, string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	obj, err := client.UploadObject(bucket, key, strings.NewReader(content))

	require.NoError(t, err)
	assert.NotNil(t, obj)
}

func TestClientUploadObjectErrorStatus(t *testing.T) {
	bucket := storageTestBucket
	key := storageTestKey

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, storagePathPrefix+bucket+"/"+key, r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	_, err := client.UploadObject(bucket, key, strings.NewReader("data"))

	require.Error(t, err)
}

func TestClientUploadObjectRequestError(t *testing.T) {
	client := NewClient("http://127.0.0.1:0", storageAPIKey)
	_, err := client.UploadObject("bucket", "key", strings.NewReader("data"))

	require.Error(t, err)
}

func TestClientDownloadObject(t *testing.T) {
	bucket := storageTestBucket
	key := storageTestKey
	content := storageTestContent

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, storagePathPrefix+bucket+"/"+key, r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		_, _ = w.Write([]byte(content))
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	readCloser, err := client.DownloadObject(bucket, key)

	require.NoError(t, err)
	defer func() { _ = readCloser.Close() }()

	body, err := io.ReadAll(readCloser)
	require.NoError(t, err)
	assert.Equal(t, content, string(body))
}

func TestClientDeleteObject(t *testing.T) {
	bucket := storageTestBucket
	key := storageTestKey

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, storagePathPrefix+bucket+"/"+key, r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	err := client.DeleteObject(bucket, key)

	require.NoError(t, err)
}

func TestClientListVersions(t *testing.T) {
	bucket := storageTestBucket
	key := storageTestKey
	expected := []Object{{ID: "obj-1", VersionID: "v1"}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/storage/versions/"+bucket+"/"+key, r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set(storageContentType, storageApplicationJSON)
		_ = json.NewEncoder(w).Encode(Response[[]Object]{Data: expected})
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	versions, err := client.ListVersions(bucket, key)

	require.NoError(t, err)
	assert.Len(t, versions, 1)
	assert.Equal(t, "v1", versions[0].VersionID)
}

func TestClientDownloadObjectWithVersion(t *testing.T) {
	bucket := storageTestBucket
	key := storageTestKey
	version := "v1"
	content := storageTestContent

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, storagePathPrefix+bucket+"/"+key, r.URL.Path)
		assert.Equal(t, version, r.URL.Query().Get("versionId"))
		_, _ = w.Write([]byte(content))
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	readCloser, err := client.DownloadObject(bucket, key, version)

	require.NoError(t, err)
	defer func() { _ = readCloser.Close() }()

	body, err := io.ReadAll(readCloser)
	require.NoError(t, err)
	assert.Equal(t, content, string(body))
}

func TestClientDownloadObjectErrorStatus(t *testing.T) {
	bucket := storageTestBucket
	key := storageTestKey

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, storagePathPrefix+bucket+"/"+key, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	_, err := client.DownloadObject(bucket, key)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api error")
}

func TestClientDownloadObjectRequestError(t *testing.T) {
	client := NewClient("http://127.0.0.1:0", storageAPIKey)
	_, err := client.DownloadObject("bucket", "key")

	require.Error(t, err)
}

func TestClientDeleteObjectWithVersion(t *testing.T) {
	bucket := storageTestBucket
	key := storageTestKey
	version := "v1"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, storagePathPrefix+bucket+"/"+key, r.URL.Path)
		assert.Equal(t, version, r.URL.Query().Get("versionId"))
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	err := client.DeleteObject(bucket, key, version)

	require.NoError(t, err)
}

func TestClientCreateBucket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/storage/buckets", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var payload struct {
			Name     string `json:"name"`
			IsPublic bool   `json:"is_public"`
		}
		err := json.NewDecoder(r.Body).Decode(&payload)
		assert.NoError(t, err)
		assert.Equal(t, storageBucketName, payload.Name)
		assert.True(t, payload.IsPublic)

		w.Header().Set(storageContentType, storageApplicationJSON)
		_ = json.NewEncoder(w).Encode(Response[Bucket]{Data: Bucket{ID: "b1", Name: storageBucketName, IsPublic: true}})
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	bucket, err := client.CreateBucket(storageBucketName, true)

	require.NoError(t, err)
	assert.Equal(t, "b1", bucket.ID)
}

func TestClientListBuckets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/storage/buckets", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set(storageContentType, storageApplicationJSON)
		_ = json.NewEncoder(w).Encode(Response[[]Bucket]{Data: []Bucket{{ID: "b1", Name: storageBucketName}}})
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	buckets, err := client.ListBuckets()

	require.NoError(t, err)
	assert.Len(t, buckets, 1)
	assert.Equal(t, storageBucketName, buckets[0].Name)
}

func TestClientDeleteBucket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, storageBucketsPath+storageBucketName, r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	err := client.DeleteBucket(context.Background(), storageBucketName)

	require.NoError(t, err)
}

func TestClientSetBucketVersioning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, storageBucketsPath+storageBucketName+"/versioning", r.URL.Path)
		assert.Equal(t, http.MethodPatch, r.Method)

		var payload struct {
			Enabled bool `json:"enabled"`
		}
		err := json.NewDecoder(r.Body).Decode(&payload)
		assert.NoError(t, err)
		assert.True(t, payload.Enabled)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	err := client.SetBucketVersioning(storageBucketName, true)

	require.NoError(t, err)
}

func TestClientGetStorageClusterStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/storage/cluster/status", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set(storageContentType, storageApplicationJSON)
		_ = json.NewEncoder(w).Encode(Response[StorageCluster]{Data: StorageCluster{Nodes: []StorageNode{{ID: "node-1"}}}})
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	status, err := client.GetStorageClusterStatus()

	require.NoError(t, err)
	assert.Len(t, status.Nodes, 1)
	assert.Equal(t, "node-1", status.Nodes[0].ID)
}

func TestClientGeneratePresignedURL(t *testing.T) {
	bucket := storageTestBucket
	key := storageTestKey
	method := http.MethodGet

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/storage/presign/"+bucket+"/"+key, r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var payload struct {
			Method    string `json:"method"`
			ExpirySec int    `json:"expiry_seconds"`
		}
		err := json.NewDecoder(r.Body).Decode(&payload)
		assert.NoError(t, err)
		assert.Equal(t, method, payload.Method)
		assert.Equal(t, 60, payload.ExpirySec)

		w.Header().Set(storageContentType, storageApplicationJSON)
		_ = json.NewEncoder(w).Encode(Response[PresignedURL]{Data: PresignedURL{URL: "http://example.com", Method: method}})
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	url, err := client.GeneratePresignedURL(bucket, key, method, 60)

	require.NoError(t, err)
	assert.Equal(t, "http://example.com", url.URL)
}

func TestClientGeneratePresignedURL_WithLeadingSlash(t *testing.T) {
	bucket := storageTestBucket
	method := http.MethodGet

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "key with leading slash",
			key:      "/file.txt",
			expected: "/storage/presign/" + bucket + "/file.txt",
		},
		{
			name:     "key without leading slash unchanged",
			key:      "file.txt",
			expected: "/storage/presign/" + bucket + "/file.txt",
		},
		{
			name:     "key is root slash only",
			key:      "/",
			expected: "/storage/presign/" + bucket + "/",
		},
		{
			name:     "key with multiple leading slashes strips only first",
			key:      "//file.txt",
			expected: "/storage/presign/" + bucket + "//file.txt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tc.expected, r.URL.Path)
				w.Header().Set(storageContentType, storageApplicationJSON)
				_ = json.NewEncoder(w).Encode(Response[PresignedURL]{Data: PresignedURL{URL: "http://example.com", Method: method}})
			}))
			defer server.Close()

			client := NewClient(server.URL, storageAPIKey)
			url, err := client.GeneratePresignedURL(bucket, tc.key, method, 60)

			require.NoError(t, err)
			assert.Equal(t, "http://example.com", url.URL)
		})
	}
}

func TestClientLifecycleRules(t *testing.T) {
	bucket := storageTestBucket
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == storageBucketsPath+bucket+"/lifecycle":
			var payload struct {
				Prefix         string `json:"prefix"`
				ExpirationDays int    `json:"expiration_days"`
				Enabled        bool   `json:"enabled"`
			}
			err := json.NewDecoder(r.Body).Decode(&payload)
			assert.NoError(t, err)
			assert.Equal(t, "logs/", payload.Prefix)
			assert.Equal(t, 30, payload.ExpirationDays)
			assert.True(t, payload.Enabled)

			w.Header().Set(storageContentType, storageApplicationJSON)
			_ = json.NewEncoder(w).Encode(Response[LifecycleRule]{Data: LifecycleRule{ID: storageRuleID, BucketName: bucket}})
		case r.Method == http.MethodGet && r.URL.Path == storageBucketsPath+bucket+"/lifecycle":
			w.Header().Set(storageContentType, storageApplicationJSON)
			_ = json.NewEncoder(w).Encode(Response[[]LifecycleRule]{Data: []LifecycleRule{{ID: storageRuleID, BucketName: bucket}}})
		case r.Method == http.MethodDelete && r.URL.Path == storageBucketsPath+bucket+"/lifecycle/"+storageRuleID:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	created, err := client.CreateLifecycleRule(bucket, "logs/", 30, true)
	require.NoError(t, err)
	assert.Equal(t, storageRuleID, created.ID)

	rules, err := client.ListLifecycleRules(bucket)
	require.NoError(t, err)
	assert.Len(t, rules, 1)

	err = client.DeleteLifecycleRule(bucket, storageRuleID)
	require.NoError(t, err)
}

func TestClientStorageListErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	_, err := client.ListObjects("bucket")
	require.Error(t, err)

	_, err = client.ListVersions("bucket", "key")
	require.Error(t, err)

	_, err = client.ListBuckets()
	require.Error(t, err)

	_, err = client.GetStorageClusterStatus()
	require.Error(t, err)

	_, err = client.ListLifecycleRules("bucket")
	require.Error(t, err)
}

func TestClientStorageCreateErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	client := NewClient(server.URL, storageAPIKey)
	_, err := client.CreateBucket("bucket", true)
	require.Error(t, err)

	_, err = client.GeneratePresignedURL("bucket", "key", http.MethodGet, 60)
	require.Error(t, err)

	_, err = client.CreateLifecycleRule("bucket", "logs/", 7, true)
	require.Error(t, err)
}
