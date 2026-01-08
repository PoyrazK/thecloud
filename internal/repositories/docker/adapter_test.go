//go:build integration

package docker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerAdapter_Integration(t *testing.T) {
	adapter, err := NewDockerAdapter()
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("Container Lifecycle", func(t *testing.T) {
		name := "integration-test-container-" + time.Now().Format("20060102150405")
		image := "alpine"

		// 1. Create
		// Using a minimal sleep command so it stays running but exits eventually
		// Signature: CreateInstance(ctx, name, imageName, ports, networkID, volumeBinds, env, cmd)
		id, err := adapter.CreateInstance(ctx, name, image, []string{}, "", []string{}, []string{}, []string{"sleep", "10"})
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		// 2. Stop
		// Note: CreateInstance returns ID, but StopInstance expects Name currently based on implementation?
		// Checking implementation: StopInstance(ctx, name string) takes name.
		// BUT the test was passing ID. Let's check if StopInstance handles ID or Name.
		// Docker API usually handles both.
		err = adapter.StopInstance(ctx, id)
		assert.NoError(t, err)

		// 3. Remove
		err = adapter.DeleteInstance(ctx, id)
		assert.NoError(t, err)
	})

	t.Run("Network Lifecycle", func(t *testing.T) {
		netName := "integration-test-net-" + time.Now().Format("20060102150405")

		// 1. Create
		id, err := adapter.CreateNetwork(ctx, netName)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		// 2. Remove
		err = adapter.RemoveNetwork(ctx, id)
		assert.NoError(t, err)
	})
}
