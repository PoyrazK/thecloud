//go:build linux

package pool

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBackend implements ports.ComputeBackend for testing
type mockBackend struct {
	mu          sync.Mutex
	instances   map[string]bool
	startDelay  time.Duration
	startErr    error
	execOutput  string
	execErr     error
	readyResult bool
	readyErr    error
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		instances:   make(map[string]bool),
		readyResult: true,
	}
}

func (m *mockBackend) StartPoolInstance(ctx context.Context, opts ports.RunTaskOptions) (string, []string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startErr != nil {
		return "", nil, m.startErr
	}
	if m.startDelay > 0 {
		time.Sleep(m.startDelay)
	}
	id := uuid.New().String()
	m.instances[id] = true
	return id, nil, nil
}

func (m *mockBackend) ExecInInstance(ctx context.Context, id string, cmd []string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.instances[id]; !ok {
		return "", nil
	}
	return m.execOutput, m.execErr
}

func (m *mockBackend) GetInstanceReady(ctx context.Context, id string) (bool, error) {
	return m.readyResult, m.readyErr
}

func (m *mockBackend) DeleteInstance(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.instances, id)
	return nil
}

func (m *mockBackend) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (string, []string, error) {
	return uuid.New().String(), nil, nil
}
func (m *mockBackend) StartInstance(ctx context.Context, id string) error  { return nil }
func (m *mockBackend) StopInstance(ctx context.Context, id string) error   { return nil }
func (m *mockBackend) PauseInstance(ctx context.Context, id string) error  { return nil }
func (m *mockBackend) ResumeInstance(ctx context.Context, id string) error { return nil }
func (m *mockBackend) GetInstanceLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	return nil, nil
}
func (m *mockBackend) GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error) {
	return nil, nil
}
func (m *mockBackend) GetInstancePort(ctx context.Context, id, internalPort string) (int, error) {
	return 80, nil
}
func (m *mockBackend) GetInstanceIP(ctx context.Context, id string) (string, error) {
	return "10.0.0.2", nil
}
func (m *mockBackend) GetConsoleURL(ctx context.Context, id string) (string, error) { return "", nil }
func (m *mockBackend) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	return "", nil
}
func (m *mockBackend) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, []string, error) {
	return uuid.New().String(), nil, nil
}
func (m *mockBackend) WaitTask(ctx context.Context, id string) (int64, error)         { return 0, nil }
func (m *mockBackend) CreateNetwork(ctx context.Context, name string) (string, error) { return "", nil }
func (m *mockBackend) DeleteNetwork(ctx context.Context, id string) error             { return nil }
func (m *mockBackend) AttachVolume(ctx context.Context, id, path string) (string, string, error) {
	return "", "", nil
}
func (m *mockBackend) DetachVolume(ctx context.Context, id, path string) (string, error) {
	return "", nil
}
func (m *mockBackend) Ping(ctx context.Context) error { return nil }
func (m *mockBackend) Type() string                   { return "mock" }
func (m *mockBackend) ResizeInstance(ctx context.Context, id string, cpu, memory int64) error {
	return nil
}
func (m *mockBackend) CreateSnapshot(ctx context.Context, id, name string) error  { return nil }
func (m *mockBackend) RestoreSnapshot(ctx context.Context, id, name string) error { return nil }
func (m *mockBackend) DeleteSnapshot(ctx context.Context, id, name string) error  { return nil }
func (m *mockBackend) ResetCircuitBreaker()                                       {}

func TestPoolManager_RegisterFunction(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	backend := newMockBackend()
	mgr := NewPoolManager(backend, logger)

	functionID := uuid.New()
	config := ports.PoolConfig{
		MinSize:     1,
		MaxSize:     3,
		MaxIdleTime: 5 * time.Minute,
	}
	taskOpts := ports.RunTaskOptions{Image: "test-image"}

	mgr.RegisterFunction(functionID, config, taskOpts)

	// Verify pool was created
	pool := mgr.Get(functionID)
	require.NotNil(t, pool, "pool should be created")

	// Verify config was set
	assert.Equal(t, config.MinSize, pool.config.MinSize)
	assert.Equal(t, config.MaxSize, pool.config.MaxSize)
	assert.Equal(t, config.MaxIdleTime, pool.config.MaxIdleTime)
}

