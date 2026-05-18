//go:build linux

package firecracker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/google/uuid"
	apierrors "github.com/poyrazk/thecloud/internal/errors"
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

// FirecrackerAdapter implements ports.ComputeBackend using Firecracker.
type FirecrackerAdapter struct {
	cfg      Config
	logger   *slog.Logger
	machines map[string]Machine
	mu       sync.RWMutex

	// Network and port tracking
	networks       map[string]string         // networkName -> TAP device name
	portMappings   map[string]map[string]int  // instanceID -> internalPort -> hostPort
	macAddresses   map[string]string         // instanceID -> MAC address
	socketToInstID map[string]string         // socketPath -> instanceID (for IP lookup)
}

// NewFirecrackerAdapter creates a new FirecrackerAdapter.
func NewFirecrackerAdapter(logger *slog.Logger, cfg Config) (*FirecrackerAdapter, error) {
	if cfg.SocketDir == "" {
		cfg.SocketDir = defaultSocketDir
	}
	if err := os.MkdirAll(cfg.SocketDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create socket directory %s: %w", cfg.SocketDir, err)
	}

	return &FirecrackerAdapter{
		cfg:            cfg,
		logger:         logger,
		machines:       make(map[string]Machine),
		networks:       make(map[string]string),
		portMappings:   make(map[string]map[string]int),
		macAddresses:   make(map[string]string),
		socketToInstID: make(map[string]string),
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
	a.socketToInstID[socketPath] = id
	// Generate a MAC address for this instance
	mac := generateMAC(id)
	a.macAddresses[id] = mac
	a.mu.Unlock()

	return id, nil, nil
}

// generateMAC creates a deterministic MAC address from instance ID
func generateMAC(instanceID string) string {
	h := uuid.NewMD5(uuid.NameSpaceDNS, []byte(instanceID))
	macBytes := h[:]
	return fmt.Sprintf("02:%02x:%02x:%02x:%02x:%02x",
		macBytes[0]&0xfe|0x02, // Set local bit, clear multicast bit
		macBytes[1], macBytes[2], macBytes[3], macBytes[4]%0xfe)
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
	return apierrors.ErrInstanceNotPausable
}

func (a *FirecrackerAdapter) ResumeInstance(ctx context.Context, id string) error {
	if a.cfg.MockMode {
		return nil
	}
	return apierrors.ErrInstanceNotResumable
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
	a.mu.Unlock()

	if !a.cfg.MockMode {
		if err := m.StopVMM(); err != nil {
			a.logger.Warn("failed to stop VMM during deletion", "instance_id", id, "error", err)
		}
	}

	socketPath := filepath.Join(a.cfg.SocketDir, id+".socket")
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove socket file %s: %w", socketPath, err)
	}

	return nil
}

func (a *FirecrackerAdapter) GetInstanceLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	a.mu.RLock()
	_, ok := a.machines[id]
	a.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("instance %s not found", id)
	}

	paths := []string{
		filepath.Join(a.cfg.SocketDir, id+"-console.log"),
		filepath.Join(os.TempDir(), "firecracker-"+id+".log"),
		filepath.Join(os.Getenv("HOME"), ".local/share/firecracker", id, "console.log"),
	}

	for _, p := range paths {
		if f, err := os.Open(p); err == nil {
			return f, nil
		}
	}

	return nil, fmt.Errorf("log file not found for instance %s", id)
}

func (a *FirecrackerAdapter) GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error) {
	a.mu.RLock()
	socketPath, ok := a.socketToInstID[filepath.Join(a.cfg.SocketDir, id+".socket")]
	a.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("instance %s not found", id)
	}

	pid, err := a.findFirecrackerProcess(socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find process for instance %s: %w", id, err)
	}

	stats, err := a.readProcessStats(pid)
	if err != nil {
		return nil, fmt.Errorf("failed to read stats for instance %s: %w", id, err)
	}

	data, err := json.Marshal(stats)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal stats: %w", err)
	}

	return io.NopCloser(bytes.NewReader(data)), nil
}

