//go:build linux

package firecracker

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFirecrackerAdapter_InterfaceCompliance(t *testing.T) {
	var _ ports.ComputeBackend = (*FirecrackerAdapter)(nil)
}

func TestNewFirecrackerAdapter(t *testing.T) {
	logger := slog.Default()
	cfg := Config{
		BinaryPath: "/usr/local/bin/firecracker",
		KernelPath: "/var/lib/thecloud/vmlinux",
		RootfsPath: "/var/lib/thecloud/rootfs.ext4",
		SocketDir:  "/tmp/firecracker-test",
	}

	adapter, err := NewFirecrackerAdapter(logger, cfg)
	require.NoError(t, err)

	assert.NotNil(t, adapter)
	assert.Equal(t, "firecracker", adapter.Type())
	assert.Equal(t, cfg.SocketDir, adapter.cfg.SocketDir)

	t.Run("InvalidSocketDir", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "fc-not-a-dir")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = NewFirecrackerAdapter(logger, Config{SocketDir: tmpFile.Name()})
		require.Error(t, err)
	})
}

func TestFirecrackerAdapter_DeleteInstance(t *testing.T) {
	logger := slog.Default()
	cfg := Config{
		SocketDir: t.TempDir(),
		MockMode:  true,
	}
	adapter, err := NewFirecrackerAdapter(logger, cfg)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("InvalidID", func(t *testing.T) {
		err := adapter.DeleteInstance(ctx, "../invalid")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid instance ID format")
	})

	t.Run("NonExistentInstance", func(t *testing.T) {
		err := adapter.DeleteInstance(ctx, "nonexistent")
		require.NoError(t, err) // Should return nil if not found
	})
}

func TestFirecrackerAdapter_WaitTask_Mock(t *testing.T) {
	logger := slog.Default()
	cfg := Config{
		MockMode: true,
	}
	adapter, err := NewFirecrackerAdapter(logger, cfg)
	require.NoError(t, err)

	ctx := context.Background()
	exitCode, err := adapter.WaitTask(ctx, "any")
	require.NoError(t, err)
	assert.Equal(t, int64(0), exitCode)
}

func TestFirecrackerAdapter_UnimplementedMethods(t *testing.T) {
	logger := slog.Default()
	adapter, err := NewFirecrackerAdapter(logger, Config{MockMode: true})
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("GetInstanceLogs", func(t *testing.T) {
		_, err := adapter.GetInstanceLogs(ctx, "id")
		require.Error(t, err)
	})

	t.Run("GetInstanceStats", func(t *testing.T) {
		_, err := adapter.GetInstanceStats(ctx, "id")
		require.Error(t, err)
	})

	t.Run("GetInstancePort", func(t *testing.T) {
		_, err := adapter.GetInstancePort(ctx, "id", "80")
		require.Error(t, err)
	})

	t.Run("GetInstanceIP", func(t *testing.T) {
		ip, err := adapter.GetInstanceIP(ctx, "id")
		require.NoError(t, err)
		assert.Equal(t, "0.0.0.0", ip)
	})

	t.Run("GetConsoleURL", func(t *testing.T) {
		_, err := adapter.GetConsoleURL(ctx, "id")
		require.Error(t, err)
	})

	t.Run("Exec", func(t *testing.T) {
		_, err := adapter.Exec(ctx, "id", []string{"ls"})
		require.Error(t, err)
	})

	t.Run("AttachDetachVolume", func(t *testing.T) {
		_, _, err := adapter.AttachVolume(ctx, "id", "path")
		require.Error(t, err)
		_, err = adapter.DetachVolume(ctx, "id", "path")
		require.Error(t, err)
	})

	t.Run("Ping", func(t *testing.T) {
		err := adapter.Ping(ctx)
		require.NoError(t, err)
	})

	t.Run("Type", func(t *testing.T) {
		assert.Equal(t, "firecracker-mock", adapter.Type())
	})
}

