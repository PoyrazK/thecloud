//go:build !linux

package firecracker

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/poyrazk/thecloud/internal/core/ports"
)

// Config holds Firecracker specific configuration.
type Config struct {
	BinaryPath string
	KernelPath string
	RootfsPath string
	SocketDir  string
	MockMode   bool
}

// FirecrackerAdapter is a no-op implementation for non-linux systems.
type FirecrackerAdapter struct {
	logger *slog.Logger
}

func NewFirecrackerAdapter(logger *slog.Logger, cfg Config) (*FirecrackerAdapter, error) {
	logger.Warn("Firecracker is only supported on Linux. This adapter will not function.")
	return &FirecrackerAdapter{logger: logger}, nil
}

func (a *FirecrackerAdapter) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (string, []string, error) {
	return "", nil, fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) StartInstance(ctx context.Context, id string) error {
	return fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) StopInstance(ctx context.Context, id string) error {
	return fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) PauseInstance(ctx context.Context, id string) error {
	return fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) ResumeInstance(ctx context.Context, id string) error {
	return fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) DeleteInstance(ctx context.Context, id string) error {
	return nil
}

func (a *FirecrackerAdapter) GetInstanceLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) GetInstancePort(ctx context.Context, id string, internalPort string) (int, error) {
	return 0, fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) GetInstanceIP(ctx context.Context, id string) (string, error) {
	return "0.0.0.0", nil
}

func (a *FirecrackerAdapter) GetConsoleURL(ctx context.Context, id string) (string, error) {
	return "", fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	return "", fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, []string, error) {
	return "", nil, fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) WaitTask(ctx context.Context, id string) (int64, error) {
	return -1, fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) CreateNetwork(ctx context.Context, name string) (string, error) {
	return "", fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) DeleteNetwork(ctx context.Context, id string) error {
	return nil
}

func (a *FirecrackerAdapter) AttachVolume(ctx context.Context, id string, volumePath string) (string, string, error) {
	return "", "", fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) DetachVolume(ctx context.Context, id string, volumePath string) (string, error) {
	return "", fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) Ping(ctx context.Context) error {
	if a.logger != nil {
		a.logger.Warn("Ping called on no-op firecracker adapter")
	}
	return nil
}

func (a *FirecrackerAdapter) Type() string {
	return "firecracker-noop"
}

func (a *FirecrackerAdapter) ResizeInstance(ctx context.Context, id string, cpu, memory int64) error {
	return fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) CreateSnapshot(ctx context.Context, id, name string) error {
	return fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) RestoreSnapshot(ctx context.Context, id, name string) error {
	return fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) DeleteSnapshot(ctx context.Context, id, name string) error {
	return fmt.Errorf("firecracker not supported on this platform")
}

func (a *FirecrackerAdapter) ResetCircuitBreaker() {}