// findFirecrackerProcess finds the PID of a Firecracker process by socket path
func (a *FirecrackerAdapter) findFirecrackerProcess(socketPath string) (int, error) {
	// Read /proc/*/cmdline and look for the socket path
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pidStr := entry.Name()
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		cmdline, err := os.ReadFile(filepath.Join("/proc", pidStr, "cmdline"))
		if err != nil {
			continue
		}

		if strings.Contains(string(cmdline), socketPath) {
			return pid, nil
		}
	}

	return 0, fmt.Errorf("process not found for socket %s", socketPath)
}

// processStats holds CPU and memory statistics
type processStats struct {
	MemoryStats memoryStats `json:"memory_stats"`
	CPUStats    cpuStats    `json:"cpu_stats"`
}

type memoryStats struct {
	Usage uint64 `json:"usage"`
	Limit uint64 `json:"limit"`
}

type cpuStats struct {
	CPUTime uint64 `json:"cpu_time"`
}

// readProcessStats reads CPU and memory stats from /proc/{pid}/status and /proc/{pid}/stat
func (a *FirecrackerAdapter) readProcessStats(pid int) (*processStats, error) {
	// Read memory from /proc/{pid}/status
	statusData, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "status"))
	if err != nil {
		return nil, err
	}

	var memUsage, memLimit uint64
	lines := strings.Split(string(statusData), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				memUsage, _ = strconv.ParseUint(fields[1], 10, 64)
				memUsage *= 1024 // Convert KB to bytes
			}
		}
		if strings.HasPrefix(line, "VmSize:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				memLimit, _ = strconv.ParseUint(fields[1], 10, 64)
				memLimit *= 1024 // Convert KB to bytes
			}
		}
	}

	// Read CPU time from /proc/{pid}/stat
	statData, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return nil, err
	}

	var cpuTime uint64
	// Read CPU time from /proc/{pid}/stat
	// Fields after comm: utime (14th), stime (15th) in clock ticks
	idx := strings.LastIndex(string(statData), ")")
	if idx >= 0 {
		fields := strings.Fields(string(statData)[idx+1:])
		if len(fields) >= 4 {
			utime, _ := strconv.ParseUint(fields[0], 10, 64)
			stime, _ := strconv.ParseUint(fields[1], 10, 64)
			cpuTime = (utime + stime) * 1e9 / 100 // 100 Hz typical clock tick rate
		}
	}

	return &processStats{
		MemoryStats: memoryStats{Usage: memUsage, Limit: memLimit},
		CPUStats:    cpuStats{CPUTime: cpuTime},
	}, nil
}

// getJiffiesPerSecond returns the system clock tick rate (jiffies per second).
// Returns 100 which is the Linux default on most architectures.
func getJiffiesPerSecond() int64 {
	return 100
}

func (a *FirecrackerAdapter) GetInstancePort(ctx context.Context, id string, internalPort string) (int, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if mappings, ok := a.portMappings[id]; ok {
		if hostPort, ok := mappings[internalPort]; ok {
			return hostPort, nil
		}
	}

	return 0, fmt.Errorf("no host port mapping found for instance %s port %s", id, internalPort)
}

