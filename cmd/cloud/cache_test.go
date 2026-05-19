package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/poyrazk/thecloud/pkg/sdk"
)

func TestFormatBytes(t *testing.T) {
	if got := formatBytes(0); got != "0 B" {
		t.Fatalf("expected 0 B, got %q", got)
	}
	if got := formatBytes(1024); got != "1.0 KB" {
		t.Fatalf("expected 1.0 KB, got %q", got)
	}
	if got := formatBytes(1024 * 1024); got != "1.0 MB" {
		t.Fatalf("expected 1.0 MB, got %q", got)
	}
}

func TestResolveCacheIDByName(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/caches" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sdk.Response[[]sdk.Cache]{
			Data: []sdk.Cache{
				{ID: "uuid-123", Name: "my-cache", Status: "RUNNING"},
			},
		})
	}))
	defer server.Close()

	client := sdk.NewClient(server.URL, "test-key")
	resolved := resolveCacheID("my-cache", client)
	if resolved != "uuid-123" {
		t.Fatalf("expected uuid-123, got %s", resolved)
	}
}

func TestResolveCacheIDByUUID(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound) // Should not be called
	}))
	defer server.Close()

	client := sdk.NewClient(server.URL, "test-key")
	id := "abc123-def456-ghi789"
	resolved := resolveCacheID(id, client)
	if resolved != id {
		t.Fatalf("expected %s, got %s", id, resolved)
	}
}

func TestResolveCacheIDNotFound(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/caches" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sdk.Response[[]sdk.Cache]{
			Data: []sdk.Cache{},
		})
	}))
	defer server.Close()

	client := sdk.NewClient(server.URL, "test-key")
	resolved := resolveCacheID("nonexistent", client)
	if resolved != "nonexistent" {
		t.Fatalf("expected nonexistent (unchanged), got %s", resolved)
	}
}

func TestResolveCacheIDAPIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := sdk.NewClient(server.URL, "test-key")
	resolved := resolveCacheID("any-name", client)
	if resolved != "any-name" {
		t.Fatalf("expected any-name (fallback), got %s", resolved)
	}
}