func TestFirecrackerAdapter_LaunchInstanceWithOptions_MockMode(t *testing.T) {
	t.Parallel()
	logger := slog.Default()
	cfg := Config{
		SocketDir: t.TempDir(),
		MockMode:  true,
	}
	adapter, err := NewFirecrackerAdapter(logger, cfg)
	require.NoError(t, err)

	ctx := context.Background()
	id, warnings, err := adapter.LaunchInstanceWithOptions(ctx, ports.CreateInstanceOptions{
		Name:        "test-instance",
		ImageName:   "test-image",
		CPULimit:    2,
		MemoryLimit: 2 * 1024 * 1024 * 1024,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, id)
	assert.Nil(t, warnings)

	adapter.mu.RLock()
	machine, ok := adapter.machines[id]
	adapter.mu.RUnlock()
	assert.True(t, ok, "machine should be stored in map")
	assert.NotNil(t, machine)
}

func TestFirecrackerAdapter_LaunchInstanceWithOptions_RealMode_StartError(t *testing.T) {
	// Not parallel: newMachineFn is a shared package-level variable
	logger := slog.Default()
	cfg := Config{
		SocketDir:  t.TempDir(),
		MockMode:   false,
		BinaryPath: "/usr/local/bin/firecracker",
		KernelPath: "/var/lib/thecloud/vmlinux",
		RootfsPath: "/var/lib/thecloud/rootfs.ext4",
	}
	adapter, err := NewFirecrackerAdapter(logger, cfg)
	require.NoError(t, err)

	ctx := context.Background()
	origNewMachineFn := newMachineFn
	newMachineFn = func(ctx context.Context, cfg firecracker.Config, opts ...firecracker.Opt) (Machine, error) {
		m := new(mockFirecrackerMachine)
		m.On("Start", mock.Anything).Return(errors.New("start failed"))
		return m, nil
	}
	t.Cleanup(func() { newMachineFn = origNewMachineFn })

	_, _, err = adapter.LaunchInstanceWithOptions(ctx, ports.CreateInstanceOptions{Name: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start machine")
}

func TestFirecrackerAdapter_LaunchInstanceWithOptions_RealMode_CreateError(t *testing.T) {
	// Not parallel: newMachineFn is a shared package-level variable
	logger := slog.Default()
	cfg := Config{
		SocketDir:  t.TempDir(),
		MockMode:   false,
		BinaryPath: "/usr/local/bin/firecracker",
		KernelPath: "/var/lib/thecloud/vmlinux",
		RootfsPath: "/var/lib/thecloud/rootfs.ext4",
	}
	adapter, err := NewFirecrackerAdapter(logger, cfg)
	require.NoError(t, err)

	ctx := context.Background()
	origNewMachineFn := newMachineFn
	t.Cleanup(func() { newMachineFn = origNewMachineFn })

	newMachineFn = func(ctx context.Context, cfg firecracker.Config, opts ...firecracker.Opt) (Machine, error) {
		return nil, errors.New("create failed")
	}

	_, _, err = adapter.LaunchInstanceWithOptions(ctx, ports.CreateInstanceOptions{Name: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create machine")
}

func TestFirecrackerAdapter_StartInstance_MockMode(t *testing.T) {
	t.Parallel()
	logger := slog.Default()
	cfg := Config{
		SocketDir: t.TempDir(),
		MockMode:  true,
	}
	adapter, err := NewFirecrackerAdapter(logger, cfg)
	require.NoError(t, err)

	ctx := context.Background()
	err = adapter.StartInstance(ctx, "any-id")
	require.NoError(t, err)
}

func TestFirecrackerAdapter_StartInstance_RealMode_NotFound(t *testing.T) {
	t.Parallel()
	logger := slog.Default()
	cfg := Config{
		SocketDir: t.TempDir(),
		MockMode:  false,
	}
	adapter, err := NewFirecrackerAdapter(logger, cfg)
	require.NoError(t, err)

	ctx := context.Background()
	err = adapter.StartInstance(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestFirecrackerAdapter_StartInstance_RealMode_StartError(t *testing.T) {
	logger := slog.Default()
	cfg := Config{
		SocketDir: t.TempDir(),
		MockMode:  false,
	}
	adapter, err := NewFirecrackerAdapter(logger, cfg)
	require.NoError(t, err)

	ctx := context.Background()
	existingID := "existing-vm"

	origNewMachineFn := newMachineFn
	t.Cleanup(func() { newMachineFn = origNewMachineFn })

	successMachine := new(mockFirecrackerMachine)
	successMachine.On("Start", mock.Anything).Return(nil).Maybe()
	successMachine.On("Shutdown", mock.Anything).Return(nil).Maybe()

	newMachineFn = func(ctx context.Context, cfg firecracker.Config, opts ...firecracker.Opt) (Machine, error) {
		return successMachine, nil
	}

	_, _, err = adapter.LaunchInstanceWithOptions(ctx, ports.CreateInstanceOptions{Name: "new"})
	require.NoError(t, err)

	failMachine := new(mockFirecrackerMachine)
	failMachine.On("Start", mock.Anything).Return(errors.New("start error"))

	adapter.mu.Lock()
	adapter.machines[existingID] = failMachine
	adapter.mu.Unlock()

	err = adapter.StartInstance(ctx, existingID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start error")
	failMachine.AssertExpectations(t)
}

func TestFirecrackerAdapter_StopInstance_MockMode(t *testing.T) {
	t.Parallel()
	logger := slog.Default()
	cfg := Config{
		SocketDir: t.TempDir(),
		MockMode:  true,
	}
	adapter, err := NewFirecrackerAdapter(logger, cfg)
	require.NoError(t, err)

	ctx := context.Background()
	err = adapter.StopInstance(ctx, "any-id")
	require.NoError(t, err)
}

func TestFirecrackerAdapter_StopInstance_RealMode_NotFound(t *testing.T) {
	t.Parallel()
	logger := slog.Default()
	cfg := Config{
		SocketDir: t.TempDir(),
		MockMode:  false,
	}
	adapter, err := NewFirecrackerAdapter(logger, cfg)
	require.NoError(t, err)

	ctx := context.Background()
	err = adapter.StopInstance(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestFirecrackerAdapter_StopInstance_RealMode_ShutdownError(t *testing.T) {
	logger := slog.Default()
	cfg := Config{
		SocketDir: t.TempDir(),
		MockMode:  false,
	}
	adapter, err := NewFirecrackerAdapter(logger, cfg)
	require.NoError(t, err)

	ctx := context.Background()
	existingID := "existing-vm"

	origNewMachineFn := newMachineFn
	newMachineFn = func(ctx context.Context, cfg firecracker.Config, opts ...firecracker.Opt) (Machine, error) {
		successMachine := new(mockFirecrackerMachine)
		successMachine.On("Start", mock.Anything).Return(nil).Maybe()
		successMachine.On("Shutdown", mock.Anything).Return(nil).Maybe()
		return successMachine, nil
	}
	t.Cleanup(func() { newMachineFn = origNewMachineFn })

	_, _, err = adapter.LaunchInstanceWithOptions(ctx, ports.CreateInstanceOptions{Name: "new"})
	require.NoError(t, err)

	failMachine := new(mockFirecrackerMachine)
	failMachine.On("Shutdown", mock.Anything).Return(errors.New("shutdown error"))

	adapter.mu.Lock()
	adapter.machines[existingID] = failMachine
	adapter.mu.Unlock()

	err = adapter.StopInstance(ctx, existingID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shutdown error")
	failMachine.AssertExpectations(t)
}