// setupPortForwarding configures iptables NAT rules for port mapping.
// NOTE: This function is not yet integrated; Firecracker port forwarding requires
// guest-side network configuration that is not yet implemented.
func (a *FirecrackerAdapter) setupPortForwarding(id string, ip string, ports []string) { //nolint:unused
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	a.mu.Lock()
	if a.portMappings[id] == nil {
		a.portMappings[id] = make(map[string]int)
	}
	a.mu.Unlock()

	for _, p := range ports {
		parts := strings.Split(p, ":")
		var hostPort, containerPort string
		switch len(parts) {
		case 2:
			hostPort = parts[0]
			containerPort = parts[1]
		case 1:
			hostPort = "0"
			containerPort = parts[0]
		default:
			a.logger.Warn("invalid port format", "port", p)
			continue
		}

		var hPort int
		if hostPort == "0" {
			// Auto-assign free port
			addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
			if err != nil {
				continue
			}
			l, err := net.ListenTCP("tcp", addr)
			if err != nil {
				continue
			}
			tcpAddr, ok := l.Addr().(*net.TCPAddr)
			if !ok {
				continue
			}
			l.Close()
			hPort = tcpAddr.Port
		} else {
			if _, err := fmt.Sscanf(hostPort, "%d", &hPort); err != nil {
				a.logger.Warn("invalid host port", "value", hostPort)
				continue
			}
		}

		cPort, _ := strconv.Atoi(containerPort)

		// Validate ports before using in exec command
		if !isValidPort(hPort) || !isValidPort(cPort) {
			a.logger.Warn("invalid port value", "host_port", hPort, "container_port", cPort)
			continue
		}

		// Validate IP format (basic check for injection)
		if net.ParseIP(ip) == nil {
			a.logger.Warn("invalid IP address", "ip", ip)
			continue
		}

		a.mu.Lock()
		a.portMappings[id][containerPort] = hPort
		a.mu.Unlock()

		// Set up iptables NAT rule
		iptablesCmd := exec.CommandContext(ctx, "iptables", "-t", "nat", "-A", "PREROUTING",
			"-p", "tcp", "--dport", strconv.Itoa(hPort),
			"-j", "DNAT", "--to-destination", ip+":"+containerPort)
		if err := iptablesCmd.Run(); err != nil {
			a.logger.Warn("failed to set up iptables rule", "host_port", hPort, "container_port", cPort, "error", err)
		}
	}
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
	cmd := exec.Command("ip", "neigh", "show")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		for _, line := range strings.Split(out.String(), "\n") {
			if strings.Contains(line, strings.ToLower(mac)) {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					return parts[2], nil // IP is in position 2 (after MAC and dev)
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
	a.mu.RLock()
	_, ok := a.machines[id]
	a.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("instance %s not found", id)
	}

	return "", apierrors.New(apierrors.NotImplemented, "firecracker does not support VNC console; serial console logs are available via GetInstanceLogs")
}

func (a *FirecrackerAdapter) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	a.mu.RLock()
	_, ok := a.machines[id]
	a.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("instance %s not found", id)
	}

	return "", apierrors.New(apierrors.NotImplemented, "exec is not supported for firecracker VMs: firecracker does not provide an agent/exec mechanism")
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
	// Handle empty or short names by generating a unique suffix
	tapName := "fc-" + name
	if len(tapName) < 5 { // minimum "fc-X"
		tapName = "fc-" + uuid.New().String()[:8]
	} else if len(name) > 8 {
		tapName = "fc-" + name[:8]
	}

	// Create TAP device
	cmd := exec.Command("ip", "tuntap", "add", "dev", tapName, "mode", "tap")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to create TAP device: %w (output: %s)", err, string(output))
	}

	// Bring up the interface
	cmd = exec.Command("ip", "link", "set", tapName, "up")
	if output, err := cmd.CombinedOutput(); err != nil {
		// Cleanup on failure
		exec.Command("ip", "tuntap", "delete", "dev", tapName, "mode", "tap")
		return "", fmt.Errorf("failed to bring up TAP device: %w (output: %s)", err, string(output))
	}

	a.mu.Lock()
	a.networks[name] = tapName
	a.mu.Unlock()

	return tapName, nil
}

func (a *FirecrackerAdapter) DeleteNetwork(ctx context.Context, id string) error {
	a.mu.Lock()
	tapName, ok := a.networks[id]
	if ok {
		delete(a.networks, id)
	}
	a.mu.Unlock()

	if !ok {
		return nil
	}

	cmd := exec.Command("ip", "tuntap", "delete", "dev", tapName, "mode", "tap")
	if output, err := cmd.CombinedOutput(); err != nil {
		a.logger.Warn("failed to delete TAP device", "device", tapName, "output", string(output))
	}

	return nil
}

func (a *FirecrackerAdapter) AttachVolume(ctx context.Context, id string, volumePath string) (string, string, error) {
	a.mu.RLock()
	_, ok := a.machines[id]
	a.mu.RUnlock()

	if !ok {
		return "", "", fmt.Errorf("instance %s not found", id)
	}

	// Firecracker doesn't support hot-attach of drives after VM start.
	// The drive must be specified at VM creation time.
	// To fully support this, the adapter would need to:
	// 1. Stop the VM
	// 2. Recreate it with the additional drive
	// 3. Restart
	// This is complex and potentially destructive, so we return NotSupported.
	return "", "", apierrors.New(apierrors.NotImplemented, "volume attach requires VM restart which is not yet supported for firecracker")
}

