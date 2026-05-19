//go:build linux

package firecracker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

const (
	defaultSocketDir = "/tmp/firecracker"
)

var (
	idRegex      = regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)
	newMachineFn = func(ctx context.Context, cfg firecracker.Config, opts ...firecracker.Opt) (Machine, error) {
		return firecracker.NewMachine(ctx, cfg, opts...)
	}
)

// Config holds Firecracker specific configuration.
type Config struct {
	BinaryPath string
	KernelPath string
	RootfsPath string
	SocketDir  string
	MockMode   bool // If true, don't start real Firecracker process
}

// Machine defines the firecracker.Machine methods used by the adapter.
// Implemented by *firecracker.Machine; satisfied by mock in tests.
type Machine interface {
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
	StopVMM() error
	Wait(ctx context.Context) error
}

// fcMachine wraps *firecracker.Machine to expose additional APIs via embedded *Client.
type fcMachine struct {
	*firecracker.Machine
}

// FirecrackerAdapter implements ports.ComputeBackend using Firecracker.
type FirecrackerAdapter struct {
	cfg      Config
	logger   *slog.Logger
	machines map[string]Machine
	mu       sync.RWMutex
	// tracking maps for instance resources
	macAddresses   map[string]string
	portMappings   map[string]string
	socketToInstID map[string]string
	networks       map[string]string
}

// NewFirecrackerAdapter creates a new FirecrackerAdapter.
func NewFirecrackerAdapter(logger *slog.Logger, cfg Config) (*FirecrackerAdapter, error) {
	if cfg.SocketDir == "" {
		cfg.SocketDir = defaultSocketDir
	}
	if err := os.MkdirAll(cfg.SocketDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create socket directory %s: %w", cfg.SocketDir, err)
	}
	// Create snapshots directory with restrictive permissions
	snapDir := filepath.Join(cfg.SocketDir, "snapshots")
	if err := os.MkdirAll(snapDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create snapshot directory %s: %w", snapDir, err)
	}

	return &FirecrackerAdapter{
		cfg:      cfg,
		logger:   logger,
		machines: make(map[string]Machine),
		macAddresses:   make(map[string]string),
		portMappings:   make(map[string]string),
		socketToInstID: make(map[string]string),
		networks:       make(map[string]string),
	}, nil
}

func (a *FirecrackerAdapter) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (string, []string, error) {
	id := uuid.New().String()
	socketPath := filepath.Join(a.cfg.SocketDir, id+".socket")

	vcpus := int64(1)
	if opts.CPULimit > 0 {
		vcpus = opts.CPULimit
	}

	mem := int64(512)
	if opts.MemoryLimit > 0 {
		mem = opts.MemoryLimit / 1024 / 1024 // Convert to MB
	}

	if a.cfg.MockMode {
		a.logger.Info("Mock mode enabled, skipping real Firecracker start", "instance_id", id)
		a.mu.Lock()
		a.machines[id] = &firecracker.Machine{} // Minimal mock
		a.mu.Unlock()
		return id, nil, nil
	}

	fcCfg := firecracker.Config{
		SocketPath:      socketPath,
		KernelImagePath: a.cfg.KernelPath,
		Drives: []models.Drive{
			{
				DriveID:      firecracker.String("1"),
				IsRootDevice: firecracker.Bool(true),
				IsReadOnly:   firecracker.Bool(false),
				PathOnHost:   firecracker.String(a.cfg.RootfsPath),
			},
		},
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(vcpus),
			MemSizeMib: firecracker.Int64(mem),
		},
	}

	cmd := firecracker.VMCommandBuilder{}.
		WithBin(a.cfg.BinaryPath).
		WithSocketPath(socketPath).
		Build(ctx)

	m, err := newMachineFn(ctx, fcCfg, firecracker.WithProcessRunner(cmd))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create machine: %w", err)
	}

	if err := m.Start(ctx); err != nil {
		return "", nil, fmt.Errorf("failed to start machine: %w", err)
	}

	a.mu.Lock()
	a.machines[id] = m
	a.mu.Unlock()

	return id, nil, nil
}

// generateMAC creates a deterministic MAC address from instance ID
func generateMAC(instanceID string) string {
	h := uuid.NewMD5(uuid.NameSpaceDNS, []byte(instanceID))
	macBytes := h[:]
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		macBytes[0]&0xfe|0x02, // Set local bit, clear multicast bit
		macBytes[1], macBytes[2], macBytes[3], macBytes[4]&0xfe)
}

