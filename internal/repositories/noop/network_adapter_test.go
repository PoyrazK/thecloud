package noop

import (
	"context"
	"log/slog"
	"testing"

	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/assert"
)

func TestNoopNetworkAdapter_Type(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	assert.Equal(t, "noop", adapter.Type())
}

func TestNoopNetworkAdapter_CreateBridge(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	err := adapter.CreateBridge(context.Background(), "br-test", 100)
	assert.NoError(t, err)
}

func TestNoopNetworkAdapter_DeleteBridge(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	err := adapter.DeleteBridge(context.Background(), "br-test")
	assert.NoError(t, err)
}

func TestNoopNetworkAdapter_ListBridges(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	result, err := adapter.ListBridges(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestNoopNetworkAdapter_AddPort(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	err := adapter.AddPort(context.Background(), "br-test", "port-test")
	assert.NoError(t, err)
}

func TestNoopNetworkAdapter_DeletePort(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	err := adapter.DeletePort(context.Background(), "br-test", "port-test")
	assert.NoError(t, err)
}

func TestNoopNetworkAdapter_CreateVXLANTunnel(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	err := adapter.CreateVXLANTunnel(context.Background(), "br-test", 50, "10.0.0.1")
	assert.NoError(t, err)
}

func TestNoopNetworkAdapter_DeleteVXLANTunnel(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	err := adapter.DeleteVXLANTunnel(context.Background(), "br-test", "10.0.0.1")
	assert.NoError(t, err)
}

func TestNoopNetworkAdapter_AddFlowRule(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	err := adapter.AddFlowRule(context.Background(), "br-test", ports.FlowRule{})
	assert.NoError(t, err)
}

func TestNoopNetworkAdapter_DeleteFlowRule(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	err := adapter.DeleteFlowRule(context.Background(), "br-test", "priority=100")
	assert.NoError(t, err)
}

func TestNoopNetworkAdapter_ListFlowRules(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	result, err := adapter.ListFlowRules(context.Background(), "br-test")
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestNoopNetworkAdapter_CreateVethPair(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	err := adapter.CreateVethPair(context.Background(), "host-end", "container-end")
	assert.NoError(t, err)
}

func TestNoopNetworkAdapter_AttachVethToBridge(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	err := adapter.AttachVethToBridge(context.Background(), "br-test", "veth-test")
	assert.NoError(t, err)
}

func TestNoopNetworkAdapter_DeleteVethPair(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	err := adapter.DeleteVethPair(context.Background(), "veth-test")
	assert.NoError(t, err)
}

func TestNoopNetworkAdapter_SetVethIP(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	err := adapter.SetVethIP(context.Background(), "veth-test", "10.0.0.1", "24")
	assert.NoError(t, err)
}

func TestNoopNetworkAdapter_SetupNATForSubnet(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	err := adapter.SetupNATForSubnet(context.Background(), "br-test", "nat-test", "10.0.0.0/24", "10.0.0.1")
	assert.NoError(t, err)
}

func TestNoopNetworkAdapter_RemoveNATForSubnet(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	err := adapter.RemoveNATForSubnet(context.Background(), "br-test", "nat-test", "10.0.0.0/24", "10.0.0.1")
	assert.NoError(t, err)
}

func TestNoopNetworkAdapter_Ping(t *testing.T) {
	t.Parallel()
	adapter := NewNoopNetworkAdapter(slog.Default())
	err := adapter.Ping(context.Background())
	assert.NoError(t, err)
}