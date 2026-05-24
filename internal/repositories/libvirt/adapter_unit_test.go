package libvirt

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"regexp"
	"testing"
	"time"

	"github.com/digitalocean/go-libvirt"
	"github.com/poyrazk/thecloud/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testEnvVar  = "FOO=bar"
	testVolPath = "/path/to/vol1"
)

func TestSanitizeDomainName(t *testing.T) {
	t.Parallel()
	a := &LibvirtAdapter{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid name",
			input:    "my-instance",
			expected: "my-instance",
		},
		{
			name:     "with invalid chars",
			input:    "my@instance#123",
			expected: "myinstance123",
		},
		{
			name:     "with spaces",
			input:    "my instance",
			expected: "myinstance",
		},
		{
			name:     "with special chars",
			input:    "test_vm.local",
			expected: "testvmlocal",
		},
		{
			name:     "only valid chars",
			input:    "abc123-test",
			expected: "abc123-test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := a.sanitizeDomainName(tt.input)
			if tt.input == "" {
				// Empty string generates UUID
				assert.NotEmpty(t, result)
				assert.Len(t, result, 8)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSanitizeDomainNameEmptyString(t *testing.T) {
	t.Parallel()
	a := &LibvirtAdapter{}
	result := a.sanitizeDomainName("")

	assert.NotEmpty(t, result, "empty name should generate UUID")
	assert.Len(t, result, 8, "generated name should be 8 chars")
}

func TestParseAndValidatePort(t *testing.T) {
	t.Parallel()
	a := &LibvirtAdapter{}

	tests := []struct {
		input     string
		wantHost  bool
		wantError bool
	}{
		{"8080:80", true, false},
		{"80", true, false},
		{"invalid", false, true},
		{"80:80:80", false, true},
		{"abc:80", false, true},
	}

	for _, tt := range tests {
		h, c, err := a.parseAndValidatePort(tt.input)
		if tt.wantError {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			assert.Positive(t, h)
			assert.Positive(t, c)
		}
	}
}

func TestGenerateUserData(t *testing.T) {
	t.Parallel()
	a := &LibvirtAdapter{}

	tests := []struct {
		name        string
		env         []string
		cmd         []string
		shouldExist []string
	}{
		{
			name:        "with env vars",
			env:         []string{testEnvVar, "DEBUG=true"},
			cmd:         nil,
			shouldExist: []string{testEnvVar, "DEBUG=true"},
		},
		{
			name:        "with command",
			env:         nil,
			cmd:         []string{"echo", "hello"},
			shouldExist: []string{"echo", "hello"},
		},
		{
			name:        "with both",
			env:         []string{"PATH=/usr/bin"},
			cmd:         []string{"/bin/sh", "-c", "echo test"},
			shouldExist: []string{"PATH=/usr/bin", "/bin/sh", "-c", "echo test"},
		},
		{
			name:        "empty",
			env:         nil,
			cmd:         nil,
			shouldExist: []string{"#cloud-config"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := a.generateUserData(tt.env, tt.cmd, "")
			content := string(result)

			assert.Contains(t, content, "#cloud-config")
			for _, expected := range tt.shouldExist {
				assert.Contains(t, content, expected)
			}
		})
	}
}

func TestResolveBinds(t *testing.T) {
	t.Parallel()
	a := &LibvirtAdapter{}

	tests := []struct {
		name  string
		binds []string
	}{
		{
			name:  "nil binds",
			binds: nil,
		},
		{
			name:  "empty binds",
			binds: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := a.resolveBinds(context.Background(), tt.binds)
			assert.Empty(t, result)
		})
	}

	t.Run("successful pool lookup", func(t *testing.T) {
		t.Parallel()
		m := new(MockLibvirtClient)
		a := &LibvirtAdapter{client: m}
		ctx := context.Background()
		pool := libvirt.StoragePool{Name: "default"}
		vol := libvirt.StorageVol{Name: "vol1"}

		m.On("StoragePoolLookupByName", ctx, "default").Return(pool, nil)
		m.On("StorageVolLookupByName", ctx, pool, "vol1").Return(vol, nil)
		m.On("StorageVolGetPath", ctx, vol).Return(testVolPath, nil)

		result := a.resolveBinds(ctx, []string{"vol1:/data"})
		assert.Len(t, result, 1)
		assert.Equal(t, testVolPath, result[0])
		m.AssertExpectations(t)
	})
}

func TestPrepareCloudInit(t *testing.T) {
	t.Parallel()
	a := &LibvirtAdapter{}

	tests := []struct {
		name     string
		env      []string
		cmd      []string
		expected string
	}{
		{
			name:     "no env or cmd",
			env:      nil,
			cmd:      nil,
			expected: "",
		},
		{
			name:     "empty slices",
			env:      []string{},
			cmd:      []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := a.prepareCloudInit(context.Background(), "test", tt.env, tt.cmd, "")
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "valid simple",
			id:      "instance-123",
			wantErr: false,
		},
		{
			name:    "valid uuid",
			id:      "abc123-def456",
			wantErr: false,
		},
		{
			name:    "path traversal",
			id:      "../etc/passwd",
			wantErr: true,
		},
		{
			name:    "absolute path",
			id:      "/etc/passwd",
			wantErr: true,
		},
		{
			name:    "dot dot",
			id:      "test/../admin",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateID(tt.id)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParseAndValidatePortExtended(t *testing.T) {
	t.Parallel()
	a := &LibvirtAdapter{}

	tests := []struct {
		name    string
		port    string
		hostP   int
		contP   int
		wantErr bool
	}{
		{
			name:    "same port",
			port:    "80:80",
			hostP:   80,
			contP:   80,
			wantErr: false,
		},
		{
			name:    "different ports",
			port:    "8080:80",
			hostP:   8080,
			contP:   80,
			wantErr: false,
		},
		{
			name:    "high port numbers",
			port:    "65535:8080",
			hostP:   65535,
			contP:   8080,
			wantErr: false,
		},
		{
			name:    "invalid format three parts",
			port:    "80:80:80",
			wantErr: true,
		},
		{
			name:    "invalid host port",
			port:    "abc:80",
			wantErr: true,
		},
		{
			name:    "invalid container port",
			port:    "80:xyz",
			wantErr: true,
		},
		{
			name:    "empty string",
			port:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			host, cont, err := a.parseAndValidatePort(tt.port)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.hostP, host)
				assert.Equal(t, tt.contP, cont)
			}
		})
	}
}

func TestResolveVolumePath(t *testing.T) {
	t.Parallel()
	a := &LibvirtAdapter{}
	skipLibvirt := fmt.Errorf("skip libvirt")

	t.Run("LVM path", func(t *testing.T) {
		t.Parallel()
		path := a.resolveVolumePath(context.Background(), "/dev/vg0/lv0", libvirt.StoragePool{}, skipLibvirt)
		assert.Equal(t, "/dev/vg0/lv0", path)
	})

	t.Run("Direct file path", func(t *testing.T) {
		t.Parallel()
		// Use a file that definitely exists
		path := a.resolveVolumePath(context.Background(), "/etc/hosts", libvirt.StoragePool{}, skipLibvirt)
		assert.Equal(t, "/etc/hosts", path)
	})

	t.Run("Non-existent path", func(t *testing.T) {
		t.Parallel()
		path := a.resolveVolumePath(context.Background(), "/non/existent/path", libvirt.StoragePool{}, skipLibvirt)
		assert.Equal(t, "", path)
	})

	t.Run("Libvirt pool lookup success", func(t *testing.T) {
		t.Parallel()
		m := new(MockLibvirtClient)
		a := &LibvirtAdapter{client: m}
		ctx := context.Background()
		pool := libvirt.StoragePool{Name: "default"}
		vol := libvirt.StorageVol{Name: "vol1"}

		m.On("StorageVolLookupByName", ctx, pool, "vol1").Return(vol, nil)
		m.On("StorageVolGetPath", ctx, vol).Return(testVolPath, nil)

		path := a.resolveVolumePath(ctx, "vol1", pool, nil)
		assert.Equal(t, testVolPath, path)
		m.AssertExpectations(t)
	})
}

func TestCleanupCreateFailure(t *testing.T) {
	t.Parallel()
	m := new(MockLibvirtClient)
	a := &LibvirtAdapter{
		client: m,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	ctx := context.Background()

	// 1. Success path
	vol := libvirt.StorageVol{Name: "test-vol"}
	m.On("StorageVolDelete", ctx, vol, uint32(0)).Return(nil).Once()
	a.cleanupCreateFailure(ctx, vol, "/tmp/iso")

	// 2. Failure path (should not panic)
	m.On("StorageVolDelete", ctx, vol, uint32(0)).Return(fmt.Errorf("delete error")).Once()
	a.cleanupCreateFailure(ctx, vol, "/tmp/iso-fail")

	m.AssertExpectations(t)
}

func TestGetInstanceLogs(t *testing.T) {
	t.Parallel()
	// Mock osOpen
	tmpFile, _ := os.CreateTemp("", "log")
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	_, _ = tmpFile.WriteString("log data")
	_ = tmpFile.Close()

	m := new(MockLibvirtClient)
	a := &LibvirtAdapter{
		client: m,
		osOpen: func(name string) (*os.File, error) {
			return os.Open(tmpFile.Name())
		},
	}
	ctx := context.Background()

	rc, err := a.GetInstanceLogs(ctx, "test")
	require.NoError(t, err)
	assert.NotNil(t, rc)
	_ = rc.Close()
}

func TestGetInstancePort(t *testing.T) {
	t.Parallel()
	m := new(MockLibvirtClient)
	a := &LibvirtAdapter{
		client:       m,
		portMappings: map[string]map[string]int{"test": {"80": 8080}},
	}
	ctx := context.Background()

	port, err := a.GetInstancePort(ctx, "test", "80")
	require.NoError(t, err)
	assert.Equal(t, 8080, port)

	_, err = a.GetInstancePort(ctx, "test", "443")
	require.Error(t, err)
}

func TestNewLibvirtAdapter(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, _ = NewLibvirtAdapter(logger, "qemu:///system")
}

func TestClose(t *testing.T) {
	t.Parallel()
	m := new(MockLibvirtClient)
	a := &LibvirtAdapter{client: m}
	m.On("Close").Return(nil)
	err := a.Close()
	require.NoError(t, err)
	m.AssertExpectations(t)
}

func TestPing(t *testing.T) {
	t.Parallel()
	m := new(MockLibvirtClient)
	a := &LibvirtAdapter{client: m}
	m.On("Connect", mock.Anything).Return(nil).Once()
	err := a.Ping(context.Background())
	require.NoError(t, err)
}

func TestType(t *testing.T) {
	t.Parallel()
	a := &LibvirtAdapter{}
	assert.Equal(t, "libvirt", a.Type())
}

func TestWaitInitialIPContextCancellation(t *testing.T) {
	t.Parallel()
	a := &LibvirtAdapter{
		ipWaitInterval: time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := a.waitInitialIP(ctx, "test-instance")
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestGetNextNetworkRange(t *testing.T) {
	t.Parallel()
	a := &LibvirtAdapter{
		networkCounter: 0,
		poolStart:      net.ParseIP(testutil.TestLibvirtPoolStart),
		poolEnd:        net.ParseIP(testutil.TestLibvirtPoolEnd),
		ipWaitInterval: time.Millisecond,
	}

	gateway1, start1, end1 := a.getNextNetworkRange()
	assert.NotEmpty(t, gateway1)
	assert.NotEmpty(t, start1)
	assert.NotEmpty(t, end1)
	assert.Equal(t, 1, a.networkCounter)

	gateway2, _, _ := a.getNextNetworkRange()
	assert.NotEqual(t, gateway1, gateway2, "subsequent calls should return different gateways")
	assert.Equal(t, 2, a.networkCounter)
}

func TestGetNextNetworkRangePoolExhaustion(t *testing.T) {
	t.Parallel()
	a := &LibvirtAdapter{
		networkCounter: 254, // The 255th /24 network
		// Test IPs for private network calculation
		poolStart: net.ParseIP("192.168.0.0"),
		poolEnd:   net.ParseIP("192.168.255.255"),
	}

	gateway, _, _ := a.getNextNetworkRange()
	// 192.168.0.0 + (254 * 256) = 192.168.254.0
	// Gateway is .1 -> 192.168.254.1
	assert.Equal(t, "192.168.254.1", gateway, "Should calculate 255th network correctly")

	// Next one should be 192.168.255.1
	gateway2, _, _ := a.getNextNetworkRange()
	assert.Equal(t, "192.168.255.1", gateway2, "Should calculate 256th network correctly")
}

func TestExecNotSupported(t *testing.T) {
	t.Parallel()
	a := &LibvirtAdapter{}
	_, err := a.Exec(context.Background(), "instance-id", []string{"echo", "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestCleanupPortMappings(t *testing.T) {
	t.Parallel()
	a := &LibvirtAdapter{
		portMappings: make(map[string]map[string]int),
	}
	instanceID := "inst-1"
	a.portMappings[instanceID] = map[string]int{"80": 30080}

	a.cleanupPortMappings(instanceID)

	assert.Empty(t, a.portMappings[instanceID])
}

func TestGenerateCloudInitISO(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	a := &LibvirtAdapter{
		logger: logger,
		execCommand: func(name string, arg ...string) *exec.Cmd {
			return exec.Command("true")
		},
		lookPath: func(file string) (string, error) {
			return "/usr/bin/true", nil
		},
	}
	ctx := context.Background()
	name := "test-asg"
	env := []string{testEnvVar}
	cmd := []string{"ls -la"}

	isoPath, err := a.generateCloudInitISO(ctx, name, env, cmd, "")
	require.NoError(t, err)
	assert.NotEmpty(t, isoPath)
	assert.Contains(t, isoPath, ".iso")

	// Cleanup the generated ISO file if it exists (though it might not if true succeeded)
	if isoPath != "" {
		_ = os.Remove(isoPath)
	}
}

func TestStopInstance_Unit(t *testing.T) {
	t.Parallel()
	m := new(MockLibvirtClient)
	a := &LibvirtAdapter{client: m, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx := context.Background()
	dom := libvirt.Domain{Name: "test-vm"}

	m.On("DomainLookupByName", mock.Anything, "test-vm").Return(dom, nil).Once()
	m.On("DomainDestroy", mock.Anything, dom).Return(nil).Once()

	err := a.StopInstance(ctx, "test-vm")
	require.NoError(t, err)
	m.AssertExpectations(t)
}

func TestDeleteInstance_Unit(t *testing.T) {
	t.Parallel()
	m := new(MockLibvirtClient)
	a := &LibvirtAdapter{
		client:       m,
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		portMappings: make(map[string]map[string]int),
	}
	ctx := context.Background()
	dom := libvirt.Domain{Name: "test-vm"}

	m.On("DomainLookupByName", mock.Anything, "test-vm").Return(dom, nil).Once()
	m.On("DomainGetState", mock.Anything, dom, uint32(0)).Return(int32(domainStateRunning), int32(0), nil).Once()
	m.On("DomainDestroy", mock.Anything, dom).Return(nil).Once()
	m.On("DomainUndefine", mock.Anything, dom).Return(nil).Once()

	// Root volume cleanup mocks
	pool := libvirt.StoragePool{Name: "default"}
	vol := libvirt.StorageVol{Name: "test-vm-root"}
	m.On("StoragePoolLookupByName", mock.Anything, "default").Return(pool, nil).Once()
	m.On("StorageVolLookupByName", mock.Anything, pool, "test-vm-root").Return(vol, nil).Once()
	m.On("StorageVolDelete", mock.Anything, vol, uint32(0)).Return(nil).Once()

	err := a.DeleteInstance(ctx, "test-vm")
	require.NoError(t, err)
	m.AssertExpectations(t)
}

func TestLibvirtAdapter_StartInstance(t *testing.T) {
	t.Parallel()
	m := new(MockLibvirtClient)
	a := &LibvirtAdapter{client: m, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx := context.Background()
	dom := libvirt.Domain{Name: "test-vm"}

	m.On("DomainLookupByName", mock.Anything, "test-vm").Return(dom, nil).Once()
	m.On("DomainCreate", mock.Anything, dom).Return(nil).Once()

	err := a.StartInstance(ctx, "test-vm")
	require.NoError(t, err)
	m.AssertExpectations(t)
}

func TestLibvirtAdapter_VolumeOps(t *testing.T) {
	t.Parallel()
	m := new(MockLibvirtClient)
	a := &LibvirtAdapter{client: m, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx := context.Background()
	dom := libvirt.Domain{Name: "test-vm"}
	volPath := "/path/to/vol"

	t.Run("AttachVolume", func(t *testing.T) {
		m.On("DomainLookupByName", mock.Anything, "test-vm").Return(dom, nil).Once()
		m.On("DomainAttachDevice", mock.Anything, dom, mock.Anything).Return(nil).Once()
		path, newContainerID, err := a.AttachVolume(ctx, "test-vm", volPath)
		require.NoError(t, err)
		assert.Equal(t, "/dev/vdb", path)
		assert.Equal(t, "", newContainerID)
	})
	t.Run("DetachVolume", func(t *testing.T) {
		m.On("DomainLookupByName", mock.Anything, "test-vm").Return(dom, nil).Once()
		m.On("DomainDetachDevice", mock.Anything, dom, mock.Anything).Return(nil).Once()
		newContainerID, err := a.DetachVolume(ctx, "test-vm", volPath)
		require.NoError(t, err)
		assert.Equal(t, "", newContainerID)
	})
	m.AssertExpectations(t)
}

func TestLibvirtAdapter_WaitInitialIP(t *testing.T) {
	t.Parallel()
	m := new(MockLibvirtClient)
	a := &LibvirtAdapter{
		client:         m,
		ipWaitInterval: time.Millisecond,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	ctx := context.Background()
	id := "test-vm"
	dom := libvirt.Domain{Name: id}
	mac := "52:54:00:12:34:56"
	xml := fmt.Sprintf("<domain><devices><interface type='network'><mac address='%s'/></interface></devices></domain>", mac)
	network := libvirt.Network{Name: "default"}

	t.Run("success after retry", func(t *testing.T) {
		m.On("DomainLookupByName", mock.Anything, id).Return(dom, nil)
		m.On("DomainGetXMLDesc", mock.Anything, dom, mock.Anything).Return(xml, nil)
		m.On("NetworkLookupByName", mock.Anything, "default").Return(network, nil)

		// First call returns no lease, second returns lease
		m.On("NetworkGetDhcpLeases", mock.Anything, network, mock.Anything, uint32(0), uint32(0)).
			Return([]libvirt.NetworkDhcpLease{}, uint32(0), nil).Once()
		m.On("NetworkGetDhcpLeases", mock.Anything, network, mock.Anything, uint32(0), uint32(0)).
			Return([]libvirt.NetworkDhcpLease{{Mac: []string{mac}, Ipaddr: "10.0.0.5"}}, uint32(1), nil).Once()

		ip, err := a.waitInitialIP(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, "10.0.0.5", ip)
	})
}

func TestLibvirtAdapter_GetInstanceIP(t *testing.T) {
	t.Parallel()
	m := new(MockLibvirtClient)
	a := &LibvirtAdapter{client: m, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx := context.Background()
	id := "test-vm"
	dom := libvirt.Domain{Name: id}
	mac := "52:54:00:12:34:56"
	xml := fmt.Sprintf("<domain><devices><interface type='network'><mac address='%s'/></interface></devices></domain>", mac)
	network := libvirt.Network{Name: "default"}

	t.Run("success", func(t *testing.T) {
		m.On("DomainLookupByName", mock.Anything, id).Return(dom, nil).Once()
		m.On("DomainGetXMLDesc", mock.Anything, dom, mock.Anything).Return(xml, nil).Once()
		m.On("NetworkLookupByName", mock.Anything, "default").Return(network, nil).Once()
		m.On("NetworkGetDhcpLeases", mock.Anything, network, mock.Anything, uint32(0), uint32(0)).
			Return([]libvirt.NetworkDhcpLease{{Mac: []string{mac}, Ipaddr: "10.0.0.5"}}, uint32(1), nil).Once()

		ip, err := a.GetInstanceIP(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, "10.0.0.5", ip)
	})
}

func TestLibvirtAdapter_StatsAndConsole(t *testing.T) {
	t.Parallel()
	m := new(MockLibvirtClient)
	a := &LibvirtAdapter{client: m, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx := context.Background()
	id := "test-vm"
	dom := libvirt.Domain{Name: id}

	t.Run("GetInstanceStats", func(t *testing.T) {
		m.On("DomainLookupByName", mock.Anything, id).Return(dom, nil).Once()
		m.On("DomainMemoryStats", mock.Anything, dom, uint32(10), uint32(0)).Return([]libvirt.DomainMemoryStat{}, nil).Once()
		m.On("DomainGetCPUStats", mock.Anything, dom, uint32(0), int32(0), uint32(0), libvirt.TypedParameterFlags(0)).Return([]libvirt.TypedParam{}, int32(0), nil).Once()
		m.On("DomainGetState", mock.Anything, dom, uint32(0)).Return(int32(1), int32(0), nil).Once()

		stats, err := a.GetInstanceStats(ctx, id)
		require.NoError(t, err)
		assert.NotNil(t, stats)
	})

	t.Run("GetInstanceStats_WithCPUStats", func(t *testing.T) {
		cpuParams := []libvirt.TypedParam{
			{Field: "cpu_time", Value: libvirt.TypedParamValue{D: typedParamULLONG, I: uint64(5000000000)}},
		}
		m.On("DomainLookupByName", mock.Anything, id).Return(dom, nil).Once()
		m.On("DomainMemoryStats", mock.Anything, dom, uint32(10), uint32(0)).Return([]libvirt.DomainMemoryStat{}, nil).Once()
		m.On("DomainGetCPUStats", mock.Anything, dom, uint32(0), int32(0), uint32(0), libvirt.TypedParameterFlags(0)).Return(cpuParams, int32(0), nil).Once()
		m.On("DomainGetState", mock.Anything, dom, uint32(0)).Return(int32(1), int32(0), nil).Once()

		stats, err := a.GetInstanceStats(ctx, id)
		require.NoError(t, err)
		assert.NotNil(t, stats)
		defer func() { _ = stats.Close() }()

		// Read and verify the stats JSON contains the CPU time
		statsBytes, err := io.ReadAll(stats)
		require.NoError(t, err)
		assert.Contains(t, string(statsBytes), `"cpu_time":5000000000`)
	})

	t.Run("GetConsoleURL", func(t *testing.T) {
		xml := "<domain><devices><graphics type='vnc' port='5900'/></devices></domain>"
		m.On("DomainLookupByName", mock.Anything, id).Return(dom, nil).Once()
		m.On("DomainGetXMLDesc", mock.Anything, dom, mock.Anything).Return(xml, nil).Once()
		url, err := a.GetConsoleURL(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, "vnc://127.0.0.1:5900", url)
	})
}

func TestLibvirtAdapter_NetworkAndVolume(t *testing.T) {
	t.Parallel()
	m := new(MockLibvirtClient)
	a := &LibvirtAdapter{
		client:    m,
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		poolStart: net.ParseIP("192.168.100.0"),
		poolEnd:   net.ParseIP("192.168.255.255"),
	}
	ctx := context.Background()

	t.Run("CreateNetwork", func(t *testing.T) {
		m.On("NetworkDefineXML", mock.Anything, mock.Anything).Return(libvirt.Network{Name: "test-net"}, nil).Once()
		m.On("NetworkCreate", mock.Anything, mock.Anything).Return(nil).Once()

		id, err := a.CreateNetwork(ctx, "10.0.0.0/24")
		require.NoError(t, err)
		assert.NotEmpty(t, id)
	})

	t.Run("DeleteNetwork", func(t *testing.T) {
		net := libvirt.Network{Name: "test-net"}
		m.On("NetworkLookupByName", mock.Anything, "test-net").Return(net, nil).Once()
		m.On("NetworkDestroy", mock.Anything, net).Return(nil).Once()
		m.On("NetworkUndefine", mock.Anything, net).Return(nil).Once()

		err := a.DeleteNetwork(ctx, "test-net")
		require.NoError(t, err)
	})

	t.Run("CreateVolume", func(t *testing.T) {
		pool := libvirt.StoragePool{Name: "default"}
		m.On("StoragePoolLookupByName", mock.Anything, "default").Return(pool, nil).Once()
		m.On("StoragePoolRefresh", mock.Anything, pool, uint32(0)).Return(nil).Once()
		m.On("StorageVolCreateXML", mock.Anything, pool, mock.Anything, uint32(0)).Return(libvirt.StorageVol{Name: "v1"}, nil).Once()

		err := a.CreateVolume(ctx, "v1")
		require.NoError(t, err)
	})

	t.Run("DeleteVolume", func(t *testing.T) {
		pool := libvirt.StoragePool{Name: "default"}
		vol := libvirt.StorageVol{Name: "v1"}
		m.On("StoragePoolLookupByName", mock.Anything, "default").Return(pool, nil).Once()
		m.On("StorageVolLookupByName", mock.Anything, pool, "v1").Return(vol, nil).Once()
		m.On("StorageVolDelete", mock.Anything, vol, uint32(0)).Return(nil).Once()

		err := a.DeleteVolume(ctx, "v1")
		require.NoError(t, err)
	})
}

func TestLibvirtAdapter_IsNotFound(t *testing.T) {
	t.Parallel()
	a := &LibvirtAdapter{}

	t.Run("Libvirt Error 42", func(t *testing.T) {
		err := libvirt.Error{Code: 42}
		assert.True(t, a.isNotFound(err))
	})

	t.Run("String Match", func(t *testing.T) {
		err := fmt.Errorf("something was not found")
		assert.True(t, a.isNotFound(err))
	})

	t.Run("No Match", func(t *testing.T) {
		err := fmt.Errorf("some other error")
		assert.False(t, a.isNotFound(err))
	})
}

func TestLibvirtAdapter_SnapshotMethods(t *testing.T) {
	m := new(MockLibvirtClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()
	dom := libvirt.Domain{Name: "test-vm"}
	pool := libvirt.StoragePool{Name: "default"}
	vol := libvirt.StorageVol{Name: "test-vm-root"}

	t.Run("CreateSnapshot", func(t *testing.T) {
		a := &LibvirtAdapter{
			client: m,
			logger: logger,
			execCommand: func(name string, arg ...string) *exec.Cmd {
				// When tar xzf is called during RestoreVolumeSnapshot rollback,
				// create a dummy qcow2 file so the "empty archive" check passes
				if name == "tar" && len(arg) >= 2 && arg[1] == "xzf" {
					for i, argVal := range arg {
						if argVal == "-C" && i+1 < len(arg) {
							tmpDir := arg[i+1]
							_ = os.MkdirAll(tmpDir, 0750)
							_, _ = os.Create(tmpDir + "/disk.qcow2")
							break
						}
					}
					return exec.Command("true")
				}
				return exec.Command("true")
			},
		}
		m.On("DomainLookupByName", mock.Anything, "test-vm").Return(dom, nil).Once()
		m.On("StoragePoolLookupByName", mock.Anything, "default").Return(pool, nil).Once()
		m.On("StorageVolLookupByName", mock.Anything, pool, "test-vm-root").Return(vol, nil).Once()
		m.On("StorageVolGetPath", mock.Anything, vol).Return("/path/to/test-vm-root", nil).Once()

		err := a.CreateSnapshot(ctx, "test-vm", "pre-resize-abc123")
		require.NoError(t, err)
		m.AssertExpectations(t)
	})

	t.Run("CreateSnapshot_DomainNotFound", func(t *testing.T) {
		m.On("DomainLookupByName", mock.Anything, "nonexistent").
			Return(libvirt.Domain{}, libvirt.Error{Code: 42}).Once()

		a := &LibvirtAdapter{client: m, logger: logger}
		err := a.CreateSnapshot(ctx, "nonexistent", "snap")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "domain not found")
	})

	t.Run("RestoreSnapshot", func(t *testing.T) {
		// Create a real tar file to extract so RestoreVolumeSnapshot doesn't get "empty archive"
		tmpDir := t.TempDir()
		snapshotPath := tmpDir + "/snapshot.tar.gz"
		qcow2File := tmpDir + "/disk.qcow2"

		// Create a dummy qcow2 file
		f, _ := os.Create(qcow2File)
		_ = f.Close()

		// Create a proper tar.gz
		cmd := exec.Command("tar", "czf", snapshotPath, "-C", tmpDir, "disk.qcow2")
		require.NoError(t, cmd.Run())

		a := &LibvirtAdapter{
			client: m,
			logger: logger,
			execCommand: func(name string, arg ...string) *exec.Cmd {
				// When tar xzf is called during RestoreVolumeSnapshot rollback,
				// create a dummy qcow2 file so the "empty archive" check passes
				if name == "tar" && len(arg) >= 2 && arg[1] == "xzf" {
					for i, argVal := range arg {
						if argVal == "-C" && i+1 < len(arg) {
							tmpDir := arg[i+1]
							_ = os.MkdirAll(tmpDir, 0750)
							_, _ = os.Create(tmpDir + "/disk.qcow2")
							break
						}
					}
					return exec.Command("true")
				}
				return exec.Command("true")
			},
		}
		m.On("DomainLookupByName", mock.Anything, "test-vm").Return(dom, nil).Once()
		m.On("StoragePoolLookupByName", mock.Anything, "default").Return(pool, nil).Once()
		m.On("StorageVolLookupByName", mock.Anything, pool, "test-vm-root").Return(vol, nil).Once()
		m.On("StorageVolGetPath", mock.Anything, vol).Return("/path/to/test-vm-root", nil).Once()

		// The call chain reaches RestoreVolumeSnapshot, but execCommand returns "true"
		// so the actual tar extraction / qemu-img convert will fail. We just verify the call.
		err := a.RestoreSnapshot(ctx, "test-vm", "pre-resize-abc123")
		require.Error(t, err) // exec "true" doesn't produce valid qcow2 → untar fails
	})

	t.Run("RestoreSnapshot_DomainNotFound", func(t *testing.T) {
		m.On("DomainLookupByName", mock.Anything, "nonexistent").
			Return(libvirt.Domain{}, libvirt.Error{Code: 42}).Once()

		a := &LibvirtAdapter{client: m, logger: logger}
		err := a.RestoreSnapshot(ctx, "nonexistent", "snap")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "domain not found")
	})

	t.Run("DeleteSnapshot", func(t *testing.T) {
		a := &LibvirtAdapter{
			client: m,
			logger: logger,
		}
		m.On("DomainLookupByName", mock.Anything, "test-vm").Return(dom, nil).Once()

		err := a.DeleteSnapshot(ctx, "test-vm", "pre-resize-abc123")
		require.NoError(t, err)
		m.AssertExpectations(t)
	})

	t.Run("DeleteSnapshot_DomainNotFound", func(t *testing.T) {
		m.On("DomainLookupByName", mock.Anything, "nonexistent").
			Return(libvirt.Domain{}, libvirt.Error{Code: 42}).Once()

		a := &LibvirtAdapter{client: m, logger: logger}
		err := a.DeleteSnapshot(ctx, "nonexistent", "snap")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "domain not found")
	})
}

func TestLibvirtAdapter_ApplyDomainResize(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	memoryRe := regexp.MustCompile(`(?i)<memory(?:\s[^>]*)?>\d+</memory>`)
	currentMemRe := regexp.MustCompile(`(?i)<currentMemory(?:\s[^>]*)?>\d+</currentMemory>`)
	vcpuRe := regexp.MustCompile(`(?i)<vcpu(?:\s[^>]*)?>\d+</vcpu>`)
	a := &LibvirtAdapter{
		logger:             logger,
		memoryResizeRe:     memoryRe,
		currentMemResizeRe: currentMemRe,
		vcpuResizeRe:       vcpuRe,
	}

	t.Run("both memory and vcpu replaced", func(t *testing.T) {
		xml := `<memory unit="KiB">1048576</memory><currentMemory unit="KiB">1048576</currentMemory><vcpu>2</vcpu>`
		result, err := a.applyDomainResize(xml, 2097152, 4)
		require.NoError(t, err)
		assert.Contains(t, result, `<memory unit="KiB">2097152</memory>`)
		assert.Contains(t, result, `<currentMemory unit="KiB">2097152</currentMemory>`)
		assert.Contains(t, result, `<vcpu>4</vcpu>`)
	})

	t.Run("missing both memory and vcpu elements", func(t *testing.T) {
		xml := `<name>test-vm</name>`
		_, err := a.applyDomainResize(xml, 2097152, 4)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no memory or vcpu elements")
	})

	t.Run("memory without unit attribute", func(t *testing.T) {
		xml := `<memory>1048576</memory><vcpu>2</vcpu>`
		result, err := a.applyDomainResize(xml, 2097152, 4)
		require.NoError(t, err)
		assert.Contains(t, result, `<memory unit="KiB">2097152</memory>`)
		assert.Contains(t, result, `<vcpu>4</vcpu>`)
	})

	t.Run("resize to same values produces error", func(t *testing.T) {
		xml := `<memory unit="KiB">1048576</memory><currentMemory unit="KiB">1048576</currentMemory><vcpu>2</vcpu>`
		// All replacements produce identical output, so result == xmlContent → error
		_, err := a.applyDomainResize(xml, 1048576, 2)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no memory or vcpu elements")
	})

	t.Run("vcpu with placement attribute", func(t *testing.T) {
		xml := `<memory unit="KiB">1048576</memory><vcpu placement="static">2</vcpu>`
		result, err := a.applyDomainResize(xml, 2097152, 4)
		require.NoError(t, err)
		assert.Contains(t, result, `<memory unit="KiB">2097152</memory>`)
		assert.Contains(t, result, `<vcpu>4</vcpu>`)
	})
}

func TestLibvirtAdapter_ResizeInstance_RollbackOnFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()
	dom := libvirt.Domain{Name: "test-vm"}
	pool := libvirt.StoragePool{Name: "default"}
	vol := libvirt.StorageVol{Name: "test-vm-root"}
	originalXML := "<domain><memory>1024</memory><vcpu>1</vcpu></domain>"

	t.Run("rollback on DomainDefineXML failure", func(t *testing.T) {
		// Each subtest gets its own mock to avoid call count conflicts
		memoryRe := regexp.MustCompile(`(?i)<memory(?:\s[^>]*)?>\d+</memory>`)
		currentMemRe := regexp.MustCompile(`(?i)<currentMemory(?:\s[^>]*)?>\d+</currentMemory>`)
		vcpuRe := regexp.MustCompile(`(?i)<vcpu(?:\s[^>]*)?>\d+</vcpu>`)
		m := new(MockLibvirtClient)
		a := &LibvirtAdapter{
			client:             m,
			logger:             logger,
			memoryResizeRe:     memoryRe,
			currentMemResizeRe: currentMemRe,
			vcpuResizeRe:       vcpuRe,
			execCommand: func(name string, arg ...string) *exec.Cmd {
				// When tar xzf is called during RestoreVolumeSnapshot rollback,
				// create a dummy qcow2 file so the "empty archive" check passes
				if name == "tar" && len(arg) >= 2 && arg[1] == "xzf" {
					// arg layout: tar xzf <source> -C <tmpDir> [files...]
					// Find tmpDir: it's the arg after "-C" flag
					for i, argVal := range arg {
						if argVal == "-C" && i+1 < len(arg) {
							tmpDir := arg[i+1]
							_ = os.MkdirAll(tmpDir, 0750)
							_, _ = os.Create(tmpDir + "/disk.qcow2")
							break
						}
					}
					return exec.Command("true")
				}
				return exec.Command("true")
			},
		}

		// Total DomainLookupByName calls = 3:
		//   1. ResizeInstance start (line 168)
		//   2. CreateSnapshot inside ResizeInstance (line 815)
		//   3. RestoreSnapshot inside rollbackFromSnapshot (line 829)
		m.On("DomainLookupByName", mock.Anything, "test-vm").Return(dom, nil).Times(3)
		m.On("DomainGetState", mock.Anything, dom, uint32(0)).Return(int32(domainStateRunning), int32(0), nil).Once()
		m.On("DomainDestroy", mock.Anything, dom).Return(nil).Once()
		m.On("DomainGetXMLDesc", mock.Anything, dom, mock.Anything).Return(originalXML, nil).Once()
		// Pool/vol called twice: CreateVolumeSnapshot (line 906) + RestoreVolumeSnapshot (line 941)
		m.On("StoragePoolLookupByName", mock.Anything, "default").Return(pool, nil).Times(2)
		m.On("StorageVolLookupByName", mock.Anything, pool, "test-vm-root").Return(vol, nil).Times(2)
		m.On("StorageVolGetPath", mock.Anything, vol).Return("/path/to/test-vm-root", nil).Times(2)
		m.On("DomainUndefine", mock.Anything, dom).Return(nil).Once()
		m.On("DomainDefineXML", mock.Anything, mock.Anything).Return(libvirt.Domain{}, fmt.Errorf("xml define failed")).Once()
		// Rollback: RestoreSnapshot calls DomainDefineXML (to redefine domain)
		// Note: we cannot use mock.Anything twice for the same method with Once() due to
		// testify matching semantics - the newDOMXML call also matches mock.Anything.
		// Instead verify via error message content.
		m.On("DomainDefineXML", mock.Anything, originalXML).Return(dom, nil).Maybe()

		err := a.ResizeInstance(ctx, "test-vm", 2e9, 2*1024*1024*1024)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to redefine domain")
		m.AssertExpectations(t)
	})

	t.Run("rollback on DomainUndefine failure", func(t *testing.T) {
		// Fresh mock instance for this subtest
		m := new(MockLibvirtClient)
		memoryRe := regexp.MustCompile(`(?i)<memory(?:\s[^>]*)?>\d+</memory>`)
		currentMemRe := regexp.MustCompile(`(?i)<currentMemory(?:\s[^>]*)?>\d+</currentMemory>`)
		vcpuRe := regexp.MustCompile(`(?i)<vcpu(?:\s[^>]*)?>\d+</vcpu>`)
		a := &LibvirtAdapter{
			client:             m,
			logger:             logger,
			memoryResizeRe:     memoryRe,
			currentMemResizeRe: currentMemRe,
			vcpuResizeRe:       vcpuRe,
			execCommand: func(name string, arg ...string) *exec.Cmd {
				// When tar xzf is called during RestoreVolumeSnapshot rollback,
				// create a dummy qcow2 file so the "empty archive" check passes
				if name == "tar" && len(arg) >= 2 && arg[1] == "xzf" {
					for i, argVal := range arg {
						if argVal == "-C" && i+1 < len(arg) {
							tmpDir := arg[i+1]
							_ = os.MkdirAll(tmpDir, 0750)
							_, _ = os.Create(tmpDir + "/disk.qcow2")
							break
						}
					}
					return exec.Command("true")
				}
				return exec.Command("true")
			},
		}

		// Total DomainLookupByName calls = 3:
		//   1. ResizeInstance start (line 168)
		//   2. CreateSnapshot inside ResizeInstance (line 815)
		//   3. RestoreSnapshot inside rollbackFromSnapshot (line 829)
		m.On("DomainLookupByName", mock.Anything, "test-vm").Return(dom, nil).Times(3)
		m.On("DomainGetState", mock.Anything, dom, uint32(0)).Return(int32(domainStateRunning), int32(0), nil).Once()
		m.On("DomainDestroy", mock.Anything, dom).Return(nil).Once()
		m.On("DomainGetXMLDesc", mock.Anything, dom, mock.Anything).Return(originalXML, nil).Once()
		// Pool/vol called twice: CreateVolumeSnapshot (line 906) + RestoreVolumeSnapshot (line 941)
		m.On("StoragePoolLookupByName", mock.Anything, "default").Return(pool, nil).Times(2)
		m.On("StorageVolLookupByName", mock.Anything, pool, "test-vm-root").Return(vol, nil).Times(2)
		m.On("StorageVolGetPath", mock.Anything, vol).Return("/path/to/test-vm-root", nil).Times(2)
		m.On("DomainUndefine", mock.Anything, dom).Return(fmt.Errorf("undefine failed")).Once()
		// Rollback: RestoreSnapshot calls DomainDefineXML (to redefine domain)
		m.On("DomainDefineXML", mock.Anything, originalXML).Return(dom, nil).Maybe()

		err := a.ResizeInstance(ctx, "test-vm", 2e9, 2*1024*1024*1024)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to undefine domain")
		m.AssertExpectations(t)
	})

	t.Run("rollback on DomainCreate failure", func(t *testing.T) {
		// Fresh mock instance for this subtest
		m := new(MockLibvirtClient)
		memoryRe := regexp.MustCompile(`(?i)<memory(?:\s[^>]*)?>\d+</memory>`)
		currentMemRe := regexp.MustCompile(`(?i)<currentMemory(?:\s[^>]*)?>\d+</currentMemory>`)
		vcpuRe := regexp.MustCompile(`(?i)<vcpu(?:\s[^>]*)?>\d+</vcpu>`)
		newDom := libvirt.Domain{Name: "test-vm-resized"}
		a := &LibvirtAdapter{
			client:             m,
			logger:             logger,
			memoryResizeRe:     memoryRe,
			currentMemResizeRe: currentMemRe,
			vcpuResizeRe:       vcpuRe,
			execCommand: func(name string, arg ...string) *exec.Cmd {
				if name == "tar" && len(arg) >= 2 && arg[1] == "xzf" {
					for i, argVal := range arg {
						if argVal == "-C" && i+1 < len(arg) {
							tmpDir := arg[i+1]
							_ = os.MkdirAll(tmpDir, 0750)
							_, _ = os.Create(tmpDir + "/disk.qcow2")
							break
						}
					}
					return exec.Command("true")
				}
				return exec.Command("true")
			},
		}

		// DomainLookupByName: ResizeInstance start (1) + CreateSnapshot (1) + RestoreSnapshot in rollback (1)
		// Note: if snapshot creation fails, rollback may not call RestoreSnapshot
		m.On("DomainLookupByName", mock.Anything, "test-vm").Return(dom, nil).Maybe()
		m.On("DomainGetState", mock.Anything, dom, uint32(0)).Return(int32(domainStateRunning), int32(0), nil).Maybe()
		m.On("DomainDestroy", mock.Anything, dom).Return(nil).Maybe()
		m.On("DomainGetXMLDesc", mock.Anything, dom, mock.Anything).Return(originalXML, nil).Maybe()
		// Pool/vol: may be called if snapshot creation succeeds; use Maybe() since snapshot creation may fail
		m.On("StoragePoolLookupByName", mock.Anything, "default").Return(pool, nil).Maybe()
		m.On("StorageVolLookupByName", mock.Anything, pool, "test-vm-root").Return(vol, nil).Maybe()
		m.On("StorageVolGetPath", mock.Anything, vol).Return("/path/to/test-vm-root", nil).Maybe()
		// Domain operations for resize
		m.On("DomainUndefine", mock.Anything, mock.Anything).Return(nil).Maybe()
		// DomainDefineXML for new domain definition
		m.On("DomainDefineXML", mock.Anything, mock.Anything).Return(newDom, nil).Maybe()
		// DomainCreate FAILS for new domain
		m.On("DomainCreate", mock.Anything, newDom).Return(fmt.Errorf("failed to start")).Maybe()
		// Rollback: DomainCreate to restart original domain (this is the error source)
		m.On("DomainCreate", mock.Anything, dom).Return(fmt.Errorf("failed to restart")).Maybe()

		err := a.ResizeInstance(ctx, "test-vm", 2e9, 2*1024*1024*1024)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to start domain after resize")
		assert.Contains(t, err.Error(), "failed to restore snapshot volume")
		m.AssertExpectations(t)
	})
}
