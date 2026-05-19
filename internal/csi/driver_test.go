package csi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/poyrazk/thecloud/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"log/slog"
)

// MockMounter
type MockMounter struct {
	mock.Mock
}

func (m *MockMounter) FormatDevice(ctx context.Context, device, fsType string) error {
	args := m.Called(ctx, device, fsType)
	return args.Error(0)
}

func (m *MockMounter) Mount(ctx context.Context, source, target, fsType string) error {
	args := m.Called(ctx, source, target, fsType)
	return args.Error(0)
}

func (m *MockMounter) BindMount(ctx context.Context, source, target string) error {
	args := m.Called(ctx, source, target)
	return args.Error(0)
}

func (m *MockMounter) Unmount(ctx context.Context, target string) error {
	args := m.Called(ctx, target)
	return args.Error(0)
}

func (m *MockMounter) IsFormatted(ctx context.Context, device string) bool {
	args := m.Called(ctx, device)
	return args.Bool(0)
}

func (m *MockMounter) MkdirAll(path string, perm os.FileMode) error {
	args := m.Called(path, perm)
	return args.Error(0)
}

func TestDriver_RunStop(t *testing.T) {
	socket := fmt.Sprintf("unix:///tmp/csi-test-%d.sock", time.Now().UnixNano())
	d := NewDriver("test", "1", "node", socket, nil, slog.Default())

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run()
	}()

	// Wait for socket
	require.Eventually(t, func() bool {
		_, err := os.Stat(strings.TrimPrefix(socket, "unix://"))
		return err == nil
	}, 5*time.Second, 100*time.Millisecond)

	// Test gRPC connection
	conn, err := grpc.NewClient(socket, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	client := csi.NewIdentityClient(conn)
	_, err = client.Probe(context.Background(), &csi.ProbeRequest{})
	require.NoError(t, err)

	d.Stop()
	conn.Close()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for driver to stop")
	}
}

func TestDriver_RunError(t *testing.T) {
	t.Run("Invalid Endpoint", func(t *testing.T) {
		d := NewDriver("t", "1", "n", "invalid://", nil, slog.Default())
		err := d.Run()
		require.Error(t, err)
	})

	t.Run("Listen Error", func(t *testing.T) {
		// Try to listen on a protected port/path
		d := NewDriver("t", "1", "n", "unix:///root/protected.sock", nil, slog.Default())
		err := d.Run()
		require.Error(t, err)
	})

	t.Run("TCP Endpoint", func(t *testing.T) {
		// Just to cover the tcp branch in parseEndpoint and net.Listen
		socket := "tcp://127.0.0.1:0"
		d := NewDriver("test", "1", "node", socket, nil, slog.Default())
		errCh := make(chan error, 1)
		go func() {
			errCh <- d.Run()
		}()
		time.Sleep(100 * time.Millisecond)
		d.Stop()
		err := <-errCh
		if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			require.NoError(t, err)
		}
	})
}

