package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/poyrazk/thecloud/pkg/sdk"
)

const (
	instanceTestContentType = "Content-Type"
	instanceTestAppJSON     = "application/json"
	instanceTestPorts       = "80:8080"
	instanceTestSubnetID    = "subnet-1"
	instanceTestVPCID       = "vpc-1"
	instanceTestAPIKey      = "test-key"
)

func TestListInstancesJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/instances" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set(instanceTestContentType, instanceTestAppJSON)
		payload := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":           "i-1",
					"name":         "web",
					"image":        "alpine",
					"status":       "RUNNING",
					"ports":        instanceTestPorts,
					"vpc_id":       instanceTestVPCID,
					"subnet_id":    instanceTestSubnetID,
					"private_ip":   "10.0.0.2",
					"container_id": "c-1",
					"version":      1,
					"created_at":   time.Now().UTC().Format(time.RFC3339),
					"updated_at":   time.Now().UTC().Format(time.RFC3339),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	opts.APIURL = server.URL
	opts.APIKey = instanceTestAPIKey
	opts.JSON = true
	defer func() { opts.JSON = false }()

	out := captureStdout(t, func() {
		listCmd.Run(listCmd, nil)
	})
	if !strings.Contains(out, "\"id\": \"i-1\"") {
		t.Fatalf("expected JSON output to include instance id, got: %s", out)
	}
}

func TestLaunchInstanceVolumeParsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/instances" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set(instanceTestContentType, instanceTestAppJSON)
		payload := map[string]interface{}{
			"data": map[string]interface{}{
				"id":           "i-2",
				"name":         "app",
				"image":        "alpine",
				"status":       "RUNNING",
				"ports":        instanceTestPorts,
				"vpc_id":       instanceTestVPCID,
				"subnet_id":    instanceTestSubnetID,
				"private_ip":   "10.0.0.3",
				"container_id": "c-2",
				"version":      1,
				"created_at":   time.Now().UTC().Format(time.RFC3339),
				"updated_at":   time.Now().UTC().Format(time.RFC3339),
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	opts.APIURL = server.URL
	opts.APIKey = instanceTestAPIKey

	_ = launchCmd.Flags().Set("name", "app")
	_ = launchCmd.Flags().Set("image", "alpine")
	_ = launchCmd.Flags().Set("port", instanceTestPorts)
	_ = launchCmd.Flags().Set("vpc", instanceTestVPCID)
	_ = launchCmd.Flags().Set("subnet", instanceTestSubnetID)
	_ = launchCmd.Flags().Set("volume", "vol-1:/data")

	out := captureStdout(t, func() {
		launchCmd.Run(launchCmd, nil)
	})
	if !strings.Contains(out, "Instance launched successfully") {
		t.Fatalf("expected success message, got: %s", out)
	}
}

func TestGetClientWithApiKeyFlag(t *testing.T) {
	oldKey := opts.APIKey
	defer func() { opts.APIKey = oldKey }()
	opts.APIKey = instanceTestAPIKey

	client := createClient(opts)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestGetClientWithEnvVar(t *testing.T) {
	oldKey := opts.APIKey
	defer func() { opts.APIKey = oldKey }()
	opts.APIKey = ""

	t.Setenv("CLOUD_API_KEY", "env-key")

	client := createClient(opts)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestListCmdCommandSetup(t *testing.T) {
	// Validate the command setup
	if instanceCmd == nil {
		t.Fatal("instanceCmd should not be nil")
	}
	if listCmd == nil {
		t.Fatal("listCmd should not be nil")
	}
}

func TestLaunchCmdFlagsSetup(t *testing.T) {
	if launchCmd.Flag("name") == nil {
		t.Error("launch command should have --name flag")
	}
	if launchCmd.Flag("image") == nil {
		t.Error("launch command should have --image flag")
	}
	if launchCmd.Flag("port") == nil {
		t.Error("launch command should have --port flag")
	}
	if launchCmd.Flag("vpc") == nil {
		t.Error("launch command should have --vpc flag")
	}
	if launchCmd.Flag("subnet") == nil {
		t.Error("launch command should have --subnet flag")
	}
	if launchCmd.Flag("volume") == nil {
		t.Error("launch command should have --volume flag")
	}
}

func TestStopCmdRequiresOneArg(t *testing.T) {
	if stopCmd.Args == nil {
		t.Fatal("stop command should require args")
	}

	// Verify it's set to require exactly 1 arg
	err := stopCmd.Args(stopCmd, []string{})
	if err == nil {
		t.Error("stop command should error with no args")
	}

	err = stopCmd.Args(stopCmd, []string{"id1", "id2"})
	if err == nil {
		t.Error("stop command should error with multiple args")
	}

	err = stopCmd.Args(stopCmd, []string{"id1"})
	if err != nil {
		t.Errorf("stop command should accept 1 arg, got error: %v", err)
	}
}

func TestLogsCmdRequiresOneArg(t *testing.T) {
	if logsCmd.Args == nil {
		t.Fatal("logs command should require args")
	}

	err := logsCmd.Args(logsCmd, []string{})
	if err == nil {
		t.Error("logs command should error with no args")
	}

	err = logsCmd.Args(logsCmd, []string{"id1"})
	if err != nil {
		t.Errorf("logs command should accept 1 arg, got error: %v", err)
	}
}

func TestShowCmdRequiresOneArg(t *testing.T) {
	if showCmd.Args == nil {
		t.Fatal("show command should require args")
	}

	err := showCmd.Args(showCmd, []string{})
	if err == nil {
		t.Error("show command should error with no args")
	}
}

func TestRmCmdRequiresOneArg(t *testing.T) {
	if rmCmd.Args == nil {
		t.Fatal("rm command should require args")
	}

	err := rmCmd.Args(rmCmd, []string{})
	if err == nil {
		t.Error("rm command should error with no args")
	}
}

func TestStatsCmdRequiresOneArg(t *testing.T) {
	if statsCmd.Args == nil {
		t.Fatal("stats command should require args")
	}

	err := statsCmd.Args(statsCmd, []string{})
	if err == nil {
		t.Error("stats command should error with no args")
	}
}

func TestInstanceCmdHasSubcommands(t *testing.T) {
	subcommands := instanceCmd.Commands()

	expectedCommands := []string{"list", "launch", "stop", "logs", "show", "rm", "resize", "stats"}
	found := make(map[string]bool)

	for _, cmd := range subcommands {
		found[cmd.Name()] = true
	}

	for _, expected := range expectedCommands {
		if !found[expected] {
			t.Errorf("instance command missing subcommand: %s", expected)
		}
	}
}

func TestResizeCmdRequiresTwoArgs(t *testing.T) {
	if resizeCmd.Args == nil {
		t.Fatal("resize command should require args")
	}

	err := resizeCmd.Args(resizeCmd, []string{})
	if err == nil {
		t.Error("resize command should error with no args")
	}

	err = resizeCmd.Args(resizeCmd, []string{"id1"})
	if err == nil {
		t.Error("resize command should error with only 1 arg")
	}

	err = resizeCmd.Args(resizeCmd, []string{"id1", "basic-4"})
	if err != nil {
		t.Errorf("resize command should accept 2 args, got error: %v", err)
	}
}

const portMapping = "8080:80"

func TestFormatAccessPorts(t *testing.T) {
	tests := []struct {
		name     string
		ports    string
		status   string
		expected string
	}{
		{
			name:     "no ports",
			ports:    "",
			status:   "RUNNING",
			expected: "-",
		},
		{
			name:     "stopped instance",
			ports:    portMapping,
			status:   "STOPPED",
			expected: "-",
		},
		{
			name:     "single port mapping",
			ports:    portMapping,
			status:   "RUNNING",
			expected: "localhost:8080->80",
		},
		{
			name:     "multiple port mappings",
			ports:    portMapping + ",8443:443",
			status:   "RUNNING",
			expected: "localhost:8080->80, localhost:8443->443",
		},
		{
			name:     "invalid port format",
			ports:    "8080",
			status:   "RUNNING",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			access := formatAccessPorts(tt.ports, tt.status)

			if access != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, access)
			}
		})
	}
}

func TestTruncateID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		expected string
	}{
		{
			name:     "short id",
			id:       "abc123",
			expected: "abc123",
		},
		{
			name:     "long id",
			id:       "abcdef123456789",
			expected: "abcdef12",
		},
		{
			name:     "exactly 8 chars",
			id:       "12345678",
			expected: "12345678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateID(tt.id)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestJSONMarshalIndent(t *testing.T) {
	inst := sdk.Instance{
		ID:     "test-id",
		Name:   "test-instance",
		Image:  "alpine",
		Status: "RUNNING",
		Ports:  portMapping,
	}

	data, err := json.MarshalIndent(inst, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	if !strings.Contains(string(data), "test-id") {
		t.Error("JSON should contain instance ID")
	}
	if !strings.Contains(string(data), "test-instance") {
		t.Error("JSON should contain instance name")
	}
}
