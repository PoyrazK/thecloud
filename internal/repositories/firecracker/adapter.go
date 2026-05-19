//go:build linux

package firecracker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
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
	if err := os.MkdirAll(cfg.SocketDir, 0755); err != nil {
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
	return nil, fmt.Errorf("not implemented")
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
