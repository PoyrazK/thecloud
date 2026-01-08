//go:build integration

package docker

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerAdapter_GetContainerStats(t *testing.T) {
	ctx := context.Background()
	adapter, err := NewDockerAdapter()
	require.NoError(t, err)

	// 1. Create a dummy container
	containerID, err := adapter.CreateInstance(ctx, "stats-test", "alpine", []string{}, "", []string{}, []string{}, []string{"sleep", "10"})
	require.NoError(t, err)
	defer adapter.DeleteInstance(ctx, containerID)

	// 2. Get stats
	statsBody, err := adapter.GetInstanceStats(ctx, containerID)
	require.NoError(t, err)
	defer statsBody.Close()

	// 3. Decode one snapshot
	var stats map[string]interface{}
	err = json.NewDecoder(statsBody).Decode(&stats)
	if err == io.EOF {
		t.Skip("Docker returned EOF for stats (common in CI/Mac without active load)")
	}
	require.NoError(t, err)

	// 4. Verify basic structure
	assert.Contains(t, stats, "cpu_stats")
	assert.Contains(t, stats, "memory_stats")
}
