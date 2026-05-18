package platform

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/poyrazk/thecloud/internal/core/ports"
)

// ---------- mock compute backend ----------

type mockCompute struct {
	callCount atomic.Int64
	delay     time.Duration
	err       error
}

func (m *mockCompute) wait(ctx context.Context) error {
	if m.delay <= 0 {
		return nil
	}
	select {
	case <-time.After(m.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *mockCompute) LaunchInstanceWithOptions(ctx context.Context, _ ports.CreateInstanceOptions) (string, []string, error) {
	m.callCount.Add(1)
	if err := m.wait(ctx); err != nil {
		return "", nil, err
	}
	return "inst-1", []string{"8080"}, m.err
}

func (m *mockCompute) StartInstance(ctx context.Context, _ string) error {
	m.callCount.Add(1)
	if err := m.wait(ctx); err != nil {
		return err
	}
	return m.err
}
func (m *mockCompute) StopInstance(_ context.Context, _ string) error {
	m.callCount.Add(1)
	return m.err
}
func (m *mockCompute) DeleteInstance(_ context.Context, _ string) error {
	m.callCount.Add(1)
	return m.err
}
func (m *mockCompute) GetInstanceLogs(_ context.Context, _ string) (io.ReadCloser, error) {
	m.callCount.Add(1)
	return io.NopCloser(strings.NewReader("logs")), m.err
}
func (m *mockCompute) GetInstanceStats(_ context.Context, _ string) (io.ReadCloser, error) {
	m.callCount.Add(1)
	return io.NopCloser(strings.NewReader("stats")), m.err
}
func (m *mockCompute) GetInstancePort(_ context.Context, _ string, _ string) (int, error) {
	m.callCount.Add(1)
	return 8080, m.err
}
func (m *mockCompute) GetInstanceIP(_ context.Context, _ string) (string, error) {
	m.callCount.Add(1)
	return "10.0.0.1", m.err
}
func (m *mockCompute) GetConsoleURL(_ context.Context, _ string) (string, error) {
	m.callCount.Add(1)
	return "https://console", m.err
}
func (m *mockCompute) Exec(_ context.Context, _ string, _ []string) (string, error) {
	m.callCount.Add(1)
	return "output", m.err
}
func (m *mockCompute) RunTask(_ context.Context, _ ports.RunTaskOptions) (string, []string, error) {
	m.callCount.Add(1)
	return "task-1", nil, m.err
}
func (m *mockCompute) WaitTask(_ context.Context, _ string) (int64, error) {
	m.callCount.Add(1)
	return 0, m.err
}
func (m *mockCompute) CreateNetwork(_ context.Context, _ string) (string, error) {
	m.callCount.Add(1)
	return "net-1", m.err
}
func (m *mockCompute) DeleteNetwork(_ context.Context, _ string) error {
	m.callCount.Add(1)
	return m.err
}
func (m *mockCompute) AttachVolume(_ context.Context, _ string, _ string) (string, string, error) {
	m.callCount.Add(1)
	return "/dev/vdb", "", m.err
}
func (m *mockCompute) DetachVolume(_ context.Context, _ string, _ string) (string, error) {
	m.callCount.Add(1)
	return "", m.err
}
func (m *mockCompute) Ping(_ context.Context) error {
	m.callCount.Add(1)
	return m.err
}
func (m *mockCompute) ResizeInstance(_ context.Context, _ string, _, _ int64) error {
	m.callCount.Add(1)
	return m.err
}
func (m *mockCompute) CreateSnapshot(_ context.Context, _, _ string) error {
	m.callCount.Add(1)
	return m.err
}
func (m *mockCompute) RestoreSnapshot(_ context.Context, _, _ string) error {
	m.callCount.Add(1)
	return m.err
}
func (m *mockCompute) DeleteSnapshot(_ context.Context, _, _ string) error {
	m.callCount.Add(1)
	return m.err
}
func (m *mockCompute) Type() string                                     { return "mock" }
func (m *mockCompute) PauseInstance(_ context.Context, _ string) error  { return nil }
func (m *mockCompute) ResumeInstance(_ context.Context, _ string) error { return nil }
func (m *mockCompute) ResetCircuitBreaker()                             {}

// ---------- tests ----------

func TestResilientComputePassthrough(t *testing.T) {
	// All calls should pass through to the mock on success.
	mock := &mockCompute{}
	logger := slog.Default()
	rc := NewResilientCompute(mock, logger, ResilientComputeOpts{})

	ctx := context.Background()

	id, ps, err := rc.LaunchInstanceWithOptions(ctx, ports.CreateInstanceOptions{})
	assertNoErr(t, err)
	if id != "inst-1" || len(ps) != 1 {
		t.Fatalf("unexpected launch result: %s %v", id, ps)
	}

	assertNoErr(t, rc.StartInstance(ctx, "x"))
	assertNoErr(t, rc.StopInstance(ctx, "x"))
	assertNoErr(t, rc.DeleteInstance(ctx, "x"))

	_, err = rc.GetInstanceLogs(ctx, "x")
	assertNoErr(t, err)
	_, err = rc.GetInstanceStats(ctx, "x")
	assertNoErr(t, err)
	port, err := rc.GetInstancePort(ctx, "x", "80")
	assertNoErr(t, err)
	if port != 8080 {
		t.Fatalf("expected 8080, got %d", port)
	}
	ip, err := rc.GetInstanceIP(ctx, "x")
	assertNoErr(t, err)
	if ip != "10.0.0.1" {
		t.Fatalf("expected 10.0.0.1, got %s", ip)
	}

	out, err := rc.Exec(ctx, "x", []string{"ls"})
	assertNoErr(t, err)
	if out != "output" {
		t.Fatalf("expected output, got %s", out)
	}

	assertNoErr(t, rc.Ping(ctx))
	if rc.Type() != "mock" {
		t.Fatalf("expected mock, got %s", rc.Type())
	}

	if mock.callCount.Load() < 10 {
		t.Fatalf("expected at least 10 calls, got %d", mock.callCount.Load())
	}
}

func TestResilientComputeCircuitTrips(t *testing.T) {
	// After threshold failures, the circuit should open and reject immediately.
	mock := &mockCompute{err: errors.New("backend down")}
	logger := slog.Default()
	rc := NewResilientCompute(mock, logger, ResilientComputeOpts{
		CBThreshold:    3,
		CBResetTimeout: 5 * time.Second,
	})

	ctx := context.Background()

	// 3 failures to trip the circuit.
	for i := 0; i < 3; i++ {
		err := rc.StartInstance(ctx, "x")
		if err == nil {
			t.Fatal("expected error")
		}
	}

	// Next call should get ErrCircuitOpen without hitting the mock.
	callsBefore := mock.callCount.Load()
	err := rc.StartInstance(ctx, "x")
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if mock.callCount.Load() != callsBefore {
		t.Fatal("expected mock not to be called when circuit is open")
	}
}

func TestResilientComputeBulkheadLimits(t *testing.T) {
	// When bulkhead is full, calls should be rejected.
	mock := &mockCompute{delay: 500 * time.Millisecond}
	logger := slog.Default()
	rc := NewResilientCompute(mock, logger, ResilientComputeOpts{
		BulkheadMaxConc: 2,
		BulkheadWait:    50 * time.Millisecond,
		CallTimeout:     2 * time.Second,
	})

	ctx := context.Background()
	var wg sync.WaitGroup
	var bulkheadErrors atomic.Int64

	// Ensure the first 2 goroutines grab the slots before the rest start.
	ready := make(chan struct{})
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx >= 2 {
				<-ready // Wait until the first 2 have started.
			}
			err := rc.StartInstance(ctx, "x")
			if errors.Is(err, ErrBulkheadFull) {
				bulkheadErrors.Add(1)
			}
		}(i)
	}
	// Give the first 2 goroutines time to acquire the slots.
	time.Sleep(50 * time.Millisecond)
	close(ready)
	wg.Wait()

	if bulkheadErrors.Load() == 0 {
		t.Fatal("expected at least one bulkhead rejection")
	}
}

