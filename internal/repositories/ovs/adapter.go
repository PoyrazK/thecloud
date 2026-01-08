// Package ovs implements the Open vSwitch network adapter.
package ovs

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"

	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
)

var (
	bridgeNameRegex = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)
)

type OvsAdapter struct {
	ovsPath   string // Path to ovs-vsctl
	ofctlPath string // Path to ovs-ofctl
	logger    *slog.Logger
}

// NewOvsAdapter creates a new OvsAdapter initialized with paths to the ovs-vsctl and ovs-ofctl binaries.
// It looks up both executables in PATH and returns an error if either is not found.
// The provided logger is stored on the adapter for use by its methods.
func NewOvsAdapter(logger *slog.Logger) (*OvsAdapter, error) {
	ovsctl, err := exec.LookPath("ovs-vsctl")
	if err != nil {
		return nil, fmt.Errorf("ovs-vsctl not found: %w", err)
	}

	ofctl, err := exec.LookPath("ovs-ofctl")
	if err != nil {
		return nil, fmt.Errorf("ovs-ofctl not found: %w", err)
	}

	return &OvsAdapter{
		ovsPath:   ovsctl,
		ofctlPath: ofctl,
		logger:    logger,
	}, nil
}

func (a *OvsAdapter) Ping(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, a.ovsPath, "show")
	return cmd.Run()
}

func (a *OvsAdapter) Type() string {
	return "ovs"
}

func (a *OvsAdapter) CreateBridge(ctx context.Context, name string, vxlanID int) error {
	if !bridgeNameRegex.MatchString(name) {
		return errors.New(errors.InvalidInput, "invalid bridge name")
	}

	cmd := exec.CommandContext(ctx, a.ovsPath, "add-br", name)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to add bridge", err)
	}

	return nil
}

func (a *OvsAdapter) DeleteBridge(ctx context.Context, name string) error {
	if !bridgeNameRegex.MatchString(name) {
		return errors.New(errors.InvalidInput, "invalid bridge name")
	}

	cmd := exec.CommandContext(ctx, a.ovsPath, "del-br", name)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete bridge", err)
	}

	return nil
}

func (a *OvsAdapter) ListBridges(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, a.ovsPath, "list-br")
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

	cmd := exec.CommandContext(ctx, a.ovsPath, "add-port", bridge, portName)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to add port", err)
	}

	return nil
}

func (a *OvsAdapter) DeletePort(ctx context.Context, bridge, portName string) error {
	if !bridgeNameRegex.MatchString(bridge) || !bridgeNameRegex.MatchString(portName) {
		return errors.New(errors.InvalidInput, "invalid bridge or port name")
	}

	cmd := exec.CommandContext(ctx, a.ovsPath, "del-port", bridge, portName)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete port", err)
	}

	return nil
}

func (a *OvsAdapter) CreateVXLANTunnel(ctx context.Context, bridge string, vni int, remoteIP string) error {
	if !bridgeNameRegex.MatchString(bridge) {
		return errors.New(errors.InvalidInput, "invalid bridge name")
	}

	tunnelName := fmt.Sprintf("vxlan-%s", strings.ReplaceAll(remoteIP, ".", "-"))
	cmd := exec.CommandContext(ctx, a.ovsPath,
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
	tunnelName := fmt.Sprintf("vxlan-%s", strings.ReplaceAll(remoteIP, ".", "-"))
	return a.DeletePort(ctx, bridge, tunnelName)
}

func (a *OvsAdapter) AddFlowRule(ctx context.Context, bridge string, rule ports.FlowRule) error {
	if !bridgeNameRegex.MatchString(bridge) {
		return errors.New(errors.InvalidInput, "invalid bridge name")
	}

	// ovs-ofctl add-flow <bridge> priority=<p>,<match>,actions=<actions>
	flowSpec := fmt.Sprintf("priority=%d,%s,actions=%s", rule.Priority, rule.Match, rule.Actions)
	cmd := exec.CommandContext(ctx, a.ofctlPath, "add-flow", bridge, flowSpec)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to add flow rule", err)
	}

	return nil
}

func (a *OvsAdapter) DeleteFlowRule(ctx context.Context, bridge string, match string) error {
	if !bridgeNameRegex.MatchString(bridge) {
		return errors.New(errors.InvalidInput, "invalid bridge name")
	}

	cmd := exec.CommandContext(ctx, a.ofctlPath, "del-flows", bridge, match)
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
	cmd := exec.CommandContext(ctx, "ip", "link", "add", hostEnd, "type", "veth", "peer", "name", containerEnd)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to create veth pair", err)
	}
	return nil
}

func (a *OvsAdapter) AttachVethToBridge(ctx context.Context, bridge, vethEnd string) error {
	if err := a.AddPort(ctx, bridge, vethEnd); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "ip", "link", "set", vethEnd, "up")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to set veth up", err)
	}

	return nil
}

func (a *OvsAdapter) DeleteVethPair(ctx context.Context, hostEnd string) error {
	cmd := exec.CommandContext(ctx, "ip", "link", "del", hostEnd)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete veth pair", err)
	}
	return nil
}

func (a *OvsAdapter) SetVethIP(ctx context.Context, vethEnd, ip, cidr string) error {
	cmd := exec.CommandContext(ctx, "ip", "addr", "add", fmt.Sprintf("%s/%s", ip, cidr), "dev", vethEnd)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to set veth ip", err)
	}

	cmdUp := exec.CommandContext(ctx, "ip", "link", "set", vethEnd, "up")
	if err := cmdUp.Run(); err != nil {
		return errors.Wrap(errors.Internal, "failed to bring veth up", err)
	}
	return nil
}