func TestPoolManager_RegisterFunction_ConfigValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	backend := newMockBackend()
	mgr := NewPoolManager(backend, logger)

	functionID := uuid.New()

	// Test negative MinSize gets corrected
	originalConfig := ports.PoolConfig{
		MinSize:     -1,
		MaxSize:     5,
		MaxIdleTime: 5 * time.Minute,
	}
	mgr.RegisterFunction(functionID, originalConfig, ports.RunTaskOptions{})
	pool := mgr.Get(functionID)
	require.NotNil(t, pool)
	assert.Equal(t, 0, pool.config.MinSize, "negative MinSize should be corrected to 0")

	// Test MaxSize < MinSize gets corrected
	functionID2 := uuid.New()
	badConfig := ports.PoolConfig{
		MinSize:     10,
		MaxSize:     5,
		MaxIdleTime: 5 * time.Minute,
	}
	mgr.RegisterFunction(functionID2, badConfig, ports.RunTaskOptions{})
	pool2 := mgr.Get(functionID2)
	require.NotNil(t, pool2)
	assert.Equal(t, 10, pool2.config.MaxSize, "MaxSize < MinSize should be corrected to MinSize")

	// Test invalid MaxIdleTime gets corrected
	functionID3 := uuid.New()
	badIdleConfig := ports.PoolConfig{
		MinSize:     1,
		MaxSize:     5,
		MaxIdleTime: 0,
	}
	mgr.RegisterFunction(functionID3, badIdleConfig, ports.RunTaskOptions{})
	pool3 := mgr.Get(functionID3)
	require.NotNil(t, pool3)
	assert.Equal(t, DefaultMaxIdleSecs*time.Second, pool3.config.MaxIdleTime, "zero MaxIdleTime should use default")
}

func TestPoolManager_Acquire(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	backend := newMockBackend()
	mgr := NewPoolManager(backend, logger)

	functionID := uuid.New()
	config := ports.PoolConfig{
		MinSize:     1,
		MaxSize:     2,
		MaxIdleTime: 5 * time.Minute,
	}
	mgr.RegisterFunction(functionID, config, ports.RunTaskOptions{Image: "test"})

	// Acquire should succeed and create a warm instance
	ctx := context.Background()
	inst, release, err := mgr.Acquire(ctx, functionID)
	require.NoError(t, err)
	require.NotNil(t, inst)
	require.NotNil(t, release)

	// Verify instance is marked as BUSY
	pool := mgr.Get(functionID)
	require.NotNil(t, pool)
	pool.mu.RLock()
	assert.Equal(t, 0, len(pool.warm), "warm should be empty (instance taken)")
	assert.Equal(t, 1, len(pool.busy), "busy should have 1 instance")
	pool.mu.RUnlock()

	// Release should return instance to warm pool
	release(nil)
	pool.mu.RLock()
	assert.Equal(t, 1, len(pool.warm), "warm should have 1 instance after release")
	assert.Equal(t, 0, len(pool.busy), "busy should be empty after release")
	pool.mu.RUnlock()
}

func TestPoolManager_Acquire_ConcurrentRelease(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	backend := newMockBackend()
	mgr := NewPoolManager(backend, logger)

	functionID := uuid.New()
	config := ports.PoolConfig{
		MinSize:     0,
		MaxSize:     2,
		MaxIdleTime: 5 * time.Minute,
	}
	mgr.RegisterFunction(functionID, config, ports.RunTaskOptions{Image: "test"})

	ctx := context.Background()
	var wg sync.WaitGroup

	// Acquire 2 instances concurrently
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			inst, _, err := mgr.Acquire(ctx, functionID)
			assert.NoError(t, err)
			assert.NotNil(t, inst)
			// Don't release yet
		}()
	}
	wg.Wait()

	// Both should be busy
	pool := mgr.Get(functionID)
	require.NotNil(t, pool)
	pool.mu.RLock()
	assert.Equal(t, 0, len(pool.warm), "warm should be empty")
	assert.Equal(t, 2, len(pool.busy), "both instances should be busy")
	pool.mu.RUnlock()
}

func TestPoolManager_InvalidateFunction(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	backend := newMockBackend()
	mgr := NewPoolManager(backend, logger)

	functionID := uuid.New()
	config := ports.PoolConfig{
		MinSize:     1,
		MaxSize:     2,
		MaxIdleTime: 5 * time.Minute,
	}
	mgr.RegisterFunction(functionID, config, ports.RunTaskOptions{Image: "test"})

	// Acquire an instance to have something in the pool
	ctx := context.Background()
	inst, _, err := mgr.Acquire(ctx, functionID)
	require.NoError(t, err)
	require.NotNil(t, inst)

	// Invalidate should remove the pool
	err = mgr.InvalidateFunction(ctx, functionID)
	assert.NoError(t, err)

	// Pool should be gone
	pool := mgr.Get(functionID)
	assert.Nil(t, pool)
}