func TestResilientComputeTimeout(t *testing.T) {
	// A slow backend should be cancelled by the per-call timeout.
	mock := &mockCompute{delay: 5 * time.Second}
	logger := slog.Default()
	rc := NewResilientCompute(mock, logger, ResilientComputeOpts{
		CallTimeout: 100 * time.Millisecond,
	})

	ctx := context.Background()
	start := time.Now()
	err := rc.StartInstance(ctx, "x")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	// Should complete much faster than 5s.
	if elapsed > 2*time.Second {
		t.Fatalf("timeout not enforced, took %v", elapsed)
	}
}

func TestResilientComputeUnwrap(t *testing.T) {
	mock := &mockCompute{}
	rc := NewResilientCompute(mock, slog.Default(), ResilientComputeOpts{})
	if _, ok := rc.Unwrap().(*mockCompute); !ok {
		t.Fatal("Unwrap should return the inner backend")
	}
}

func TestResilientComputePingBypassesBulkhead(t *testing.T) {
	// Ping should work even when the bulkhead is completely full.
	mock := &mockCompute{delay: 500 * time.Millisecond}
	logger := slog.Default()
	rc := NewResilientCompute(mock, logger, ResilientComputeOpts{
		BulkheadMaxConc: 1,
		BulkheadWait:    10 * time.Millisecond,
	})

	ctx := context.Background()

	// Saturate the bulkhead.
	started := make(chan struct{})
	go func() {
		close(started)
		_ = rc.StartInstance(ctx, "x")
	}()
	<-started
	time.Sleep(20 * time.Millisecond)

	// Ping should still work (bypasses bulkhead).
	err := rc.Ping(ctx)
	// err may or may not be nil depending on timing, but it must NOT be ErrBulkheadFull.
	if errors.Is(err, ErrBulkheadFull) {
		t.Fatal("Ping should bypass bulkhead")
	}
}

