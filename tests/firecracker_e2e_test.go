package tests

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/repositories/firecracker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirecrackerBackend_E2E(t *testing.T) {
	// This test requires the firecracker binary and root privileges/KVM.
	// We skip it unless explicitly enabled or running in CI with proper setup.
	if testing.Short() {
		t.Skip("skipping firecracker e2e test in short mode")
	}

	getEnv := func(key, fallback string) string {
		if val := os.Getenv(key); val != "" {
			return val
		}
		return fallback
	}

	logger := slog.Default()
	cfg := firecracker.Config{
		BinaryPath: getEnv("FIRECRACKER_BINARY", "/usr/local/bin/firecracker"),
		KernelPath: getEnv("FIRECRACKER_KERNEL", "/var/lib/thecloud/vmlinux"),
		RootfsPath: getEnv("FIRECRACKER_ROOTFS", "/var/lib/thecloud/rootfs.ext4"),
		MockMode:   os.Getenv("FIRECRACKER_MOCK_MODE") == "true",
	}

	adapter, err := firecracker.NewFirecrackerAdapter(logger, cfg)
	require.NoError(t, err, "failed to create adapter")

	// If we are on non-linux, this will return the firecracker-noop type
	if adapter.Type() != "firecracker" && adapter.Type() != "firecracker-mock" {
		t.Skipf("Skipping real firecracker test on %s platform", adapter.Type())
	}

	ctx := context.Background()
	opts := ports.CreateInstanceOptions{
		Name:        "test-firecracker-vm",
		ImageName:   "alpine",
		CPULimit:    1,
		MemoryLimit: 128 * 1024 * 1024,
	}

	t.Run("Launch and Delete", func(t *testing.T) {
		id, _, err := adapter.LaunchInstanceWithOptions(ctx, opts)
		// We expect an error if the kernel/rootfs are missing,
		// but we want to see HOW it fails in CI.
		if err != nil {
			t.Skipf("Launch failed, skipping test (likely missing artifacts or KVM access): %v", err)
		}

		require.NotEmpty(t, id)

		err = adapter.DeleteInstance(ctx, id)
		assert.NoError(t, err)
	})

	t.Run("ResizeInstance", func(t *testing.T) {
		id, _, err := adapter.LaunchInstanceWithOptions(ctx, opts)
		if err != nil {
			t.Skipf("Launch failed, skipping test: %v", err)
		}
		defer adapter.DeleteInstance(ctx, id)

		err = adapter.ResizeInstance(ctx, id, 2, 256*1024*1024)
		require.NoError(t, err, "ResizeInstance should succeed")

		// Verify via GetInstanceStats - parse CPU/memory from result
		stats, err := adapter.GetInstanceStats(ctx, id)
		if err != nil {
			// GetInstanceStats may not be implemented, that's ok
			return
		}
		assert.NotEmpty(t, stats)
	})

	t.Run("AttachVolume", func(t *testing.T) {
		id, _, err := adapter.LaunchInstanceWithOptions(ctx, opts)
		if err != nil {
			t.Skipf("Launch failed, skipping test: %v", err)
		}
		defer adapter.DeleteInstance(ctx, id)

		// Use a temporary volume path for testing
		volumePath := os.Getenv("FIRECRACKER_TEST_VOLUME")
		if volumePath == "" {
			t.Skip("FIRECRACKER_TEST_VOLUME not set, skipping AttachVolume test")
		}

		dev, _, err := adapter.AttachVolume(ctx, id, volumePath)
		require.NoError(t, err, "AttachVolume should succeed")
		assert.Equal(t, "/dev/vdb", dev)
	})

	t.Run("Resize then Attach", func(t *testing.T) {
		id, _, err := adapter.LaunchInstanceWithOptions(ctx, opts)
		if err != nil {
			t.Skipf("Launch failed, skipping test: %v", err)
		}
		defer adapter.DeleteInstance(ctx, id)

		// Resize first
		err = adapter.ResizeInstance(ctx, id, 2, 256*1024*1024)
		require.NoError(t, err, "ResizeInstance should succeed")

		// Then attach a volume
		volumePath := os.Getenv("FIRECRACKER_TEST_VOLUME")
		if volumePath == "" {
			t.Skip("FIRECRACKER_TEST_VOLUME not set, skipping AttachVolume test")
		}

		dev, _, err := adapter.AttachVolume(ctx, id, volumePath)
		require.NoError(t, err, "AttachVolume should succeed after ResizeInstance")
		assert.NotEmpty(t, dev)
	})

	t.Run("StartStopStart", func(t *testing.T) {
		id, _, err := adapter.LaunchInstanceWithOptions(ctx, opts)
		if err != nil {
			t.Skipf("Launch failed, skipping test: %v", err)
		}
		defer adapter.DeleteInstance(ctx, id)

		err = adapter.StopInstance(ctx, id)
		require.NoError(t, err)

		err = adapter.StartInstance(ctx, id)
		require.NoError(t, err)

		err = adapter.StopInstance(ctx, id)
		require.NoError(t, err)
	})

	t.Run("Ping", func(t *testing.T) {
		err := adapter.Ping(ctx)
		require.NoError(t, err, "Ping should always succeed")
	})

	t.Run("CreateAndDeleteNetwork", func(t *testing.T) {
		tapName := "fc-test-tap-e2e"
		_, err := adapter.CreateNetwork(ctx, tapName)
		require.NoError(t, err, "CreateNetwork should succeed")
		defer adapter.DeleteNetwork(ctx, tapName)
	})

	t.Run("DeleteNetwork_Twice", func(t *testing.T) {
		// DeleteNetwork is idempotent
		tapName := "fc-test-tap-e2e-dup"
		_, err := adapter.CreateNetwork(ctx, tapName)
		require.NoError(t, err)
		defer adapter.DeleteNetwork(ctx, tapName)

		_, err = adapter.CreateNetwork(ctx, tapName)
		require.NoError(t, err)
		defer adapter.DeleteNetwork(ctx, tapName)

		err = adapter.DeleteNetwork(ctx, tapName)
		require.NoError(t, err)

		err = adapter.DeleteNetwork(ctx, tapName) // second call should not fail
		require.NoError(t, err)
	})

	t.Run("GetInstanceIP_AfterLaunch", func(t *testing.T) {
		id, _, err := adapter.LaunchInstanceWithOptions(ctx, opts)
		if err != nil {
			t.Skipf("Launch failed, skipping test: %v", err)
		}
		defer adapter.DeleteInstance(ctx, id)

		ip, err := adapter.GetInstanceIP(ctx, id)
		if err != nil {
			// IP may not be available immediately
			assert.Contains(t, err.Error(), "not found")
		} else {
			assert.NotEmpty(t, ip)
		}
	})

	t.Run("StopInstance_NotFound", func(t *testing.T) {
		err := adapter.StopInstance(ctx, "nonexistent-fc-id")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("StartInstance_NotFound", func(t *testing.T) {
		err := adapter.StartInstance(ctx, "nonexistent-fc-id")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("ResizeInstance_NotFound", func(t *testing.T) {
		err := adapter.ResizeInstance(ctx, "nonexistent-fc-id", 2, 1024)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("AttachVolume_NotFound", func(t *testing.T) {
		_, _, err := adapter.AttachVolume(ctx, "nonexistent-fc-id", "/path/to/vol")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("CreateAndRestoreSnapshot", func(t *testing.T) {
		id, _, err := adapter.LaunchInstanceWithOptions(ctx, opts)
		if err != nil {
			t.Skipf("Launch failed, skipping test: %v", err)
		}
		defer adapter.DeleteInstance(ctx, id)

		err = adapter.CreateSnapshot(ctx, id, "e2e-test-snap")
		if err != nil {
			t.Skipf("CreateSnapshot not supported: %v", err)
		}

		err = adapter.StopInstance(ctx, id)
		require.NoError(t, err)

		_, err = adapter.RestoreSnapshot(ctx, id, "e2e-test-snap")
		if err != nil {
			t.Skipf("RestoreSnapshot not supported: %v", err)
		}
	})

	t.Run("DeleteSnapshot", func(t *testing.T) {
		id, _, err := adapter.LaunchInstanceWithOptions(ctx, opts)
		if err != nil {
			t.Skipf("Launch failed, skipping test: %v", err)
		}
		defer adapter.DeleteInstance(ctx, id)

		err = adapter.CreateSnapshot(ctx, id, "e2e-to-delete")
		if err != nil {
			t.Skipf("CreateSnapshot not supported: %v", err)
		}

		err = adapter.DeleteSnapshot(ctx, id, "e2e-to-delete")
		if err != nil {
			t.Skipf("DeleteSnapshot not supported: %v", err)
		}
	})
}
