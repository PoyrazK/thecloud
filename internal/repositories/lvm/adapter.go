// Package lvm implements the StorageBackend interface using Linux Logical Volume Manager.
package lvm

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/poyrazk/thecloud/internal/core/ports"
)

// execer abstracts command execution for testing
type execer interface {
	Run(name string, args ...string) ([]byte, error)
}

// realExecer implements execer using os/exec
type realExecer struct {
	ctx context.Context
}

func (r *realExecer) Run(name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(r.ctx, name, args...) //nolint:gosec // LVM commands (lvcreate, lvs, etc.) with validated paths
	return cmd.CombinedOutput()
}

// LvmAdapter implements storage backend operations using LVM.
type LvmAdapter struct {
	vgName string
	execer execer
}

// NewLvmAdapter constructs an LvmAdapter for the given volume group.
func NewLvmAdapter(vgName string) *LvmAdapter {
	return &LvmAdapter{
		vgName: vgName,
		execer: nil, // Will be set at runtime
	}
}

func (a *LvmAdapter) CreateVolume(ctx context.Context, name string, sizeGB int) (string, error) {
	if a.execer == nil {
		a.execer = &realExecer{ctx: ctx}
	}

	out, err := a.execer.Run("lvcreate", "-L", fmt.Sprintf("%dG", sizeGB), "-n", name, a.vgName)
	if err != nil {
		return "", fmt.Errorf("failed to create logical volume: %w, output: %s", err, string(out))
	}
	return fmt.Sprintf("/dev/%s/%s", a.vgName, name), nil
}

func (a *LvmAdapter) DeleteVolume(ctx context.Context, name string) error {
	if a.execer == nil {
		a.execer = &realExecer{ctx: ctx}
	}

	out, err := a.execer.Run("lvremove", "-f", fmt.Sprintf("%s/%s", a.vgName, name))
	if err != nil {
		return fmt.Errorf("failed to remove logical volume: %w, output: %s", err, string(out))
	}
	return nil
}

func (a *LvmAdapter) ResizeVolume(ctx context.Context, name string, newSizeGB int) error {
	if a.execer == nil {
		a.execer = &realExecer{ctx: ctx}
	}

	// Use -r to automatically resize the underlying filesystem (resize2fs, xfs_growfs, etc.)
	out, err := a.execer.Run("lvextend", "-r", "-L", fmt.Sprintf("%dG", newSizeGB), fmt.Sprintf("%s/%s", a.vgName, name))
	if err != nil {
		return fmt.Errorf("failed to extend logical volume and resize filesystem: %w, output: %s", err, string(out))
	}
	return nil
}

func (a *LvmAdapter) AttachVolume(ctx context.Context, volumeName, instanceID string) (string, error) {
	// Attaching in LVM context usually means making it available to the hypervisor.
	// For Libvirt, it's about adding the disk to the XML.
	// This might be better handled in the Compute Service or by a direct Libvirt call.
	// For now, we'll consider it a no-op if the volume is already in /dev/vg/vol.
	return "/dev/" + a.vgName + "/" + volumeName, nil
}

func (a *LvmAdapter) DetachVolume(ctx context.Context, volumeName, instanceID string) error {
	return nil
}

func (a *LvmAdapter) CreateSnapshot(ctx context.Context, volumeName, snapshotName string) error {
	if a.execer == nil {
		a.execer = &realExecer{ctx: ctx}
	}

	out, err := a.execer.Run("lvcreate", "-s", "-n", snapshotName, "-L", "1G", fmt.Sprintf("/dev/%s/%s", a.vgName, volumeName))
	if err != nil {
		return fmt.Errorf("failed to create lvm snapshot: %w, output: %s", err, string(out))
	}
	return nil
}

func (a *LvmAdapter) RestoreSnapshot(ctx context.Context, volumeName, snapshotName string) error {
	if a.execer == nil {
		a.execer = &realExecer{ctx: ctx}
	}

	out, err := a.execer.Run("lvconvert", "--merge", fmt.Sprintf("%s/%s", a.vgName, snapshotName))
	if err != nil {
		return fmt.Errorf("failed to restore lvm snapshot: %w, output: %s", err, string(out))
	}
	return nil
}

func (a *LvmAdapter) DeleteSnapshot(ctx context.Context, snapshotName string) error {
	if a.execer == nil {
		a.execer = &realExecer{ctx: ctx}
	}

	out, err := a.execer.Run("lvremove", "-f", fmt.Sprintf("%s/%s", a.vgName, snapshotName))
	if err != nil {
		return fmt.Errorf("failed to remove lvm snapshot: %w, output: %s", err, string(out))
	}
	return nil
}

func (a *LvmAdapter) Ping(ctx context.Context) error {
	if a.execer == nil {
		a.execer = &realExecer{ctx: ctx}
	}

	_, err := a.execer.Run("vgs", a.vgName)
	return err
}

func (a *LvmAdapter) Type() string {
	return "lvm"
}

// Ensure LvmAdapter implements StorageBackend
var _ ports.StorageBackend = (*LvmAdapter)(nil)