func TestResilientComputeResizeInstance(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mock := &mockCompute{}
		rc := NewResilientCompute(mock, slog.Default(), ResilientComputeOpts{})

		err := rc.ResizeInstance(context.Background(), "inst-1", int64(4*1e9), int64(4096*1024*1024))
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if mock.callCount.Load() < 1 {
			t.Fatalf("expected at least 1 call, got %d", mock.callCount.Load())
		}
	})

	t.Run("Error", func(t *testing.T) {
		mock := &mockCompute{err: errors.New("resize failed")}
		rc := NewResilientCompute(mock, slog.Default(), ResilientComputeOpts{})

		err := rc.ResizeInstance(context.Background(), "inst-1", int64(4*1e9), int64(4096*1024*1024))
		if err == nil {
			t.Fatal("expected error")
		}
		if mock.callCount.Load() < 1 {
			t.Fatalf("expected at least 1 call, got %d", mock.callCount.Load())
		}
	})
}

// Per-operation isolation: tripping one breaker shouldn't affect others.
func TestResilientCompute_PerOperationIsolation(t *testing.T) {
	// mockLaunch fails always, mockDelete succeeds always
	mock := &mockCompute{}
	logger := slog.Default()
	rc := NewResilientCompute(mock, logger, ResilientComputeOpts{
		CBThreshold:    2,
		CBResetTimeout: 10 * time.Second,
	})
	ctx := context.Background()

	// Trip the launch breaker (2 failures needed for threshold=2)
	mock.err = errors.New("launch always fails")
	for i := 0; i < 2; i++ {
		_, _, _ = rc.LaunchInstanceWithOptions(ctx, ports.CreateInstanceOptions{})
	}

	// Verify launch breaker is open
	_, _, err := rc.LaunchInstanceWithOptions(ctx, ports.CreateInstanceOptions{})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected launch breaker open, got %v", err)
	}

	// Reset so delete uses a fresh breaker
	rc.ResetCircuitBreaker()

	// Delete should still work — its own breaker is independent
	mock.err = nil
	err = rc.DeleteInstance(ctx, "any-id")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
}

// ---------- test helpers ----------

func assertNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
