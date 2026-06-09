//go:build linux

package firecracker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
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
	PID() (int, error)
}

// FirecrackerAdapter implements ports.ComputeBackend using Firecracker.
type FirecrackerAdapter struct {
	cfg      Config
	logger   *slog.Logger
	machines map[string]Machine
	mu       sync.RWMutex
}

// NewFirecrackerAdapter creates a new FirecrackerAdapter.
func NewFirecrackerAdapter(logger *slog.Logger, cfg Config) (*FirecrackerAdapter, error) {
	if cfg.SocketDir == "" {
		cfg.SocketDir = defaultSocketDir
	}
	if err := os.MkdirAll(cfg.SocketDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create socket directory %s: %w", cfg.SocketDir, err)
	}

	return &FirecrackerAdapter{
		cfg:      cfg,
		logger:   logger,
		machines: make(map[string]Machine),
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
// TODO(cni): wire up to CreateNetwork once TAP device networking is integrated
//
//nolint:unused
func generateMAC(instanceID string) string {
	h := uuid.NewMD5(uuid.NameSpaceDNS, []byte(instanceID))
	macBytes := h[:]
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		macBytes[0]&0xfe|0x02, // Set local bit, clear multicast bit
		macBytes[1], macBytes[2], macBytes[3], macBytes[4], macBytes[5]&0xfe)
}

// getJiffiesPerSecond returns the kernel HZ value (CONFIG_HZ)
func getJiffiesPerSecond() int64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) >= 8 {
				// user+nice+sys+idle+iowait+irq+softirq+steal
				var total int64
				for i := 1; i <= 7; i++ {
					var v int64
					if _, err := fmt.Sscanf(fields[i], "%d", &v); err == nil {
						total += v
					}
				}
				return total
			}
		}
	}
	return 0
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
	return nil, fmt.Errorf("not implemented")
}

func (a *FirecrackerAdapter) GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error) {
	a.mu.RLock()
	m, ok := a.machines[id]
	a.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("instance %s not found", id)
	}

	if a.cfg.MockMode {
		return nil, fmt.Errorf("not implemented in mock mode")
	}

	pid, err := m.PID()
	if err != nil {
		return nil, fmt.Errorf("failed to get VM PID: %w", err)
	}

	stats, err := a.collectStats(pid)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// collectStats reads CPU and memory stats from /proc/{pid}/
func (a *FirecrackerAdapter) collectStats(pid int) (io.ReadCloser, error) {
	var cpuTime int64

	// Read CPU time from /proc/{pid}/stat
	// Format: pid (comm) state ppid pgrp session tty_nr tpgid flags minflt cminflt majflt cmajflt utime stime cutime cstime ...
	statData, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return nil, fmt.Errorf("failed to read proc stat: %w", err)
	}

	// Find the last ')' to skip comm field
	lastParen := strings.LastIndex(string(statData), ")")
	if lastParen > 0 {
		fields := strings.Fields(string(statData)[lastParen+2:])
		if len(fields) >= 13 {
			var utime, stime int64
			if _, err := fmt.Sscanf(fields[11], "%d", &utime); err == nil {
				cpuTime += utime
			}
			if _, err := fmt.Sscanf(fields[12], "%d", &stime); err == nil {
				cpuTime += stime
			}
		}
	}

	var memUsage, memLimit int64

	// Read memory from /proc/{pid}/status
	// VmRSS: resident set size (actual memory used)
	// VmSize: virtual memory size (includes all allocated, not just resident)
	statusData, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return nil, fmt.Errorf("failed to read proc status: %w", err)
	}

	for _, line := range strings.Split(string(statusData), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			if _, err := fmt.Sscanf(line, "VmRSS:\t%d kB", &memUsage); err == nil {
				memUsage *= 1024 // Convert to bytes
			}
		}
		if strings.HasPrefix(line, "VmSize:") {
			if _, err := fmt.Sscanf(line, "VmSize:\t%d kB", &memLimit); err == nil {
				memLimit *= 1024 // Convert to bytes
			}
		}
	}

	// Get jiffies per second for CPU percentage calculation
	jiffies := getJiffiesPerSecond()
	if jiffies == 0 {
		a.logger.Warn("could not detect kernel HZ, using fallback 100")
		jiffies = 100 // Fallback assumption
	}

	// Convert CPU time to nanoseconds (jiffies * (1e9 / jiffies_per_sec) = nanoseconds)
	cpuNanoseconds := (cpuTime * 1e9) / jiffies

	// Calculate memory percentage
	var memPercentage float64
	if memLimit > 0 {
		memPercentage = float64(memUsage) / float64(memLimit) * 100
	}

	stats := &domain.InstanceStats{
		CPUPercentage:    0, // TODO: requires delta between two calls to calculate CPU%
		MemoryUsageBytes: float64(memUsage),
		MemoryLimitBytes: float64(memLimit),
		MemoryPercentage: memPercentage,
	}
	if cpuNanoseconds >= 0 {
		v := uint64(cpuNanoseconds)
		stats.CPUTimeNanoseconds = &v
	}

	data, err := json.Marshal(stats)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal stats: %w", err)
	}

	return io.NopCloser(strings.NewReader(string(data))), nil
}

func (a *FirecrackerAdapter) GetInstancePort(ctx context.Context, id string, internalPort string) (int, error) {
	return 0, fmt.Errorf("firecracker adapter does not support port mapping yet")
}

func (a *FirecrackerAdapter) GetInstanceIP(ctx context.Context, id string) (string, error) {
	return "0.0.0.0", nil
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
	return uuid.New().String(), nil
}

func (a *FirecrackerAdapter) DeleteNetwork(ctx context.Context, id string) error {
	return nil
}

func (a *FirecrackerAdapter) AttachVolume(ctx context.Context, id string, volumePath string) (string, string, error) {
	return "", "", fmt.Errorf("attach volume not implemented for firecracker")
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
	return fmt.Errorf("resize not supported on firecracker")
}

func (a *FirecrackerAdapter) CreateSnapshot(ctx context.Context, id, name string) error {
	return fmt.Errorf("snapshots not supported on firecracker")
}

func (a *FirecrackerAdapter) RestoreSnapshot(ctx context.Context, id, name string) error {
	return fmt.Errorf("snapshots not supported on firecracker")
}

func (a *FirecrackerAdapter) DeleteSnapshot(ctx context.Context, id, name string) error {
	return fmt.Errorf("snapshots not supported on firecracker")
}

// ResetCircuitBreaker is a no-op for the raw Firecracker adapter.
// The circuit breaker lives in ResilientCompute wrapping this backend.
func (a *FirecrackerAdapter) ResetCircuitBreaker() {}
