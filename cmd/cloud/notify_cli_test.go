package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/poyrazk/thecloud/pkg/sdk"
)

const (
	notifyTestAPIKey  = "notify-key"
	notifyTestTopicID = "topic-1"
	notifyTestTopic   = "alerts"
)

func TestCreateTopicCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/notify/topics" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := map[string]interface{}{
			"data": map[string]interface{}{
				"id":   notifyTestTopicID,
				"name": notifyTestTopic,
				"arn":  "arn:cloud:notify:topic-1",
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = notifyTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	out := captureStdout(t, func() {
		createTopicCmd.Run(createTopicCmd, []string{notifyTestTopic})
	})
	if !strings.Contains(out, "Topic created") || !strings.Contains(out, notifyTestTopicID) {
		t.Fatalf("expected success output, got: %s", out)
	}
}

func TestPublishCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/notify/topics/"+notifyTestTopicID+"/publish" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = notifyTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	out := captureStdout(t, func() {
		publishCmd.Run(publishCmd, []string{notifyTestTopicID, "hello"})
	})
	if !strings.Contains(out, "Message published") {
		t.Fatalf("expected success output, got: %s", out)
	}
}

func TestResolveTopicIDByName(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/notify/topics" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sdk.Response[[]sdk.Topic]{
			Data: []sdk.Topic{
				{ID: "uuid-789", Name: "alerts", ARN: "arn:cloud:notify:alerts"},
			},
		})
	}))
	defer server.Close()

	client := sdk.NewClient(server.URL, "test-key")
	resolved := resolveTopicID("alerts", client)
	if resolved != "uuid-789" {
		t.Fatalf("expected uuid-789, got %s", resolved)
	}
}

func TestResolveTopicIDByUUID(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound) // Should not be called
	}))
	defer server.Close()

	client := sdk.NewClient(server.URL, "test-key")
	id := "abc123-def456"
	resolved := resolveTopicID(id, client)
	if resolved != id {
		t.Fatalf("expected %s, got %s", id, resolved)
	}
}