func (a *FirecrackerAdapter) StartInstance(ctx context.Context, id string) error {
	if a.cfg.MockMode {
		return nil
	}
	a.mu.RLock()
	m, ok := a.machines[id]
	a.mu.RUnlock()

	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	return m.Start(ctx)
}

func (a *FirecrackerAdapter) StopInstance(ctx context.Context, id string) error {
	if a.cfg.MockMode {
		return nil
	}
	a.mu.RLock()
	m, ok := a.machines[id]
	a.mu.RUnlock()

	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	return m.Shutdown(ctx)
}

func (a *FirecrackerAdapter) PauseInstance(ctx context.Context, id string) error {
	if a.cfg.MockMode {
		return nil
	}
	return nil // Firecracker does not support pause/resume
}

func (a *FirecrackerAdapter) ResumeInstance(ctx context.Context, id string) error {
	if a.cfg.MockMode {
		return nil
	}
	return nil // Firecracker does not support pause/resume
}

func (a *FirecrackerAdapter) DeleteInstance(ctx context.Context, id string) error {
	if !idRegex.MatchString(id) {
		return fmt.Errorf("invalid instance ID format: %s", id)
	}

	a.mu.Lock()
	m, ok := a.machines[id]
	if !ok {
		a.mu.Unlock()
		return nil // Already gone
	}
	delete(a.machines, id)
	delete(a.macAddresses, id)
	delete(a.portMappings, id)
	socketPath := filepath.Join(a.cfg.SocketDir, id+".socket")
	delete(a.socketToInstID, socketPath)
	a.mu.Unlock()

	if !a.cfg.MockMode {
		if err := m.StopVMM(); err != nil {
			a.logger.Warn("failed to stop VMM during deletion", "instance_id", id, "error", err)
		}
	}

	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove socket file %s: %w", socketPath, err)
	}

	return nil
}

func (a *FirecrackerAdapter) GetInstanceLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *FirecrackerAdapter) GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *FirecrackerAdapter) GetInstancePort(ctx context.Context, id string, internalPort string) (int, error) {
	return 0, fmt.Errorf("firecracker adapter does not support port mapping yet")
}

func (a *FirecrackerAdapter) GetInstanceIP(ctx context.Context, id string) (string, error) {
	a.mu.RLock()
	mac, ok := a.macAddresses[id]
	a.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("instance %s not found", id)
	}

	ip, err := a.getIPFromARP(mac)
	if err != nil {
		return "", fmt.Errorf("failed to get IP for instance %s: %w", id, err)
	}
	return ip, nil
}

// getIPFromARP queries the ARP table for the IP associated with a MAC address
func (a *FirecrackerAdapter) getIPFromARP(mac string) (string, error) {
	// Try `ip neigh show` first
	// Format: IP dev DEVICE lladdr MAC STATE
	cmd := exec.Command("ip", "neigh", "show")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		for _, line := range strings.Split(out.String(), "\n") {
			if strings.Contains(line, strings.ToLower(mac)) {
				parts := strings.Fields(line)
				if len(parts) >= 1 {
					return parts[0], nil // IP is the first field
				}
			}
		}
	}

	// Fallback: parse /proc/net/arp directly
	data, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return "", fmt.Errorf("no IP found for MAC %s: %w", mac, err)
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines[1:] { // Skip header
		fields := strings.Fields(line)
		if len(fields) >= 4 && strings.EqualFold(fields[3], mac) {
			return fields[0], nil // IP is in first field
		}
	}

	return "", fmt.Errorf("no IP found for MAC %s", mac)
}

func (a *FirecrackerAdapter) GetConsoleURL(ctx context.Context, id string) (string, error) {
	return "", fmt.Errorf("console not implemented for firecracker")
}

func (a *FirecrackerAdapter) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	return "", fmt.Errorf("exec not implemented for firecracker")
}

func (a *FirecrackerAdapter) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, []string, error) {
	return a.LaunchInstanceWithOptions(ctx, ports.CreateInstanceOptions{
		Name:        opts.Name,
		ImageName:   opts.Image,
		Env:         opts.Env,
		Cmd:         opts.Command,
		CPULimit:    int64(opts.CPUs),
		MemoryLimit: opts.MemoryMB * 1024 * 1024,
	})
}

func (a *FirecrackerAdapter) WaitTask(ctx context.Context, id string) (int64, error) {
	if a.cfg.MockMode {
		return 0, nil
	}
	a.mu.RLock()
	m, ok := a.machines[id]
	a.mu.RUnlock()

	if !ok {
		return -1, fmt.Errorf("task %s not found", id)
	}

	err := m.Wait(ctx)
	if err != nil {
		return 1, err
	}
	return 0, nil
}