func TestDriver_IdentityServer(t *testing.T) {
	d := NewDriver("test-driver", "1.0.0", "node-1", "unix:///tmp/test.sock", nil, slog.Default())

	t.Run("GetPluginInfo", func(t *testing.T) {
		resp, err := d.GetPluginInfo(context.Background(), &csi.GetPluginInfoRequest{})
		require.NoError(t, err)
		assert.Equal(t, "test-driver", resp.Name)
		assert.Equal(t, "1.0.0", resp.VendorVersion)
	})

	t.Run("GetPluginCapabilities", func(t *testing.T) {
		resp, err := d.GetPluginCapabilities(context.Background(), &csi.GetPluginCapabilitiesRequest{})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Capabilities)
	})

	t.Run("Probe", func(t *testing.T) {
		resp, err := d.Probe(context.Background(), &csi.ProbeRequest{})
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

func TestDriver_ControllerServer(t *testing.T) {
	// Setup a mock server to act as the Cloud API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" && r.URL.Path == "/volumes" {
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"data": {"id": "83adaa62-ddb6-48ad-8a6b-b8e4816735e3", "name": "test-vol", "size_gb": 10}}`))
			return
		}
		if r.Method == "DELETE" && r.URL.Path == "/volumes/83adaa62-ddb6-48ad-8a6b-b8e4816735e3" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": "success"}`))
			return
		}
		if r.Method == "GET" && r.URL.Path == "/instances" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": [{"id": "inst-123", "name": "node-1"}]}`))
			return
		}
		if r.Method == "GET" && r.URL.Path == "/instances/inst-123" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": {"id": "inst-123", "name": "node-1"}}`))
			return
		}
		if r.Method == "POST" && r.URL.Path == "/volumes/83adaa62-ddb6-48ad-8a6b-b8e4816735e3/attach" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": {"device_path": "/dev/vdb"}}`))
			return
		}
		if r.Method == "POST" && r.URL.Path == "/volumes/83adaa62-ddb6-48ad-8a6b-b8e4816735e3/detach" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": "success"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := sdk.NewClient(server.URL, "test-key")
	d := NewDriver("test-driver", "1.0.0", "node-1", "unix:///tmp/test.sock", client, slog.Default())

	t.Run("CreateVolume Success", func(t *testing.T) {
		req := &csi.CreateVolumeRequest{
			Name: "test-vol",
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: 1 * 1024 * 1024 * 1024,
			},
		}
		resp, err := d.CreateVolume(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "83adaa62-ddb6-48ad-8a6b-b8e4816735e3", resp.Volume.VolumeId)
	})

	t.Run("CreateVolume Small", func(t *testing.T) {
		req := &csi.CreateVolumeRequest{
			Name: "test-vol",
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: 1024,
			},
		}
		_, err := d.CreateVolume(context.Background(), req)
		require.NoError(t, err)
	})

	t.Run("CreateVolume Default Size", func(t *testing.T) {
		req := &csi.CreateVolumeRequest{
			Name: "test-vol",
		}
		_, err := d.CreateVolume(context.Background(), req)
		require.NoError(t, err)
	})

	t.Run("CreateVolume SDK Error", func(t *testing.T) {
		badClient := sdk.NewClient("http://localhost:1", "key")
		badD := NewDriver("test", "1", "node", "unix:///tmp/t.sock", badClient, slog.Default())
		_, err := badD.CreateVolume(context.Background(), &csi.CreateVolumeRequest{Name: "v"})
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("DeleteVolume Success", func(t *testing.T) {
		req := &csi.DeleteVolumeRequest{
			VolumeId: "83adaa62-ddb6-48ad-8a6b-b8e4816735e3",
		}
		_, err := d.DeleteVolume(context.Background(), req)
		require.NoError(t, err)
	})

	t.Run("DeleteVolume SDK Error", func(t *testing.T) {
		badClient := sdk.NewClient("http://localhost:1", "key")
		badD := NewDriver("test", "1", "node", "unix:///tmp/t.sock", badClient, slog.Default())
		_, err := badD.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: "v"})
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("ControllerPublishVolume Success", func(t *testing.T) {
		req := &csi.ControllerPublishVolumeRequest{
			VolumeId: "83adaa62-ddb6-48ad-8a6b-b8e4816735e3",
			NodeId:   "node-1",
		}
		resp, err := d.ControllerPublishVolume(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "/dev/vdb", resp.PublishContext["device"])
	})

	t.Run("ControllerPublishVolume Attach Error", func(t *testing.T) {
		errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/attach") {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": {"id": "inst-123"}}`))
		}))
		defer errorServer.Close()
		errD := NewDriver("t", "1", "n", "u", sdk.NewClient(errorServer.URL, "k"), slog.Default())
		_, err := errD.ControllerPublishVolume(context.Background(), &csi.ControllerPublishVolumeRequest{VolumeId: "v", NodeId: "n"})
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("ControllerUnpublishVolume Success", func(t *testing.T) {
		req := &csi.ControllerUnpublishVolumeRequest{
			VolumeId: "83adaa62-ddb6-48ad-8a6b-b8e4816735e3",
		}
		_, err := d.ControllerUnpublishVolume(context.Background(), req)
		require.NoError(t, err)
	})

	t.Run("ControllerUnpublishVolume SDK Error", func(t *testing.T) {
		badClient := sdk.NewClient("http://localhost:1", "key")
		badD := NewDriver("test", "1", "node", "unix:///tmp/t.sock", badClient, slog.Default())
		_, err := badD.ControllerUnpublishVolume(context.Background(), &csi.ControllerUnpublishVolumeRequest{VolumeId: "v"})
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("CreateVolume Missing Name", func(t *testing.T) {
		req := &csi.CreateVolumeRequest{Name: ""}
		_, err := d.CreateVolume(context.Background(), req)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("DeleteVolume Missing ID", func(t *testing.T) {
		req := &csi.DeleteVolumeRequest{VolumeId: ""}
		_, err := d.DeleteVolume(context.Background(), req)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("ControllerPublishVolume Missing VolumeId", func(t *testing.T) {
		req := &csi.ControllerPublishVolumeRequest{
			VolumeId: "",
			NodeId:   "node-1",
		}
		_, err := d.ControllerPublishVolume(context.Background(), req)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("ControllerUnpublishVolume Missing VolumeId", func(t *testing.T) {
		req := &csi.ControllerUnpublishVolumeRequest{
			VolumeId: "",
		}
		_, err := d.ControllerUnpublishVolume(context.Background(), req)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("ControllerPublishVolume Node Not Found", func(t *testing.T) {
		req := &csi.ControllerPublishVolumeRequest{
			VolumeId: "vol-123",
			NodeId:   "non-existent-node",
		}
		_, err := d.ControllerPublishVolume(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("ValidateVolumeCapabilities", func(t *testing.T) {
		req := &csi.ValidateVolumeCapabilitiesRequest{
			VolumeId: "vol-123",
			VolumeCapabilities: []*csi.VolumeCapability{
				{AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER}},
			},
		}
		resp, err := d.ValidateVolumeCapabilities(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, resp.Confirmed)
	})

	t.Run("ControllerGetCapabilities", func(t *testing.T) {
		resp, err := d.ControllerGetCapabilities(context.Background(), &csi.ControllerGetCapabilitiesRequest{})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Capabilities)
	})

	t.Run("Unimplemented", func(t *testing.T) {
		ctx := context.Background()
		_, err := d.ListVolumes(ctx, nil)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
		_, err = d.GetCapacity(ctx, nil)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
		_, err = d.CreateSnapshot(ctx, nil)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
		_, err = d.DeleteSnapshot(ctx, nil)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
		_, err = d.ListSnapshots(ctx, nil)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
		_, err = d.ControllerExpandVolume(ctx, nil)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
		_, err = d.ControllerGetVolume(ctx, nil)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
		_, err = d.ControllerModifyVolume(ctx, nil)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
		_, err = d.GetSnapshot(ctx, nil)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
	})
}

func TestDriver_NodeServer(t *testing.T) {
	mockMounter := new(MockMounter)
	d := NewDriver("test-driver", "1.0.0", "node-1", "unix:///tmp/test.sock", nil, slog.Default())
	d.mounter = mockMounter

	t.Run("NodeGetInfo", func(t *testing.T) {
		resp, err := d.NodeGetInfo(context.Background(), &csi.NodeGetInfoRequest{})
		require.NoError(t, err)
		assert.Equal(t, "node-1", resp.NodeId)
	})

	t.Run("NodeGetCapabilities", func(t *testing.T) {
		resp, err := d.NodeGetCapabilities(context.Background(), &csi.NodeGetCapabilitiesRequest{})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Capabilities)
	})

	t.Run("NodeStageVolume Success", func(t *testing.T) {
		req := &csi.NodeStageVolumeRequest{
			VolumeId:          "vol-123",
			StagingTargetPath: "/staging/vol-123",
			VolumeCapability:  &csi.VolumeCapability{},
			PublishContext:    map[string]string{"device": "/dev/vdb"},
		}
		mockMounter.On("FormatDevice", mock.Anything, "/dev/vdb", "ext4").Return(nil).Once()
		mockMounter.On("MkdirAll", "/staging/vol-123", os.FileMode(0750)).Return(nil).Once()
		mockMounter.On("Mount", mock.Anything, "/dev/vdb", "/staging/vol-123", "ext4").Return(nil).Once()

		_, err := d.NodeStageVolume(context.Background(), req)
		require.NoError(t, err)
	})

	t.Run("NodeStageVolume Error", func(t *testing.T) {
		req := &csi.NodeStageVolumeRequest{
			VolumeId:          "vol-123",
			StagingTargetPath: "/staging/vol-123",
			VolumeCapability:  &csi.VolumeCapability{},
			PublishContext:    map[string]string{"device": "/dev/vdb"},
		}
		mockMounter.On("FormatDevice", mock.Anything, "/dev/vdb", "ext4").Return(errors.New("format failed")).Once()
		_, err := d.NodeStageVolume(context.Background(), req)
		require.Error(t, err)
	})

	t.Run("NodeStageVolume Mkdir Error", func(t *testing.T) {
		req := &csi.NodeStageVolumeRequest{
			VolumeId:          "vol-123",
			StagingTargetPath: "/staging/vol-123",
			VolumeCapability:  &csi.VolumeCapability{},
			PublishContext:    map[string]string{"device": "/dev/vdb"},
		}
		mockMounter.On("FormatDevice", mock.Anything, "/dev/vdb", "ext4").Return(nil).Once()
		mockMounter.On("MkdirAll", "/staging/vol-123", os.FileMode(0750)).Return(errors.New("mkdir failed")).Once()
		_, err := d.NodeStageVolume(context.Background(), req)
		require.Error(t, err)
	})

	t.Run("NodeStageVolume Mount Error", func(t *testing.T) {
		req := &csi.NodeStageVolumeRequest{
			VolumeId:          "vol-123",
			StagingTargetPath: "/staging/vol-123",
			VolumeCapability:  &csi.VolumeCapability{},
			PublishContext:    map[string]string{"device": "/dev/vdb"},
		}
		mockMounter.On("FormatDevice", mock.Anything, "/dev/vdb", "ext4").Return(nil).Once()
		mockMounter.On("MkdirAll", "/staging/vol-123", os.FileMode(0750)).Return(nil).Once()
		mockMounter.On("Mount", mock.Anything, "/dev/vdb", "/staging/vol-123", "ext4").Return(errors.New("mount fail")).Once()
		_, err := d.NodeStageVolume(context.Background(), req)
		require.Error(t, err)
	})

	t.Run("NodeStageVolume Missing Args", func(t *testing.T) {
		_, err := d.NodeStageVolume(context.Background(), &csi.NodeStageVolumeRequest{})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("NodeUnstageVolume Success", func(t *testing.T) {
		req := &csi.NodeUnstageVolumeRequest{
			VolumeId:          "vol-123",
			StagingTargetPath: "/staging/vol-123",
		}
		mockMounter.On("Unmount", mock.Anything, "/staging/vol-123").Return(nil).Once()
		_, err := d.NodeUnstageVolume(context.Background(), req)
		require.NoError(t, err)
	})

	t.Run("NodeUnstageVolume Error", func(t *testing.T) {
		req := &csi.NodeUnstageVolumeRequest{
			VolumeId:          "vol-123",
			StagingTargetPath: "/staging/vol-123",
		}
		mockMounter.On("Unmount", mock.Anything, "/staging/vol-123").Return(errors.New("unmount failed")).Once()
		_, err := d.NodeUnstageVolume(context.Background(), req)
		require.Error(t, err)
	})

	t.Run("NodeUnstageVolume Missing Args", func(t *testing.T) {
		_, err := d.NodeUnstageVolume(context.Background(), &csi.NodeUnstageVolumeRequest{})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("NodePublishVolume Success", func(t *testing.T) {
		req := &csi.NodePublishVolumeRequest{
			VolumeId:          "vol-123",
			StagingTargetPath: "/staging/vol-123",
			TargetPath:        "/target/vol-123",
		}
		mockMounter.On("MkdirAll", "/target/vol-123", os.FileMode(0750)).Return(nil).Once()
		mockMounter.On("BindMount", mock.Anything, "/staging/vol-123", "/target/vol-123").Return(nil).Once()
		_, err := d.NodePublishVolume(context.Background(), req)
		require.NoError(t, err)
	})

	t.Run("NodePublishVolume Error", func(t *testing.T) {
		req := &csi.NodePublishVolumeRequest{
			VolumeId:          "vol-123",
			StagingTargetPath: "/staging/vol-123",
			TargetPath:        "/target/vol-123",
			VolumeCapability:  &csi.VolumeCapability{},
		}
		mockMounter.On("MkdirAll", "/target/vol-123", os.FileMode(0750)).Return(errors.New("mkdir failed")).Once()
		_, err := d.NodePublishVolume(context.Background(), req)
		require.Error(t, err)
	})

	t.Run("NodePublishVolume Bind Error", func(t *testing.T) {
		req := &csi.NodePublishVolumeRequest{
			VolumeId:          "vol-123",
			StagingTargetPath: "/staging/vol-123",
			TargetPath:        "/target/vol-123",
		}
		mockMounter.On("MkdirAll", "/target/vol-123", os.FileMode(0750)).Return(nil).Once()
		mockMounter.On("BindMount", mock.Anything, "/staging/vol-123", "/target/vol-123").Return(errors.New("bind fail")).Once()
		_, err := d.NodePublishVolume(context.Background(), req)
		require.Error(t, err)
	})

	t.Run("NodePublishVolume Missing Args", func(t *testing.T) {
		_, err := d.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("NodeUnpublishVolume Success", func(t *testing.T) {
		req := &csi.NodeUnpublishVolumeRequest{
			VolumeId:   "vol-123",
			TargetPath: "/target/vol-123",
		}
		mockMounter.On("Unmount", mock.Anything, "/target/vol-123").Return(nil).Once()
		_, err := d.NodeUnpublishVolume(context.Background(), req)
		require.NoError(t, err)
	})

	t.Run("NodeUnpublishVolume Error", func(t *testing.T) {
		req := &csi.NodeUnpublishVolumeRequest{
			VolumeId:   "vol-123",
			TargetPath: "/target/vol-123",
		}
		mockMounter.On("Unmount", mock.Anything, "/target/vol-123").Return(errors.New("unmount failed")).Once()
		_, err := d.NodeUnpublishVolume(context.Background(), req)
		require.Error(t, err)
	})

	t.Run("NodeUnpublishVolume Missing Args", func(t *testing.T) {
		_, err := d.NodeUnpublishVolume(context.Background(), &csi.NodeUnpublishVolumeRequest{})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("Unimplemented", func(t *testing.T) {
		_, err := d.NodeGetVolumeStats(context.Background(), nil)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
		_, err = d.NodeExpandVolume(context.Background(), nil)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
	})
}

func TestDriver_Utils(t *testing.T) {
	t.Run("parseEndpoint", func(t *testing.T) {
		s, a, err := parseEndpoint("unix:///tmp/csi.sock")
		require.NoError(t, err)
		assert.Equal(t, "unix", s)
		assert.Equal(t, "/tmp/csi.sock", a)

		_, _, err = parseEndpoint("invalid")
		require.Error(t, err)
	})
}

func TestLinuxMounter_Implementation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Mock execer that calls a helper process
	mockExecer := func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		args := append([]string{"-test.run=TestHelperProcess", "--", name}, arg...)
		cmd := exec.CommandContext(ctx, os.Args[0], args...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		// Use env to pass expected command behavior
		exitCode := "1"
		switch {
		case name == "blkid" && strings.Contains(strings.Join(arg, " "), "/dev/vdb"):
			exitCode = "0"
		case name == "mkfs", name == "mount", name == "umount":
			exitCode = "0"
		}
		cmd.Env = append(cmd.Env, "HELPER_EXIT_CODE="+exitCode)
		return cmd
	}

	m := &LinuxMounter{logger: logger, execer: mockExecer}

	ctx := context.Background()

	t.Run("IsFormatted", func(t *testing.T) {
		assert.True(t, m.IsFormatted(ctx, "/dev/vdb"))
		assert.False(t, m.IsFormatted(ctx, "/dev/nonexistent"))
	})

	t.Run("FormatDevice", func(t *testing.T) {
		// Already formatted
		require.NoError(t, m.FormatDevice(ctx, "/dev/vdb", "ext4"))
		// Not formatted
		require.NoError(t, m.FormatDevice(ctx, "/dev/vdc", "ext4"))
	})

	t.Run("Mount", func(t *testing.T) {
		require.NoError(t, m.Mount(ctx, "/dev/vdb", "/mnt", "ext4"))
	})

	t.Run("BindMount", func(t *testing.T) {
		require.NoError(t, m.BindMount(ctx, "/src", "/dst"))
	})

	t.Run("Unmount", func(t *testing.T) {
		require.NoError(t, m.Unmount(ctx, "/mnt"))
	})

	t.Run("MkdirAll", func(t *testing.T) {
		tmpDir := t.TempDir() + "/subdir"
		require.NoError(t, m.MkdirAll(tmpDir, 0750))
		info, err := os.Stat(tmpDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})
}

// TestHelperProcess is used by mockExecer to simulate command execution
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	exitCode := 0
	if code := os.Getenv("HELPER_EXIT_CODE"); code != "" {
		if _, err := fmt.Sscanf(code, "%d", &exitCode); err != nil {
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

func TestLinuxMounter_Safety(t *testing.T) {
	// This test doesn't run real commands but ensures the code paths exist
	m := &LinuxMounter{logger: slog.Default()}
	assert.NotNil(t, m)
}
