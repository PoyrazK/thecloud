package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

const (
	rbacTestAPIKey   = "rbac-key"
	rbacTestRoleID   = "22222222-2222-2222-2222-222222222222"
	rbacTestRoleName = "viewer"
)

func TestCreateRoleCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/rbac/roles" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := map[string]interface{}{
			"data": map[string]interface{}{
				"id":          rbacTestRoleID,
				"name":        rbacTestRoleName,
				"description": "read-only",
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	t.Setenv("HOME", t.TempDir())
	saveConfig(rbacTestAPIKey)

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = rbacTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	_ = createRoleCmd.Flags().Set("description", "read-only")
	_ = createRoleCmd.Flags().Set("permissions", string(domain.PermissionInstanceRead))

	out := captureStdout(t, func() {
		createRoleCmd.Run(createRoleCmd, []string{rbacTestRoleName})
	})
	if !strings.Contains(out, "Role created") || !strings.Contains(out, rbacTestRoleID) {
		t.Fatalf("expected success output, got: %s", out)
	}
}

func TestListRolesCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/rbac/roles" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":          rbacTestRoleID,
					"name":        rbacTestRoleName,
					"permissions": []string{"instance:read"},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	t.Setenv("HOME", t.TempDir())
	saveConfig(rbacTestAPIKey)

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = rbacTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	out := captureStdout(t, func() {
		listRolesCmd.Run(listRolesCmd, nil)
	})
	if !strings.Contains(out, rbacTestRoleName) {
		t.Fatalf("expected list output to include role name, got: %s", out)
	}
}

func TestBindRoleCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/rbac/bindings" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("HOME", t.TempDir())
	saveConfig(rbacTestAPIKey)

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = rbacTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	out := captureStdout(t, func() {
		bindRoleCmd.Run(bindRoleCmd, []string{"user@example.com", rbacTestRoleName})
	})
	if !strings.Contains(out, "Role") || !strings.Contains(out, rbacTestRoleName) {
		t.Fatalf("expected success output, got: %s", out)
	}
}

func TestDeleteRoleCmd(t *testing.T) {
	roleID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/rbac/roles/"+roleID.String() || r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("HOME", t.TempDir())
	saveConfig(rbacTestAPIKey)

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	opts.APIURL = server.URL
	opts.APIKey = rbacTestAPIKey
	defer func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
	}()

	out := captureStdout(t, func() {
		deleteRoleCmd.Run(deleteRoleCmd, []string{roleID.String()})
	})
	if !strings.Contains(out, "Role") || !strings.Contains(out, roleID.String()) {
		t.Fatalf("expected delete output, got: %s", out)
	}
}