func (a *FirecrackerAdapter) CreateNetwork(ctx context.Context, name string) (string, error) {
	// Derive a unique, conflict-free TAP device name using MD5 hash of the full name.
	// This avoids collisions that would occur with simple truncation (e.g. "my-network"
	// and "my-networx" both truncate to "fc-my-netw").
	// TAP device names must be ≤ 15 chars, so we use first 8 hex chars from MD5.
	h := uuid.NewMD5(uuid.NameSpaceDNS, []byte(name))
	tapName := fmt.Sprintf("fc-%x", h[:4]) // e.g. "fc-a1b2c3d4" (12 chars)

	// Create TAP device
	cmd := exec.Command("ip", "tuntap", "add", "dev", tapName, "mode", "tap")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to create TAP device: %w (output: %s)", err, string(output))
	}

	// Bring up the interface
	cmd = exec.Command("ip", "link", "set", tapName, "up")
	if output, err := cmd.CombinedOutput(); err != nil {
		// Cleanup on failure - ignore error since we're already failing
		if cleanupCmd := exec.Command("ip", "tuntap", "delete", "dev", tapName, "mode", "tap"); cleanupCmd.Run() != nil {
			a.logger.Warn("failed to cleanup TAP device after bring-up failure", "device", tapName)
		}
		return "", fmt.Errorf("failed to bring up TAP device: %w (output: %s)", err, string(output))
	}

	a.mu.Lock()
	a.networks[tapName] = tapName
	a.mu.Unlock()

	return tapName, nil
}

func (a *FirecrackerAdapter) DeleteNetwork(ctx context.Context, id string) error {
	return nil
}

func (a *FirecrackerAdapter) AttachVolume(ctx context.Context, id string, volumePath string) (string, string, error) {
	a.mu.RLock()
	m, ok := a.machines[id]
	a.mu.RUnlock()

	if !ok {
		return "", "", fmt.Errorf("instance %s not found", id)
	}

	// Get the underlying *firecracker.Machine to call SDK methods directly
	fcM, ok := m.(*firecracker.Machine)
	if !ok {
		return "", "", fmt.Errorf("instance %s is not a real firecracker machine", id)
	}

	// Firecracker supports hot-attach of drives via PUT /drives/{drive_id}
	// Drive ID "1" is reserved for root, use "2" for first additional drive
	drive := models.Drive{
		DriveID:      firecracker.String("2"),
		PathOnHost:   firecracker.String(volumePath),
		IsRootDevice: firecracker.Bool(false),
		IsReadOnly:   firecracker.Bool(false),
	}
	if _, err := fcM.Client.PutGuestDriveByID(ctx, "2", &drive); err != nil {
		return "", "", fmt.Errorf("failed to attach drive: %w", err)
	}

	return "/dev/vdb", "", nil
}

func (a *FirecrackerAdapter) DetachVolume(ctx context.Context, id string, volumePath string) (string, error) {
	return "", fmt.Errorf("detach volume not implemented for firecracker")
}

func (a *FirecrackerAdapter) Ping(ctx context.Context) error {
	return nil
}

func (a *FirecrackerAdapter) Type() string {
	if a.cfg.MockMode {
		return "firecracker-mock"
	}
	return "firecracker"
}

func (a *FirecrackerAdapter) ResizeInstance(ctx context.Context, id string, cpu, memory int64) error {
	a.mu.RLock()
	m, ok := a.machines[id]
	a.mu.RUnlock()

	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	a.logger.Info("resizing firecracker instance", "instance_id", id, "cpu", cpu, "memory", memory)

	// Stop the VM
	if err := m.Shutdown(ctx); err != nil {
		a.logger.Warn("failed to shutdown instance for resize", "instance_id", id, "error", err)
	}

	// Get the underlying *firecracker.Machine to call SDK methods directly
	fcM, ok := m.(*firecracker.Machine)
	if !ok {
		return fmt.Errorf("instance %s is not a real firecracker machine", id)
	}

	// Update machine config via Firecracker API socket
	machineCfg := models.MachineConfiguration{
		VcpuCount:  firecracker.Int64(cpu),
		MemSizeMib: firecracker.Int64(memory),
	}
	if _, err := fcM.Client.PutMachineConfiguration(ctx, &machineCfg); err != nil {
		a.logger.Warn("failed to update machine config, trying restart anyway", "instance_id", id, "error", err)
	}

	// Restart with new config
	if err := m.Start(ctx); err != nil {
		return fmt.Errorf("failed to restart instance after resize: %w", err)
	}

	return nil
}