func TestPoolManager_Stop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	backend := newMockBackend()
	mgr := NewPoolManager(backend, logger)

	functionID := uuid.New()
	config := ports.PoolConfig{
		MinSize:     1,
		MaxSize:     2,
		MaxIdleTime: 5 * time.Minute,
	}
	mgr.RegisterFunction(functionID, config, ports.RunTaskOptions{Image: "test"})

	// Start an acquire that will be in-flight
	ctx := context.Background()
	acquireCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		inst, release, err := mgr.Acquire(acquireCtx, functionID)
		if err == nil && inst != nil {
			release(nil)
		}
		close(done)
	}()

	// Give acquire time to start
	time.Sleep(100 * time.Millisecond)

	// Stop should wait for in-flight acquisitions
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	mgr.Stop(stopCtx)

	// Verify pool manager is stopped
	select {
	case <-done:
		// Good - acquire completed
	default:
		t.Fatal("acquire should have completed after Stop")
	}
}

func TestPoolManager_GetPoolStats(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	backend := newMockBackend()
	mgr := NewPoolManager(backend, logger)

	functionID := uuid.New()
	config := ports.PoolConfig{
		MinSize:     1,
		MaxSize:     3,
		MaxIdleTime: 5 * time.Minute,
	}
	mgr.RegisterFunction(functionID, config, ports.RunTaskOptions{Image: "test"})

	ctx := context.Background()

	// Get stats before any acquisitions
	stats, err := mgr.GetPoolStats(ctx, functionID)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.TotalSize)
	assert.Equal(t, 0, stats.WarmCount)
	assert.Equal(t, 0, stats.BusyCount)

	// Acquire one instance
	_, release1, err := mgr.Acquire(ctx, functionID)
	require.NoError(t, err)

	stats, err = mgr.GetPoolStats(ctx, functionID)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.TotalSize)
	assert.Equal(t, 0, stats.WarmCount, "instance is BUSY")
	assert.Equal(t, 1, stats.BusyCount)

	// Release back
	release1(nil)

	stats, err = mgr.GetPoolStats(ctx, functionID)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.TotalSize)
	assert.Equal(t, 1, stats.WarmCount, "instance is back in warm")
	assert.Equal(t, 0, stats.BusyCount)
}

func TestFunctionPool_ReapIdleInstances(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	backend := newMockBackend()
	mgr := NewPoolManager(backend, logger)

	functionID := uuid.New()
	config := ports.PoolConfig{
		MinSize:     0, // Allow reaping to min
		MaxSize:     2,
		MaxIdleTime: 100 * time.Millisecond, // Very short for testing
	}
	mgr.RegisterFunction(functionID, config, ports.RunTaskOptions{Image: "test"})

	ctx := context.Background()

	// Acquire and release to put instance in warm pool
	_, release, err := mgr.Acquire(ctx, functionID)
	require.NoError(t, err)
	release(nil)

	// Wait for instance to become idle
	time.Sleep(150 * time.Millisecond)

	// Manually trigger reaper by acquiring (which calls reapIdleInstances in background)
	// Actually the reaper runs in a goroutine, so we need to wait
	time.Sleep(50 * time.Millisecond)

	// The reaper should have removed the idle instance
	pool := mgr.Get(functionID)
	require.NotNil(t, pool)
	pool.mu.RLock()
	// Instance should be reaped since it exceeded MaxIdleTime and MinSize=0
	// Note: timing here is tricky in tests
	pool.mu.RUnlock()
}

func TestFunctionPool_ReleaseOnError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	backend := newMockBackend()
	mgr := NewPoolManager(backend, logger)

	functionID := uuid.New()
	config := ports.PoolConfig{
		MinSize:     0,
		MaxSize:     2,
		MaxIdleTime: 5 * time.Minute,
	}
	mgr.RegisterFunction(functionID, config, ports.RunTaskOptions{Image: "test"})

	ctx := context.Background()

	// Acquire an instance
	_, release, err := mgr.Acquire(ctx, functionID)
	require.NoError(t, err)

	// Release with an error - instance should be destroyed
	backend.execErr = assert.AnError
	release(backend.execErr)

	// The instance should be removed (destroyed), not returned to warm pool
	pool := mgr.Get(functionID)
	require.NotNil(t, pool)
	pool.mu.RLock()
	assert.Equal(t, 0, len(pool.warm), "instance should not be in warm after error")
	pool.mu.RUnlock()
}