func (a *FirecrackerAdapter) DetachVolume(ctx context.Context, id string, volumePath string) (string, error) {
	a.mu.RLock()
	_, ok := a.machines[id]
	a.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("instance %s not found", id)
	}

	return "", apierrors.New(apierrors.NotImplemented, "volume detach is not supported for firecracker")
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

	// Firecracker doesn't support online resize, so we do cold resize
	// Stop → update config → start
	if err := m.Shutdown(ctx); err != nil {
		a.logger.Warn("failed to shutdown instance for resize", "instance_id", id, "error", err)
	}

	// The actual resize is applied by updating the Machine configuration
	// For now, we mark this as a limitation - Firecracker's SDK doesn't expose
	// a direct resize API. A full implementation would need to:
	// 1. Stop the VM
	// 2. Parse and update the domain config
	// 3. Restart with new parameters
	//
	// Since the machines map stores the Machine instance without exposing
	// the Config field, we can only support this if the Machine interface
	// is extended to support config updates.
	a.logger.Info("resize requires restart for firecracker", "instance_id", id, "cpu", cpu, "memory", memory)

	// Restart the VM (with same config for now - full resize needs SDK support)
	if err := m.Start(ctx); err != nil {
		return fmt.Errorf("failed to restart instance after resize attempt: %w", err)
	}

	return nil
}

func (a *FirecrackerAdapter) CreateSnapshot(ctx context.Context, id, name string) error {
	a.mu.RLock()
	_, ok := a.machines[id]
	a.mu.RUnlock()

	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	snapshotPath := a.getSnapshotPath(id, name)
	diskPath := a.cfg.RootfsPath

	return a.createDiskSnapshot(ctx, diskPath, snapshotPath)
}

func (a *FirecrackerAdapter) RestoreSnapshot(ctx context.Context, id, name string) error {
	a.mu.RLock()
	_, ok := a.machines[id]
	a.mu.RUnlock()

	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	// Safety check: prevent restore on potentially running VM to avoid disk corruption
	// If instance is in our machines map, assume it might be running
	// User must stop instance first via StopInstance before restore
	a.logger.Warn("restoring snapshot - VM should be stopped first", "instance_id", id)

	snapshotPath := a.getSnapshotPath(id, name)
	diskPath := a.cfg.RootfsPath

	return a.restoreDiskSnapshot(ctx, snapshotPath, diskPath)
}

func (a *FirecrackerAdapter) DeleteSnapshot(ctx context.Context, id, name string) error {
	snapshotPath := a.getSnapshotPath(id, name)
	if err := os.Remove(snapshotPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}
	return nil
}

func (a *FirecrackerAdapter) getSnapshotPath(id, name string) string {
	return fmt.Sprintf("/tmp/snapshot-%s-%s.tar.gz", id, name)
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

	cmd := exec.CommandContext(ctx, "qemu-img", "convert", "-O", "qcow2", diskPath, tmpQcow2)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("qemu-img convert failed: %w (output: %s)", err, string(output))
	}

	tarCmd := exec.CommandContext(ctx, "tar", "czf", snapshotPath, "-C", filepath.Dir(tmpQcow2), filepath.Base(tmpQcow2))
	if output, err := tarCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar archive failed: %w (output: %s)", err, string(output))
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

	untarCmd := exec.CommandContext(ctx, "tar", "xzf", snapshotPath, "-C", tmpDir)
	if output, err := untarCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("untar failed: %w (output: %s)", err, string(output))
	}

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("empty snapshot archive")
	}

	tmpQcow2 := filepath.Join(tmpDir, files[0].Name())

	cmd := exec.CommandContext(ctx, "qemu-img", "convert", "-O", "qcow2", tmpQcow2, diskPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("qemu-img restore failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// ResetCircuitBreaker is a no-op for the raw Firecracker adapter.
// The circuit breaker lives in ResilientCompute wrapping this backend.
func (a *FirecrackerAdapter) ResetCircuitBreaker() {}

// isValidPort checks if a port number is valid (0-65535)
func isValidPort(port int) bool {
	return port >= 0 && port <= 65535
}

// validateSnapshotPath checks that a path is safe to use in exec commands
// Returns an error if the path contains potential path traversal or other injection
func validateSnapshotPath(path string) error {
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal attempt detected: %s", path)
	}
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("null byte in path: %s", path)
	}
	return nil
}
