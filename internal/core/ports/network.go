package ports

import (
	"context"
)

// FlowRule represents an OpenFlow rule for OVS
type FlowRule struct {
	ID       string
	Priority int
	Match    string // e.g., "in_port=1,dl_type=0x0800,nw_proto=6,tp_dst=80"
	Actions  string // e.g., "allow", "drop", "output:2"
}

// NetworkBackend abstracts Open vSwitch operations to decouple networking from compute
type NetworkBackend interface {
	// Bridge Management
	CreateBridge(ctx context.Context, name string, vxlanID int) error
	DeleteBridge(ctx context.Context, name string) error
	ListBridges(ctx context.Context) ([]string, error)

	// Port Management
	AddPort(ctx context.Context, bridge, portName string) error
	DeletePort(ctx context.Context, bridge, portName string) error

	// VXLAN Tunnels (multi-node overlay)
	CreateVXLANTunnel(ctx context.Context, bridge string, vni int, remoteIP string) error
	DeleteVXLANTunnel(ctx context.Context, bridge string, remoteIP string) error

	// Security Groups (OpenFlow rules)
	AddFlowRule(ctx context.Context, bridge string, rule FlowRule) error
	DeleteFlowRule(ctx context.Context, bridge string, match string) error
	ListFlowRules(ctx context.Context, bridge string) ([]FlowRule, error)

	// Health & Type
	Ping(ctx context.Context) error
	Type() string
}
