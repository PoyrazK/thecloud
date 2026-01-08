package ovs_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/repositories/ovs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOvsAdapter_Integration(t *testing.T) {
	if os.Getenv("OVS_INTEGRATION_TEST") != "true" {
		t.Skip("Skipping OVS integration test. Set OVS_INTEGRATION_TEST=true to run.")
	}

	if _, err := os.Stat("/usr/bin/ovs-vsctl"); os.IsNotExist(err) {
		t.Skip("Skipping OVS integration test: ovs-vsctl not found at /usr/bin/ovs-vsctl")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	adapter, err := ovs.NewOvsAdapter(logger)
	require.NoError(t, err)

	ctx := context.Background()
	bridgeName := "br-test-" + uuid.New().String()[:8]

	// 1. Create Bridge
	t.Run("CreateBridge", func(t *testing.T) {
		err := adapter.CreateBridge(ctx, bridgeName, 100)
		assert.NoError(t, err)
	})

	// 2. Add Flow Rule
	t.Run("AddFlowRule", func(t *testing.T) {
		rule := ports.FlowRule{
			Priority: 100,
			Match:    "ip,nw_src=10.0.0.1",
			Actions:  "drop",
		}
		err := adapter.AddFlowRule(ctx, bridgeName, rule)
		assert.NoError(t, err)
	})

	// 3. Delete Flow Rule
	t.Run("DeleteFlowRule", func(t *testing.T) {
		err := adapter.DeleteFlowRule(ctx, bridgeName, "ip,nw_src=10.0.0.1")
		assert.NoError(t, err)
	})

	// 4. Cleanup
	t.Run("Cleanup", func(t *testing.T) {
		err := adapter.DeleteBridge(ctx, bridgeName)
		assert.NoError(t, err)
	})
}
