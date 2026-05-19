package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/poyrazk/thecloud/pkg/sdk"
	"github.com/spf13/cobra"
)

func TestSGCommands(t *testing.T) {
	if sgCmd == nil {
		t.Fatal("sgCmd is nil")
	}
	if !sgCmd.HasSubCommands() {
		t.Fatal("sgCmd should have subcommands")
	}

	subs := map[string][]string{
		"create":      {"vpc-id", "description"},
		"list":        {"vpc-id"},
		"get":         {},
		"rm":          {},
		"add-rule":    {"direction", "protocol", "port-min", "port-max", "cidr", "priority"},
		"remove-rule": {},
		"attach":      {},
		"detach":      {},
	}

	for cmdName, flags := range subs {
		var sub *cobra.Command
		for _, c := range sgCmd.Commands() {
			if c.Name() == cmdName {
				sub = c
				break
			}
		}
		if sub == nil {
			t.Errorf("sgCmd should have subcommand %q", cmdName)
			continue
		}

		for _, flag := range flags {
			if sub.Flag(flag) == nil {
				t.Errorf("subcommand %q should have flag %q", cmdName, flag)
			}
		}
	}
}

func TestResolveSGIDByName(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/security-groups" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sdk.Response[[]sdk.SecurityGroup]{
			Data: []sdk.SecurityGroup{
				{ID: "uuid-sg-1", Name: "my-sg", VPCID: "vpc-1", ARN: "arn:cloud:sg:1"},
			},
		})
	}))
	defer server.Close()

	client := sdk.NewClient(server.URL, "test-key")
	resolved := resolveSGID("my-sg", client)
	if resolved != "uuid-sg-1" {
		t.Fatalf("expected uuid-sg-1, got %s", resolved)
	}
}

func TestResolveSGIDByUUID(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound) // Should not be called
	}))
	defer server.Close()

	client := sdk.NewClient(server.URL, "test-key")
	id := "abc123-def456"
	resolved := resolveSGID(id, client)
	if resolved != id {
		t.Fatalf("expected %s, got %s", id, resolved)
	}
}