func (a *FirecrackerAdapter) CreateSnapshot(ctx context.Context, id, name string) error {
	return fmt.Errorf("snapshots not supported on firecracker")
}

func (a *FirecrackerAdapter) RestoreSnapshot(ctx context.Context, id, name string) error {
	return fmt.Errorf("snapshots not supported on firecracker")
}

func (a *FirecrackerAdapter) DeleteSnapshot(ctx context.Context, id, name string) error {
	snapshotPath := a.getSnapshotPath(id, name)
	if err := os.Remove(snapshotPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}
	return nil
}

func (a *FirecrackerAdapter) getSnapshotPath(id, name string) string {
	// Use SocketDir/snapshots/ for per-instance snapshot files instead of /tmp.
	// Directory is created with restrictive permissions (0700).
	snapDir := filepath.Join(a.cfg.SocketDir, "snapshots")
	return filepath.Join(snapDir, fmt.Sprintf("snapshot-%s-%s.tar.gz", id, name))
}

func (a *FirecrackerAdapter) createDiskSnapshot(ctx context.Context, diskPath, snapshotPath string) error {
	tmpQcow2 := snapshotPath + ".qcow2"

	if err := validateSnapshotPath(diskPath); err != nil {
		return fmt.Errorf("invalid disk path: %w", err)
	}
	if err := validateSnapshotPath(snapshotPath); err != nil {
		return fmt.Errorf("invalid snapshot path: %w", err)
	}
	if err := validateSnapshotPath(tmpQcow2); err != nil {
		return fmt.Errorf("invalid temp path: %w", err)
	}

	// Use wrapper for qemu-img to avoid G204 gosec warning
	qemuReq := qemuImgRequest{
		Command:    "convert",
		SourcePath: diskPath,
		TargetPath: tmpQcow2,
		Format:     "qcow2",
	}
	if err := execWrapper(ctx, a.cfg.QemuImgWrapper, qemuReq); err != nil {
		return fmt.Errorf("qemu-img convert failed: %w", err)
	}

	// Use wrapper for tar to avoid G204 gosec warning
	tarReq := tarRequest{
		Command:     "create",
		ArchivePath: snapshotPath,
		TargetDir:   filepath.Dir(tmpQcow2),
		FileName:    filepath.Base(tmpQcow2),
	}
	if err := execWrapper(ctx, a.cfg.TarWrapper, tarReq); err != nil {
		return fmt.Errorf("tar archive failed: %w", err)
	}

	if err := os.Remove(tmpQcow2); err != nil {
		a.logger.Warn("failed to remove temp qcow2 file", "path", tmpQcow2, "error", err)
	}

	return nil
}

func (a *FirecrackerAdapter) restoreDiskSnapshot(ctx context.Context, snapshotPath, diskPath string) error {
	if err := validateSnapshotPath(snapshotPath); err != nil {
		return fmt.Errorf("invalid snapshot path: %w", err)
	}
	if err := validateSnapshotPath(diskPath); err != nil {
		return fmt.Errorf("invalid disk path: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "firecracker-restore-")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			a.logger.Warn("failed to remove temp dir", "path", tmpDir, "error", err)
		}
	}()

	// Use wrapper for tar to avoid G204 gosec warning
	tarReq := tarRequest{
		Command:     "extract",
		ArchivePath: snapshotPath,
		TargetDir:   tmpDir,
	}
	if err := execWrapper(ctx, a.cfg.TarWrapper, tarReq); err != nil {
		return fmt.Errorf("untar failed: %w", err)
	}

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("empty snapshot archive")
	}

	tmpQcow2 := filepath.Join(tmpDir, files[0].Name())

	// Use wrapper for qemu-img to avoid G204 gosec warning
	qemuReq := qemuImgRequest{
		Command:    "convert",
		SourcePath: tmpQcow2,
		TargetPath: diskPath,
		Format:     "qcow2",
	}
	if err := execWrapper(ctx, a.cfg.QemuImgWrapper, qemuReq); err != nil {
		return fmt.Errorf("qemu-img restore failed: %w", err)
	}

	return nil
}

// ResetCircuitBreaker is a no-op for the raw Firecracker adapter.
// The circuit breaker lives in ResilientCompute wrapping this backend.
func (a *FirecrackerAdapter) ResetCircuitBreaker() {}
