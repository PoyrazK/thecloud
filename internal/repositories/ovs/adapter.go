// Package ovs implements the Open vSwitch network adapter.
package ovs

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"regexp"
	"strings"

	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
)

var (
	bridgeNameRegex = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)
)

const invalidBridgeNameMsg = "invalid bridge name"

// OvsAdapter implements NetworkBackend using Open vSwitch commands.
type OvsAdapter struct {
	ovsPath   string // Path to ovs-vsctl
	ofctlPath string // Path to ovs-ofctl
	logger    *slog.Logger
	exec      execer
}

type execer interface {
	LookPath(file string) (string, error)
	CommandContext(ctx context.Context, name string, args ...string) cmd
}

type cmd interface {
	Run() error
	Output() ([]byte, error)
}

type osExecer struct{}

func (osExecer) LookPath(file string) (string, error) { return exec.LookPath(file) }

func (osExecer) CommandContext(ctx context.Context, name string, args ...string) cmd {
	return exec.CommandContext(ctx, name, args...) //nolint:gosec // known OVS commands (ovs-vsctl, ovs-ofctl) with validated paths
}

// NewOvsAdapter creates an OvsAdapter with required binaries resolved.
func NewOvsAdapter(logger *slog.Logger) (*OvsAdapter, error) {
	ex := osExecer{}
	ovsctl, err := ex.LookPath("ovs-vsctl")
	if err != nil {
		return nil, fmt.Errorf("ovs-vsctl not found: %w", err)
	}

	ofctl, err := ex.LookPath("ovs-ofctl")
	if err != nil {
		return nil, fmt.Errorf("ovs-ofctl not found: %w", err)
	}

	return &OvsAdapter{
		ovsPath:   ovsctl,
		ofctlPath: ofctl,
		logger:    logger,
		exec:      ex,
	}, nil
}

func (a *OvsAdapter) Ping(ctx context.Context) error {
	cmd := a.exec.CommandContext(ctx, a.ovsPath, "show")
	return cmd.Run()
}

func (a *OvsAdapter) Type() string {
	return "ovs"
}

func (a *OvsAdapter) CreateBridge(ctx context.Context, name string, vxlanID int) error {
	if !bridgeNameRegex.MatchString(name) {
		return errors.New(errors.InvalidInput, invalidBridgeNameMsg)
	}

	cmd := a.exec.CommandContext(ctx, a.ovsPath, "add-br", name)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to add bridge", err)
	}

	return nil
}

func (a *OvsAdapter) DeleteBridge(ctx context.Context, name string) error {
	if !bridgeNameRegex.MatchString(name) {
		return errors.New(errors.InvalidInput, invalidBridgeNameMsg)
	}

	cmd := a.exec.CommandContext(ctx, a.ovsPath, "del-br", name)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete bridge", err)
	}

	return nil
}

func (a *OvsAdapter) ListBridges(ctx context.Context) ([]string, error) {
	cmd := a.exec.CommandContext(ctx, a.ovsPath, "list-br")
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list bridges", err)
	}

	bridges := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(bridges) == 1 && bridges[0] == "" {
		return []string{}, nil
	}
	return bridges, nil
}

func (a *OvsAdapter) AddPort(ctx context.Context, bridge, portName string) error {
	if !bridgeNameRegex.MatchString(bridge) || !bridgeNameRegex.MatchString(portName) {
		return errors.New(errors.InvalidInput, "invalid bridge or port name")
	}

	cmd := a.exec.CommandContext(ctx, a.ovsPath, "add-port", bridge, portName)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to add port", err)
	}

	return nil
}

func (a *OvsAdapter) DeletePort(ctx context.Context, bridge, portName string) error {
	if !bridgeNameRegex.MatchString(bridge) || !bridgeNameRegex.MatchString(portName) {
		return errors.New(errors.InvalidInput, "invalid bridge or port name")
	}

	cmd := a.exec.CommandContext(ctx, a.ovsPath, "del-port", bridge, portName)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete port", err)
	}

	return nil
}

func (a *OvsAdapter) CreateVXLANTunnel(ctx context.Context, bridge string, vni int, remoteIP string) error {
	if !bridgeNameRegex.MatchString(bridge) {
		return errors.New(errors.InvalidInput, invalidBridgeNameMsg)
	}
	if net.ParseIP(remoteIP) == nil {
		return errors.New(errors.InvalidInput, "invalid remote IP address")
	}

	tunnelName := fmt.Sprintf("vxlan-%s", strings.ReplaceAll(remoteIP, ".", "-"))
	cmd := a.exec.CommandContext(ctx, a.ovsPath,
		"add-port", bridge, tunnelName,
		"--", "set", "interface", tunnelName,
		"type=vxlan",
		fmt.Sprintf("options:remote_ip=%s", remoteIP),
		fmt.Sprintf("options:key=%d", vni),
	)

	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to create vxlan tunnel", err)
	}

	return nil
}

func (a *OvsAdapter) DeleteVXLANTunnel(ctx context.Context, bridge string, remoteIP string) error {
	if net.ParseIP(remoteIP) == nil {
		return errors.New(errors.InvalidInput, "invalid remote IP address")
	}
	tunnelName := fmt.Sprintf("vxlan-%s", strings.ReplaceAll(remoteIP, ".", "-"))
	return a.DeletePort(ctx, bridge, tunnelName)
}

