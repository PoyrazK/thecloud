package libvirt

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockCompute struct {
	mock.Mock
}

func (m *mockCompute) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (string, []string, error) {
	return "", nil, nil
}
func (m *mockCompute) StartInstance(ctx context.Context, id string) error  { return nil }
func (m *mockCompute) StopInstance(ctx context.Context, id string) error   { return nil }
func (m *mockCompute) PauseInstance(ctx context.Context, id string) error  { return nil }
func (m *mockCompute) ResumeInstance(ctx context.Context, id string) error { return nil }
func (m *mockCompute) DeleteInstance(ctx context.Context, id string) error { return nil }
func (m *mockCompute) GetInstanceLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (m *mockCompute) GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (m *mockCompute) GetInstancePort(ctx context.Context, id string, internalPort string) (int, error) {
	return 0, nil
}
func (m *mockCompute) GetInstanceIP(ctx context.Context, id string) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}
func (m *mockCompute) GetConsoleURL(ctx context.Context, id string) (string, error) { return "", nil }
func (m *mockCompute) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	return "", nil
}
func (m *mockCompute) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, []string, error) {
	return "", nil, nil
}
func (m *mockCompute) WaitTask(ctx context.Context, id string) (int64, error)         { return 0, nil }
func (m *mockCompute) CreateNetwork(ctx context.Context, name string) (string, error) { return "", nil }
func (m *mockCompute) DeleteNetwork(ctx context.Context, id string) error             { return nil }
func (m *mockCompute) AttachVolume(ctx context.Context, id string, volumePath string) (string, string, error) {
	return "/dev/vdb", "", nil
}
func (m *mockCompute) DetachVolume(ctx context.Context, id string, volumePath string) (string, error) {
	return "", nil
}
func (m *mockCompute) Ping(ctx context.Context) error { return nil }
func (m *mockCompute) Type() string                   { return "mock" }
func (m *mockCompute) ResizeInstance(ctx context.Context, id string, cpu, memory int64) error {
	return nil
}
func (m *mockCompute) CreateSnapshot(ctx context.Context, id, name string) error  { return nil }
func (m *mockCompute) RestoreSnapshot(ctx context.Context, id, name string) error { return nil }
func (m *mockCompute) DeleteSnapshot(ctx context.Context, id, name string) error  { return nil }
func (m *mockCompute) ResetCircuitBreaker()                                       {}

func TestLBProxyAdapter(t *testing.T) {
	mc := new(mockCompute)
	adapter := NewLBProxyAdapter(mc)

	// Mock execCommandContext
	executedCommands := []string{}
	adapter.execCommandContext = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		executedCommands = append(executedCommands, name)
		// Return a command that does nothing (true on unix)
		return exec.CommandContext(ctx, "true")
	}

	lb := &domain.LoadBalancer{
		ID:        uuid.New(),
		Port:      80,
		Algorithm: "round-robin",
	}
	targets := []*domain.LBTarget{
		{InstanceID: uuid.New(), Port: 8080, Weight: 1},
	}

	ctx := context.Background()
	mc.On("GetInstanceIP", mock.Anything, targets[0].InstanceID.String()).Return("10.0.0.1", nil)

	t.Run("DeployProxy", func(t *testing.T) {
		id, err := adapter.DeployProxy(ctx, lb, targets)
		require.NoError(t, err)
		assert.Equal(t, lb.ID.String(), id)
		assert.Contains(t, executedCommands, "nginx")

		// Verify config file exists
		configPath := filepath.Join("/tmp", "thecloud", "lb", lb.ID.String(), "nginx.conf")
		_, err = os.Stat(configPath)
		require.NoError(t, err)

		content, _ := os.ReadFile(filepath.Clean(configPath))
		assert.Contains(t, string(content), "server 10.0.0.1:8080 weight=1;")
	})

	t.Run("UpdateProxyConfig", func(t *testing.T) {
		executedCommands = []string{}
		err := adapter.UpdateProxyConfig(ctx, lb, targets)
		require.NoError(t, err)
		assert.Contains(t, executedCommands, "nginx")
	})

	t.Run("RemoveProxy", func(t *testing.T) {
		executedCommands = []string{}

		// Create a dummy pid file to trigger stop command
		configDir := filepath.Join("/tmp", "thecloud", "lb", lb.ID.String())
		_ = os.MkdirAll(configDir, 0750)
		_ = os.WriteFile(filepath.Join(configDir, "nginx.pid"), []byte("1234"), 0600)

		err := adapter.RemoveProxy(ctx, lb.ID)
		require.NoError(t, err)
		assert.Contains(t, executedCommands, "nginx")

		// Verify directory removed
		_, err = os.Stat(configDir)
		assert.True(t, os.IsNotExist(err))
	})
}