func (a *OvsAdapter) AddFlowRule(ctx context.Context, bridge string, rule ports.FlowRule) error {
	if !bridgeNameRegex.MatchString(bridge) {
		return errors.New(errors.InvalidInput, invalidBridgeNameMsg)
	}

	// Basic validation to prevent command/flow injection
	if strings.ContainsAny(rule.Match, ";|&><`$") || strings.ContainsAny(rule.Actions, ";|&><`$") {
		return errors.New(errors.InvalidInput, "invalid characters in flow rule")
	}

	// ovs-ofctl add-flow <bridge> priority=<p>,<match>,actions=<actions>
	flowSpec := fmt.Sprintf("priority=%d,%s,actions=%s", rule.Priority, rule.Match, rule.Actions)
	cmd := a.exec.CommandContext(ctx, a.ofctlPath, "add-flow", bridge, flowSpec)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to add flow rule", err)
	}

	return nil
}

func (a *OvsAdapter) DeleteFlowRule(ctx context.Context, bridge string, match string) error {
	if !bridgeNameRegex.MatchString(bridge) {
		return errors.New(errors.InvalidInput, invalidBridgeNameMsg)
	}

	cmd := a.exec.CommandContext(ctx, a.ofctlPath, "del-flows", bridge, match)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete flow rule", err)
	}

	return nil
}

func (a *OvsAdapter) ListFlowRules(_ context.Context, _ string) ([]ports.FlowRule, error) {
	// Parsing ovs-ofctl dump-flows output is complex and will be implemented in a follow-up.
	return []ports.FlowRule{}, nil
}

func (a *OvsAdapter) CreateVethPair(ctx context.Context, hostEnd, containerEnd string) error {
	cmd := a.exec.CommandContext(ctx, "ip", "link", "add", hostEnd, "type", "veth", "peer", "name", containerEnd)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to create veth pair", err)
	}
	return nil
}

func (a *OvsAdapter) AttachVethToBridge(ctx context.Context, bridge, vethEnd string) error {
	if err := a.AddPort(ctx, bridge, vethEnd); err != nil {
		return err
	}

	cmd := a.exec.CommandContext(ctx, "ip", "link", "set", vethEnd, "up")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to set veth up", err)
	}

	return nil
}

func (a *OvsAdapter) DeleteVethPair(ctx context.Context, hostEnd string) error {
	cmd := a.exec.CommandContext(ctx, "ip", "link", "del", hostEnd)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete veth pair", err)
	}
	return nil
}

func (a *OvsAdapter) SetVethIP(ctx context.Context, vethEnd, ip, cidr string) error {
	cmd := a.exec.CommandContext(ctx, "ip", "addr", "add", fmt.Sprintf("%s/%s", ip, cidr), "dev", vethEnd)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to set veth ip", err)
	}

	cmdUp := a.exec.CommandContext(ctx, "ip", "link", "set", vethEnd, "up")
	if err := cmdUp.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to bring veth up", err)
	}
	return nil
}

// SetupNATForSubnet configures iptables SNAT rules for outbound traffic from a subnet.
func (a *OvsAdapter) SetupNATForSubnet(ctx context.Context, bridge, natVethEnd, subnetCIDR, egressIP string) error {
	// Get the physical interface that the NAT gateway's veth is attached to
	// We use iptables to perform SNAT on traffic from the subnet

	// First, ensure IP forwarding is enabled
	cmd := a.exec.CommandContext(ctx, "sysctl", "-w", "net.ipv4.ip_forward=1")
	if err := cmd.Run(); err != nil {
		a.logger.Warn("failed to enable IP forwarding", "error", err)
	}

	// Add SNAT rule: traffic from subnet going out via the nat veth endpoint gets SNATed
	// -t nat: use NAT table
	// -A POSTROUTING: append to POSTROUTING chain (after routing decision)
	// -s <subnetCIDR>: match traffic from this subnet
	// -o <natVethEnd>: output device is the NAT veth
	// -j SNAT --to-source <egressIP>: SNAT to the egress IP
	cmd = a.exec.CommandContext(ctx, "iptables", "-t", "nat", "-A", "POSTROUTING",
		"-s", subnetCIDR, "-o", natVethEnd, "-j", "SNAT", "--to-source", egressIP)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to setup NAT SNAT rule", err)
	}

	a.logger.Info("NAT SNAT rule configured",
		"subnet", subnetCIDR,
		"egress_ip", egressIP,
		"nat_veth", natVethEnd)

	return nil
}

// RemoveNATForSubnet removes iptables SNAT rules for a subnet.
// egressIP is used to precisely match the SNAT rule when deleting.
func (a *OvsAdapter) RemoveNATForSubnet(ctx context.Context, bridge, natVethEnd, subnetCIDR, egressIP string) error {
	// Use -C to check if rule exists (with egressIP), then -D to delete it
	checkCmd := a.exec.CommandContext(ctx, "iptables", "-t", "nat", "-C", "POSTROUTING",
		"-s", subnetCIDR, "-o", natVethEnd, "-j", "SNAT", "--to-source", egressIP)
	// If rule exists, delete it; if not, just log and return (no error)
	if checkCmd.Run() == nil {
		// Rule exists - now delete it
		cmd := a.exec.CommandContext(ctx, "iptables", "-t", "nat", "-D", "POSTROUTING",
			"-s", subnetCIDR, "-o", natVethEnd, "-j", "SNAT", "--to-source", egressIP)
		if err := cmd.Run(); err != nil {
			return errors.Wrap(errors.Internal, "failed to remove NAT SNAT rule", err)
		}
	} else {
		a.logger.Warn("NAT SNAT rule does not exist, nothing to remove", "subnet", subnetCIDR)
	}

	a.logger.Info("NAT SNAT rule removed", "subnet", subnetCIDR, "nat_veth", natVethEnd)
	return nil
}
